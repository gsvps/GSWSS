package transport

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gswss/gs-protocol/protocol"
)

// ProbeTCPRelay checks whether the worker can open a raw TCP relay to the target.
func ProbeTCPRelay(ctx context.Context, cfg RelayConfig, targetHost string, targetPort uint16) error {
	wsConn, resp, err := dialWebSocket(ctx, cfg)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("websocket dial (HTTP %d): %w", resp.StatusCode, err)
		}
		return fmt.Errorf("websocket dial: %w", err)
	}
	defer wsConn.Close()

	connectFrame, err := protocol.EncodeFrame(protocol.Frame{
		Version: protocol.Version,
		Type:    protocol.TypeConnect,
		Payload: protocol.EncodeConnectPayload(protocol.ConnectPayload{
			Host:     targetHost,
			Port:     targetPort,
			Password: cfg.Password,
		}),
	})
	if err != nil {
		return fmt.Errorf("encode connect: %w", err)
	}
	if err := wsConn.WriteMessage(websocket.BinaryMessage, connectFrame); err != nil {
		return fmt.Errorf("send connect: %w", err)
	}
	if err := waitConnectAckV1(ctx, wsConn); err != nil {
		return err
	}
	closeFrame, _ := protocol.EncodeFrame(protocol.Frame{Version: protocol.Version, Type: protocol.TypeClose})
	_ = wsConn.WriteMessage(websocket.BinaryMessage, closeFrame)
	return nil
}

// FetchHTTP performs an HTTP(S) request via Worker fetch (non-transparent TLS).
func FetchHTTP(ctx context.Context, cfg RelayConfig, req *http.Request) (*http.Response, error) {
	wsConn, resp, err := dialWebSocket(ctx, cfg)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("websocket dial (HTTP %d): %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("websocket dial: %w", err)
	}
	defer wsConn.Close()

	url := requestURL(req)
	var body []byte
	if req.Body != nil {
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("read request body: %w", err)
		}
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(body))
	}

	headers := make([][2]string, 0, len(req.Header))
	for key, vals := range req.Header {
		for _, val := range vals {
			headers = append(headers, [2]string{key, val})
		}
	}

	frame, err := protocol.EncodeFrame(protocol.Frame{
		Version: protocol.Version,
		Type:    protocol.TypeHTTPReq,
		Payload: protocol.EncodeHTTPReqPayload(protocol.HTTPReqPayload{
			Password: cfg.Password,
			Method:   req.Method,
			URL:      url,
			Headers:  headers,
			Body:     body,
		}),
	})
	if err != nil {
		return nil, err
	}
	if err := wsConn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
		return nil, fmt.Errorf("send HTTP_REQ: %w", err)
	}

	deadline := time.Now().Add(60 * time.Second)
	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("HTTP_RESP timeout")
		}
		_ = wsConn.SetReadDeadline(time.Now().Add(15 * time.Second))
		msgType, data, err := wsConn.ReadMessage()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				default:
					continue
				}
			}
			return nil, fmt.Errorf("read HTTP_RESP: %w", err)
		}
		if msgType != websocket.BinaryMessage {
			continue
		}
		frame, err := protocol.DecodeFrame(newBytesReader(data))
		if err != nil {
			return nil, err
		}
		switch frame.Type {
		case protocol.TypeHTTPResp:
			return httpResponseFromPayload(req, frame.Payload)
		case protocol.TypeError:
			ep, _ := protocol.DecodeErrorPayload(frame.Payload)
			return nil, fmt.Errorf("fetch rejected (code %d): %s", ep.Code, ep.Message)
		case protocol.TypePing:
			pong, _ := protocol.EncodeFrame(protocol.Frame{Version: protocol.Version, Type: protocol.TypePong})
			_ = wsConn.WriteMessage(websocket.BinaryMessage, pong)
		}
	}
}

func requestURL(req *http.Request) string {
	if req.URL != nil && req.URL.IsAbs() {
		return req.URL.String()
	}
	host := req.Host
	if host == "" && req.URL != nil {
		host = req.URL.Host
	}
	path := "/"
	if req.URL != nil && req.URL.RequestURI() != "" {
		path = req.URL.RequestURI()
	} else if req.URL != nil && req.URL.Path != "" {
		if req.URL.RawQuery != "" {
			path = req.URL.Path + "?" + req.URL.RawQuery
		} else {
			path = req.URL.Path
		}
	}
	scheme := "http"
	if req.TLS != nil || strings.HasSuffix(host, ":443") {
		scheme = "https"
	}
	if strings.Contains(host, ":443") {
		host = strings.TrimSuffix(host, ":443")
	}
	return fmt.Sprintf("%s://%s%s", scheme, host, path)
}

func httpResponseFromPayload(req *http.Request, payload []byte) (*http.Response, error) {
	p, err := protocol.DecodeHTTPRespPayload(payload)
	if err != nil {
		return nil, err
	}
	header := make(http.Header)
	for _, pair := range p.Headers {
		header.Add(pair[0], pair[1])
	}
	return &http.Response{
		Status:        fmt.Sprintf("%d %s", p.Status, http.StatusText(int(p.Status))),
		StatusCode:    int(p.Status),
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(p.Body)),
		ContentLength: int64(len(p.Body)),
		Request:       req,
	}, nil
}

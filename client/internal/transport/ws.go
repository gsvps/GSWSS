// Package transport provides WebSocket-based GSP1 relay connections.
package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/gorilla/websocket"
	"github.com/gswss/gs-protocol/protocol"

	"github.com/gswss/gs-protocol/client/internal/log"
)

// RelayConfig holds settings for a single relay session.
type RelayConfig struct {
	ServerURL string
	Password  string
	UseTLS    bool
	UseMux    bool
	UseFetch  bool
	Timeout   time.Duration
}

// Relay establishes a GSP1 session and pipes data to localConn (v2 mux, v1 fallback on open failure).
func Relay(ctx context.Context, cfg RelayConfig, targetHost string, targetPort uint16, localConn net.Conn) error {
	defer localConn.Close()

	if !cfg.UseMux {
		return relayV1(ctx, cfg, targetHost, targetPort, localConn)
	}

	session, pooled, err := acquireSession(ctx, cfg)
	if err != nil {
		log.L().Debug("v2 session failed, using v1", zap.Error(err))
		return relayV1(ctx, cfg, targetHost, targetPort, localConn)
	}
	if !pooled {
		defer session.Close()
	}

	stream, err := session.OpenStream(ctx, targetHost, targetPort)
	if err != nil {
		log.L().Debug("v2 stream open failed, using v1", zap.Error(err))
		return relayV1(ctx, cfg, targetHost, targetPort, localConn)
	}
	defer stream.Close()

	errCh := make(chan error, 2)
	go func() { errCh <- copyToStream(localConn, stream) }()
	go func() { errCh <- copyFromStream(stream, localConn) }()
	var firstErr error
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil && !isClosedErr(err) && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func toWebSocketURL(serverURL string) string {
	if len(serverURL) >= 5 && serverURL[:5] == "https" {
		return "wss" + serverURL[5:]
	}
	if len(serverURL) >= 4 && serverURL[:4] == "http" {
		return "ws" + serverURL[4:]
	}
	if len(serverURL) >= 3 && (serverURL[:3] == "wss" || serverURL[:2] == "ws") {
		return serverURL
	}
	return "wss://" + serverURL
}

func isClosedErr(err error) bool {
	if err == nil {
		return false
	}
	if err == io.EOF {
		return true
	}
	if opErr, ok := err.(*net.OpError); ok {
		return opErr.Err.Error() == "use of closed network connection"
	}
	return false
}

// dialWebSocket opens a raw WebSocket (used by connection test).
func dialWebSocket(ctx context.Context, cfg RelayConfig) (*websocket.Conn, *http.Response, error) {
	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
	if !cfg.UseTLS {
		dialer.TLSClientConfig = nil
	}
	wsURL := toWebSocketURL(cfg.ServerURL)
	return dialer.DialContext(ctx, wsURL, http.Header{"User-Agent": []string{"GS-Client/0.2"}})
}

// waitConnectAckV1 waits for CONNECT ack on a v1 session (connection test fallback).
func waitConnectAckV1(ctx context.Context, conn *websocket.Conn) error {
	deadline := time.Now().Add(10 * time.Second)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("connect ack timeout")
		}
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					continue
				}
			}
			return fmt.Errorf("read connect ack: %w", err)
		}
		if msgType != websocket.BinaryMessage {
			continue
		}
		frame, err := protocol.DecodeFrame(newBytesReader(data))
		if err != nil {
			return err
		}
		switch frame.Type {
		case protocol.TypeError:
			ep, _ := protocol.DecodeErrorPayload(frame.Payload)
			return fmt.Errorf("connect rejected (code %d): %s", ep.Code, ep.Message)
		case protocol.TypeData:
			return nil
		case protocol.TypeClose:
			return fmt.Errorf("connect rejected: connection closed by server")
		case protocol.TypePing:
			pong, _ := protocol.EncodeFrame(protocol.Frame{Version: protocol.Version, Type: protocol.TypePong})
			_ = conn.WriteMessage(websocket.BinaryMessage, pong)
		}
	}
}

type bytesReader struct {
	data []byte
	off  int
}

func newBytesReader(data []byte) *bytesReader {
	return &bytesReader{data: data}
}

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.off >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.off:])
	r.off += n
	return n, nil
}

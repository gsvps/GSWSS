// Package transport provides WebSocket-based GSP1 relay connections.
package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gswss/gs-protocol/protocol"
	"go.uber.org/zap"

	"github.com/gswss/gs-protocol/client/internal/log"
)

// RelayConfig holds settings for a single relay session.
type RelayConfig struct {
	ServerURL string
	Password  string
	UseTLS    bool
	Timeout   time.Duration
}

// Relay establishes a GSP1 session to the worker and pipes data to localConn.
func Relay(ctx context.Context, cfg RelayConfig, targetHost string, targetPort uint16, localConn net.Conn) error {
	defer localConn.Close()

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
	header := http.Header{}
	header.Set("User-Agent", "GS-Client/0.1")

	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	wsConn, resp, err := dialer.DialContext(dialCtx, wsURL, header)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("websocket dial (HTTP %d): %w", resp.StatusCode, err)
		}
		return fmt.Errorf("websocket dial: %w", err)
	}
	defer wsConn.Close()

	conn := &wsConnWrapper{conn: wsConn}

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

	if err := waitConnectAck(ctx, conn); err != nil {
		return err
	}

	errCh := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		errCh <- copyLocalToRemote(ctx, localConn, conn)
	}()

	go func() {
		defer wg.Done()
		errCh <- copyRemoteToLocal(ctx, conn, localConn)
	}()

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil && !isClosedErr(err) {
			return err
		}
	}
	return nil
}

func waitConnectAck(ctx context.Context, conn *wsConnWrapper) error {
	deadline := time.Now().Add(10 * time.Second)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("connect ack timeout")
		}
		_ = conn.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		frame, err := conn.readFrame()
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
		switch frame.Type {
		case protocol.TypeError:
			ep, decErr := protocol.DecodeErrorPayload(frame.Payload)
			if decErr != nil {
				return fmt.Errorf("connect rejected: decode error frame: %w", decErr)
			}
			return fmt.Errorf("connect rejected (code %d): %s", ep.Code, ep.Message)
		case protocol.TypeData:
			// Worker may send initial data; push back is handled by relay loop — for ack, treat as success.
			return nil
		case protocol.TypeClose:
			return fmt.Errorf("connect rejected: connection closed by server")
		default:
			log.L().Debug("unexpected frame during connect", zap.Uint8("type", frame.Type))
		}
	}
}

func copyLocalToRemote(ctx context.Context, local net.Conn, remote *wsConnWrapper) error {
	buf := make([]byte, 32*1024)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		n, err := local.Read(buf)
		if n > 0 {
			frame, encErr := protocol.EncodeFrame(protocol.Frame{
				Version: protocol.Version,
				Type:    protocol.TypeData,
				Payload: buf[:n],
			})
			if encErr != nil {
				return encErr
			}
			if writeErr := remote.writeFrame(frame); writeErr != nil {
				return writeErr
			}
		}
		if err != nil {
			if err == io.EOF {
				closeFrame, _ := protocol.EncodeFrame(protocol.Frame{
					Version: protocol.Version,
					Type:    protocol.TypeClose,
				})
				_ = remote.writeFrame(closeFrame)
				return nil
			}
			return err
		}
	}
}

func copyRemoteToLocal(ctx context.Context, remote *wsConnWrapper, local net.Conn) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		frame, err := remote.readFrame()
		if err != nil {
			return err
		}
		switch frame.Type {
		case protocol.TypeData:
			if _, err := local.Write(frame.Payload); err != nil {
				return err
			}
		case protocol.TypeClose:
			return nil
		case protocol.TypeError:
			ep, _ := protocol.DecodeErrorPayload(frame.Payload)
			return fmt.Errorf("remote error (code %d): %s", ep.Code, ep.Message)
		case protocol.TypePing:
			pong, _ := protocol.EncodeFrame(protocol.Frame{
				Version: protocol.Version,
				Type:    protocol.TypePong,
			})
			if err := remote.writeFrame(pong); err != nil {
				return err
			}
		}
	}
}

type wsConnWrapper struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (w *wsConnWrapper) writeFrame(data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteMessage(websocket.BinaryMessage, data)
}

func (w *wsConnWrapper) readFrame() (protocol.Frame, error) {
	for {
		msgType, data, err := w.conn.ReadMessage()
		if err != nil {
			return protocol.Frame{}, err
		}
		if msgType != websocket.BinaryMessage {
			continue
		}
		frame, err := protocol.DecodeFrame(newBytesReader(data))
		if err != nil {
			return protocol.Frame{}, err
		}
		return frame, nil
	}
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

package transport

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/gswss/gs-protocol/protocol"
)

// relayV1 uses one WebSocket per connection (GSP1 v1).
func relayV1(ctx context.Context, cfg RelayConfig, targetHost string, targetPort uint16, localConn net.Conn) error {
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

	wsWrap := &v1Conn{conn: wsConn}
	errCh := make(chan error, 2)
	go func() { errCh <- copyToStreamV1(localConn, wsWrap) }()
	go func() { errCh <- copyFromStreamV1(wsWrap, localConn) }()
	var firstErr error
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil && !isClosedErr(err) && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

type v1Conn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (w *v1Conn) writeFrame(data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteMessage(websocket.BinaryMessage, data)
}

func copyToStreamV1(local net.Conn, remote *v1Conn) error {
	buf := make([]byte, relayBufSize)
	for {
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
			if isClosedErr(err) {
				closeFrame, _ := protocol.EncodeFrame(protocol.Frame{Version: protocol.Version, Type: protocol.TypeClose})
				_ = remote.writeFrame(closeFrame)
				return nil
			}
			return err
		}
	}
}

func copyFromStreamV1(remote *v1Conn, local net.Conn) error {
	for {
		msgType, data, err := remote.conn.ReadMessage()
		if err != nil {
			return err
		}
		if msgType != websocket.BinaryMessage {
			continue
		}
		frame, err := protocol.DecodeFrame(newBytesReader(data))
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
			pong, _ := protocol.EncodeFrame(protocol.Frame{Version: protocol.Version, Type: protocol.TypePong})
			_ = remote.writeFrame(pong)
		}
	}
}

package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gswss/gs-protocol/protocol"
)

// TestWorker dials the worker WebSocket and verifies auth with a CONNECT probe.
func TestWorker(ctx context.Context, cfg RelayConfig) error {
	if cfg.ServerURL == "" {
		return fmt.Errorf("server URL is required")
	}
	if cfg.Password == "" {
		return fmt.Errorf("password is required")
	}

	if err := pingWorkerHTTP(ctx, cfg); err != nil {
		return fmt.Errorf("worker health check: %w", err)
	}

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
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	wsConn, resp, err := dialer.DialContext(dialCtx, wsURL, http.Header{"User-Agent": []string{"GS-Client/0.1"}})
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
			Host:     "example.com",
			Port:     80,
			Password: cfg.Password,
		}),
	})
	if err != nil {
		return fmt.Errorf("encode connect: %w", err)
	}
	if err := wsConn.WriteMessage(websocket.BinaryMessage, connectFrame); err != nil {
		return fmt.Errorf("send connect: %w", err)
	}

	conn := &wsConnWrapper{conn: wsConn}
	if err := waitConnectAck(ctx, conn); err != nil {
		return err
	}
	closeFrame, _ := protocol.EncodeFrame(protocol.Frame{
		Version: protocol.Version,
		Type:    protocol.TypeClose,
	})
	_ = wsConn.WriteMessage(websocket.BinaryMessage, closeFrame)
	return nil
}

func pingWorkerHTTP(ctx context.Context, cfg RelayConfig) error {
	base := workerBaseURL(cfg.ServerURL)
	if base == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func workerBaseURL(serverURL string) string {
	wsURL := toWebSocketURL(serverURL)
	if strings.HasPrefix(wsURL, "wss://") {
		path := strings.TrimPrefix(wsURL, "wss://")
		if idx := strings.Index(path, "/"); idx >= 0 {
			path = path[:idx]
		}
		return "https://" + path
	}
	if strings.HasPrefix(wsURL, "ws://") {
		path := strings.TrimPrefix(wsURL, "ws://")
		if idx := strings.Index(path, "/"); idx >= 0 {
			path = path[:idx]
		}
		return "http://" + path
	}
	return ""
}

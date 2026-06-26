package transport

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// TestWorker dials the worker WebSocket and verifies auth with a v2 SESSION + CONNECT probe.
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

	session := newMuxSession(cfg)
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := session.Connect(dialCtx); err != nil {
		return err
	}
	defer session.Close()

	probeCtx, probeCancel := context.WithTimeout(ctx, 15*time.Second)
	defer probeCancel()
	stream, err := session.OpenStream(probeCtx, "gopher.floodgap.com", 70)
	if err != nil {
		return err
	}
	return stream.Close()
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

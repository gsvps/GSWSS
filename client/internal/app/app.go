// Package app orchestrates proxy servers and lifecycle management.
package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/gswss/gs-protocol/client/internal/config"
	httpproxy "github.com/gswss/gs-protocol/client/internal/proxy/httpproxy"
	"github.com/gswss/gs-protocol/client/internal/proxy/socks"
	"github.com/gswss/gs-protocol/client/internal/log"
	"github.com/gswss/gs-protocol/client/internal/transport"
)

const pidFileName = "gs-client.pid"

// App manages the running GS client instance.
type App struct {
	cfg     config.Config
	pidPath string
}

// New creates an App from configuration.
func New(cfg config.Config) *App {
	return &App{
		cfg:     cfg,
		pidPath: defaultPIDPath(),
	}
}

// Start runs SOCKS5 and/or HTTP proxy servers until ctx is cancelled.
func (a *App) Start(ctx context.Context) error {
	if err := a.writePID(); err != nil {
		return err
	}
	defer a.removePID()

	relayCfg := transport.RelayConfig{
		ServerURL: a.cfg.Server,
		Password:  a.cfg.Password,
		UseTLS:    a.cfg.TLS,
		Timeout:   15 * time.Second,
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	if a.cfg.LocalSocks != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			srv := &socks.Server{ListenAddr: a.cfg.LocalSocks, Relay: relayCfg}
			if err := srv.ListenAndServe(ctx); err != nil {
				errCh <- fmt.Errorf("socks: %w", err)
			}
		}()
	}

	if a.cfg.LocalHTTP != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			srv := &httpproxy.Server{ListenAddr: a.cfg.LocalHTTP, Relay: relayCfg}
			if err := srv.ListenAndServe(ctx); err != nil {
				errCh <- fmt.Errorf("http: %w", err)
			}
		}()
	}

	log.L().Info("GS client started",
		zap.String("server", a.cfg.Server),
		zap.String("socks", a.cfg.LocalSocks),
		zap.String("http", a.cfg.LocalHTTP),
	)

	select {
	case <-ctx.Done():
		wg.Wait()
		return nil
	case err := <-errCh:
		return err
	}
}

// Status returns whether the client is running.
func Status() (bool, int, error) {
	path := defaultPIDPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, err
	}
	var pid int
	_, err = fmt.Sscanf(string(data), "%d", &pid)
	if err != nil {
		return false, 0, err
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, pid, nil
	}
	// On Windows, Signal(0) is not supported; FindProcess success is enough for MVP.
	_ = proc
	return true, pid, nil
}

func (a *App) writePID() error {
	pid := os.Getpid()
	return os.WriteFile(a.pidPath, []byte(fmt.Sprintf("%d", pid)), 0o644)
}

func (a *App) removePID() {
	_ = os.Remove(a.pidPath)
}

func defaultPIDPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "gs-protocol", pidFileName)
}

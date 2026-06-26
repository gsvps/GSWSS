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
	"github.com/gswss/gs-protocol/client/internal/mitm"
	"github.com/gswss/gs-protocol/client/internal/transport"
)

// Runner manages start/stop lifecycle for the proxy client.
type Runner struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
	cfg    config.Config
}

// NewRunner creates a Runner instance.
func NewRunner() *Runner {
	return &Runner{}
}

// Config returns the active configuration when running.
func (r *Runner) Config() config.Config {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cfg
}

// Running reports whether the proxy is active.
func (r *Runner) Running() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cancel != nil
}

// Start launches proxy servers with the given configuration.
func (r *Runner) Start(cfg config.Config) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		return fmt.Errorf("client already running")
	}
	if err := config.Validate(cfg); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	r.cancel = cancel
	r.done = done
	r.cfg = cfg

	go func() {
		defer close(done)
		application := New(cfg)
		if err := application.startProxy(ctx); err != nil && ctx.Err() == nil {
			log.L().Error("proxy stopped", zap.Error(err))
		}
		r.mu.Lock()
		r.cancel = nil
		r.done = nil
		r.mu.Unlock()
	}()
	return nil
}

// Stop shuts down running proxy servers.
func (r *Runner) Stop() {
	r.mu.Lock()
	cancel := r.cancel
	done := r.done
	r.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}

func (a *App) startProxy(ctx context.Context) error {
	var mitmCA *mitm.CA
	if a.cfg.Fetch {
		dir, err := os.UserConfigDir()
		if err != nil {
			dir = os.TempDir()
		}
		caDir := filepath.Join(dir, "gs-protocol")
		ca, certPath, err := mitm.LoadOrCreate(caDir)
		if err != nil {
			return fmt.Errorf("mitm ca: %w", err)
		}
		mitmCA = ca
		log.L().Info("HTTPS fetch fallback enabled (MITM for blocked TCP targets)",
			zap.String("trust_ca", certPath),
		)
	}

	relayCfg := transport.RelayConfig{
		ServerURL: a.cfg.Server,
		Password:  a.cfg.Password,
		UseTLS:    a.cfg.TLS,
		UseMux:    a.cfg.Mux,
		UseFetch:  a.cfg.Fetch,
		Timeout:   15 * time.Second,
	}

	if a.cfg.Mux {
		transport.InitPool(relayCfg, 3)
		go transport.WarmPool(ctx)
		defer transport.ClosePool()
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
			srv := &httpproxy.Server{ListenAddr: a.cfg.LocalHTTP, Relay: relayCfg, MITM: mitmCA}
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

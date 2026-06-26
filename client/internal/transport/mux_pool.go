package transport

import (
	"context"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"

	"github.com/gswss/gs-protocol/client/internal/log"
)

const defaultPoolSize = 3

// SessionPool maintains warm multiplexed WebSocket sessions.
type SessionPool struct {
	cfg  RelayConfig
	size int

	mu       sync.Mutex
	sessions []*MuxSession
	rr       uint32
	closed   bool
}

var (
	globalPool   *SessionPool
	globalPoolMu sync.Mutex
)

// InitPool creates the global session pool.
func InitPool(cfg RelayConfig, size int) {
	if size <= 0 {
		size = defaultPoolSize
	}
	globalPoolMu.Lock()
	globalPool = &SessionPool{cfg: cfg, size: size}
	globalPoolMu.Unlock()
}

// ClosePool closes all pooled sessions.
func ClosePool() {
	globalPoolMu.Lock()
	p := globalPool
	globalPool = nil
	globalPoolMu.Unlock()
	if p != nil {
		p.Close()
	}
}

// WarmPool pre-connects and authenticates pool sessions.
func WarmPool(ctx context.Context) {
	globalPoolMu.Lock()
	p := globalPool
	globalPoolMu.Unlock()
	if p == nil {
		return
	}
	p.Warm(ctx)
}

func (p *SessionPool) Warm(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	for len(p.sessions) < p.size {
		s := newMuxSession(p.cfg)
		if err := s.Connect(ctx); err != nil {
			log.L().Warn("pool warm connect failed", zap.Error(err))
			s.Close()
			continue
		}
		p.sessions = append(p.sessions, s)
	}
	log.L().Info("session pool warmed", zap.Int("sessions", len(p.sessions)))
}

func (p *SessionPool) Acquire(ctx context.Context) (*MuxSession, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, context.Canceled
	}
	if len(p.sessions) == 0 {
		s := newMuxSession(p.cfg)
		p.mu.Unlock()
		if err := s.Connect(ctx); err != nil {
			s.Close()
			return nil, err
		}
		return s, nil
	}

	// Pick session with fewest active streams (round-robin tiebreak).
	bestIdx := 0
	bestLoad := p.sessions[0].ActiveStreams()
	start := int(atomic.AddUint32(&p.rr, 1) % uint32(len(p.sessions)))
	for i := 0; i < len(p.sessions); i++ {
		idx := (start + i) % len(p.sessions)
		load := p.sessions[idx].ActiveStreams()
		if load < bestLoad {
			bestLoad = load
			bestIdx = idx
		}
	}
	s := p.sessions[bestIdx]
	p.mu.Unlock()
	return s, nil
}

func (p *SessionPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	for _, s := range p.sessions {
		s.Close()
	}
	p.sessions = nil
}

func acquireSession(ctx context.Context, cfg RelayConfig) (*MuxSession, bool, error) {
	globalPoolMu.Lock()
	p := globalPool
	globalPoolMu.Unlock()
	if p != nil {
		s, err := p.Acquire(ctx)
		return s, true, err
	}
	s := newMuxSession(cfg)
	if err := s.Connect(ctx); err != nil {
		s.Close()
		return nil, false, err
	}
	return s, false, nil
}

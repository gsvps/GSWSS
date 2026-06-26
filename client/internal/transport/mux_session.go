package transport

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gswss/gs-protocol/protocol"
)

const (
	relayBufSize   = 128 * 1024
	coalesceMin    = 4096
	coalesceWait   = 2 * time.Millisecond
	streamReadWait = 30 * time.Second
)

// MuxSession multiplexes many TCP streams over one WebSocket (GSP1 v2).
type MuxSession struct {
	cfg    RelayConfig
	conn   *websocket.Conn
	writeMu sync.Mutex

	streams   map[uint32]*muxStream
	streamsMu sync.RWMutex

	nextStreamID uint32
	authed       bool
	connected    bool
	closed       atomic.Bool

	activeStreams atomic.Int32
	readDone      chan struct{}
	closeOnce     sync.Once
}

type muxStream struct {
	session *MuxSession
	id      uint32

	readBuf []byte
	readErr error
	readMu  sync.Mutex
	readCh  chan struct{}

	ackOnce sync.Once
	ackErr  error
	acked   atomic.Bool
	ackCh   chan struct{}

	done atomic.Bool
}

func newMuxSession(cfg RelayConfig) *MuxSession {
	return &MuxSession{
		cfg:          cfg,
		streams:      make(map[uint32]*muxStream),
		nextStreamID: 0,
		readDone:     make(chan struct{}),
	}
}

func (s *MuxSession) ActiveStreams() int32 {
	return s.activeStreams.Load()
}

func (s *MuxSession) Connect(ctx context.Context) error {
	if s.connected {
		return nil
	}
	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
	if !s.cfg.UseTLS {
		dialer.TLSClientConfig = nil
	}
	wsURL := toWebSocketURL(s.cfg.ServerURL)
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	wsConn, resp, err := dialer.DialContext(dialCtx, wsURL, http.Header{"User-Agent": []string{"GS-Client/0.2"}})
	if err != nil {
		if resp != nil {
			return fmt.Errorf("websocket dial (HTTP %d): %w", resp.StatusCode, err)
		}
		return fmt.Errorf("websocket dial: %w", err)
	}
	s.conn = wsConn
	s.connected = true
	go s.readLoop()
	return s.auth(ctx)
}

func (s *MuxSession) auth(ctx context.Context) error {
	if s.authed {
		return nil
	}
	st := &muxStream{
		session: s,
		id:      0,
		ackCh:   make(chan struct{}),
		readCh:  make(chan struct{}, 1),
	}
	s.registerStream(st)

	frame, err := protocol.EncodeFrameV2(protocol.Frame{
		Version:  protocol.Version2,
		Type:     protocol.TypeSession,
		StreamID: 0,
		Payload:  protocol.EncodeSessionPayload(s.cfg.Password),
	})
	if err != nil {
		return err
	}
	if err := s.writeRaw(frame); err != nil {
		return err
	}
	select {
	case <-st.ackCh:
		if st.ackErr != nil {
			return st.ackErr
		}
		s.authed = true
		s.unregisterStream(0)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Second):
		return fmt.Errorf("session auth timeout")
	case <-s.readDone:
		return fmt.Errorf("session closed during auth")
	}
}

func (s *MuxSession) OpenStream(ctx context.Context, host string, port uint16) (*muxStream, error) {
	if !s.connected || !s.authed {
		return nil, fmt.Errorf("session not ready")
	}
	id := atomic.AddUint32(&s.nextStreamID, 1)
	st := &muxStream{
		session: s,
		id:      id,
		ackCh:   make(chan struct{}),
		readCh:  make(chan struct{}, 8),
	}
	s.registerStream(st)
	s.activeStreams.Add(1)

	frame, err := protocol.EncodeFrameV2(protocol.Frame{
		Version:  protocol.Version2,
		Type:     protocol.TypeConnect,
		StreamID: id,
		Payload:  protocol.EncodeTargetPayload(host, port),
	})
	if err != nil {
		s.closeStream(st, nil)
		return nil, err
	}
	if err := s.writeRaw(frame); err != nil {
		s.closeStream(st, nil)
		return nil, err
	}

	select {
	case <-st.ackCh:
		if st.ackErr != nil {
			return nil, st.ackErr
		}
		return st, nil
	case <-ctx.Done():
		s.closeStream(st, nil)
		return nil, ctx.Err()
	case <-time.After(15 * time.Second):
		s.closeStream(st, nil)
		return nil, fmt.Errorf("stream connect timeout")
	case <-s.readDone:
		return nil, fmt.Errorf("session closed during connect")
	}
}

func (s *MuxSession) registerStream(st *muxStream) {
	s.streamsMu.Lock()
	s.streams[st.id] = st
	s.streamsMu.Unlock()
}

func (s *MuxSession) unregisterStream(id uint32) {
	s.streamsMu.Lock()
	delete(s.streams, id)
	s.streamsMu.Unlock()
}

func (s *MuxSession) writeRaw(data []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.conn == nil {
		return fmt.Errorf("session not connected")
	}
	return s.conn.WriteMessage(websocket.BinaryMessage, data)
}

func (s *MuxSession) writeStreamData(id uint32, payload []byte) error {
	frame, err := protocol.EncodeFrameV2(protocol.Frame{
		Version:  protocol.Version2,
		Type:     protocol.TypeData,
		StreamID: id,
		Payload:  payload,
	})
	if err != nil {
		return err
	}
	return s.writeRaw(frame)
}

func (s *MuxSession) readLoop() {
	defer close(s.readDone)
	for {
		if s.closed.Load() {
			return
		}
		msgType, data, err := s.conn.ReadMessage()
		if err != nil {
			s.failAll(err)
			return
		}
		if msgType != websocket.BinaryMessage {
			continue
		}
		frame, err := protocol.DecodeFrameAny(data)
		if err != nil {
			s.failAll(err)
			return
		}
		s.dispatch(frame)
	}
}

func (s *MuxSession) dispatch(frame protocol.Frame) {
	id := frame.StreamID
	s.streamsMu.RLock()
	st := s.streams[id]
	s.streamsMu.RUnlock()

	switch frame.Type {
	case protocol.TypeSessionOK:
		if st != nil {
			st.signalAck(nil)
		}
	case protocol.TypeData:
		if st == nil {
			return
		}
		if !st.isAcked() && len(frame.Payload) == 0 {
			st.signalAck(nil)
			return
		}
		st.push(frame.Payload)
	case protocol.TypeClose:
		if st != nil {
			s.closeStream(st, io.EOF)
		}
	case protocol.TypeError:
		ep, _ := protocol.DecodeErrorPayload(frame.Payload)
		err := fmt.Errorf("remote error (code %d): %s", ep.Code, ep.Message)
		if st != nil {
			if !st.isAcked() {
				st.signalAck(err)
			} else {
				s.closeStream(st, err)
			}
		}
	case protocol.TypePing:
		pong, _ := protocol.EncodeFrameV2(protocol.Frame{
			Version:  protocol.Version2,
			Type:     protocol.TypePong,
			StreamID: 0,
		})
		_ = s.writeRaw(pong)
	}
}

func (s *MuxSession) failAll(err error) {
	s.streamsMu.RLock()
	defer s.streamsMu.RUnlock()
	for _, st := range s.streams {
		if !st.isAcked() {
			st.signalAck(err)
		} else {
			s.closeStream(st, err)
		}
	}
}

func (s *MuxSession) Close() {
	s.closeOnce.Do(func() {
		s.closed.Store(true)
		if s.conn != nil {
			_ = s.conn.Close()
		}
	})
}

func (s *MuxSession) closeStream(st *muxStream, err error) {
	if st.done.Swap(true) {
		return
	}
	if !st.isAcked() {
		if err == nil {
			err = io.EOF
		}
		st.signalAck(err)
	}
	st.pushDone(err)
	s.unregisterStream(st.id)
	s.activeStreams.Add(-1)
}

func (st *muxStream) isAcked() bool {
	return st.acked.Load()
}

func (st *muxStream) signalAck(err error) {
	st.ackOnce.Do(func() {
		st.ackErr = err
		st.acked.Store(true)
		close(st.ackCh)
	})
}

func (st *muxStream) push(data []byte) {
	if len(data) == 0 || st.done.Load() {
		return
	}
	st.readMu.Lock()
	st.readBuf = append(st.readBuf, data...)
	st.readMu.Unlock()
	select {
	case st.readCh <- struct{}{}:
	default:
	}
}

func (st *muxStream) pushDone(err error) {
	if err == nil {
		err = io.EOF
	}
	st.readMu.Lock()
	st.readErr = err
	st.readMu.Unlock()
	select {
	case st.readCh <- struct{}{}:
	default:
	}
}

func (st *muxStream) Read(p []byte) (int, error) {
	for {
		st.readMu.Lock()
		if len(st.readBuf) > 0 {
			n := copy(p, st.readBuf)
			st.readBuf = st.readBuf[n:]
			st.readMu.Unlock()
			return n, nil
		}
		err := st.readErr
		st.readMu.Unlock()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return 0, io.EOF
			}
			return 0, err
		}
		if st.done.Load() {
			return 0, io.EOF
		}
		select {
		case <-st.readCh:
		case <-time.After(streamReadWait):
			return 0, fmt.Errorf("stream read timeout")
		}
	}
}

func (st *muxStream) Write(p []byte) (int, error) {
	if st.done.Load() {
		return 0, errors.New("stream closed")
	}
	if err := st.session.writeStreamData(st.id, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (st *muxStream) Close() error {
	if st.done.Load() {
		return nil
	}
	frame, _ := protocol.EncodeFrameV2(protocol.Frame{
		Version:  protocol.Version2,
		Type:     protocol.TypeClose,
		StreamID: st.id,
	})
	_ = st.session.writeRaw(frame)
	st.session.closeStream(st, io.EOF)
	return nil
}

func copyToStream(local net.Conn, stream *muxStream) error {
	buf := make([]byte, relayBufSize)
	for {
		n, err := local.Read(buf)
		if n > 0 {
			end := n
			if end < coalesceMin && err == nil {
				_ = local.SetReadDeadline(time.Now().Add(coalesceWait))
				n2, err2 := local.Read(buf[end:])
				_ = local.SetReadDeadline(time.Time{})
				if n2 > 0 {
					end += n2
				}
				_ = err2
			}
			if _, werr := stream.Write(buf[:end]); werr != nil {
				return werr
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func copyFromStream(stream *muxStream, local net.Conn) error {
	buf := make([]byte, relayBufSize)
	for {
		n, err := stream.Read(buf)
		if n > 0 {
			if _, werr := local.Write(buf[:n]); werr != nil {
				return werr
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

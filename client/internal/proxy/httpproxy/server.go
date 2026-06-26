// Package httpproxy implements a local HTTP proxy (CONNECT, GET, POST).
package httpproxy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"go.uber.org/zap"

	"github.com/gswss/gs-protocol/client/internal/log"
	"github.com/gswss/gs-protocol/client/internal/transport"
)

// Server is a local HTTP proxy that relays via GSP1.
type Server struct {
	ListenAddr string
	Relay      transport.RelayConfig
}

// ListenAndServe starts the HTTP proxy and blocks until ctx is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.ListenAddr)
	if err != nil {
		return fmt.Errorf("http proxy listen: %w", err)
	}
	defer ln.Close()

	log.L().Info("HTTP proxy listening", zap.String("addr", s.ListenAddr))

	var wg sync.WaitGroup
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				wg.Wait()
				return nil
			default:
				log.L().Warn("http accept error", zap.Error(err))
				continue
			}
		}
		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			s.handleConn(ctx, c)
		}(conn)
	}
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	br := bufio.NewReader(conn)
	req, err := http.ReadRequest(br)
	if err != nil {
		log.L().Debug("http read request failed", zap.Error(err))
		return
	}

	host, port, err := parseHostPort(req)
	if err != nil {
		writeHTTPError(conn, http.StatusBadRequest, err.Error())
		return
	}

	log.L().Info("http proxy request",
		zap.String("method", req.Method),
		zap.String("target", fmt.Sprintf("%s:%d", host, port)),
	)

	switch req.Method {
	case http.MethodConnect:
		s.handleConnect(ctx, conn, host, port)
	default:
		s.handlePlainHTTP(ctx, conn, br, req, host, port)
	}
}

func (s *Server) handleConnect(ctx context.Context, conn net.Conn, host string, port uint16) {
	if _, err := conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		return
	}
	if err := transport.Relay(ctx, s.Relay, host, port, conn); err != nil && ctx.Err() == nil {
		log.L().Debug("http connect relay ended", zap.Error(err))
	}
}

func (s *Server) handlePlainHTTP(ctx context.Context, conn net.Conn, br *bufio.Reader, req *http.Request, host string, port uint16) {
	remoteConn, err := dialViaRelay(ctx, s.Relay, host, port)
	if err != nil {
		writeHTTPError(conn, http.StatusBadGateway, err.Error())
		return
	}
	defer remoteConn.Close()

	req.RequestURI = ""
	req.Header.Del("Proxy-Connection")
	if req.Header.Get("Connection") == "" {
		req.Header.Set("Connection", "close")
	}

	if err := req.Write(remoteConn); err != nil {
		writeHTTPError(conn, http.StatusBadGateway, err.Error())
		return
	}

	resp, err := http.ReadResponse(bufio.NewReader(remoteConn), req)
	if err != nil {
		writeHTTPError(conn, http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()

	if err := resp.Write(conn); err != nil {
		return
	}
	_, _ = io.Copy(conn, resp.Body)
}

// dialViaRelay opens a relay connection and returns the local side after CONNECT ack.
func dialViaRelay(ctx context.Context, cfg transport.RelayConfig, host string, port uint16) (net.Conn, error) {
	clientConn, serverConn := net.Pipe()
	go func() {
		_ = transport.Relay(ctx, cfg, host, port, serverConn)
	}()
	return clientConn, nil
}

func parseHostPort(req *http.Request) (string, uint16, error) {
	host := req.Host
	if host == "" {
		return "", 0, fmt.Errorf("missing Host header")
	}
	if strings.Contains(host, ":") {
		h, p, err := net.SplitHostPort(host)
		if err != nil {
			return "", 0, err
		}
		portNum, err := parsePort(p)
		if err != nil {
			return "", 0, err
		}
		return h, portNum, nil
	}
	if req.Method == http.MethodConnect {
		return host, 443, nil
	}
	return host, 80, nil
}

func parsePort(s string) (uint16, error) {
	var port uint16
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid port: %s", s)
		}
		port = port*10 + uint16(c-'0')
	}
	if port == 0 {
		return 0, fmt.Errorf("invalid port: %s", s)
	}
	return port, nil
}

func writeHTTPError(conn net.Conn, code int, msg string) {
	resp := fmt.Sprintf("HTTP/1.1 %d %s\r\nContent-Length: 0\r\nConnection: close\r\n\r\n", code, http.StatusText(code))
	_, _ = conn.Write([]byte(resp))
	log.L().Debug("http proxy error", zap.Int("code", code), zap.String("msg", msg))
}

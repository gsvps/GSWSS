// Package socks implements a SOCKS5 proxy server (CONNECT only).
package socks

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"

	"go.uber.org/zap"

	"github.com/gswss/gs-protocol/client/internal/log"
	"github.com/gswss/gs-protocol/client/internal/transport"
)

const (
	socksVersion5 = 0x05
	cmdConnect    = 0x01
	atypIPv4      = 0x01
	atypDomain    = 0x03
	atypIPv6      = 0x04

	repSuccess       = 0x00
	repGeneralFail   = 0x01
	repHostUnreach   = 0x04
	repCmdNotSupport = 0x07
	repAddrNotSupport = 0x08
)

// Server is a local SOCKS5 proxy that relays via GSP1.
type Server struct {
	ListenAddr string
	Relay      transport.RelayConfig
}

// ListenAndServe starts the SOCKS5 server and blocks until ctx is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.ListenAddr)
	if err != nil {
		return fmt.Errorf("socks listen: %w", err)
	}
	defer ln.Close()

	log.L().Info("SOCKS5 proxy listening", zap.String("addr", s.ListenAddr))

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
				log.L().Warn("socks accept error", zap.Error(err))
				continue
			}
		}
		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.L().Error("socks handler panic", zap.Any("recover", r))
				}
			}()
			s.handleConn(ctx, c)
		}(conn)
	}
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	remote := conn.RemoteAddr().String()

	if err := s.handshake(conn); err != nil {
		log.L().Debug("socks handshake failed", zap.String("remote", remote), zap.Error(err))
		return
	}

	host, port, err := s.readRequest(conn)
	if err != nil {
		log.L().Debug("socks request failed", zap.String("remote", remote), zap.Error(err))
		return
	}

	log.L().Info("socks connect", zap.String("remote", remote), zap.String("target", fmt.Sprintf("%s:%d", host, port)))

	if err := s.sendReply(conn, repSuccess, net.IPv4zero, 0); err != nil {
		return
	}

	if err := transport.Relay(ctx, s.Relay, host, port, conn); err != nil && ctx.Err() == nil {
		log.L().Debug("socks relay ended", zap.String("target", fmt.Sprintf("%s:%d", host, port)), zap.Error(err))
	}
}

func (s *Server) handshake(conn net.Conn) error {
	buf := make([]byte, 257)
	n, err := io.ReadAtLeast(conn, buf, 2)
	if err != nil {
		return err
	}
	if buf[0] != socksVersion5 {
		return fmt.Errorf("unsupported socks version: %d", buf[0])
	}
	nmethods := int(buf[1])
	if n < 2+nmethods {
		if _, err := io.ReadFull(conn, buf[n:2+nmethods]); err != nil {
			return err
		}
	}
	// No authentication required.
	if _, err := conn.Write([]byte{socksVersion5, 0x00}); err != nil {
		return err
	}
	return nil
}

func (s *Server) readRequest(conn net.Conn) (string, uint16, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", 0, err
	}
	if header[0] != socksVersion5 {
		return "", 0, fmt.Errorf("invalid version")
	}
	if header[1] != cmdConnect {
		_ = s.sendReply(conn, repCmdNotSupport, net.IPv4zero, 0)
		return "", 0, fmt.Errorf("unsupported command: %d", header[1])
	}

	var host string
	switch header[3] {
	case atypIPv4:
		addr := make([]byte, 4)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return "", 0, err
		}
		host = net.IP(addr).String()
	case atypDomain:
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return "", 0, err
		}
		domain := make([]byte, lenBuf[0])
		if _, err := io.ReadFull(conn, domain); err != nil {
			return "", 0, err
		}
		host = string(domain)
	case atypIPv6:
		addr := make([]byte, 16)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return "", 0, err
		}
		host = net.IP(addr).String()
	default:
		_ = s.sendReply(conn, repAddrNotSupport, net.IPv4zero, 0)
		return "", 0, fmt.Errorf("unsupported address type: %d", header[3])
	}

	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBuf); err != nil {
		return "", 0, err
	}
	port := binary.BigEndian.Uint16(portBuf)
	return host, port, nil
}

func (s *Server) sendReply(conn net.Conn, rep byte, ip net.IP, port uint16) error {
	resp := []byte{socksVersion5, rep, 0x00, atypIPv4, 0, 0, 0, 0, 0, 0}
	if ip != nil && len(ip) == 4 {
		copy(resp[4:8], ip.To4())
	}
	binary.BigEndian.PutUint16(resp[8:10], port)
	_, err := conn.Write(resp)
	return err
}

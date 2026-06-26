package mitm

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"

	"go.uber.org/zap"

	"github.com/gswss/gs-protocol/client/internal/log"
	"github.com/gswss/gs-protocol/client/internal/transport"
)

// ServeHTTPS terminates TLS locally and forwards HTTP via Worker fetch.
func ServeHTTPS(ctx context.Context, conn net.Conn, host string, cfg transport.RelayConfig, ca *CA) {
	defer conn.Close()
	if ca == nil {
		return
	}

	leaf, err := ca.CertForHost(host)
	if err != nil {
		log.L().Debug("mitm cert error", zap.String("host", host), zap.Error(err))
		return
	}

	tlsConn := tlsServer(conn, leaf)
	if err := tlsConn.Handshake(); err != nil {
		log.L().Debug("mitm handshake failed", zap.String("host", host), zap.Error(err))
		return
	}
	defer tlsConn.Close()

	br := bufio.NewReader(tlsConn)
	for {
		if ctx.Err() != nil {
			return
		}
		req, err := http.ReadRequest(br)
		if err != nil {
			if err != io.EOF {
				log.L().Debug("mitm read request", zap.String("host", host), zap.Error(err))
			}
			return
		}

		req.URL.Scheme = "https"
		if req.Host == "" {
			req.Host = host
		}
		req.RequestURI = ""
		req.Header.Del("Proxy-Connection")

		resp, err := transport.FetchHTTP(ctx, cfg, req)
		if err != nil {
			log.L().Debug("mitm fetch failed", zap.String("host", host), zap.Error(err))
			_ = writeHTTPError(tlsConn, http.StatusBadGateway, err.Error())
			return
		}

		if err := resp.Write(tlsConn); err != nil {
			resp.Body.Close()
			return
		}
		resp.Body.Close()

		if req.Close || resp.Close ||
			req.Header.Get("Connection") == "close" ||
			resp.Header.Get("Connection") == "close" {
			return
		}
	}
}

func tlsServer(conn net.Conn, cert tls.Certificate) *tls.Conn {
	return tls.Server(conn, &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"http/1.1"},
		MinVersion:   tls.VersionTLS12,
	})
}

func writeHTTPError(w io.Writer, code int, msg string) error {
	resp := fmt.Sprintf("HTTP/1.1 %d %s\r\nContent-Type: text/plain\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s",
		code, http.StatusText(code), len(msg), msg)
	_, err := w.Write([]byte(resp))
	return err
}

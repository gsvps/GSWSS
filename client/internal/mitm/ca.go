package mitm

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CA holds a local MITM certificate authority for HTTPS fetch fallback.
type CA struct {
	cert    tls.Certificate
	certPEM []byte
	mu      sync.Mutex
	leaves  map[string]tls.Certificate
}

// LoadOrCreate loads or generates a CA in dir. Returns path to ca.crt for trust installation.
func LoadOrCreate(dir string) (*CA, string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, "", err
	}
	certPath := filepath.Join(dir, "mitm-ca.crt")
	keyPath := filepath.Join(dir, "mitm-ca.key")

	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		ca, err := generateCA()
		if err != nil {
			return nil, "", err
		}
		if err := os.WriteFile(certPath, ca.certPEM, 0o644); err != nil {
			return nil, "", err
		}
		keyDER, err := x509.MarshalECPrivateKey(ca.cert.PrivateKey.(*ecdsa.PrivateKey))
		if err != nil {
			return nil, "", err
		}
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
		if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
			return nil, "", err
		}
		ca.leaves = make(map[string]tls.Certificate)
		return ca, certPath, nil
	}

	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, "", err
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, "", err
	}
	return &CA{cert: cert, certPEM: certPEM, leaves: make(map[string]tls.Certificate)}, certPath, nil
}

func generateCA() (*CA, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "GS Protocol MITM CA", Organization: []string{"GS Protocol"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &CA{cert: tlsCert, certPEM: certPEM, leaves: make(map[string]tls.Certificate)}, nil
}

// CertForHost returns a leaf certificate for the given hostname.
func (c *CA) CertForHost(host string) (tls.Certificate, error) {
	name := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		name = h
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if cert, ok := c.leaves[name]; ok {
		return cert, nil
	}

	caCert, err := x509.ParseCertificate(c.cert.Certificate[0])
	if err != nil {
		return tls.Certificate{}, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{name},
	}
	if ip := net.ParseIP(name); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, c.cert.PrivateKey)
	if err != nil {
		return tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	leaf, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, err
	}
	c.leaves[name] = leaf
	return leaf, nil
}

// CertPathHint returns a user-facing hint for trusting the CA.
func CertPathHint(path string) string {
	return fmt.Sprintf("Install MITM CA for HTTPS fetch fallback: %s", path)
}

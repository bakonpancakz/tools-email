package email

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

// Specify a path to a SSL Certificate, Key, and Certificate Authority Bundle
func (e *Engine) SetupTLS(cert string, key string, ca string) error {

	// Read and Parse from Disk
	pair, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return err
	}
	caBytes, err := os.ReadFile(ca)
	if err != nil {
		return err
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caBytes) {
		return fmt.Errorf("bad response from AppendCertsFromPEM")
	}

	// compatiblity be damned
	e.TLSConfig = &tls.Config{
		Certificates: []tls.Certificate{pair},
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS13,
		MaxVersion:   tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
	}
	return nil
}

// Specify Location to RSA Key for use with DKIM Signing
func (e *Engine) SetupDKIM(key string) error {

	// Read and Parse from Disk
	b, err := os.ReadFile(key)
	if err != nil {
		return err
	}
	p, _ := pem.Decode(b)
	pkey, err := x509.ParsePKCS8PrivateKey(p.Bytes)
	if err != nil {
		return err
	}
	v, ok := pkey.(*rsa.PrivateKey)
	if !ok {
		return errors.New("expected rsa private key")
	}

	// Enable Outgoing DKIM Signing on Server
	e.OutgoingDKIMEnabled = true
	e.OutgoingDKIMSigner = v
	return nil
}

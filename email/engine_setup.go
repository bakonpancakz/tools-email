package email

import (
	"crypto"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

// Provide a path to an SSL Certificate, Key, and CA Bundle for parsing. Returning a TLS v1.3 Configuration
func LoadTLSConfig(cert string, key string, ca string) (*tls.Config, error) {
	pair, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}
	caBytes, err := os.ReadFile(ca)
	if err != nil {
		return nil, err
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caBytes) {
		return nil, fmt.Errorf("invalid or malformed certificate(s) in CA bundle")
	}
	return &tls.Config{
		Certificates: []tls.Certificate{pair},
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS13,
		MaxVersion:   tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
	}, nil
}

// Provide a path to an PKCS#8 Encoded RSA Private Key. Returning a crypto.Signer instance
func LoadDKIMSigner(key string) (crypto.Signer, error) {
	b, err := os.ReadFile(key)
	if err != nil {
		return nil, err
	}
	p, _ := pem.Decode(b)
	pkey, err := x509.ParsePKCS8PrivateKey(p.Bytes)
	if err != nil {
		return nil, err
	}
	v, ok := pkey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("expected rsa private key")
	}
	return v, nil
}

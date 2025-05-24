package env

import (
	"crypto"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"log"
	"os"
	"strings"
	"time"

	_ "embed"

	"github.com/bakonpancakz/project-suzzy/email/include"
	_ "github.com/joho/godotenv/autoload"
)

const (
	REQUEST_MAX_SIZE = 20 * 1024 * 1024 // 20MB
	REQUEST_TIMEOUT  = 30 * time.Second
)

var (
	TLS_CONFIG            *tls.Config
	DKIM_SIGNER           crypto.Signer
	SMTP_ADDRESS_DMARC    string
	SMTP_ADDRESS_NOREPLY  string
	SMTP_ADDRESS_FORWARD  string
	HTTP_PASSPHRASE       = envString("HTTP_PASSPHRASE", "teto")
	HTTP_ADDRESS          = envString("HTTP_ADDRESS", "localhost:8800")
	SMTP_ADDRESS          = envString("SMTP_ADDRESS", "localhost:2525")
	SMTP_DOMAIN           = envString("SMTP_DOMAIN", "example.org")
	SMTP_USERNAME_DMARC   = envString("SMTP_USERNAME_DMARC", "dmarc")
	SMTP_USERNAME_NOREPLY = envString("SMTP_USERNAME_NOREPLY", "noreply")
	SMTP_USERNAME_FORWARD = envString("SMTP_USERNAME_FORWARD", "support")
	SMTP_FORWARD_ADDRESS  = envString("SMTP_FORWARD_ADDRESS", "")
	SMTP_DISABLE_NOREPLY  = envString("SMTP_DISABLE_NOREPLY", "")
	TLS_CERT              = envString("TLS_CERT", "tls_crt.pem")
	TLS_KEY               = envString("TLS_KEY", "tls_key.pem")
	TLS_CA                = envString("TLS_CA", "tls_ca.pem")
	DKIM_KEY              = envString("DKIM", "dkim.pem")
)

func init() {
	// Initialize Templates & Variables
	include.NoreplyIndex = strings.ReplaceAll(include.NoreplyIndex, "{{DOMAIN}}", SMTP_DOMAIN)
	SMTP_ADDRESS_DMARC = SMTP_USERNAME_DMARC + "@" + SMTP_DOMAIN
	SMTP_ADDRESS_NOREPLY = SMTP_USERNAME_NOREPLY + "@" + SMTP_DOMAIN
	SMTP_ADDRESS_FORWARD = SMTP_USERNAME_FORWARD + "@" + SMTP_DOMAIN

	// Decode RSA Key from Disk
	{
		b, err := os.ReadFile(DKIM_KEY)
		if err != nil {
			log.Fatalln("[env/dkim] Cannot Read Key:", err)
		}
		p, _ := pem.Decode(b)
		pkey, err := x509.ParsePKCS8PrivateKey(p.Bytes)
		if err != nil {
			log.Fatalln("[env/dkim] Cannot Parse Key:", err)
		}
		v, ok := pkey.(*rsa.PrivateKey)
		if !ok {
			log.Fatalln("[env/dkim] Expected RSA Private Key")
		}
		DKIM_SIGNER = v
	}

	// Load and Parse TLS Configuration from Disk
	{
		cert, err := tls.LoadX509KeyPair(TLS_CERT, TLS_KEY)
		if err != nil {
			log.Fatalln("[env/tls] Cannot Load Keypair", err)
		}
		caBytes, err := os.ReadFile(TLS_CA)
		if err != nil {
			log.Fatalln("[env/tls] Cannot Read CA File", err)
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caBytes) {
			log.Fatalln("[env/tls] Cannot Append Certificates")
		}
		TLS_CONFIG = &tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientCAs:    caPool,
			MinVersion:   tls.VersionTLS13,
			MaxVersion:   tls.VersionTLS13,
			CipherSuites: []uint16{
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_CHACHA20_POLY1305_SHA256,
			},
		}
	}
}

// Reads Variable from Environment
func envString(field, initial string) string {
	var Value = os.Getenv(field)
	if Value == "" {
		if initial == "\x00" {
			log.Printf("[env] Variable '%s' was not set.\n", field)
			os.Exit(2)
		}
		return initial
	}
	return Value
}

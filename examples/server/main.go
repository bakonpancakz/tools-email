package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "embed"

	"github.com/bakonpancakz/tools-email/email"
)

var (
	//go:embed noreply.html
	noReplyIndex string

	//go:embed noreply.png
	noReplyImage []byte

	PATH_RSA     = envString("PATH_RSA", "dkim_rsa.pem")
	PATH_TLS_KEY = envString("PATH_TLS_KEY", "tls_key.pem")
	PATH_TLS_CRT = envString("PATH_TLS_CRT", "tls_crt.pem")
	PATH_TLS_CA  = envString("PATH_TLS_CA", "tls_ca.pem")
	SMTP_DOMAIN  = envString("SMTP_DOMAIN", "example.org")
	SMTP_ADDRESS = envString("SMTP_ADDRESS", "0.0.0.0:25")
	HTTP_ADDRESS = envString("HTTP_ADDRESS", "0.0.0.0:80")
)

func init() {
	// Preprocess Template
	noReplyIndex = strings.ReplaceAll(noReplyIndex, "{{DOMAIN}}", SMTP_DOMAIN)
}

func main() {
	// Create a new engine instance for our domain
	e := email.New(SMTP_DOMAIN)

	// Setting Handlers
	// 	The Default Error Handler collects a stack trace and outputs to stderr which is fine
	// 	for our example, but could harshly affect performance in a production environment.
	e.ErrorLogger = email.DefaultErrorLogger

	// By default the Auth Handler only allow requests from a loopback address
	// You can implement your own authorization handler, below are a few examples you can implement,
	// but for this example server we'll be accepting all incoming requests.
	e.AuthHandler = func(r *http.Request) bool {
		// Example 1: Passphrase
		// 	Compare Authorization Header against a string (preferable from environment variables)
		// return r.Header.Get("Authorization") == "KasaneTeto0401"

		// Example 2: Address Allowlist
		// 	Allow requests from specific IP ranges, this one allow loopback requests
		// return net.ParseIP(ip).IsLoopback()

		// Example 3: Allow all
		// 	Don't do this but you could if you really wanted to... (>_>)

		log.Println("Incoming Requests from:", r.RemoteAddr)
		return true
	}

	// In the case an email comes in with no valid recipient we can write a function to log the email.
	// 	Please note that the SMTP Server will still respond with a '550 Invalid Recipient'
	// 	error and this behaviour cannot be modified.
	e.NoInboxHandler = func(e *email.Email) error {
		log.Printf("No Inbox for To=%v, Subject=%q, From=%q\n", e.To, e.Subject, e.From)
		return nil
	}

	// Registering Inboxes
	// 	Our application sends out emails as 'noreply@{{DOMAIN}}' in the case our user
	// 	accidentally send an email to our noreply inbox we can reply with a friendly message!
	e.RegisterInbox("noreply", func(em *email.Email) error {
		e.QueueEmail(&email.Email{
			To:      []email.Address{{Name: em.From.Name, Address: em.From.Address}},
			From:    email.Address{Name: "Example Inc.", Address: "noreply@" + e.Domain},
			Subject: "beep boop (Need Help?)",
			Content: noReplyIndex,
			HTML:    true,
			Attachments: []email.Attachment{{
				ContentType: "image/png",
				Filename:    "robot.png",
				Data:        noReplyImage,
				Inline:      true,
			}},
		})
		return nil
	})

	// Using Middleware
	// 	We can use middleware to filter inbound emails or cancel outbound emails.
	// 	Additionally we can provide an error which will be passed to our engine error logger.
	e.UseIncoming(func(em *email.Email) (bool, error) {
		// Example: Basic Spam Filter
		if em.From.Address == "hatsunemiku@crypton.co.jp" {
			return false, nil
		}
		return true, nil
	})
	// Example: Basic Inbound Email Logger
	e.UseIncoming(func(em *email.Email) (bool, error) {
		log.Println("Incoming Email from", em.From.Address)
		return true, nil
	})
	// Example: Basic Outbound Email Logger
	e.UseOutgoing(func(em *email.Email) (bool, error) {
		log.Println("Sending Email with Subject", em.Subject)
		return true, nil
	})

	// Startup Servers
	// 	We use the provided Load functions to quickly parse and initialize a TLS Configuration and DKIM Signer.
	// 	For this example TLS on the REST API is disabled by passing nil, but you should enable this in production.
	dkimSigner, err := email.LoadDKIMSigner(PATH_RSA)
	if err != nil {
		log.Fatalln("Cannot Load DKIM Key: ", err)
	}
	tlsConfig, err := email.LoadTLSConfig(PATH_TLS_CRT, PATH_TLS_KEY, PATH_TLS_CA)
	if err != nil {
		log.Fatalln("Cannot Setup TLS:", err)
	}
	go e.StartSMTP(SMTP_ADDRESS, dkimSigner, tlsConfig)
	go e.StartHTTP(HTTP_ADDRESS, nil)

	// Shutdown Server
	// 	We await a SIGINT/SIGTERM signal from the OS, the Shutdown function will return once all connections
	// 	have closed and all our emails have been sent out.
	cancel := make(chan os.Signal, 1)
	signal.Notify(cancel, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-cancel

	timeout, finish := context.WithTimeout(context.Background(), time.Minute)
	defer finish()
	go func() {
		<-timeout.Done()
		if timeout.Err() == context.DeadlineExceeded {
			log.Fatalln("Cleanup timed out, exiting now!")
		}
	}()
	e.Shutdown(timeout)

	log.Println("All done, bye bye!")
	os.Exit(0)
}

func envString(field, initial string) string {
	var Value = os.Getenv(field)
	if Value == "" {
		if initial == "\x00" {
			fmt.Printf("Variable '%s' was not set\n", field)
			os.Exit(2)
		}
		return initial
	}
	return Value
}

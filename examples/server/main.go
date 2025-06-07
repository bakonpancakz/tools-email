package main

import (
	"context"
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

const (
	PATH_RSA     = "dkim_rsa.pem"
	PATH_TLS_KEY = "tls_key.pem"
	PATH_TLS_CRT = "tls_crt.pem"
	PATH_TLS_CA  = "tls_ca.pem"
	DOMAIN       = "example.org"   // Domain to advertise for SMTP Server
	ADDRESS_SMTP = "localhost:587" // Port 587 with TLS, port 25 without
	ADDRESS_HTTP = "localhost:443" // Port 443 for HTTPS, 80 for HTTP
)

//go:embed noreply.html
var noReplyIndex string

//go:embed noreply.png
var noReplyImage []byte

func init() {
	// Preprocess Template
	noReplyIndex = strings.ReplaceAll(noReplyIndex, "{{DOMAIN}}", DOMAIN)
}

func main() {
	// Create a new engine instance for our domain
	e := email.New(DOMAIN)

	// Setting Handlers
	// 	The Default Error Handler collects a stack trace and outputs to stderr which is fine
	// 	for our example, but could harshly affect performance in a production environment.
	e.HandlerError = email.DefaultHandlerError

	// 	The Default Authorization Handler checks the Incoming Requests Authorization Header for the passphrase "teto".
	// 	This insecure and should be replaced with a custom handler used for filtering incoming requests to the REST API.
	e.HandlerAuthorization = func(r *http.Request) bool {
		// Example 1: Passphrase
		// 	Compare Authorization Header against a string (preferable from environment variables)
		// return r.Header.Get("Authorization") == "KasaneTeto0401"

		// Example 2: Address Allowlist
		// 	Allow requests from specific IP ranges, this one allow loopback requests
		// return net.ParseIP(ip).IsLoopback()

		// Example 3: Allow all
		// 	Don't do this but you could if you really wanted to... (>_>)
		return true
	}

	// In the case an email comes in with no valid recipient we can write a function to log the email.
	// 	Please note that the SMTP Server will still respond with a '550 Invalid Recipient'
	// 	error and this behaviour cannot be modified.
	e.HandlerNoInbox = func(e *email.Email) error {
		log.Printf("No Inbox for To=%v, Subject=%q, From=%q\n", e.To, e.Subject, e.From)
		return nil
	}

	// Registering Inboxes
	// 	Our application sends out emails as 'noreply@{{DOMAIN}}' in the case our user
	// 	accidentally send an email to our noreply inbox we can reply with a friendly message!
	e.RegisterInbox("noreply", func(em *email.Email) error {
		e.QueueEmail(&email.Email{
			To:      []email.Address{{Name: em.From.Name, Address: em.From.Address}},
			From:    email.Address{Name: "Robot", Address: "noreply@" + e.Domain},
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

	// 	Although nothing is done with the provided DMARC email you should probably register
	// 	an inbox for them otherwise your mailserver might get flagged as spam
	e.RegisterInbox("dmarc", func(em *email.Email) error {
		return nil
	})

	// Using Middleware
	// 	We can use middleware to filter inbound emails or cancel outbound emails.
	// 	Additionally we can provide an error which will be passed to our engine error handler.
	e.UseIncoming(func(em *email.Email) (bool, error) {
		// Example: Basic Spam Filter
		if em.From.Address == "hatsunemiku@crypton.co.jp" {
			return false, nil
		}
		return true, nil
	})
	e.UseIncoming(func(em *email.Email) (bool, error) {
		// Example: Basic Inbound Logger
		log.Println("Incoming Email from", em.From.Address)
		return true, nil
	})
	e.UseOutgoing(func(em *email.Email) (bool, error) {
		// Example: Basic Outbound Logger
		log.Println("Sending Email with Subject", em.Subject)
		return true, nil
	})

	// Initialize TLS and DKIM
	// 	We can use these setup functions to handle reading and parsing our TLS Configuration and DKIM Key
	// 	You can set these manually by modifying the TLSConfig, OutgoingDKIMSigner, and OutgoingDKIMEnabled fields
	if err := e.SetupDKIM(PATH_RSA); err != nil {
		log.Fatalln("Cannot Setup DKIM: ", err)
	}
	if err := e.SetupTLS(PATH_TLS_KEY, PATH_TLS_CRT, PATH_TLS_CA); err != nil {
		log.Fatalln("Cannot Setup TLS:", err)
	}

	// Server Shutdown
	// 	Example Graceful shutdown on SIGINT/SIGTERM to let the queue flush before exit
	go func() {
		log.Println("Starting:", ADDRESS_SMTP, ADDRESS_HTTP)
		if err := e.Start(ADDRESS_SMTP, ADDRESS_HTTP); err != nil {
			log.Fatalln("Startup Error:", err)
		}
	}()

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

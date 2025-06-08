package email

import (
	"context"
	"crypto"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/emersion/go-smtp"
)

type HandlerAuthorization = func(r *http.Request) bool
type HandlerMiddleware = func(e *Email) (bool, error)
type HandlerEmail = func(e *Email) error
type HandlerError = func(e error)

type Engine struct {
	activeClosing         sync.Once               // Prevents multiple shutdowns
	activeWorkers         sync.WaitGroup          // Tracks open email workers
	OutgoingWorkerCount   int                     // Thread Count for Queue Processing (Defaults to the value of runtime.NumCPUs())
	OutgoingTimeout       time.Duration           // Outgoing Email Timeout
	outgoingQueue         chan *Email             // Outgoing Email Queue
	outgoingMiddleware    []HandlerMiddleware     // Outgoing Email Middleware
	OutgoingDKIMEnabled   bool                    // Sign Outgoing Emails?
	OutgoingDKIMSigner    crypto.Signer           // Private Key for DKIM Signing
	OutgoingSelectorName  string                  // DKIM selector used for signing outgoing emails (default: "default")
	IncomingValidateDKIM  bool                    // Validate Incoming Emails with DKIM? (Defaults to true)
	IncomingMaxRecipients int                     // Reject Incoming Email if amount of recipients is larger than given value (Defaults to 5)
	IncomingMaxBytes      int64                   // Reject Incoming Email if payload is larger than x bytes (Defaults to 10MB)
	IncomingTimeout       time.Duration           // Reject Incoming Email if processing takes longer than given duration
	incomingMiddleware    []HandlerMiddleware     // Incoming Email Middleware
	Domain                string                  // Advertising Domain for SMTP Server
	HandlerError          HandlerError            // Provided Error Handler
	HandlerNoInbox        HandlerEmail            // Provided No Inbox Handler
	HandlerAuthorization  HandlerAuthorization    // Determines if a REST API request is authorized
	inboxes               map[string]HandlerEmail // Incoming Email Inbox Handlers
	TLSEnabledHttp        bool                    // Use TLS for HTTP Server
	TLSConfig             *tls.Config             // TLS Configuration for SMTP
	smtpServer            *smtp.Server            // Email Server
	httpServer            *http.Server            // HTTP Server
}

// Startup the REST API, SMTP Server, and Outbound Queue Threads.
// Enabling and using TLS if properly configured.
func (e *Engine) Start(smtpAddr, httpAddr string) error {
	listenErr := make(chan error, 2)

	// Initialize HTTP
	httpServer := http.Server{
		Addr:         httpAddr,
		Handler:      newHttpHandler(e),
		TLSConfig:    e.TLSConfig,
		WriteTimeout: e.IncomingTimeout,
		ReadTimeout:  e.IncomingTimeout,
	}
	go func() {
		if e.TLSEnabledHttp {
			listenErr <- httpServer.ListenAndServeTLS("", "")
		} else {
			listenErr <- httpServer.ListenAndServe()
		}
	}()
	e.httpServer = &httpServer

	// Initialize SMTP
	smtpServer := smtp.NewServer(&Backend{engine: e})
	smtpServer.Addr = smtpAddr
	smtpServer.Domain = e.Domain
	smtpServer.ReadTimeout = e.IncomingTimeout
	smtpServer.WriteTimeout = e.OutgoingTimeout
	smtpServer.MaxMessageBytes = e.IncomingMaxBytes
	smtpServer.TLSConfig = e.TLSConfig
	smtpServer.MaxRecipients = e.IncomingMaxRecipients
	go func() {
		listenErr <- smtpServer.ListenAndServe()
	}()
	e.smtpServer = smtpServer

	// Initialize Workers
	for i := 0; i < e.OutgoingWorkerCount; i++ {
		e.activeWorkers.Add(1)
		go func() {
			defer e.activeWorkers.Done()
			for email := range e.outgoingQueue {
				if err := e.SendEmail(email); err != nil {
					e.HandlerError(err)
				}
			}
		}()
	}

	if err := <-listenErr; err != http.ErrServerClosed && err != smtp.ErrServerClosed {
		return err
	}
	return nil
}

// Gracefully attempt to shutdown the REST API and SMTP servers, waiting for all
// open connections to close and queued emails to complete before returning. It
// is safe to call this function multiple times.
func (e *Engine) Shutdown(ctx context.Context) {
	e.activeClosing.Do(func() {
		var wg sync.WaitGroup
		wg.Add(3)
		go func() {
			// Wait for incoming HTTP Requests to Finish
			defer wg.Done()
			if err := e.httpServer.Shutdown(ctx); err != nil {
				log.Println("HTTP shutdown error:", err)
			}
		}()
		go func() {
			// Wait for incoming SMTP Connections to Finish
			defer wg.Done()
			if err := e.smtpServer.Shutdown(ctx); err != nil {
				log.Println("SMTP shutdown error:", err)
			}
		}()
		go func() {
			// Wait for Outgoing Queue to Complete
			defer wg.Done()
			close(e.outgoingQueue)
			e.activeWorkers.Wait()
		}()
		wg.Wait()
	})
}

// Default Error Logger, prints a stack trace and error to stderr
func DefaultHandlerError(err error) {
	b := make([]byte, 4096)
	n := runtime.Stack(b, false)
	fmt.Fprintf(os.Stderr, "%s\n%s\n", err, b[:n])
}

// Default Authorization Handler, allows request if Authorization Header equals "teto"
func DefaultHandlerAuthorization(r *http.Request) bool {
	return r.Header.Get("Authorization") == "teto"
}

// Create a New Engine using the Default Settings
func New(domain string) Engine {
	return Engine{
		Domain:                domain,
		OutgoingWorkerCount:   runtime.NumCPU(),
		OutgoingTimeout:       30 * time.Second,
		outgoingQueue:         make(chan *Email, 1024),
		outgoingMiddleware:    []HandlerMiddleware{},
		OutgoingSelectorName:  "default",
		IncomingValidateDKIM:  true,
		IncomingMaxRecipients: 5,
		IncomingMaxBytes:      10 << 20,
		IncomingTimeout:       30 * time.Second,
		incomingMiddleware:    []HandlerMiddleware{},
		HandlerAuthorization:  DefaultHandlerAuthorization,
		HandlerError:          DefaultHandlerError,
		inboxes:               make(map[string]HandlerEmail),
	}
}

package email

import (
	"context"
	"crypto"
	"crypto/tls"
	"log"
	"net/http"
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
	outgoingDKIMSigner    crypto.Signer           // Private Key for DKIM Signing
	OutgoingSelectorName  string                  // DKIM selector used for signing outgoing emails (default: "default")
	IncomingValidateDKIM  bool                    // Validate Incoming Emails with DKIM? (Defaults to true)
	IncomingMaxRecipients int                     // Reject Incoming Email if amount of recipients is larger than given value (Defaults to 5)
	IncomingMaxBytes      int64                   // Reject Incoming Email if payload is larger than x bytes (Defaults to 10MB)
	IncomingTimeout       time.Duration           // Reject Incoming Email if processing takes longer than given duration
	incomingMiddleware    []HandlerMiddleware     // Incoming Email Middleware
	Domain                string                  // Advertising Domain for SMTP Server
	ErrorLogger           HandlerError            // Provided Error Handler
	NoInboxHandler        HandlerEmail            // Provided No Inbox Handler
	AuthHandler           HandlerAuthorization    // Determines if a REST API request is authorized
	inboxes               map[string]HandlerEmail // Incoming Email Inbox Handlers
	smtpServer            *smtp.Server            // Email Server
	httpServer            *http.Server            // HTTP Server
}

// Start the internal REST API for externally queueing emails.
// Provide a nil tlsConfig to disable HTTPS.
func (e *Engine) StartHTTP(addr string, tlsConfig *tls.Config) error {
	httpServer := http.Server{
		Addr:         addr,
		Handler:      newHttpHandler(e),
		TLSConfig:    tlsConfig,
		WriteTimeout: e.IncomingTimeout,
		ReadTimeout:  e.IncomingTimeout,
	}
	e.httpServer = &httpServer
	if tlsConfig != nil {
		return httpServer.ListenAndServeTLS("", "")
	}
	return httpServer.ListenAndServe()
}

// Start the internal SMTP Server and Outbound Queue Workers.
// Provide a nil tlsConfig to disable TLS.
// Provide a nil dkimSigner to disable the signing of outbound emails.
func (e *Engine) StartSMTP(addr string, dkimSigner crypto.Signer, tlsConfig *tls.Config) error {

	// Initialize Server
	smtpServer := smtp.NewServer(&Backend{engine: e})
	smtpServer.Addr = addr
	smtpServer.Domain = e.Domain
	smtpServer.ReadTimeout = e.IncomingTimeout
	smtpServer.WriteTimeout = e.OutgoingTimeout
	smtpServer.MaxMessageBytes = e.IncomingMaxBytes
	smtpServer.MaxRecipients = e.IncomingMaxRecipients
	smtpServer.TLSConfig = tlsConfig
	e.outgoingDKIMSigner = dkimSigner
	e.smtpServer = smtpServer

	// Start Worker Threads
	for i := 0; i < e.OutgoingWorkerCount; i++ {
		e.activeWorkers.Add(1)
		go func() {
			defer e.activeWorkers.Done()
			for email := range e.outgoingQueue {
				if err := e.SendEmail(email); err != nil {
					e.ErrorLogger(err)
				}
			}
		}()
	}

	return smtpServer.ListenAndServe()
}

// Gracefully attempt to shutdown the REST API and SMTP servers if started.
// It will return once all connections are closed and emails have been sent.
// It is safe to call this function multiple times.
func (e *Engine) Shutdown(ctx context.Context) {
	e.activeClosing.Do(func() {
		var wg sync.WaitGroup
		if e.httpServer != nil {
			wg.Add(1)
			go func() {
				// Wait for incoming HTTP Requests to Finish
				defer wg.Done()
				if err := e.httpServer.Shutdown(ctx); err != nil {
					log.Println("HTTP shutdown error:", err)
				}
			}()
		}
		if e.smtpServer != nil {
			wg.Add(1)
			go func() {
				// Wait for incoming SMTP Connections to Finish
				defer wg.Done()
				if err := e.smtpServer.Shutdown(ctx); err != nil {
					log.Println("SMTP shutdown error:", err)
				}
			}()
			wg.Add(1)
			go func() {
				// Wait for Outgoing Queue to Complete
				defer wg.Done()
				close(e.outgoingQueue)
				e.activeWorkers.Wait()
			}()
		}
		wg.Wait()
	})
}

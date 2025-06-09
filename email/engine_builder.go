package email

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"time"
)

// Default Error Logger, collects a stack trace and outputs to stderr
func DefaultErrorLogger(err error) {
	b := make([]byte, 4096)
	n := runtime.Stack(b, false)
	fmt.Fprintf(os.Stderr, "%s\n%s\n", err, b[:n])
}

// Default Authorization Handler, allows all requests from localhost (loopback) by default
func DefaultAuthHandler(r *http.Request) bool {
	return net.ParseIP(r.RemoteAddr).IsLoopback()
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
		AuthHandler:           DefaultAuthHandler,
		ErrorLogger:           DefaultErrorLogger,
		inboxes:               make(map[string]HandlerEmail),
	}
}

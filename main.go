package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/bakonpancakz/project-suzzy/email/env"
	"github.com/emersion/go-smtp"
	"github.com/go-playground/validator/v10"
)

func main() {
	// Startup Services
	var stopCtx, stop = context.WithCancel(context.Background())
	var stopWg sync.WaitGroup
	go StartupHTTP(stopCtx, &stopWg)
	go StartupSMTP(stopCtx, &stopWg)
	go env.StartupWorkers(stopCtx, &stopWg)

	// Await Shutdown Signal
	cancel := make(chan os.Signal, 1)
	signal.Notify(cancel, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-cancel
	stop()

	// Begin Shutdown Process
	timeout, finish := context.WithTimeout(context.Background(), time.Minute)
	defer finish()
	go func() {
		<-timeout.Done()
		if timeout.Err() == context.DeadlineExceeded {
			log.Fatalln("[main] Cleanup timeout! Exiting now.")
		}
	}()
	stopWg.Wait()
	log.Println("[main] All done, bye bye!")
	os.Exit(0)
}

func StartupSMTP(stop context.Context, await *sync.WaitGroup) {
	var svr = smtp.NewServer(&env.Backend{})
	svr.Addr = env.SMTP_ADDRESS
	svr.ReadTimeout = env.REQUEST_TIMEOUT
	svr.WriteTimeout = env.REQUEST_TIMEOUT
	svr.MaxMessageBytes = env.REQUEST_MAX_SIZE
	svr.TLSConfig = env.TLS_CONFIG
	svr.MaxRecipients = 5
	svr.Domain = env.SMTP_DOMAIN

	// Shutdown Logic
	await.Add(1)
	go func() {
		defer await.Done()
		<-stop.Done()
		svr.Shutdown(context.Background())
		log.Println("[smtp] Closed Server")
	}()

	// Server Startup
	log.Printf("[smtp] Listening @ %s\n", svr.Addr)
	if err := svr.ListenAndServeTLS(); err != nil {
		log.Fatalln("[smtp] Listen Error:", err)
	}
}

func StartupHTTP(stop context.Context, await *sync.WaitGroup) {
	validate := validator.New()
	r := http.NewServeMux()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		// Validate Headers
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Authorization") != env.HTTP_PASSPHRASE {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Parse Request Body
		var incomingEmail env.Email
		if b, err := io.ReadAll(r.Body); err != nil {
			w.WriteHeader(http.StatusExpectationFailed)
			return
		} else if err := json.Unmarshal(b, &incomingEmail); err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		} else if err := validate.Struct(incomingEmail); err != nil {
			w.Write([]byte(err.Error()))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Queue Email for Processing
		env.QueueEmail(incomingEmail)
		w.WriteHeader(http.StatusCreated)
	})

	svr := http.Server{
		Handler:           r,
		Addr:              env.HTTP_ADDRESS,
		TLSConfig:         env.TLS_CONFIG,
		MaxHeaderBytes:    4096,
		IdleTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		ReadTimeout:       30 * time.Second,
	}

	// Shutdown Logic
	await.Add(1)
	go func() {
		defer await.Done()
		<-stop.Done()
		svr.Shutdown(context.Background())
		log.Println("[http] Closed Server")
	}()

	// Server Startup
	log.Printf("[http] Listening @ %s\n", svr.Addr)
	if err := svr.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
		log.Fatalln("[http] Listen Error:", err)
	}
}

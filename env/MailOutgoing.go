package env

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"net/url"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-msgauth/dkim"
	"github.com/jhillyerd/enmime"
)

var emailDebounce sync.Map
var emailQueue = make(chan Email, 1000)

type Email struct {
	ToName      string            `validate:"required" json:"to_name"`
	ToAddress   string            `validate:"required" json:"to_address"`
	FromName    string            `validate:"required" json:"from_name"`
	FromAddress string            `validate:"required" json:"from_address"`
	Subject     string            `validate:"required" json:"subject"`
	Content     string            `validate:"required" json:"content"`
	HTML        bool              `validate:"required" json:"html"`
	Attachments []EmailAttachment `json:"attachments"`
}
type EmailAttachment struct {
	ContentType string `validate:"required" json:"content_type"`
	Filename    string `validate:"required" json:"filename"`
	Data        []byte `validate:"required" json:"data"`
	Inline      bool   `validate:"required" json:"inline"`
}

func init() {
	// Regularly clear the Debounce map
	go func() {
		t := time.NewTimer(time.Hour)
		for range t.C {
			emailDebounce.Range(func(key, value any) bool {
				sent := value.(time.Time)
				if time.Now().After(sent) {
					emailDebounce.Delete(key)
				}
				return true
			})
		}
	}()
}

func StartupWorkers(stop context.Context, await *sync.WaitGroup) {
	// Startup Workers for Queue Processing
	activeWorkers := sync.WaitGroup{}
	for i := 0; i < max(runtime.NumCPU(), 8); i++ {
		activeWorkers.Add(1)
		go func() {
			defer activeWorkers.Done()
			for e := range emailQueue {
				t := time.Now()
				err := SendEmail(e)
				log.Printf(
					"%s <%s> => %s <%s> : %s (%d Attachments) (%s) (Error: %s)",
					e.FromName, e.FromAddress,
					e.ToName, e.ToAddress,
					e.Subject, len(e.Attachments),
					time.Since(t),
					err,
				)
			}
		}()
	}

	// Shutdown Logic
	await.Add(1)
	go func() {
		defer await.Done()
		<-stop.Done()
		close(emailQueue)
		activeWorkers.Wait()
		log.Println("[workers] Finished Queue")
	}()
}

func QueueEmail(e Email) {
	select {
	case emailQueue <- e:
	default:
		log.Printf("[email] Queue full! Dropped email to %s <%s>\n", e.ToName, e.ToAddress)
	}
}

// Takes the provided email and sends it, you should probably use QueueEmail unless this email is that important.
func SendEmail(e Email) error {

	// Create Email with Mailbuilder
	var output = bytes.Buffer{}
	var envelope = bytes.Buffer{}
	var builder = enmime.Builder().
		From(e.FromName, e.FromAddress).
		To(e.ToName, e.ToAddress).
		Subject(e.Subject)
	if e.HTML {
		builder = builder.HTML([]byte(e.Content))
	} else {
		builder = builder.Text([]byte(e.Content))
	}
	for i := range e.Attachments {
		a := &e.Attachments[i]
		if a.Inline {
			builder = builder.AddInline(a.Data, a.ContentType, a.Filename, a.Filename)
		} else {
			builder = builder.AddAttachment(a.Data, a.ContentType, a.Filename)
		}
	}

	// Build Email and Sign it
	if p, err := builder.Build(); err != nil {
		return fmt.Errorf("building error %s", err)
	} else if err := p.Encode(&output); err != nil {
		return fmt.Errorf("encoding error %s", err)
	}
	err := dkim.Sign(&envelope, &output, &dkim.SignOptions{
		Domain:   SMTP_DOMAIN,
		Signer:   DKIM_SIGNER,
		Selector: "default",
	})
	if err != nil {
		return fmt.Errorf("signing error %s", err)
	}

	// Lookup MX records for address host
	var host string
	if addr, err := url.Parse("email://" + e.ToAddress); err != nil {
		return fmt.Errorf("malformed email address %s", addr)
	} else {
		host = addr.Hostname()
	}
	mxRecords, err := net.LookupMX(host)
	if err != nil {
		if e, ok := err.(*net.DNSError); ok && e.IsNotFound {
			return fmt.Errorf("no mx records available for %s", host)
		} else {
			return fmt.Errorf("dns lookup failed for %s", host)
		}
	}

	// The smtp.SendMail function has a built-in timeout of 10s so we cap
	// the MX records we know about to 3 so that way it's a 30s timeout
	sort.Slice(mxRecords, func(i, j int) bool {
		return mxRecords[i].Pref < mxRecords[j].Pref
	})
	if len(mxRecords) > 3 {
		mxRecords = mxRecords[:3]
	}

	// Attempt to Deliver Envelope
	var sendError []string
	for _, mx := range mxRecords {
		err := smtp.SendMail(
			fmt.Sprintf("%s:%d", mx.Host, 25),
			nil, e.FromAddress,
			[]string{e.ToAddress}, envelope.Bytes(),
		)
		if err == nil {
			return nil // Success!
		}
		sendError = append(sendError, err.Error())
	}
	return fmt.Errorf("cannot send email: %s", strings.Join(sendError, ", "))
}

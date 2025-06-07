package email

import (
	"bytes"
	"fmt"
	"net"
	"net/smtp"
	"sort"
	"strings"

	"github.com/emersion/go-msgauth/dkim"
	"github.com/jhillyerd/enmime"
)

// todo: better errorhandler usage

// Extracts the Host from an Email Address (e.g. bakonpancakz@gmail.com => gmail.com)
func extractHostFromAddress(address string) (string, error) {
	parts := strings.SplitN(address, "@", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", fmt.Errorf("invalid email address: %s", address)
	}
	return parts[1], nil
}

// Append a middleware function for outgoing emails
func (e *Engine) UseOutgoing(handler HandlerMiddleware) {
	e.outgoingMiddleware = append(e.outgoingMiddleware, handler)
}

// Queue an Outgoing Email, returns false if email was dropped for being full
func (e *Engine) QueueEmail(email *Email) bool {
	select {
	case e.outgoingQueue <- email:
		return true
	default:
		return false
	}
}

// Bypass the Outbound Email Queue and Send an Email Immediately
func (e *Engine) SendEmail(email *Email) error {

	// Sanity Checks
	if len(email.To) == 0 {
		return fmt.Errorf("no recipients")
	}

	// Run Middleware
	for _, mw := range e.outgoingMiddleware {
		if proceed, err := mw(email); !proceed {
			return fmt.Errorf("cancelled by middleware: %s", err)
		}
	}

	// Generate Unique Email for Each Recipient
	// 	Because sending an email to 10 people probably isn't the
	// 	behaviour you were hoping for
	for _, addressee := range email.To {

		// Create New Envelope for Recipient
		var envelope bytes.Buffer
		builder := enmime.Builder().
			From(email.From.Address, email.From.Name).
			To(addressee.Name, addressee.Address).
			Subject(email.Subject)

		// Append Content
		if email.HTML {
			builder = builder.HTML([]byte(email.Content))
		} else {
			builder = builder.Text([]byte(email.Content))
		}

		// Append Attachments
		for i := range email.Attachments {
			a := &email.Attachments[i]
			if a.Inline {
				builder = builder.AddInline(a.Data, a.ContentType, a.Filename, a.Filename)
			} else {
				builder = builder.AddAttachment(a.Data, a.ContentType, a.Filename)
			}
		}

		// Build Envelope
		if p, err := builder.Build(); err != nil {
			return fmt.Errorf("unable to build envelope: %s", err)
		} else if err := p.Encode(&envelope); err != nil {
			return fmt.Errorf("unable to encode envelope: %s", err)
		}

		// Sign Envelope
		var complete bytes.Buffer
		if e.OutgoingDKIMEnabled {
			// Sign Email using DKIM Key
			if err := dkim.Sign(&envelope, &complete, &dkim.SignOptions{
				Domain:   e.Domain,
				Signer:   e.OutgoingDKIMSigner,
				Selector: e.OutgoingSelectorName,
			}); err != nil {
				return fmt.Errorf("unable to sign envelope: %s", err)
			}
		} else {
			// inb4 marked as spam or rejected
			complete = envelope
		}

		// Lookup MX Records for Provided Addressee
		host, err := extractHostFromAddress(addressee.Address)
		if err != nil {
			return err
		}
		records, err := net.LookupMX(host)
		if err != nil {
			if e, ok := err.(*net.DNSError); ok && e.IsNotFound {
				return fmt.Errorf("no mx records present for %s", host)
			} else {
				return fmt.Errorf("mx records unresolvable for %s: %s", host, err)
			}
		}
		sort.Slice(records, func(i, j int) bool {
			// These should already be sorted, but we sort them ourselves jic
			return records[i].Pref < records[j].Pref
		})

		// Attempt to Deliver Envelope
		//	The smtp.SendEmail function has an internal timeout of 10 seconds (lame)
		// 	We additionally want to cycle through as many available servers as possible
		attemptErrors := []string{}
		attemptTotal := max(int(e.OutgoingTimeout.Seconds()/10), 1)
		for i := 0; i < attemptTotal; i++ {
			host := records[i%len(records)].Host
			if err := smtp.SendMail(
				fmt.Sprint(host, ":", 25),
				nil, // anonymous
				email.From.Address,
				[]string{addressee.Address},
				complete.Bytes(),
			); err != nil {
				message := fmt.Sprintf("attempt %d/%d failed: %s", i+1, attemptTotal, err.Error())
				attemptErrors = append(attemptErrors, message)
				continue
			}
			return nil
		}
		return fmt.Errorf("email delivery failed: %s", strings.Join(attemptErrors, "\n"))
	}
	return nil
}

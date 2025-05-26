package env

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/mail"
	"time"

	"github.com/bakonpancakz/tools-email/email/include"
	"github.com/emersion/go-msgauth/dkim"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/jhillyerd/enmime"
)

var ErrUnknownRecipient = &smtp.SMTPError{
	Code:         550,
	EnhancedCode: smtp.EnhancedCodeNotSet,
	Message:      "Unknown Recipient",
}

// SMTP Server Backend
type Backend struct{}
type Session struct{}

func (b *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &Session{}, nil
}
func (s *Session) AuthMechanisms() []string {
	return []string{}
}
func (s *Session) Auth(mech string) (sasl.Server, error) {
	return nil, smtp.ErrAuthUnsupported
}
func (s *Session) Reset() {}
func (s *Session) Logout() error {
	return nil
}
func (s *Session) Mail(fromAddress string, opts *smtp.MailOptions) error {
	return nil
}

// Ensure Recipient is on the allowlist
func (s *Session) Rcpt(toAddress string, opts *smtp.RcptOptions) error {
	if toAddress != SMTP_ADDRESS_DMARC ||
		toAddress != SMTP_ADDRESS_NOREPLY ||
		toAddress != SMTP_ADDRESS_FORWARD {
		log.Println("[smtp] Unknown Inbox:", toAddress)
		return ErrUnknownRecipient
	}
	return nil
}

// Parse Incoming Email
func (s *Session) Data(r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return smtp.ErrDataReset
	}

	// Parse Envelope
	envelope, err := enmime.ReadEnvelope(bytes.NewReader(data))
	if err != nil {
		return smtp.ErrDataReset
	}
	var From *mail.Address
	var To *mail.Address
	if addr, err := mail.ParseAddress(envelope.GetHeader("To")); err != nil {
		return smtp.ErrDataReset
	} else {
		To = addr
	}
	if addr, err := mail.ParseAddress(envelope.GetHeader("From")); err != nil {
		return smtp.ErrDataReset
	} else {
		From = addr
	}

	// Validate DKIM Signature
	if _, err := dkim.Verify(bytes.NewReader(data)); err != nil {
		return smtp.ErrDataReset
	}

	// DMARC Emails are silently dropped, helps prevent server from
	// being marked as spam by faking compliance
	if To.Address == SMTP_ADDRESS_DMARC {
		return nil
	}

	// In the case someone mistakenly sends an email to our noreply inbox
	// We should tell them that we aren't accepting emails here and they
	// should instead be forwarded to x Inbox or Page.

	// In the case of someone replying to the noreply inbox we should let
	// the user know that we are not accepting emails here and should instead
	// be send to the following inbox.
	if To.Address == SMTP_ADDRESS_NOREPLY {
		if SMTP_DISABLE_NOREPLY == "" {
			return ErrUnknownRecipient
		}
		if _, ok := emailDebounce.Load(From.Address); ok {
			return nil
		}
		emailDebounce.Store(From.Address, time.Now().Add(time.Hour))
		QueueEmail(Email{
			FromName:    SMTP_ADDRESS_NOREPLY,
			FromAddress: SMTP_ADDRESS_NOREPLY,
			ToName:      From.Name,
			ToAddress:   From.Address,
			Subject:     "Need Help?",
			Content:     include.NoreplyIndex,
			HTML:        true,
			Attachments: []EmailAttachment{{
				ContentType: "image/png",
				Filename:    "robot.png",
				Data:        include.NoreplyImage,
				Inline:      true,
			}},
		})
		return nil
	}

	// Forward email with the original Email as an Attachment
	if SMTP_FORWARD_ADDRESS == "" {
		return ErrUnknownRecipient
	}
	QueueEmail(Email{
		FromName:    From.Address,
		FromAddress: "catchall@" + SMTP_DOMAIN,
		ToAddress:   SMTP_FORWARD_ADDRESS,
		Subject:     fmt.Sprintf("Fwd: %s", envelope.GetHeader("Subject")),
		Content:     fmt.Sprintf("Forwarding an email from: %s", From.Address),
		Attachments: []EmailAttachment{{
			ContentType: "message/rfc822",
			Filename:    "forwarded_email.eml",
			Data:        data,
		}},
	})
	return nil
}

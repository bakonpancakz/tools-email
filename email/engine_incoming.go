package email

import (
	"bytes"
	"fmt"
	"io"
	"net/mail"

	"github.com/emersion/go-msgauth/dkim"
	"github.com/emersion/go-smtp"
	"github.com/jhillyerd/enmime"
)

// Append a middleware function for incoming emails
func (e *Engine) UseIncoming(handler HandlerMiddleware) {
	e.incomingMiddleware = append(e.incomingMiddleware, handler)
}

// Register an Inbox to Handle Incoming Emails
func (e *Engine) RegisterInbox(username string, handler HandlerEmail) error {
	address := fmt.Sprint(username, "@", e.Domain)
	if _, exists := e.inboxes[address]; exists {
		return fmt.Errorf("an inbox already exists with that username: %s", address)
	}
	e.inboxes[address] = handler
	return nil
}

func (e *Engine) incomingHandler(r io.Reader) error {

	// Read Incoming Envelope
	// 	Additionally we need to clone this message otherwise the DKIM Reader
	//  will consume it before we can parse it :<
	body, err := io.ReadAll(r)
	if err != nil {
		e.ErrorLogger(fmt.Errorf("incoming email cannot be read: %s", err))
		return smtp.ErrDataReset
	}
	envelope, err := enmime.ReadEnvelope(bytes.NewReader(body))
	if err != nil {
		e.ErrorLogger(fmt.Errorf("incoming email is invalid or malfored: %s", err))
		return smtp.ErrDataReset
	}

	// Validate Incoming Addresses
	var emailFrom *mail.Address
	var emailTo []*mail.Address
	if emailTo, err = mail.ParseAddressList(envelope.GetHeader("To")); err != nil {
		e.ErrorLogger(fmt.Errorf("incoming email contains an invalid 'To' header: %s", err))
		return smtp.ErrDataReset
	}
	if emailFrom, err = mail.ParseAddress(envelope.GetHeader("From")); err != nil {
		e.ErrorLogger(fmt.Errorf("incoming email contains an invalid 'From' header: %s", err))
		return smtp.ErrDataReset
	}
	if len(emailTo) > e.IncomingMaxRecipients {
		// SMTP Backend should have filtered this out earlier, but we stop it here jic
		e.ErrorLogger(fmt.Errorf("incoming email includes too many recipients"))
		return smtp.ErrDataReset
	}

	// Validate Incoming Signature
	if e.IncomingValidateDKIM {
		if _, err := dkim.Verify(bytes.NewReader(body)); err != nil {
			e.ErrorLogger(fmt.Errorf("incoming email failed dkim signature validation: %s", err))
			return smtp.ErrDataReset
		}
	}

	// Apply Abstraction
	incomingAttachments := make([]Attachment, 0, len(envelope.Attachments)+len(envelope.Inlines))
	for i := range envelope.Attachments {
		a := envelope.Attachments[i]
		incomingAttachments = append(incomingAttachments, Attachment{
			Filename:    a.FileName,
			ContentType: a.ContentType,
			Data:        a.Content,
			Inline:      false,
		})
	}
	for i := range envelope.Inlines {
		a := envelope.Inlines[i]
		incomingAttachments = append(incomingAttachments, Attachment{
			Filename:    a.FileName,
			ContentType: a.ContentType,
			Data:        a.Content,
			Inline:      true,
		})
	}
	incomingRecipients := make([]Address, 0, len(emailTo))
	for _, recipient := range emailTo {
		incomingRecipients = append(incomingRecipients, Address{
			Name:    recipient.Name,
			Address: recipient.Address,
		})
	}
	email := &Email{
		From: Address{
			Address: emailFrom.Address,
			Name:    emailFrom.Name,
		},
		To:          incomingRecipients,
		Subject:     envelope.GetHeader("Subject"),
		Attachments: incomingAttachments,
	}
	if envelope.HTML == "" {
		email.Content = envelope.Text
		email.HTML = false
	} else {
		email.Content = envelope.HTML
		email.HTML = true
	}

	// Run Middleware
	for _, mw := range e.incomingMiddleware {
		if proceed, err := mw(email); !proceed {
			if err != nil {
				e.ErrorLogger(fmt.Errorf("incoming middleware encountered an error: %s", err))
			}
			return smtp.ErrDataReset
		}
	}

	// Route to Appropriate Inboxes
	receivedBy := 0
	for _, recipient := range emailTo {
		if handler, ok := e.inboxes[recipient.Address]; ok {
			if err := handler(email); err != nil {
				e.ErrorLogger(fmt.Errorf("inbox handler encountered an error: %s", err))
				return smtp.ErrDataReset
			}
			receivedBy++
		}
	}
	if receivedBy == 0 {
		if e.NoInboxHandler != nil {
			if err := e.NoInboxHandler(email); err != nil {
				e.ErrorLogger(fmt.Errorf("no inbox handler encountered an error: %s", err))
				return smtp.ErrDataReset
			}
		}
		return &smtp.SMTPError{
			Code:         550,
			EnhancedCode: smtp.EnhancedCodeNotSet,
			Message:      "Unknown Recipient",
		}
	}

	return nil
}

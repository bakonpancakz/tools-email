package email

import (
	"io"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

// todo: comment

type Backend struct {
	engine *Engine
}
type Session struct {
	engine *Engine
}

func (b *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &Session{engine: b.engine}, nil
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
func (s *Session) Rcpt(toAddress string, opts *smtp.RcptOptions) error {
	return nil
}
func (s *Session) Data(r io.Reader) error {
	return s.engine.incomingHandler(r)
}

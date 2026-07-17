// Package mailer provides SMTP email delivery for notifications, with a
// no-op fallback when SMTP is not configured.
package mailer

import (
	"errors"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"

	"github.com/it4nodummies/heureum/internal/config"
)

// ErrMailerDisabled is returned by NoopMailer.Send when SMTP is not
// configured. Callers should treat it as "nothing was sent" and must not
// mark the notification as delivered, so a later SMTP configuration still
// flushes the backlog.
var ErrMailerDisabled = errors.New("mailer disabled")

// Mailer sends a plaintext email.
type Mailer interface {
	Send(to, subject, body string) error
}

// SMTPMailer delivers mail over SMTP using net/smtp.
type SMTPMailer struct {
	host string
	port int
	user string
	pass string
	from string
}

// NewSMTPMailer builds an SMTPMailer.
func NewSMTPMailer(host string, port int, user, pass, from string) *SMTPMailer {
	return &SMTPMailer{host: host, port: port, user: user, pass: pass, from: from}
}

// Send delivers a message to a single recipient. PLAIN auth is used when a
// username is configured, otherwise no auth is sent.
func (m *SMTPMailer) Send(to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", m.host, m.port)
	var auth smtp.Auth
	if m.user != "" {
		auth = smtp.PlainAuth("", m.user, m.pass, m.host)
	}
	msg := buildMessage(m.from, to, subject, body)
	return smtp.SendMail(addr, auth, m.from, []string{to}, msg)
}

// NoopMailer logs the message and does nothing else. Used when SMTP is
// unconfigured so dev/CI behavior is unchanged.
type NoopMailer struct{}

// Send logs the intended email and returns ErrMailerDisabled so callers know
// nothing was actually delivered and must not mark the notification sent.
func (m *NoopMailer) Send(to, subject, body string) error {
	slog.Info("email notification (SMTP disabled, not sent)", "to", to, "subject", subject)
	return ErrMailerDisabled
}

// NewFromConfig returns an SMTPMailer when SMTPHost is set, otherwise a
// NoopMailer.
func NewFromConfig(cfg config.Config) Mailer {
	if cfg.SMTPHost == "" {
		return &NoopMailer{}
	}
	return NewSMTPMailer(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom)
}

// headerSanitizer strips CR/LF so untrusted values (recipient, subject) can't
// inject additional email headers.
var headerSanitizer = strings.NewReplacer("\r", "", "\n", "")

// buildMessage builds a minimal RFC822 message with From/To/Subject headers.
func buildMessage(from, to, subject, body string) []byte {
	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"UTF-8\"\r\n\r\n%s\r\n",
		headerSanitizer.Replace(from),
		headerSanitizer.Replace(to),
		headerSanitizer.Replace(subject),
		body,
	)
	return []byte(msg)
}

package mailer

import (
	"errors"
	"strings"
	"testing"

	"github.com/it4nodummies/heureum/internal/config"
)

func TestNewFromConfigEmptyHostReturnsNoop(t *testing.T) {
	m := NewFromConfig(config.Config{})
	if _, ok := m.(*NoopMailer); !ok {
		t.Fatalf("NewFromConfig with empty host = %T, want *NoopMailer", m)
	}
	if err := m.Send("to@example.com", "subj", "body"); !errors.Is(err, ErrMailerDisabled) {
		t.Errorf("NoopMailer.Send() error = %v, want ErrMailerDisabled", err)
	}
}

func TestNewFromConfigWithHostReturnsSMTP(t *testing.T) {
	cfg := config.Config{
		SMTPHost: "smtp.example.com",
		SMTPPort: 2525,
		SMTPUser: "user",
		SMTPPass: "pass",
		SMTPFrom: "noreply@example.com",
	}
	m := NewFromConfig(cfg)
	sm, ok := m.(*SMTPMailer)
	if !ok {
		t.Fatalf("NewFromConfig with host = %T, want *SMTPMailer", m)
	}
	if sm.host != "smtp.example.com" || sm.port != 2525 || sm.user != "user" || sm.pass != "pass" || sm.from != "noreply@example.com" {
		t.Errorf("SMTPMailer fields mismatch: %+v", sm)
	}
}

func TestBuildMessageContainsHeaders(t *testing.T) {
	msg := string(buildMessage("from@example.com", "to@example.com", "Hello Subject", "Body text here"))
	for _, want := range []string{
		"From: from@example.com",
		"To: to@example.com",
		"Subject: Hello Subject",
		"Body text here",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("buildMessage() missing %q in:\n%s", want, msg)
		}
	}
}

func TestBuildMessageStripsHeaderInjection(t *testing.T) {
	msg := string(buildMessage(
		"from@example.com",
		"to@example.com\r\nBcc: attacker@evil.com",
		"Legit\r\nX-Injected: yes",
		"Body",
	))
	// Header block (before the empty line) must contain only the intended
	// header lines — no injected Bcc:/X-Injected: lines.
	headerBlock := strings.SplitN(msg, "\r\n\r\n", 2)[0]
	for _, line := range strings.Split(headerBlock, "\r\n") {
		if strings.HasPrefix(line, "Bcc:") || strings.HasPrefix(line, "X-Injected:") {
			t.Errorf("buildMessage() allowed header injection line %q in:\n%s", line, msg)
		}
	}
	// From, To, Subject, MIME-Version, Content-Type = 5 lines / 4 separators.
	if strings.Count(headerBlock, "\r\n") != 4 {
		t.Errorf("unexpected header line count in:\n%s", headerBlock)
	}
}

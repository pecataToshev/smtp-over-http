package sender

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"

	"github.com/pecataToshev/smtp-over-http/shared/email"
)

// Sender sends raw or simple emails via external SMTP.
type Sender struct {
	host     string
	port     string
	user     string
	pass     string
	from     string
	insecure bool
	auth     smtp.Auth
	addr     string
}

// New creates a Sender. host:port without scheme.
func New(host, port, user, pass, from string, insecure bool) *Sender {
	s := &Sender{
		host:     host,
		port:     port,
		user:     user,
		pass:     pass,
		from:     from,
		insecure: insecure,
		addr:     host + ":" + port,
	}
	if user != "" && pass != "" {
		s.auth = smtp.PlainAuth("", user, pass, host)
	}
	return s
}

// SendRaw sends the provided raw RFC5322 message bytes.
// The 'from' argument to smtp.SendMail must be the envelope sender (often the From header without display name).
func (s *Sender) SendRaw(raw []byte, envelopeFrom string, recipients []string) error {
	// If auth is required, we must use smtp.SendMail with auth.
	// For STARTTLS on 587 we rely on smtp.SendMail to handle it.
	// For SMTPS (465) we would need a different path; we assume STARTTLS (587) or plain (25) based on env.
	if s.auth != nil || !s.insecure {
		// Use smtp.SendMail; it will do STARTTLS if server advertises.
		// smtp.SendMail expects the raw message including headers.
		if err := smtp.SendMail(s.addr, s.auth, envelopeFrom, recipients, raw); err != nil {
			slog.Error("smtp send failed", "host", s.host, "port", s.port, "error", err)
			return err
		}
		return nil
	}
	// Insecure path (plain, no TLS). Rare.
	c, err := smtp.Dial(s.addr)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := c.Mail(envelopeFrom); err != nil {
		return err
	}
	for _, r := range recipients {
		if err := c.Rcpt(r); err != nil {
			return err
		}
	}
	wc, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := wc.Write(raw); err != nil {
		return err
	}
	if err := wc.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// SendFromMessage decodes the raw base64 payload from our Message and sends it.
// If Message.From is empty, we fall back to configured SMTP_FROM for envelope.
func (s *Sender) SendFromMessage(m email.Message) error {
	raw, err := base64.StdEncoding.DecodeString(m.Raw)
	if err != nil {
		return fmt.Errorf("decode raw: %w", err)
	}

	envFrom := extractAddr(m.From)
	if envFrom == "" {
		envFrom = extractAddr(s.from)
	}
	if envFrom == "" {
		return fmt.Errorf("no envelope from available")
	}

	toAddrs := make([]string, 0, len(m.To))
	for _, t := range m.To {
		if a := extractAddr(t); a != "" {
			toAddrs = append(toAddrs, a)
		}
	}
	if len(toAddrs) == 0 {
		return fmt.Errorf("no valid recipients")
	}

	return s.SendRaw(raw, envFrom, toAddrs)
}

func extractAddr(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// If "Name <addr@ex.com>" form
	if i := strings.LastIndex(s, "<"); i >= 0 {
		if j := strings.LastIndex(s, ">"); j > i {
			return strings.TrimSpace(s[i+1 : j])
		}
	}
	return s
}

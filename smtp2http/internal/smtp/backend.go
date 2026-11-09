package smtp

import (
	"bytes"
	"encoding/base64"
	"io"
	"log/slog"
	"time"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/pecataToshev/smtp-over-http/shared/email"
	cfg "github.com/pecataToshev/smtp-over-http/smtp2http/internal/config"
)

// Backend implements smtp.Backend.
type Backend struct {
	Users     []cfg.Credential
	ForwardFn func(msg email.Message) error
}

// NewSession creates a new SMTP session.
func (b *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	slog.Debug("new SMTP connection established", "remote_addr", c.Conn().RemoteAddr().String())
	return &session{
		backend: b,
	}, nil
}

// session implements smtp.Session and smtp.AuthSession.
type session struct {
	backend *Backend

	authed   bool
	username string
	from     string
	to       []string
	raw      bytes.Buffer
}

func (s *session) AuthMechanisms() []string {
	if len(s.backend.Users) == 0 {
		return nil
	}
	// Only LOGIN is supported
	return []string{sasl.Login}
}

func (s *session) Auth(mech string) (sasl.Server, error) {
	slog.Debug("SMTP client requested authentication", "mechanism", mech)
	if len(s.backend.Users) == 0 {
		slog.Warn("authentication attempted but no users are configured")
		return nil, smtp.ErrAuthUnsupported
	}
	if mech != sasl.Login {
		slog.Warn("unsupported authentication mechanism attempted", "mechanism", mech)
		return nil, smtp.ErrAuthUnsupported
	}
	return newLoginServer(func(username, password string) error {
		for _, c := range s.backend.Users {
			if username == c.User && password == c.Pass {
				s.authed = true
				s.username = username
				slog.Debug("SMTP client authenticated successfully", "user", username)
				return nil
			}
		}
		slog.Warn("SMTP authentication failed", "user", username)
		return smtp.ErrAuthFailed
	}), nil
}

func (s *session) Mail(from string, _ *smtp.MailOptions) error {
	if len(s.backend.Users) > 0 {
		if !s.authed {
			return smtp.ErrAuthRequired
		}
	}
	s.from = from
	s.raw.Reset()
	s.to = nil
	return nil
}

func (s *session) Rcpt(to string, _ *smtp.RcptOptions) error {
	s.to = append(s.to, to)
	return nil
}

func (s *session) Data(r io.Reader) error {
	start := time.Now()

	s.raw.Reset()
	if _, err := io.Copy(&s.raw, r); err != nil {
		slog.Error("failed to read SMTP data",
			"user", s.username,
			"elapsed_ms", time.Since(start).Milliseconds(),
			"error", err)
		return err
	}

	if s.from == "" || len(s.to) == 0 || s.raw.Len() == 0 {
		slog.Warn("skipping empty SMTP message forwarding",
			"user", s.username,
			"from", s.from,
			"to", s.to,
			"size_bytes", s.raw.Len())
		return nil
	}

	msg := email.Message{
		From: s.from,
		To:   s.to,
		Raw:  base64.StdEncoding.EncodeToString(s.raw.Bytes()),
	}

	slog.Debug("forwarding message to target",
		"user", s.username,
		"from", s.from,
		"to", s.to,
		"size_bytes", s.raw.Len())
	if err := s.backend.ForwardFn(msg); err != nil {
		slog.Error("failed to forward SMTP message",
			"user", s.username,
			"from", s.from,
			"to", s.to,
			"size_bytes", s.raw.Len(),
			"elapsed_ms", time.Since(start).Milliseconds(),
			"error", err)
		return err
	}

	slog.Info("SMTP message processed and forwarded successfully",
		"user", s.username,
		"from", s.from,
		"to", s.to,
		"size_bytes", s.raw.Len(),
		"elapsed_ms", time.Since(start).Milliseconds())
	return nil
}

func (s *session) Reset() {
	s.from = ""
	s.to = nil
	s.raw.Reset()
}

func (s *session) Logout() error {
	slog.Debug("SMTP client logged out / disconnected", "user", s.username)
	return nil
}

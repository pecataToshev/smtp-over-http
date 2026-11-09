package smtp

import (
	"log/slog"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/pecataToshev/smtp-over-http/shared/email"
	cfg "github.com/pecataToshev/smtp-over-http/smtp2http/internal/config"
	"github.com/pecataToshev/smtp-over-http/smtp2http/internal/forward"
)

// NewServer creates the SMTP server that forwards messages via fwd.
func NewServer(config cfg.Config, fwd *forward.Forwarder) *smtp.Server {
	be := &Backend{
		Users: config.Users,
		ForwardFn: func(msg email.Message) error {
			return fwd.Send(msg)
		},
	}

	s := smtp.NewServer(be)
	s.Addr = ":587"
	s.Domain = config.Domain
	s.ReadTimeout = 30 * time.Second
	s.WriteTimeout = 30 * time.Second
	s.MaxMessageBytes = 10 * 1024 * 1024
	s.MaxRecipients = 100
	s.AllowInsecureAuth = true

	// Enable STARTTLS by providing a self-signed TLS config
	if tlsCfg, err := generateTLSConfig(config.Domain, config.TLSDNSNames); err == nil {
		s.TLSConfig = tlsCfg
	} else {
		slog.Warn("failed to generate self-signed TLS config, STARTTLS will not be available", "error", err)
	}

	return s
}

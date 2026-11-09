package config

import (
	"fmt"
	"os"
	"time"
)

// Config for http2smtp.
type Config struct {
	Port         string
	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPass     string
	SMTPFrom     string
	SMTPInsecure bool
	APIKey       string
	HealthURL    string // optional healthchecks.io URL; we ping every 12h when set
}

func Load() (Config, error) {
	cfg := Config{
		Port:      os.Getenv("PORT"),
		SMTPHost:  os.Getenv("SMTP_HOST"),
		SMTPPort:  os.Getenv("SMTP_PORT"),
		SMTPUser:  os.Getenv("SMTP_USER"),
		SMTPPass:  os.Getenv("SMTP_PASS"),
		SMTPFrom:  os.Getenv("SMTP_FROM"),
		APIKey:    os.Getenv("API_KEY"),
		HealthURL: os.Getenv("HEALTHCHECK_URL"),
	}

	insecureStr := os.Getenv("SMTP_INSECURE")
	if insecureStr == "" {
		return cfg, fmt.Errorf("missing required environment variable: SMTP_INSECURE")
	}
	cfg.SMTPInsecure = insecureStr == "true" || insecureStr == "1"

	var missing []string
	for name, val := range map[string]string{
		"PORT":      cfg.Port,
		"SMTP_HOST": cfg.SMTPHost,
		"SMTP_PORT": cfg.SMTPPort,
		"SMTP_USER": cfg.SMTPUser,
		"SMTP_PASS": cfg.SMTPPass,
		"SMTP_FROM": cfg.SMTPFrom,
		"API_KEY":   cfg.APIKey,
	} {
		if val == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return cfg, fmt.Errorf("missing required environment variables: %v", missing)
	}

	return cfg, nil
}

// HealthPingInterval is the interval for the external healthchecks.io ping.
const HealthPingInterval = 1 * time.Hour

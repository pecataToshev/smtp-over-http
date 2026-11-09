package config

import (
	"fmt"
	"os"
	"strings"
)

// Credential is one allowed SMTP LOGIN credential.
type Credential struct {
	User string
	Pass string
}

// Config holds smtp2http runtime settings.
type Config struct {
	HealthAddr  string
	TargetURL   string
	APIKey      string
	Users       []Credential
	Domain      string
	TLSDNSNames []string
}

// Load reads environment and validates required fields.
// All listed env vars are required (no defaults).
func Load() (Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	cfg := Config{
		TargetURL:  os.Getenv("TARGET_URL"),
		APIKey:     os.Getenv("API_KEY"),
		HealthAddr: ":" + port,
		Domain:     os.Getenv("SMTP_DOMAIN"),
	}

	for _, name := range strings.Split(os.Getenv("SMTP_TLS_DNS_NAMES"), ",") {
		if name = strings.TrimSpace(name); name != "" {
			cfg.TLSDNSNames = append(cfg.TLSDNSNames, name)
		}
	}

	var users []Credential
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]
		if strings.HasPrefix(key, "SMTP_USERS_") {
			val = strings.TrimSpace(val)
			if val == "" {
				continue
			}
			idx := strings.Index(val, ":")
			if idx <= 0 || idx == len(val)-1 {
				return cfg, fmt.Errorf("invalid credential format in %s, expected username:password", key)
			}
			users = append(users, Credential{User: val[:idx], Pass: val[idx+1:]})
		}
	}
	cfg.Users = users

	var missing []string
	for name, val := range map[string]string{
		"TARGET_URL":  cfg.TargetURL,
		"API_KEY":     cfg.APIKey,
		"SMTP_DOMAIN": cfg.Domain,
	} {
		if val == "" {
			missing = append(missing, name)
		}
	}
	if len(cfg.Users) == 0 {
		missing = append(missing, "SMTP_USERS_<service>")
	}
	if len(missing) > 0 {
		return cfg, fmt.Errorf("missing required environment variables: %v", missing)
	}

	return cfg, nil
}

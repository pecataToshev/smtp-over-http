package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/pecataToshev/smtp-over-http/shared/buildinfo"
	"github.com/pecataToshev/smtp-over-http/smtp2http/internal/config"
	"github.com/pecataToshev/smtp-over-http/smtp2http/internal/forward"
	"github.com/pecataToshev/smtp-over-http/smtp2http/internal/server"
	"github.com/pecataToshev/smtp-over-http/smtp2http/internal/smtp"
)

func main() {
	level := slog.LevelInfo
	if os.Getenv("DEBUG") == "true" {
		level = slog.LevelDebug
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration error", "error", err)
		os.Exit(1)
	}

	var usernames []string
	for _, u := range cfg.Users {
		usernames = append(usernames, u.User)
	}

	slog.Info("smtp2http starting",
		"commit", buildinfo.Commit,
		"build_time", buildinfo.BuildTime,
		"source", buildinfo.Source,
		"allowed_users", usernames)

	fwd := forward.New(cfg.TargetURL, cfg.APIKey)

	s := smtp.NewServer(cfg, fwd)
	httpServer := server.NewHealthServer(cfg.HealthAddr)

	errCh := make(chan error, 2)

	go func() {
		slog.Info("smtp2http SMTP listening", "addr", ":587")
		if err := s.ListenAndServe(); err != nil && err != gosmtp.ErrServerClosed {
			errCh <- err
		}
	}()

	go func() {
		slog.Info("smtp2http health listening", "addr", cfg.HealthAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sig:
		slog.Info("shutting down")
	case err := <-errCh:
		slog.Error("server error", "error", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Close()
	_ = httpServer.Shutdown(ctx)
}

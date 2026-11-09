package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pecataToshev/smtp-over-http/http2smtp/internal/config"
	"github.com/pecataToshev/smtp-over-http/http2smtp/internal/sender"
	"github.com/pecataToshev/smtp-over-http/http2smtp/internal/server"
	"github.com/pecataToshev/smtp-over-http/shared/buildinfo"
)

func main() {
	// Text logs for the external service (plain text, human readable)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{})))

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	slog.Info("http2smtp starting",
		"commit", buildinfo.Commit,
		"build_time", buildinfo.BuildTime,
		"source", buildinfo.Source)

	snd := sender.New(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom, cfg.SMTPInsecure)

	srv := server.New(snd, cfg.APIKey)

	// Optional healthchecks.io ping every 12 hours (fire-and-forget)
	if cfg.HealthURL != "" {
		go func() {
			ping(cfg.HealthURL)
			t := time.NewTicker(config.HealthPingInterval)
			defer t.Stop()
			for range t.C {
				ping(cfg.HealthURL)
			}
		}()
	}

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("http2smtp listening", "port", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	slog.Info("shutting down")

	_ = httpServer.Close()
}

func ping(url string) {
	// fire and forget; ignore errors
	_, _ = http.Get(url)
}

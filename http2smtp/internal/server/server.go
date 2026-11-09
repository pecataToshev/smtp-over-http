package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"mime"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/pecataToshev/smtp-over-http/http2smtp/internal/sender"
	"github.com/pecataToshev/smtp-over-http/shared/auth"
	"github.com/pecataToshev/smtp-over-http/shared/buildinfo"
	"github.com/pecataToshev/smtp-over-http/shared/email"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// Server serves the HTTP API for http2smtp.
type Server struct {
	sender *sender.Sender
	apiKey string
	mux    *http.ServeMux
}

// New creates the server.
func New(snd *sender.Sender, apiKey string) *Server {
	s := &Server{
		sender: snd,
		apiKey: apiKey,
		mux:    http.NewServeMux(),
	}
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/send", s.handleSend)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
	s.mux.ServeHTTP(lrw, r)
	slog.Info("request handled",
		"method", r.Method,
		"path", r.URL.Path,
		"status", lrw.statusCode,
		"duration", time.Since(start),
	)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":     "ok",
		"commit":     buildinfo.Commit,
		"build_time": buildinfo.BuildTime,
		"source":     buildinfo.Source,
	})
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !auth.CheckAPIKey(r, s.apiKey) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var msg email.Message
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&msg); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// Minimal validation
	if len(msg.To) == 0 || msg.Raw == "" {
		http.Error(w, "missing to or raw", http.StatusBadRequest)
		return
	}

	subject := extractSubject(msg.Raw)
	slog.Info("sending email", "to", msg.To, "subject", subject)

	if err := s.sender.SendFromMessage(msg); err != nil {
		slog.Error("send failed", "error", err)
		http.Error(w, "send failed", http.StatusBadGateway)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("sent"))
}

func extractSubject(rawB64 string) string {
	raw, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil {
		return ""
	}

	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return ""
	}

	subject := msg.Header.Get("Subject")
	if decoded, err := (&mime.WordDecoder{}).DecodeHeader(subject); err == nil {
		subject = decoded
	}

	return strings.TrimSpace(subject)
}

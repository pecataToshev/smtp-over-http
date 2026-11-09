package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/pecataToshev/smtp-over-http/shared/buildinfo"
)

// NewHealthServer creates the HTTP health server.
func NewHealthServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":     "ok",
			"commit":     buildinfo.Commit,
			"build_time": buildinfo.BuildTime,
			"source":     buildinfo.Source,
		})
	})

	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

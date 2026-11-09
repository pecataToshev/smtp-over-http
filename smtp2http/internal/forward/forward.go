package forward

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/pecataToshev/smtp-over-http/shared/email"
)

// Forwarder posts a Message to the target URL with Bearer auth.
type Forwarder struct {
	target string
	apiKey string
	client *http.Client
}

// New creates a Forwarder.
func New(target, apiKey string) *Forwarder {
	return &Forwarder{
		target: target,
		apiKey: apiKey,
		client: &http.Client{},
	}
}

// Send marshals and POSTs the message. Returns error on non-2xx or network failure.
func (f *Forwarder) Send(msg email.Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		slog.Error("marshal email failed", "error", err)
		return err
	}

	req, err := http.NewRequest(http.MethodPost, f.target, bytes.NewReader(body))
	if err != nil {
		slog.Error("build request failed", "error", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+f.apiKey)

	resp, err := f.client.Do(req)
	if err != nil {
		slog.Error("post to target failed", "url", f.target, "error", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		slog.Error("target returned error", "status", resp.StatusCode, "body", strings.TrimSpace(string(slurp)))
		return errors.New("target error")
	}
	return nil
}

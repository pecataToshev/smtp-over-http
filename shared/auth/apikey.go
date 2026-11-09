package auth

import "net/http"

// CheckAPIKey returns true if the Authorization header is "Bearer <expected>".
func CheckAPIKey(r *http.Request, expected string) bool {
	if expected == "" {
		return false
	}
	got := r.Header.Get("Authorization")
	return got == "Bearer "+expected
}

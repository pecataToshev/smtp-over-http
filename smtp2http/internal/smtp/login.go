package smtp

import (
	"github.com/emersion/go-sasl"
)

// loginServer implements sasl.Server for the LOGIN mechanism (server side only).
// We advertise and accept only LOGIN (no PLAIN).
type loginServer struct {
	username     string
	authenticate func(username, password string) error
}

func (s *loginServer) Next(response []byte) (challenge []byte, done bool, err error) {
	if s.username == "" {
		if response == nil {
			// Standard first challenge for LOGIN.
			return []byte("Username:"), false, nil
		}
		// Client provided initial response (AUTH LOGIN <b64user>).
		s.username = string(response)
		return []byte("Password:"), false, nil
	}
	// Second response is the password.
	password := string(response)
	done = true
	err = s.authenticate(s.username, password)
	return nil, done, err
}

// newLoginServer creates a sasl.Server for LOGIN auth.
func newLoginServer(auth func(username, password string) error) sasl.Server {
	return &loginServer{authenticate: auth}
}

var _ sasl.Server = (*loginServer)(nil)

// Reference to ensure the symbol is linked if needed; we only use Login.
var _ = sasl.Login

package email

type Message struct {
	From string   `json:"from"`
	To   []string `json:"to"`
	Raw  string   `json:"raw"` // base64-encoded full RFC 5322 message
}

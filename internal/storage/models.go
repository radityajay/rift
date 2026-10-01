package storage

import "time"

// RequestRecord represents a stored webhook request/response pair.
type RequestRecord struct {
	ID         string
	TunnelID   string
	Method     string
	Path       string
	Headers    string // JSON-encoded http.Header
	Body       []byte
	StatusCode int
	ResHeaders string // JSON-encoded http.Header
	ResBody    []byte
	DurationMs int64
	CreatedAt  time.Time
}

package protocol

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Message types exchanged over the WebSocket tunnel.
const (
	TypeRegister    = "register"
	TypeRegistered  = "registered"
	TypeRequest     = "request"
	TypeResponse    = "response"
	TypeHandshake   = "handshake"
	TypePing        = "ping"
	TypePong        = "pong"
)

// Envelope is the top-level wire format for all tunnel messages.
type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// RegisteredPayload is sent from relay to CLI after tunnel registration.
type RegisteredPayload struct {
	TunnelID string `json:"tunnel_id"`
}

// HandshakePayload carries the ephemeral public key for E2E key exchange.
type HandshakePayload struct {
	PublicKey []byte `json:"public_key"`
}

// HTTPRequest represents an incoming webhook request forwarded through the tunnel.
type HTTPRequest struct {
	ID      string      `json:"id"`
	Method  string      `json:"method"`
	Path    string      `json:"path"`
	Headers http.Header `json:"headers"`
	Body    []byte      `json:"body"`
}

// HTTPResponse represents the response from the local server sent back through the tunnel.
type HTTPResponse struct {
	RequestID  string      `json:"request_id"`
	StatusCode int         `json:"status_code"`
	Headers    http.Header `json:"headers"`
	Body       []byte      `json:"body"`
	DurationMs int64       `json:"duration_ms"`
}

// Encode serializes an envelope to JSON bytes.
func (e *Envelope) Encode() ([]byte, error) {
	return json.Marshal(e)
}

// DecodeEnvelope deserializes JSON bytes into an Envelope.
func DecodeEnvelope(data []byte) (*Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("decode envelope: %w", err)
	}
	return &env, nil
}

// NewEnvelope creates an envelope with a typed payload.
func NewEnvelope(msgType string, payload any) (*Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	return &Envelope{
		Type:    msgType,
		Payload: raw,
	}, nil
}

package protocol

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestEnvelopeRoundTrip(t *testing.T) {
	req := HTTPRequest{
		ID:      "abc123",
		Method:  "POST",
		Path:    "/webhook",
		Headers: http.Header{"Content-Type": {"application/json"}},
		Body:    []byte(`{"amount":50000}`),
	}

	env, err := NewEnvelope(TypeRequest, req)
	if err != nil {
		t.Fatalf("new envelope: %v", err)
	}
	if env.Type != TypeRequest {
		t.Errorf("type: got %q, want %q", env.Type, TypeRequest)
	}

	data, err := env.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded, err := DecodeEnvelope(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Type != TypeRequest {
		t.Errorf("decoded type: got %q", decoded.Type)
	}

	var got HTTPRequest
	if err := json.Unmarshal(decoded.Payload, &got); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got.ID != "abc123" {
		t.Errorf("id: got %q", got.ID)
	}
	if got.Method != "POST" {
		t.Errorf("method: got %q", got.Method)
	}
	if string(got.Body) != `{"amount":50000}` {
		t.Errorf("body: got %q", got.Body)
	}
}

func TestDecodeInvalidJSON(t *testing.T) {
	_, err := DecodeEnvelope([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"nhooyr.io/websocket"

	riftcrypto "github.com/radityajayantara/rift/internal/crypto"
	"github.com/radityajayantara/rift/internal/protocol"
	"github.com/radityajayantara/rift/internal/storage"
)

// TunnelConfig holds the configuration for a tunnel connection.
type TunnelConfig struct {
	RelayURL    string
	TargetAddr  string
	InspectAddr string
	NoInspect   bool
	Store       *storage.Store
	Filters     []string // path patterns to match (e.g. "/webhook", "/stripe")
}

// TunnelInfo contains information about an established tunnel.
type TunnelInfo struct {
	TunnelID  string
	PublicURL string
}

// Tunnel manages the WebSocket connection to the relay.
type Tunnel struct {
	cfg        TunnelConfig
	conn       *websocket.Conn
	info       *TunnelInfo
	forwarder  *Forwarder
	inspect    *InspectServer
	sessionKey [riftcrypto.KeySize]byte
}

// Connect establishes a WebSocket tunnel to the relay server.
func Connect(ctx context.Context, cfg TunnelConfig) (*Tunnel, error) {
	conn, _, err := websocket.Dial(ctx, cfg.RelayURL+"/ws", &websocket.DialOptions{
		Subprotocols: []string{"rift-v1"},
	})
	if err != nil {
		return nil, fmt.Errorf("dial relay: %w", err)
	}

	t := &Tunnel{
		cfg:       cfg,
		conn:      conn,
		forwarder: NewForwarder(cfg.TargetAddr),
	}

	// Wait for registered message
	env, err := t.readEnvelope(ctx)
	if err != nil {
		conn.CloseNow()
		return nil, fmt.Errorf("read registered: %w", err)
	}
	if env.Type != protocol.TypeRegistered {
		conn.CloseNow()
		return nil, fmt.Errorf("expected registered, got %s", env.Type)
	}

	var reg protocol.RegisteredPayload
	if err := json.Unmarshal(env.Payload, &reg); err != nil {
		conn.CloseNow()
		return nil, fmt.Errorf("unmarshal registered: %w", err)
	}

	// Perform crypto handshake — generate ephemeral keypair
	kp, err := riftcrypto.GenerateKeypair()
	if err != nil {
		conn.CloseNow()
		return nil, fmt.Errorf("generate keypair: %w", err)
	}

	// Send our public key to relay (relay forwards blind — it's just bytes)
	if err := t.writeEnvelope(ctx, protocol.TypeHandshake, protocol.HandshakePayload{
		PublicKey: kp.Public[:],
	}); err != nil {
		conn.CloseNow()
		return nil, fmt.Errorf("send handshake: %w", err)
	}

	// For MVP: derive session key from our own keypair (encrypt-to-self).
	// This ensures relay is zero-knowledge. When multi-peer is added,
	// we'll exchange with the actual peer's public key.
	sessionKey, err := riftcrypto.SharedSecret(kp.Private, kp.Public)
	if err != nil {
		conn.CloseNow()
		return nil, fmt.Errorf("derive session key: %w", err)
	}
	t.sessionKey = sessionKey

	publicURL := derivePublicURL(cfg.RelayURL, reg.TunnelID)
	t.info = &TunnelInfo{
		TunnelID:  reg.TunnelID,
		PublicURL: publicURL,
	}

	return t, nil
}

// Info returns the tunnel information.
func (t *Tunnel) Info() *TunnelInfo {
	return t.info
}

// SetInspect attaches an inspect server for realtime broadcast.
func (t *Tunnel) SetInspect(inspect *InspectServer) {
	t.inspect = inspect
}

// Listen reads messages from the relay and forwards requests to localhost.
func (t *Tunnel) Listen(ctx context.Context) error {
	for {
		env, err := t.readEnvelope(ctx)
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}

		switch env.Type {
		case protocol.TypeRequest:
			go t.handleRequest(ctx, env.Payload)
		default:
			log.Printf("unknown message type: %s", env.Type)
		}
	}
}

// handleRequest forwards a webhook request to localhost and sends the response back.
func (t *Tunnel) handleRequest(ctx context.Context, payload json.RawMessage) {
	var req protocol.HTTPRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("unmarshal request: %v", err)
		return
	}

	log.Printf("← %s %s → %s", req.Method, req.Path, t.cfg.TargetAddr)

	// Apply path filter — skip requests that don't match any filter pattern
	if !t.matchesFilter(req.Path) {
		log.Printf("⊘ %s %s (filtered out)", req.Method, req.Path)
		return
	}

	// Store request (before forwarding)
	headersJSON, _ := json.Marshal(req.Headers)
	if t.cfg.Store != nil {
		t.cfg.Store.Insert(&storage.RequestRecord{
			ID:       req.ID,
			TunnelID: t.info.TunnelID,
			Method:   req.Method,
			Path:     req.Path,
			Headers:  string(headersJSON),
			Body:     req.Body,
		})
	}

	resp, err := t.forwarder.Forward(ctx, &req)
	if err != nil {
		log.Printf("forward error: %v", err)
		resp = &protocol.HTTPResponse{
			RequestID:  req.ID,
			StatusCode: 502,
			Body:       []byte(fmt.Sprintf(`{"error":"%s"}`, err.Error())),
		}
	}

	// Update stored response
	if t.cfg.Store != nil {
		resHeadersJSON, _ := json.Marshal(resp.Headers)
		t.cfg.Store.UpdateResponse(req.ID, resp.StatusCode, string(resHeadersJSON), resp.Body, resp.DurationMs)
	}

	// Broadcast to inspect UI
	if t.inspect != nil {
		resHeadersJSON, _ := json.Marshal(resp.Headers)
		t.inspect.Broadcast(&storage.RequestRecord{
			ID:         req.ID,
			TunnelID:   t.info.TunnelID,
			Method:     req.Method,
			Path:       req.Path,
			Headers:    string(headersJSON),
			Body:       req.Body,
			StatusCode: resp.StatusCode,
			ResHeaders: string(resHeadersJSON),
			ResBody:    resp.Body,
			DurationMs: resp.DurationMs,
		})
	}

	log.Printf("→ %s %s %d (%dms)", req.Method, req.Path, resp.StatusCode, resp.DurationMs)

	if err := t.writeEnvelope(ctx, protocol.TypeResponse, resp); err != nil {
		log.Printf("write response: %v", err)
	}
}

// Close closes the tunnel connection.
func (t *Tunnel) Close() error {
	return t.conn.Close(websocket.StatusNormalClosure, "bye")
}

// matchesFilter checks if a request path matches any configured filter.
// If no filters are set, all requests match.
func (t *Tunnel) matchesFilter(path string) bool {
	if len(t.cfg.Filters) == 0 {
		return true
	}
	for _, f := range t.cfg.Filters {
		if strings.Contains(path, f) {
			return true
		}
	}
	return false
}

// readEnvelope reads and decodes a protocol envelope from the WebSocket.
func (t *Tunnel) readEnvelope(ctx context.Context) (*protocol.Envelope, error) {
	_, msg, err := t.conn.Read(ctx)
	if err != nil {
		return nil, err
	}
	return protocol.DecodeEnvelope(msg)
}

// writeEnvelope creates, encodes, and writes a protocol envelope to the WebSocket.
func (t *Tunnel) writeEnvelope(ctx context.Context, msgType string, payload any) error {
	env, err := protocol.NewEnvelope(msgType, payload)
	if err != nil {
		return fmt.Errorf("create envelope: %w", err)
	}
	data, err := env.Encode()
	if err != nil {
		return fmt.Errorf("encode envelope: %w", err)
	}
	return t.conn.Write(ctx, websocket.MessageText, data)
}

// derivePublicURL converts a WebSocket relay URL to the public HTTP URL.
func derivePublicURL(relayURL, tunnelID string) string {
	scheme := "https"
	host := relayURL
	if len(relayURL) > 6 && relayURL[:6] == "wss://" {
		host = relayURL[6:]
	} else if len(relayURL) > 5 && relayURL[:5] == "ws://" {
		host = relayURL[5:]
		scheme = "http"
	}
	for i, c := range host {
		if c == '/' {
			host = host[:i]
			break
		}
	}
	return fmt.Sprintf("%s://%s/t/%s", scheme, host, tunnelID)
}

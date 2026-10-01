package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"nhooyr.io/websocket"

	"github.com/radityajayantara/rift/internal/protocol"
)

const (
	// webhookTimeout is how long to wait for CLI to respond to a forwarded webhook.
	webhookTimeout = 30 * time.Second
	maxBodySize    = 10 * 1024 * 1024 // 10MB
)

// Server is the relay HTTP server that manages tunnels and forwards webhooks.
type Server struct {
	registry *Registry
	mux      *http.ServeMux
}

// NewServer creates a new relay server.
func NewServer() *Server {
	s := &Server{
		registry: NewRegistry(),
		mux:      http.NewServeMux(),
	}
	s.mux.HandleFunc("/ws", s.handleWebSocket)
	s.mux.HandleFunc("/t/", s.handleWebhook)
	s.mux.HandleFunc("/health", s.handleHealth)
	return s
}

// Handler returns the HTTP handler for the relay server.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// handleWebSocket accepts a new tunnel connection from a CLI client.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols: []string{"rift-v1"},
	})
	if err != nil {
		log.Printf("websocket accept: %v", err)
		return
	}

	ctx := r.Context()
	tunnelID := uuid.New().String()[:8]
	tunnel := NewTunnel(tunnelID, conn, ctx)

	s.registry.Register(tunnel)
	defer s.registry.Unregister(tunnelID)
	defer conn.CloseNow()

	log.Printf("tunnel registered: %s (active: %d)", tunnelID, s.registry.Count())

	// Send registered message
	if err := s.sendEnvelope(ctx, conn, protocol.TypeRegistered, protocol.RegisteredPayload{
		TunnelID: tunnelID,
	}); err != nil {
		log.Printf("send registered: %v", err)
		return
	}

	// Read loop — receive responses from CLI and route to pending webhook handlers
	for {
		_, msg, err := conn.Read(ctx)
		if err != nil {
			log.Printf("tunnel %s closed: %v", tunnelID, err)
			return
		}

		env, err := protocol.DecodeEnvelope(msg)
		if err != nil {
			log.Printf("tunnel %s decode: %v", tunnelID, err)
			continue
		}

		switch env.Type {
		case protocol.TypeResponse:
			var resp protocol.HTTPResponse
			if err := json.Unmarshal(env.Payload, &resp); err != nil {
				log.Printf("tunnel %s unmarshal response: %v", tunnelID, err)
				continue
			}
			if !tunnel.Resolve(resp.RequestID, msg) {
				log.Printf("tunnel %s: no pending request for %s", tunnelID, resp.RequestID)
			}
		case protocol.TypeHandshake:
			// Relay is blind — ignore handshake payloads (E2E key exchange).
			// In multi-peer mode, relay would forward this to the other peer.
			log.Printf("tunnel %s: handshake received (relay blind, ignored)", tunnelID)
		default:
			log.Printf("tunnel %s: unexpected message type %s", tunnelID, env.Type)
		}
	}
}

// handleWebhook receives incoming webhook requests and forwards them to the tunnel.
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	tunnelID, webhookPath := parseTunnelPath(r.URL.Path)
	if tunnelID == "" {
		http.Error(w, `{"error":"invalid tunnel path"}`, http.StatusBadRequest)
		return
	}

	tunnel := s.registry.Get(tunnelID)
	if tunnel == nil {
		http.Error(w, `{"error":"tunnel not found"}`, http.StatusNotFound)
		return
	}

	body, err := readBody(r)
	if err != nil {
		http.Error(w, `{"error":"read body failed"}`, http.StatusBadRequest)
		return
	}

	reqID := uuid.New().String()[:8]
	httpReq := protocol.HTTPRequest{
		ID:      reqID,
		Method:  r.Method,
		Path:    webhookPath,
		Headers: r.Header.Clone(),
		Body:    body,
	}

	// Register pending request BEFORE sending to CLI
	respCh := tunnel.WaitFor(reqID)

	// Send request to CLI via WebSocket
	if err := s.sendEnvelope(tunnel.Ctx, tunnel.Conn, protocol.TypeRequest, httpReq); err != nil {
		http.Error(w, `{"error":"tunnel write failed"}`, http.StatusBadGateway)
		return
	}

	log.Printf("→ %s %s %s (req: %s)", tunnelID, r.Method, webhookPath, reqID)

	// Wait for response from CLI
	select {
	case respData, ok := <-respCh:
		if !ok {
			// Channel closed — tunnel disconnected
			http.Error(w, `{"error":"tunnel disconnected"}`, http.StatusBadGateway)
			return
		}
		s.writeHTTPResponse(w, respData)

	case <-time.After(webhookTimeout):
		http.Error(w, `{"error":"timeout waiting for response"}`, http.StatusGatewayTimeout)

	case <-r.Context().Done():
		// Webhook provider cancelled the request
		return
	}
}

// writeHTTPResponse extracts the HTTPResponse from the envelope and writes it as HTTP.
func (s *Server) writeHTTPResponse(w http.ResponseWriter, data []byte) {
	env, err := protocol.DecodeEnvelope(data)
	if err != nil {
		http.Error(w, `{"error":"decode response"}`, http.StatusInternalServerError)
		return
	}

	var resp protocol.HTTPResponse
	if err := json.Unmarshal(env.Payload, &resp); err != nil {
		http.Error(w, `{"error":"unmarshal response"}`, http.StatusInternalServerError)
		return
	}

	// Copy response headers
	for k, vals := range resp.Headers {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}

	w.WriteHeader(resp.StatusCode)
	if resp.Body != nil {
		w.Write(resp.Body)
	}
}

// handleHealth returns relay status.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","tunnels":%d}`, s.registry.Count())
}

// sendEnvelope creates and writes a protocol envelope to a WebSocket connection.
func (s *Server) sendEnvelope(ctx context.Context, conn *websocket.Conn, msgType string, payload any) error {
	env, err := protocol.NewEnvelope(msgType, payload)
	if err != nil {
		return fmt.Errorf("create envelope: %w", err)
	}
	data, err := env.Encode()
	if err != nil {
		return fmt.Errorf("encode envelope: %w", err)
	}
	return conn.Write(ctx, websocket.MessageText, data)
}

// parseTunnelPath extracts tunnel ID and webhook path from /t/<id>/optional/path.
func parseTunnelPath(path string) (tunnelID, webhookPath string) {
	if len(path) < 4 || path[:3] != "/t/" {
		return "", ""
	}
	sub := path[3:]
	webhookPath = "/"
	tunnelID = sub
	for i, c := range sub {
		if c == '/' {
			tunnelID = sub[:i]
			webhookPath = sub[i:]
			break
		}
	}
	if tunnelID == "" {
		return "", ""
	}
	return tunnelID, webhookPath
}

func readBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	defer r.Body.Close()
	return io.ReadAll(io.LimitReader(r.Body, maxBodySize))
}

// Serve starts the relay server. Blocks until context is cancelled.
func Serve(ctx context.Context, addr string, tlsCert, tlsKey string) error {
	srv := NewServer()
	httpSrv := &http.Server{
		Addr:    addr,
		Handler: srv.Handler(),
	}

	go func() {
		<-ctx.Done()
		httpSrv.Close()
	}()

	log.Printf("rift relay listening on %s", addr)
	if tlsCert != "" && tlsKey != "" {
		return httpSrv.ListenAndServeTLS(tlsCert, tlsKey)
	}
	return httpSrv.ListenAndServe()
}

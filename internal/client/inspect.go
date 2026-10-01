package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"nhooyr.io/websocket"

	"github.com/radityajayantara/rift/internal/storage"
	riftweb "github.com/radityajayantara/rift/web"
)

// InspectServer serves the local inspect web UI with API and realtime updates.
type InspectServer struct {
	addr  string
	store *storage.Store

	// subscribers for realtime updates
	subsMu sync.Mutex
	subs   map[*websocket.Conn]context.Context
}

// NewInspectServer creates a new inspect server.
func NewInspectServer(addr string, store *storage.Store) *InspectServer {
	return &InspectServer{
		addr:  addr,
		store: store,
		subs:  make(map[*websocket.Conn]context.Context),
	}
}

// Broadcast sends a request record to all connected browser WebSocket clients.
func (s *InspectServer) Broadcast(rec *storage.RequestRecord) {
	data, err := json.Marshal(rec)
	if err != nil {
		return
	}

	s.subsMu.Lock()
	defer s.subsMu.Unlock()

	for conn, ctx := range s.subs {
		if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
			conn.CloseNow()
			delete(s.subs, conn)
		}
	}
}

// Start serves the inspect UI. Blocks until context is cancelled.
func (s *InspectServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Static UI assets
	mux.Handle("/", http.FileServer(http.FS(riftweb.Assets)))

	// API endpoints
	mux.HandleFunc("/api/requests", s.handleListRequests)
	mux.HandleFunc("/api/requests/", s.handleGetRequest)

	// WebSocket for realtime updates
	mux.HandleFunc("/ws", s.handleWebSocket)

	srv := &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		// Close all subscriber connections
		s.subsMu.Lock()
		for conn := range s.subs {
			conn.CloseNow()
		}
		s.subsMu.Unlock()
		srv.Close()
	}()

	log.Printf("inspect UI: http://%s", s.addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("inspect server: %w", err)
	}
	return nil
}

// handleListRequests returns the most recent requests as JSON.
func (s *InspectServer) handleListRequests(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	records, err := s.store.List(100)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if records == nil {
		records = []*storage.RequestRecord{}
	}
	writeJSON(w, http.StatusOK, records)
}

// handleGetRequest returns a single request by ID.
func (s *InspectServer) handleGetRequest(w http.ResponseWriter, r *http.Request) {
	// Extract ID from /api/requests/<id>
	id := r.URL.Path[len("/api/requests/"):]
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing request ID"})
		return
	}

	if s.store == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	rec, err := s.store.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

// handleWebSocket upgrades to WebSocket for realtime request streaming.
func (s *InspectServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // local only
	})
	if err != nil {
		log.Printf("inspect ws accept: %v", err)
		return
	}

	ctx := r.Context()
	s.subsMu.Lock()
	s.subs[conn] = ctx
	s.subsMu.Unlock()

	// Keep connection alive until client disconnects
	for {
		_, _, err := conn.Read(ctx)
		if err != nil {
			break
		}
	}

	s.subsMu.Lock()
	delete(s.subs, conn)
	s.subsMu.Unlock()
	conn.CloseNow()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

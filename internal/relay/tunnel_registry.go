package relay

import (
	"context"
	"sync"

	"nhooyr.io/websocket"
)

// Tunnel represents an active WebSocket tunnel to a CLI client.
type Tunnel struct {
	ID   string
	Conn *websocket.Conn
	Ctx  context.Context

	// pending tracks in-flight webhook requests waiting for CLI response.
	// Key: request ID, Value: channel that receives the raw response envelope.
	pending   map[string]chan []byte
	pendingMu sync.Mutex
}

// NewTunnel creates a new tunnel instance.
func NewTunnel(id string, conn *websocket.Conn, ctx context.Context) *Tunnel {
	return &Tunnel{
		ID:      id,
		Conn:    conn,
		Ctx:     ctx,
		pending: make(map[string]chan []byte),
	}
}

// WaitFor registers a pending request and returns a channel for the response.
func (t *Tunnel) WaitFor(reqID string) chan []byte {
	ch := make(chan []byte, 1)
	t.pendingMu.Lock()
	t.pending[reqID] = ch
	t.pendingMu.Unlock()
	return ch
}

// Resolve delivers a response to a pending request.
func (t *Tunnel) Resolve(reqID string, data []byte) bool {
	t.pendingMu.Lock()
	ch, ok := t.pending[reqID]
	if ok {
		delete(t.pending, reqID)
	}
	t.pendingMu.Unlock()
	if ok {
		ch <- data
		return true
	}
	return false
}

// CancelAll closes all pending request channels (called on tunnel disconnect).
func (t *Tunnel) CancelAll() {
	t.pendingMu.Lock()
	defer t.pendingMu.Unlock()
	for id, ch := range t.pending {
		close(ch)
		delete(t.pending, id)
	}
}

// Registry tracks active tunnels in memory.
type Registry struct {
	mu      sync.RWMutex
	tunnels map[string]*Tunnel
}

// NewRegistry creates a new tunnel registry.
func NewRegistry() *Registry {
	return &Registry{
		tunnels: make(map[string]*Tunnel),
	}
}

// Register adds a tunnel to the registry.
func (r *Registry) Register(t *Tunnel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tunnels[t.ID] = t
}

// Unregister removes a tunnel from the registry and cancels pending requests.
func (r *Registry) Unregister(id string) {
	r.mu.Lock()
	t, ok := r.tunnels[id]
	delete(r.tunnels, id)
	r.mu.Unlock()
	if ok {
		t.CancelAll()
	}
}

// Get retrieves a tunnel by ID. Returns nil if not found.
func (r *Registry) Get(id string) *Tunnel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tunnels[id]
}

// Count returns the number of active tunnels.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tunnels)
}

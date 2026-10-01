package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/radityajayantara/rift/internal/client"
	"github.com/radityajayantara/rift/internal/relay"
	"github.com/radityajayantara/rift/internal/storage"
)

// TestInspectAPI verifies the inspect web UI API returns stored requests
// after webhooks flow through the tunnel.
func TestInspectAPI(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 1. Local webhook receiver
	localMux := http.NewServeMux()
	localMux.HandleFunc("/hook", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	})
	localLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	localSrv := &http.Server{Handler: localMux}
	go localSrv.Serve(localLn)
	defer localSrv.Close()

	// 2. Relay
	relaySrv := relay.NewServer()
	relayLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	relayHTTP := &http.Server{Handler: relaySrv.Handler()}
	go relayHTTP.Serve(relayLn)
	defer relayHTTP.Close()

	// 3. Storage
	store, err := storage.New(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// 4. Inspect server
	inspectLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	inspectAddr := inspectLn.Addr().String()
	inspectLn.Close() // free port, InspectServer will re-bind

	inspectSrv := client.NewInspectServer(inspectAddr, store)

	// 5. Tunnel
	tunnel, err := client.Connect(ctx, client.TunnelConfig{
		RelayURL:   "ws://" + relayLn.Addr().String(),
		TargetAddr: localLn.Addr().String(),
		NoInspect:  true,
		Store:      store,
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer tunnel.Close()
	tunnel.SetInspect(inspectSrv)

	go tunnel.Listen(ctx)
	go inspectSrv.Start(ctx)
	time.Sleep(200 * time.Millisecond)

	// 6. Send webhook
	info := tunnel.Info()
	webhookURL := fmt.Sprintf("http://%s/t/%s/hook", relayLn.Addr().String(), info.TunnelID)
	resp, err := http.Post(webhookURL, "application/json", strings.NewReader(`{"test":1}`))
	if err != nil {
		t.Fatalf("webhook: %v", err)
	}
	resp.Body.Close()
	time.Sleep(200 * time.Millisecond)

	// 7. Query inspect API
	apiResp, err := http.Get("http://" + inspectAddr + "/api/requests")
	if err != nil {
		t.Fatalf("inspect API: %v", err)
	}
	defer apiResp.Body.Close()
	body, _ := io.ReadAll(apiResp.Body)

	if apiResp.StatusCode != 200 {
		t.Fatalf("inspect API status: %d, body: %s", apiResp.StatusCode, body)
	}

	var records []storage.RequestRecord
	if err := json.Unmarshal(body, &records); err != nil {
		t.Fatalf("unmarshal: %v, body: %s", err, body)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	rec := records[0]
	if rec.Method != "POST" {
		t.Errorf("method: got %q, want POST", rec.Method)
	}
	if rec.Path != "/hook" {
		t.Errorf("path: got %q, want /hook", rec.Path)
	}
	if rec.StatusCode != 200 {
		t.Errorf("status: got %d, want 200", rec.StatusCode)
	}

	t.Logf("✓ inspect API returned: %s %s → %d", rec.Method, rec.Path, rec.StatusCode)
}

package integration

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/radityajayantara/rift/internal/client"
	"github.com/radityajayantara/rift/internal/relay"
)

// TestEndToEnd spins up a relay, a local HTTP server, and a CLI tunnel,
// then sends a webhook through the relay and verifies it reaches the local server.
func TestEndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 1. Start local "webhook receiver" HTTP server
	localReceived := make(chan string, 1)
	localMux := http.NewServeMux()
	localMux.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		localReceived <- string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"received":true}`)
	})
	localLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	localSrv := &http.Server{Handler: localMux}
	go localSrv.Serve(localLn)
	defer localSrv.Close()
	localAddr := localLn.Addr().String()
	t.Logf("local server: %s", localAddr)

	// 2. Start relay server
	relaySrv := relay.NewServer()
	relayLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	relayHTTP := &http.Server{Handler: relaySrv.Handler()}
	go relayHTTP.Serve(relayLn)
	defer relayHTTP.Close()
	relayAddr := relayLn.Addr().String()
	t.Logf("relay server: %s", relayAddr)

	// 3. Connect CLI tunnel
	tunnel, err := client.Connect(ctx, client.TunnelConfig{
		RelayURL:   "ws://" + relayAddr,
		TargetAddr: localAddr,
		NoInspect:  true,
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer tunnel.Close()

	info := tunnel.Info()
	t.Logf("tunnel ID: %s, public URL: %s", info.TunnelID, info.PublicURL)

	// Start listening in background
	listenErr := make(chan error, 1)
	go func() {
		listenErr <- tunnel.Listen(ctx)
	}()

	// Give tunnel a moment to be ready
	time.Sleep(100 * time.Millisecond)

	// 4. Send webhook via relay's public endpoint
	webhookURL := fmt.Sprintf("http://%s/t/%s/webhook", relayAddr, info.TunnelID)
	t.Logf("sending webhook to: %s", webhookURL)

	payload := `{"event":"payment.success","amount":50000}`
	resp, err := http.Post(webhookURL, "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("webhook POST: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	t.Logf("webhook response: %d %s", resp.StatusCode, respBody)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, respBody)
	}

	// 5. Verify local server received the webhook
	select {
	case body := <-localReceived:
		if body != payload {
			t.Fatalf("local received wrong body: %q", body)
		}
		t.Logf("✓ local server received: %s", body)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for local server to receive webhook")
	}

	// Verify response body
	if !strings.Contains(string(respBody), `"received":true`) {
		t.Fatalf("unexpected response body: %s", respBody)
	}
}

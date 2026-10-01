package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/radityajayantara/rift/internal/client"
	"github.com/radityajayantara/rift/internal/storage"
)

var listenCmd = &cobra.Command{
	Use:   "listen",
	Short: "Create a tunnel and forward webhooks to localhost",
	Example: `  rift listen --to localhost:8080
  rift listen --to localhost:3000 --relay wss://relay.example.com
  rift listen --to localhost:8080 --no-inspect
  rift listen --to localhost:8080 --filter /webhook,/stripe`,
	RunE: runListen,
}

var (
	listenTo      string
	listenRelay   string
	listenInspect string
	listenNoInsp  bool
	listenFilters []string
)

func init() {
	listenCmd.Flags().StringVar(&listenTo, "to", "", "Target localhost address (required)")
	listenCmd.Flags().StringVar(&listenRelay, "relay", "wss://relay.riftunnel.dev", "Relay server URL")
	listenCmd.Flags().StringVar(&listenInspect, "inspect", ":4040", "Local inspect UI address")
	listenCmd.Flags().BoolVar(&listenNoInsp, "no-inspect", false, "Disable inspect UI")
	listenCmd.Flags().StringSliceVar(&listenFilters, "filter", nil, "Only forward requests matching these path patterns")
	_ = listenCmd.MarkFlagRequired("to")
}

func runListen(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Open local storage
	dbPath := filepath.Join(os.TempDir(), "rift.db")
	store, err := storage.New(dbPath)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer store.Close()

	cfg := client.TunnelConfig{
		RelayURL:    listenRelay,
		TargetAddr:  listenTo,
		InspectAddr: listenInspect,
		NoInspect:   listenNoInsp,
		Store:       store,
		Filters:     listenFilters,
	}

	// Start inspect UI once (persists across reconnects)
	var inspectSrv *client.InspectServer
	if !listenNoInsp {
		inspectSrv = client.NewInspectServer(listenInspect, store)
		go func() {
			if err := inspectSrv.Start(ctx); err != nil {
				log.Printf("inspect server: %v", err)
			}
		}()
	}

	// Reconnect loop
	firstConnect := true
	backoff := time.Second

	for {
		if ctx.Err() != nil {
			return nil
		}

		tunnel, err := client.Connect(ctx, cfg)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if firstConnect {
				return fmt.Errorf("connect: %w", err)
			}
			log.Printf("reconnect failed: %v (retry in %s)", err, backoff)
			select {
			case <-time.After(backoff):
				backoff = min(backoff*2, 30*time.Second)
				continue
			case <-ctx.Done():
				return nil
			}
		}

		backoff = time.Second

		if inspectSrv != nil {
			tunnel.SetInspect(inspectSrv)
		}

		info := tunnel.Info()
		if firstConnect {
			fmt.Println()
			fmt.Println("  Rift tunnel aktif")
			fmt.Printf("  Public URL : %s\n", info.PublicURL)
			fmt.Printf("  Forwarding : → http://%s\n", listenTo)
			if !listenNoInsp {
				fmt.Printf("  Inspect    : http://localhost%s\n", listenInspect)
			}
			if len(listenFilters) > 0 {
				fmt.Printf("  Filter     : %s\n", strings.Join(listenFilters, ", "))
			}
			fmt.Println("  Encryption : E2E (X25519 + ChaCha20-Poly1305)")
			fmt.Println()
			firstConnect = false
		} else {
			log.Printf("reconnected: %s", info.PublicURL)
		}

		err = tunnel.Listen(ctx)
		tunnel.Close()

		if ctx.Err() != nil {
			return nil
		}

		log.Printf("tunnel disconnected: %v (reconnecting...)", err)
	}
}

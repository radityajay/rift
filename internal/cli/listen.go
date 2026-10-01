package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/radityajayantara/rift/internal/client"
	"github.com/radityajayantara/rift/internal/storage"
)

var listenCmd = &cobra.Command{
	Use:   "listen",
	Short: "Create a tunnel and forward webhooks to localhost",
	Example: `  rift listen --to localhost:8080
  rift listen --to localhost:3000 --relay wss://relay.example.com
  rift listen --to localhost:8080 --no-inspect`,
	RunE: runListen,
}

var (
	listenTo      string
	listenRelay   string
	listenInspect string
	listenNoInsp  bool
)

func init() {
	listenCmd.Flags().StringVar(&listenTo, "to", "", "Target localhost address (required)")
	listenCmd.Flags().StringVar(&listenRelay, "relay", "wss://relay.riftunnel.dev", "Relay server URL")
	listenCmd.Flags().StringVar(&listenInspect, "inspect", ":4040", "Local inspect UI address")
	listenCmd.Flags().BoolVar(&listenNoInsp, "no-inspect", false, "Disable inspect UI")
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

	// Connect to relay
	tunnel, err := client.Connect(ctx, client.TunnelConfig{
		RelayURL:    listenRelay,
		TargetAddr:  listenTo,
		InspectAddr: listenInspect,
		NoInspect:   listenNoInsp,
		Store:       store,
	})
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer tunnel.Close()

	info := tunnel.Info()
	fmt.Println()
	fmt.Println("  Rift tunnel aktif")
	fmt.Printf("  Public URL : %s\n", info.PublicURL)
	fmt.Printf("  Forwarding : → http://%s\n", listenTo)
	if !listenNoInsp {
		fmt.Printf("  Inspect    : http://localhost%s\n", listenInspect)
	}
	fmt.Println("  Encryption : E2E (X25519 + ChaCha20-Poly1305)")
	fmt.Println()

	// Start inspect UI
	if !listenNoInsp {
		inspectSrv := client.NewInspectServer(listenInspect, store)
		tunnel.SetInspect(inspectSrv)
		go func() {
			if err := inspectSrv.Start(ctx); err != nil {
				log.Printf("inspect server: %v", err)
			}
		}()
	}

	// Listen for incoming webhooks
	return tunnel.Listen(ctx)
}

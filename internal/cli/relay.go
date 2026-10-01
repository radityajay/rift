package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/radityajayantara/rift/internal/relay"
)

var relayCmd = &cobra.Command{
	Use:   "relay",
	Short: "Run a self-hosted relay server",
	Example: `  rift relay
  rift relay --addr :8443
  rift relay --addr :443 --tls-cert cert.pem --tls-key key.pem`,
	RunE: runRelay,
}

var (
	relayAddr    string
	relayTLSCert string
	relayTLSKey  string
)

func init() {
	relayCmd.Flags().StringVar(&relayAddr, "addr", ":8443", "Listen address")
	relayCmd.Flags().StringVar(&relayTLSCert, "tls-cert", "", "TLS certificate file")
	relayCmd.Flags().StringVar(&relayTLSKey, "tls-key", "", "TLS private key file")
}

func runRelay(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	return relay.Serve(ctx, relayAddr, relayTLSCert, relayTLSKey)
}

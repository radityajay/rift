package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "rift",
	Short: "Privacy-first webhook tunnel",
	Long:  "Rift creates an encrypted tunnel between your localhost and a public relay server to receive webhooks privately.",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(listenCmd)
	rootCmd.AddCommand(relayCmd)
	rootCmd.AddCommand(replayCmd)
}

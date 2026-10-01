package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/radityajayantara/rift/internal/storage"
)

var replayCmd = &cobra.Command{
	Use:   "replay <request-id>",
	Short: "Replay a stored webhook request to localhost",
	Args:  cobra.ExactArgs(1),
	Example: `  rift replay a1b2c3d4
  rift replay a1b2c3d4 --to localhost:3000`,
	RunE: runReplay,
}

var replayTo string

func init() {
	replayCmd.Flags().StringVar(&replayTo, "to", "localhost:8080", "Target localhost address")
}

func runReplay(cmd *cobra.Command, args []string) error {
	reqID := args[0]

	dbPath := filepath.Join(os.TempDir(), "rift.db")
	store, err := storage.New(dbPath)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer store.Close()

	rec, err := store.Get(reqID)
	if err != nil {
		return fmt.Errorf("request %s not found: %w", reqID, err)
	}

	// Reconstruct and send HTTP request
	url := fmt.Sprintf("http://%s%s", replayTo, rec.Path)
	httpReq, err := http.NewRequest(rec.Method, url, bytes.NewReader(rec.Body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// Restore original headers
	var headers http.Header
	if err := json.Unmarshal([]byte(rec.Headers), &headers); err == nil {
		httpReq.Header = headers
	}

	client := &http.Client{Timeout: 30 * time.Second}
	start := time.Now()
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("replay to %s: %w", replayTo, err)
	}
	defer resp.Body.Close()
	duration := time.Since(start)

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))

	fmt.Printf("Replayed %s %s → %s\n", rec.Method, rec.Path, replayTo)
	fmt.Printf("Status: %d (%s)\n", resp.StatusCode, duration.Round(time.Millisecond))
	if len(body) > 0 {
		fmt.Printf("Body: %s\n", body)
	}

	return nil
}

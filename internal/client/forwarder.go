package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/radityajayantara/rift/internal/protocol"
)

// Forwarder sends requests to the local target server.
type Forwarder struct {
	targetAddr string
	client     *http.Client
}

// NewForwarder creates a new forwarder for the given local address.
func NewForwarder(targetAddr string) *Forwarder {
	return &Forwarder{
		targetAddr: targetAddr,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Forward sends an HTTP request to the local target and returns the response.
func (f *Forwarder) Forward(ctx context.Context, req *protocol.HTTPRequest) (*protocol.HTTPResponse, error) {
	url := fmt.Sprintf("http://%s%s", f.targetAddr, req.Path)

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, url, bytes.NewReader(req.Body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Copy headers from the original request
	for k, v := range req.Headers {
		for _, val := range v {
			httpReq.Header.Add(k, val)
		}
	}
	// Override Host header to target
	httpReq.Host = f.targetAddr

	start := time.Now()
	resp, err := f.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("forward to %s: %w", f.targetAddr, err)
	}
	defer resp.Body.Close()
	duration := time.Since(start)

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	return &protocol.HTTPResponse{
		RequestID:  req.ID,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header.Clone(),
		Body:       body,
		DurationMs: duration.Milliseconds(),
	}, nil
}

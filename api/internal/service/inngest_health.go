package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// InngestHealth checks the configured self-hosted runtime through its authenticated
// ingress. Cloud mode has no configured runtime to probe.
func InngestHealth(ctx context.Context) error {
	base := InngestBaseURLFromEnv()
	if base == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	client := NewInngestHTTPClient(3 * time.Second)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("inngest health HTTP status %d", resp.StatusCode)
	}
	var health struct {
		Status int `json:"status"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&health); err != nil {
		return fmt.Errorf("invalid inngest health response: %w", err)
	}
	if health.Status != http.StatusOK {
		return fmt.Errorf("inngest health status %d", health.Status)
	}
	return nil
}

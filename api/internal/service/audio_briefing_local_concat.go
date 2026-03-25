package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type LocalAudioConcatClient struct {
	http    *http.Client
	baseURL string
}

func NewLocalAudioConcatClient() *LocalAudioConcatClient {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("AUDIO_BRIEFING_CONCAT_LOCAL_URL")), "/")
	if baseURL == "" {
		baseURL = defaultAudioBriefingLocalConcatURL
	}
	return &LocalAudioConcatClient{
		http:    &http.Client{Timeout: 15 * time.Second},
		baseURL: baseURL,
	}
}

func (c *LocalAudioConcatClient) Enabled() bool {
	return c != nil && c.baseURL != ""
}

func (c *LocalAudioConcatClient) RunAudioConcat(ctx context.Context, req AudioConcatRunRequest) (*AudioConcatRunResponse, error) {
	if !c.Enabled() {
		return nil, ErrAudioConcatRunnerDisabled
	}
	rawBody, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/run", bytes.NewReader(rawBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("local audio concat run failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var payload AudioConcatRunResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if strings.TrimSpace(payload.ExecutionName) == "" {
		return nil, fmt.Errorf("local audio concat response missing execution_name")
	}
	return &payload, nil
}

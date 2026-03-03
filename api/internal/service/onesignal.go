package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type OneSignalClient struct {
	appID  string
	apiKey string
	base   string
	http   *http.Client
}

func NewOneSignalClient() *OneSignalClient {
	appID := strings.TrimSpace(os.Getenv("ONESIGNAL_APP_ID"))
	if appID == "" {
		appID = strings.TrimSpace(os.Getenv("NEXT_PUBLIC_ONESIGNAL_APP_ID"))
	}
	return &OneSignalClient{
		appID:  appID,
		apiKey: strings.TrimSpace(os.Getenv("ONESIGNAL_REST_API_KEY")),
		base:   "https://api.onesignal.com",
		http:   &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *OneSignalClient) Enabled() bool {
	return c != nil && c.appID != "" && c.apiKey != ""
}

type OneSignalSendResult struct {
	ID         string `json:"id"`
	Recipients int    `json:"recipients"`
}

func (c *OneSignalClient) SendToExternalID(ctx context.Context, externalID, title, body, targetURL string, data map[string]any) (*OneSignalSendResult, error) {
	if !c.Enabled() {
		return nil, nil
	}
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return nil, fmt.Errorf("external_id is required")
	}
	if strings.TrimSpace(title) == "" || strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("title/body are required")
	}
	payload := map[string]any{
		"app_id": c.appID,
		"include_aliases": map[string]any{
			"external_id": []string{externalID},
		},
		"target_channel": "push",
		"headings": map[string]string{
			"en": title,
			"ja": title,
		},
		"contents": map[string]string{
			"en": body,
			"ja": body,
		},
	}
	if targetURL = strings.TrimSpace(targetURL); targetURL != "" {
		payload["url"] = targetURL
	}
	if len(data) > 0 {
		payload["data"] = data
	}
	reqBody, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/notifications", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Key "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var res struct {
		ID         string `json:"id"`
		Recipients int    `json:"recipients"`
		Errors     any    `json:"errors"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("onesignal: status=%d id=%s errors=%v", resp.StatusCode, res.ID, res.Errors)
	}
	return &OneSignalSendResult{ID: res.ID, Recipients: res.Recipients}, nil
}

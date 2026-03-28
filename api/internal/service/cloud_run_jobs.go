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

type CloudRunJobsClient struct {
	http        *http.Client
	tokenSource *GoogleWIFTokenSource
	projectID   string
	region      string
	jobName     string
}

type CloudRunJobEnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func NewCloudRunJobsClient() *CloudRunJobsClient {
	return &CloudRunJobsClient{
		http:        &http.Client{Timeout: 30 * time.Second},
		tokenSource: NewGoogleWIFTokenSource(),
		projectID:   strings.TrimSpace(os.Getenv("AUDIO_BRIEFING_CLOUD_RUN_PROJECT")),
		region:      strings.TrimSpace(os.Getenv("AUDIO_BRIEFING_CLOUD_RUN_REGION")),
		jobName:     strings.TrimSpace(os.Getenv("AUDIO_BRIEFING_CLOUD_RUN_JOB")),
	}
}

func (c *CloudRunJobsClient) Enabled() bool {
	return c != nil &&
		c.projectID != "" &&
		c.region != "" &&
		c.jobName != "" &&
		c.tokenSource != nil &&
		c.tokenSource.Enabled()
}

func (c *CloudRunJobsClient) RunAudioConcat(ctx context.Context, req AudioConcatRunRequest) (*AudioConcatRunResponse, error) {
	if !c.Enabled() {
		return nil, ErrAudioConcatRunnerDisabled
	}
	accessToken, err := c.tokenSource.AccessToken(ctx)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("https://run.googleapis.com/v2/projects/%s/locations/%s/jobs/%s:run", c.projectID, c.region, c.jobName)
	body := map[string]any{
		"overrides": map[string]any{
			"containerOverrides": []map[string]any{{
				"env": []CloudRunJobEnvVar{
					{Name: "AUDIO_BRIEFING_JOB_ID", Value: req.JobID},
					{Name: "AUDIO_BRIEFING_USER_ID", Value: req.UserID},
					{Name: "AUDIO_BRIEFING_REQUEST_ID", Value: req.RequestID},
					{Name: "AUDIO_BRIEFING_CALLBACK_URL", Value: req.CallbackURL},
					{Name: "AUDIO_BRIEFING_CALLBACK_TOKEN", Value: req.CallbackToken},
					{Name: "AUDIO_BRIEFING_AUDIO_OBJECT_KEYS_JSON", Value: marshalAudioObjectKeysForEnv(req.AudioObjectKeys)},
					{Name: "AUDIO_BRIEFING_OUTPUT_OBJECT_KEY", Value: req.OutputObjectKey},
					{Name: "AUDIO_BRIEFING_BGM_ENABLED", Value: boolEnvValue(req.BGMEnabled)},
					{Name: "AUDIO_BRIEFING_BGM_R2_PREFIX", Value: req.BGMR2Prefix},
				},
			}},
		},
	}
	rawBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(rawBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("cloud run jobs execute failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if strings.TrimSpace(payload.Name) == "" {
		return nil, fmt.Errorf("cloud run jobs execute response missing name")
	}
	return &AudioConcatRunResponse{ExecutionName: payload.Name}, nil
}

func marshalAudioObjectKeysForEnv(keys []string) string {
	raw, err := json.Marshal(keys)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func boolEnvValue(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

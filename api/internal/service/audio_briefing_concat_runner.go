package service

import (
	"context"
	"fmt"
	"os"
	"strings"
)

var ErrAudioConcatRunnerDisabled = fmt.Errorf("audio concat runner disabled")

const (
	audioBriefingConcatModeCloudRun = "cloud_run"
	audioBriefingConcatModeLocal    = "local"

	defaultAudioBriefingLocalConcatURL       = "http://audio-concat-local:8080"
	defaultAudioBriefingLocalCallbackBaseURL = "http://api:8080"
)

type AudioConcatRunRequest struct {
	JobID           string   `json:"job_id"`
	UserID          string   `json:"user_id"`
	RequestID       string   `json:"request_id"`
	CallbackURL     string   `json:"callback_url"`
	CallbackToken   string   `json:"callback_token"`
	AudioObjectKeys []string `json:"audio_object_keys"`
	OutputObjectKey string   `json:"output_object_key"`
	BGMEnabled      bool     `json:"bgm_enabled"`
	BGMR2Prefix     string   `json:"bgm_r2_prefix"`
}

type AudioConcatRunResponse struct {
	ExecutionName string `json:"execution_name"`
}

type AudioConcatRunner interface {
	Enabled() bool
	RunAudioConcat(ctx context.Context, req AudioConcatRunRequest) (*AudioConcatRunResponse, error)
}

func NewAudioConcatRunnerFromEnv() AudioConcatRunner {
	switch AudioBriefingConcatModeFromEnv() {
	case audioBriefingConcatModeLocal:
		return NewLocalAudioConcatClient()
	case "", audioBriefingConcatModeCloudRun:
		return NewCloudRunJobsClient()
	default:
		return nil
	}
}

func AudioBriefingConcatModeFromEnv() string {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("AUDIO_BRIEFING_CONCAT_MODE")))
	if mode == "" {
		return audioBriefingConcatModeCloudRun
	}
	return mode
}

func audioBriefingCallbackBaseURL(mode string) string {
	if mode == audioBriefingConcatModeLocal {
		if value := strings.TrimRight(strings.TrimSpace(os.Getenv("AUDIO_BRIEFING_LOCAL_CALLBACK_BASE_URL")), "/"); value != "" {
			return value
		}
		return defaultAudioBriefingLocalCallbackBaseURL
	}
	return strings.TrimRight(strings.TrimSpace(AppBaseURLFromEnv()), "/")
}

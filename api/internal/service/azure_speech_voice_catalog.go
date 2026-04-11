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

type AzureSpeechVoiceCatalogService struct {
	http *http.Client
}

type AzureSpeechVoiceCatalogEntry struct {
	VoiceID     string   `json:"voice_id"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Locale      string   `json:"locale"`
	Gender      string   `json:"gender"`
	LocalName   string   `json:"local_name"`
	Styles      []string `json:"styles,omitempty"`
}

type AzureSpeechVoicesResponse struct {
	Provider string                         `json:"provider"`
	Source   string                         `json:"source"`
	Region   string                         `json:"region"`
	Voices   []AzureSpeechVoiceCatalogEntry `json:"voices"`
}

type azureSpeechVoicePayload struct {
	ShortName   string   `json:"ShortName"`
	LocalName   string   `json:"LocalName"`
	DisplayName string   `json:"DisplayName"`
	Locale      string   `json:"Locale"`
	Gender      string   `json:"Gender"`
	StyleList   []string `json:"StyleList"`
}

func NewAzureSpeechVoiceCatalogService() *AzureSpeechVoiceCatalogService {
	return &AzureSpeechVoiceCatalogService{
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

func (s *AzureSpeechVoiceCatalogService) FetchVoices(ctx context.Context, apiKey, region string) (*AzureSpeechVoicesResponse, error) {
	if s == nil || s.http == nil {
		return nil, fmt.Errorf("azure speech voice catalog service is not configured")
	}
	normalizedAPIKey := strings.TrimSpace(apiKey)
	normalizedRegion := strings.TrimSpace(region)
	if normalizedAPIKey == "" {
		return nil, fmt.Errorf("azure speech api key is required")
	}
	if normalizedRegion == "" {
		return nil, fmt.Errorf("azure speech region is required")
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("https://%s.tts.speech.microsoft.com/cognitiveservices/voices/list", normalizedRegion),
		nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", normalizedAPIKey)

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("azure speech voices api status %d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload []azureSpeechVoicePayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	voices := make([]AzureSpeechVoiceCatalogEntry, 0, len(payload))
	for _, voice := range payload {
		if strings.TrimSpace(voice.Locale) != "ja-JP" {
			continue
		}
		styles := make([]string, 0, len(voice.StyleList))
		for _, style := range voice.StyleList {
			style = strings.TrimSpace(style)
			if style != "" {
				styles = append(styles, style)
			}
		}
		label := strings.TrimSpace(voice.LocalName)
		if label == "" {
			label = strings.TrimSpace(voice.DisplayName)
		}
		if label == "" {
			label = strings.TrimSpace(voice.ShortName)
		}
		descriptionParts := []string{"ja-JP"}
		if gender := strings.TrimSpace(voice.Gender); gender != "" {
			descriptionParts = append(descriptionParts, gender)
		}
		if len(styles) > 0 {
			descriptionParts = append(descriptionParts, strings.Join(styles, ", "))
		}
		voices = append(voices, AzureSpeechVoiceCatalogEntry{
			VoiceID:     strings.TrimSpace(voice.ShortName),
			Label:       label,
			Description: strings.Join(descriptionParts, " / "),
			Locale:      "ja-JP",
			Gender:      strings.TrimSpace(voice.Gender),
			LocalName:   strings.TrimSpace(voice.LocalName),
			Styles:      styles,
		})
	}
	return &AzureSpeechVoicesResponse{
		Provider: "azure_speech",
		Source:   "azure_speech_voices_ja",
		Region:   normalizedRegion,
		Voices:   voices,
	}, nil
}

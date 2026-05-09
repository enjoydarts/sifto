package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const cartesiaAPIVersion = "2026-03-01"

type CartesiaTTSCatalogService struct {
	baseURL string
	http    *http.Client
}

type CartesiaTTSModelCatalogEntry struct {
	ModelID     string `json:"model_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type CartesiaVoiceCatalogEntry struct {
	VoiceID     string         `json:"voice_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Language    string         `json:"language"`
	PreviewURL  string         `json:"preview_url"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type CartesiaTTSCatalogResponse struct {
	Provider string                         `json:"provider"`
	Source   string                         `json:"source"`
	Models   []CartesiaTTSModelCatalogEntry `json:"models"`
	Voices   []CartesiaVoiceCatalogEntry    `json:"voices"`
}

type CartesiaVoicePreviewAudio struct {
	Bytes       []byte
	ContentType string
}

type cartesiaVoicesPayload struct {
	Data     []cartesiaRemoteVoice `json:"data"`
	HasMore  bool                  `json:"has_more"`
	NextPage string                `json:"next_page"`
}

type cartesiaRemoteVoice struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	Language       string         `json:"language"`
	PreviewFileURL string         `json:"preview_file_url"`
	IsOwner        bool           `json:"is_owner"`
	IsPublic       bool           `json:"is_public"`
	Metadata       map[string]any `json:"metadata"`
}

func NewCartesiaTTSCatalogService() *CartesiaTTSCatalogService {
	baseURL := strings.TrimSpace(os.Getenv("CARTESIA_API_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://api.cartesia.ai"
	}
	return &CartesiaTTSCatalogService{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 20 * time.Second},
	}
}

func (s *CartesiaTTSCatalogService) FetchCatalog(ctx context.Context, apiKey string) (*CartesiaTTSCatalogResponse, error) {
	if s == nil || s.http == nil {
		return nil, fmt.Errorf("cartesia tts catalog service is not configured")
	}
	normalizedAPIKey := strings.TrimSpace(apiKey)
	if normalizedAPIKey == "" {
		return nil, fmt.Errorf("cartesia api key is required")
	}

	voices, err := s.fetchJapaneseVoices(ctx, normalizedAPIKey)
	if err != nil {
		return nil, err
	}
	return &CartesiaTTSCatalogResponse{
		Provider: "cartesia",
		Source:   "cartesia_api_voices_ja",
		Models:   cartesiaTTSModelCatalog(),
		Voices:   voices,
	}, nil
}

func (s *CartesiaTTSCatalogService) fetchJapaneseVoices(ctx context.Context, apiKey string) ([]CartesiaVoiceCatalogEntry, error) {
	voices := make([]CartesiaVoiceCatalogEntry, 0, 64)
	seen := make(map[string]struct{})
	startingAfter := ""
	for {
		payload, err := s.fetchVoicesPage(ctx, apiKey, startingAfter)
		if err != nil {
			return nil, err
		}
		for _, voice := range payload.Data {
			entry := cartesiaVoiceToCatalogEntry(voice)
			if entry.VoiceID == "" {
				continue
			}
			if entry.PreviewURL == "" {
				if detailed, detailErr := s.fetchVoiceDetail(ctx, apiKey, entry.VoiceID); detailErr == nil && detailed != nil {
					detailedEntry := cartesiaVoiceToCatalogEntry(*detailed)
					if detailedEntry.Name != "" {
						entry.Name = detailedEntry.Name
					}
					if detailedEntry.Description != "" {
						entry.Description = detailedEntry.Description
					}
					if detailedEntry.Language != "" {
						entry.Language = detailedEntry.Language
					}
					entry.PreviewURL = detailedEntry.PreviewURL
				}
			}
			if _, exists := seen[entry.VoiceID]; exists {
				continue
			}
			seen[entry.VoiceID] = struct{}{}
			voices = append(voices, entry)
		}
		if !payload.HasMore || strings.TrimSpace(payload.NextPage) == "" {
			break
		}
		startingAfter = strings.TrimSpace(payload.NextPage)
	}
	return voices, nil
}

func (s *CartesiaTTSCatalogService) FetchVoicePreview(ctx context.Context, apiKey string, voiceID string) (*CartesiaVoicePreviewAudio, error) {
	if s == nil || s.http == nil {
		return nil, fmt.Errorf("cartesia tts catalog service is not configured")
	}
	normalizedAPIKey := strings.TrimSpace(apiKey)
	if normalizedAPIKey == "" {
		return nil, fmt.Errorf("cartesia api key is required")
	}
	normalizedVoiceID := strings.TrimSpace(voiceID)
	if normalizedVoiceID == "" {
		return nil, fmt.Errorf("cartesia voice id is required")
	}

	detail, err := s.fetchVoiceDetail(ctx, normalizedAPIKey, normalizedVoiceID)
	if err != nil {
		return nil, err
	}
	previewURL := strings.TrimSpace(detail.PreviewFileURL)
	if previewURL == "" {
		return nil, fmt.Errorf("cartesia voice preview is not available")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, previewURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+normalizedAPIKey)
	req.Header.Set("Cartesia-Version", cartesiaAPIVersion)

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("cartesia voice preview status %d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	audioBytes, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return nil, err
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "audio/mpeg"
	}
	return &CartesiaVoicePreviewAudio{Bytes: audioBytes, ContentType: contentType}, nil
}

func (s *CartesiaTTSCatalogService) fetchVoiceDetail(ctx context.Context, apiKey string, voiceID string) (*cartesiaRemoteVoice, error) {
	u, err := url.Parse(s.baseURL + "/voices/" + url.PathEscape(voiceID))
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("expand[]", "preview_file_url")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Cartesia-Version", cartesiaAPIVersion)

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("cartesia voice detail api status %d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload cartesiaRemoteVoice
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (s *CartesiaTTSCatalogService) fetchVoicesPage(ctx context.Context, apiKey string, startingAfter string) (*cartesiaVoicesPayload, error) {
	u, err := url.Parse(s.baseURL + "/voices")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("limit", "100")
	q.Set("language", "ja")
	if strings.TrimSpace(startingAfter) != "" {
		q.Set("starting_after", strings.TrimSpace(startingAfter))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Cartesia-Version", cartesiaAPIVersion)

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("cartesia voices api status %d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload cartesiaVoicesPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func cartesiaTTSModelCatalog() []CartesiaTTSModelCatalogEntry {
	return []CartesiaTTSModelCatalogEntry{
		{
			ModelID:     "sonic-3.5",
			Name:        "Sonic 3.5",
			Description: "Recommended stable Cartesia Sonic TTS model with broad Japanese support.",
		},
		{
			ModelID:     "sonic-3",
			Name:        "Sonic 3",
			Description: "Stable Sonic TTS model for speed, volume, and emotion controls.",
		},
		{
			ModelID:     "sonic-turbo",
			Name:        "Sonic Turbo",
			Description: "Stable low-latency Sonic model for Japanese and other supported languages.",
		},
	}
}

func cartesiaVoiceToCatalogEntry(voice cartesiaRemoteVoice) CartesiaVoiceCatalogEntry {
	return CartesiaVoiceCatalogEntry{
		VoiceID:     strings.TrimSpace(voice.ID),
		Name:        strings.TrimSpace(voice.Name),
		Description: strings.TrimSpace(voice.Description),
		Language:    strings.TrimSpace(voice.Language),
		PreviewURL:  strings.TrimSpace(voice.PreviewFileURL),
		Metadata:    voice.Metadata,
	}
}

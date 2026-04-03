package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

type XAIVoiceCatalogService struct {
	baseURL string
	http    *http.Client
}

func NewXAIVoiceCatalogService() *XAIVoiceCatalogService {
	baseURL := strings.TrimSpace(os.Getenv("XAI_API_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://api.x.ai"
	}
	return NewXAIVoiceCatalogServiceWithBaseURL(baseURL)
}

func NewXAIVoiceCatalogServiceWithBaseURL(baseURL string) *XAIVoiceCatalogService {
	return &XAIVoiceCatalogService{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		http:    &http.Client{Timeout: 20 * time.Second},
	}
}

type xaiVoiceCatalogResponse struct {
	Voices []xaiRemoteVoice `json:"voices"`
}

type xaiRemoteVoice struct {
	VoiceID     string         `json:"voice_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Language    string         `json:"language"`
	PreviewURL  string         `json:"preview_url"`
	Raw         map[string]any `json:"-"`
}

func (v *xaiRemoteVoice) UnmarshalJSON(data []byte) error {
	type alias xaiRemoteVoice
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var out alias
	if err := json.Unmarshal(data, &out); err != nil {
		return err
	}
	*v = xaiRemoteVoice(out)
	v.Raw = decoded
	return nil
}

func (s *XAIVoiceCatalogService) FetchVoices(ctx context.Context, apiKey string) ([]repository.XAIVoiceSnapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/v1/tts/voices", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("xai voices api status %d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload xaiVoiceCatalogResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	fetchedAt := time.Now().UTC()
	rows := make([]repository.XAIVoiceSnapshot, 0, len(payload.Voices))
	for _, voice := range payload.Voices {
		metadata := normalizeXAIVoiceMetadata(voice.Raw)
		rows = append(rows, repository.XAIVoiceSnapshot{
			VoiceID:      strings.TrimSpace(voice.VoiceID),
			Name:         strings.TrimSpace(voice.Name),
			Description:  strings.TrimSpace(voice.Description),
			Language:     strings.TrimSpace(voice.Language),
			PreviewURL:   strings.TrimSpace(voice.PreviewURL),
			MetadataJSON: metadata,
			FetchedAt:    fetchedAt,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].VoiceID == rows[j].VoiceID {
			return rows[i].Name < rows[j].Name
		}
		return rows[i].VoiceID < rows[j].VoiceID
	})
	return rows, nil
}

func normalizeXAIVoiceMetadata(raw map[string]any) []byte {
	if len(raw) == 0 {
		return nil
	}
	filtered := make(map[string]any, len(raw))
	for key, value := range raw {
		switch key {
		case "voice_id", "name", "description", "language", "preview_url":
			continue
		default:
			filtered[key] = value
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	metadata, err := json.Marshal(filtered)
	if err != nil {
		return nil
	}
	return metadata
}

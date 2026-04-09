package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type ElevenLabsVoiceCatalogService struct {
	baseURL string
	http    *http.Client
}

type ElevenLabsVoiceCatalogEntry struct {
	VoiceID     string         `json:"voice_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	PreviewURL  string         `json:"preview_url"`
	Labels      map[string]any `json:"labels,omitempty"`
	Languages   []string       `json:"languages,omitempty"`
}

type ElevenLabsVoicesResponse struct {
	Provider string                        `json:"provider"`
	Source   string                        `json:"source"`
	Voices   []ElevenLabsVoiceCatalogEntry `json:"voices"`
}

type elevenLabsVoicesPayload struct {
	Voices     []elevenLabsRemoteVoice `json:"voices"`
	HasMore    bool                    `json:"has_more"`
	LastSortID *string                 `json:"last_sort_id"`
}

type elevenLabsRemoteVoice struct {
	VoiceID           string                           `json:"voice_id"`
	Name              string                           `json:"name"`
	Description       string                           `json:"description"`
	Category          string                           `json:"category"`
	PreviewURL        string                           `json:"preview_url"`
	Labels            map[string]any                   `json:"labels"`
	Language          string                           `json:"language"`
	Locale            string                           `json:"locale"`
	VerifiedLanguages []elevenLabsVerifiedLanguageHint `json:"verified_languages"`
}

type elevenLabsVerifiedLanguageHint struct {
	Language   string `json:"language"`
	Locale     string `json:"locale"`
	PreviewURL string `json:"preview_url"`
}

func NewElevenLabsVoiceCatalogService() *ElevenLabsVoiceCatalogService {
	baseURL := strings.TrimSpace(os.Getenv("ELEVENLABS_API_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://api.elevenlabs.io"
	}
	return &ElevenLabsVoiceCatalogService{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 20 * time.Second},
	}
}

func (s *ElevenLabsVoiceCatalogService) FetchVoices(ctx context.Context, apiKey string) (*ElevenLabsVoicesResponse, error) {
	if s == nil || s.http == nil {
		return nil, fmt.Errorf("elevenlabs voice catalog service is not configured")
	}
	normalizedAPIKey := strings.TrimSpace(apiKey)
	if normalizedAPIKey == "" {
		return nil, fmt.Errorf("elevenlabs api key is required")
	}

	seen := make(map[string]struct{})
	voices := make([]ElevenLabsVoiceCatalogEntry, 0, 64)
	page := 0
	for {
		payload, err := s.fetchPage(ctx, normalizedAPIKey, page)
		if err != nil {
			return nil, err
		}
		for _, voice := range payload.Voices {
			entry := elevenLabsVoiceToCatalogEntry(voice)
			if entry.VoiceID == "" {
				continue
			}
			if !entrySupportsJapanese(entry) {
				continue
			}
			if _, exists := seen[entry.VoiceID]; exists {
				continue
			}
			seen[entry.VoiceID] = struct{}{}
			voices = append(voices, entry)
		}
		if !payload.HasMore {
			break
		}
		page++
	}
	return &ElevenLabsVoicesResponse{
		Provider: "elevenlabs",
		Source:   "elevenlabs_shared_voices_ja",
		Voices:   voices,
	}, nil
}

func (s *ElevenLabsVoiceCatalogService) fetchPage(ctx context.Context, apiKey string, page int) (*elevenLabsVoicesPayload, error) {
	u, err := url.Parse(s.baseURL + "/v1/shared-voices")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("page_size", "100")
	q.Set("page", strconv.Itoa(page))
	q.Set("language", "ja")
	q.Set("locale", "ja-JP")
	q.Set("sort", "trending")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("xi-api-key", apiKey)

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("elevenlabs voices api status %d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload elevenLabsVoicesPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func elevenLabsVoiceToCatalogEntry(voice elevenLabsRemoteVoice) ElevenLabsVoiceCatalogEntry {
	languages := make([]string, 0, len(voice.VerifiedLanguages))
	previewURL := preferredJapanesePreviewURL(voice)
	if locale := strings.TrimSpace(voice.Locale); locale != "" && isJapaneseHint(locale) {
		languages = append(languages, locale)
	}
	if language := strings.TrimSpace(voice.Language); language != "" && isJapaneseHint(language) && !containsFoldedString(languages, language) {
		languages = append(languages, language)
	}
	for _, hint := range voice.VerifiedLanguages {
		candidate := strings.TrimSpace(hint.Locale)
		if candidate == "" {
			candidate = strings.TrimSpace(hint.Language)
		}
		if candidate == "" || !isJapaneseHint(candidate) {
			continue
		}
		if !containsFoldedString(languages, candidate) {
			languages = append(languages, candidate)
		}
	}
	return ElevenLabsVoiceCatalogEntry{
		VoiceID:     strings.TrimSpace(voice.VoiceID),
		Name:        strings.TrimSpace(voice.Name),
		Description: strings.TrimSpace(voice.Description),
		Category:    strings.TrimSpace(voice.Category),
		PreviewURL:  previewURL,
		Labels:      voice.Labels,
		Languages:   languages,
	}
}

func preferredJapanesePreviewURL(voice elevenLabsRemoteVoice) string {
	var jaPreview string
	for _, hint := range voice.VerifiedLanguages {
		if isExactJapaneseLocale(hint.Locale) && strings.TrimSpace(hint.PreviewURL) != "" {
			return strings.TrimSpace(hint.PreviewURL)
		}
		if isJapaneseHint(hint.Locale) || isJapaneseHint(hint.Language) {
			if preview := strings.TrimSpace(hint.PreviewURL); preview != "" && jaPreview == "" {
				jaPreview = preview
			}
		}
	}
	if jaPreview != "" {
		return jaPreview
	}
	return strings.TrimSpace(voice.PreviewURL)
}

func entrySupportsJapanese(entry ElevenLabsVoiceCatalogEntry) bool {
	for _, language := range entry.Languages {
		if isJapaneseHint(language) {
			return true
		}
	}
	for key, value := range entry.Labels {
		if !labelKeyMayDescribeLanguage(key) {
			continue
		}
		if isJapaneseHint(fmt.Sprint(value)) {
			return true
		}
	}
	return false
}

func labelKeyMayDescribeLanguage(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	return normalized == "language" || normalized == "locale" || normalized == "accent"
}

func isJapaneseHint(v string) bool {
	normalized := strings.ToLower(strings.TrimSpace(v))
	if normalized == "" {
		return false
	}
	return normalized == "ja" ||
		normalized == "ja-jp" ||
		normalized == "japanese" ||
		strings.HasPrefix(normalized, "ja-") ||
		strings.Contains(v, "日本語")
}

func isExactJapaneseLocale(v string) bool {
	return strings.EqualFold(strings.TrimSpace(v), "ja-JP")
}

func containsFoldedString(values []string, target string) bool {
	normalizedTarget := strings.ToLower(strings.TrimSpace(target))
	for _, value := range values {
		if strings.ToLower(strings.TrimSpace(value)) == normalizedTarget {
			return true
		}
	}
	return false
}

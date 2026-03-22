package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

type MeilisearchService struct {
	baseURL    string
	masterKey  string
	itemsIndex string
	client     *http.Client
}

func NewMeilisearchServiceFromEnv() (*MeilisearchService, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("MEILISEARCH_URL")), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("MEILISEARCH_URL is required")
	}

	itemsIndex := strings.TrimSpace(os.Getenv("MEILISEARCH_ITEMS_INDEX"))
	if itemsIndex == "" {
		itemsIndex = "items"
	}

	return &MeilisearchService{
		baseURL:    baseURL,
		masterKey:  strings.TrimSpace(os.Getenv("MEILISEARCH_MASTER_KEY")),
		itemsIndex: itemsIndex,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (s *MeilisearchService) Enabled() bool {
	return s != nil && s.baseURL != ""
}

func (s *MeilisearchService) ItemsIndexName() string {
	if s == nil {
		return ""
	}
	return s.itemsIndex
}

func (s *MeilisearchService) Health(ctx context.Context) error {
	if !s.Enabled() {
		return fmt.Errorf("meilisearch not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	if s.masterKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.masterKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("meilisearch health status=%d", resp.StatusCode)
	}
	return nil
}

func (s *MeilisearchService) ensureItemsIndex(ctx context.Context) error {
	if !s.Enabled() {
		return fmt.Errorf("meilisearch not configured")
	}

	body := map[string]any{
		"uid":        s.itemsIndex,
		"primaryKey": "id",
	}
	if err := s.doJSON(ctx, http.MethodPost, "/indexes", body, nil, true); err != nil {
		return err
	}

	settings := map[string]any{
		"searchableAttributes": []string{
			"title",
			"translated_title",
			"summary",
			"facts_text",
			"content_text",
			"topics",
		},
		"filterableAttributes": []string{
			"user_id",
			"source_id",
			"status",
			"is_deleted",
			"is_read",
			"is_favorite",
			"is_later",
			"topics",
		},
		"sortableAttributes": []string{
			"published_at",
			"created_at",
		},
	}
	return s.doJSON(ctx, http.MethodPatch, "/indexes/"+s.itemsIndex+"/settings", settings, nil, false)
}

func (s *MeilisearchService) UpsertItemDocuments(ctx context.Context, docs []model.ItemSearchDocument) error {
	if len(docs) == 0 {
		return nil
	}
	if err := s.ensureItemsIndex(ctx); err != nil {
		return err
	}
	return s.doJSON(ctx, http.MethodPost, "/indexes/"+s.itemsIndex+"/documents", docs, nil, false)
}

func (s *MeilisearchService) DeleteItemDocuments(ctx context.Context, itemIDs []string) error {
	if len(itemIDs) == 0 {
		return nil
	}
	return s.doJSON(ctx, http.MethodPost, "/indexes/"+s.itemsIndex+"/documents/delete-batch", itemIDs, nil, false)
}

type MeilisearchSearchParams struct {
	Query       string
	SearchMode  string
	Filters     []string
	Offset      int
	Limit       int
	CropLength  int
}

type MeilisearchSearchHit struct {
	ItemID         string
	SearchSnippets []model.ItemSearchSnippet
}

type MeilisearchSearchResult struct {
	Hits       []MeilisearchSearchHit
	Total      int
	Mode       string
}

func NormalizeSearchMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "and":
		return "and"
	case "or":
		return "or"
	default:
		return "natural"
	}
}

func (s *MeilisearchService) SearchItems(ctx context.Context, params MeilisearchSearchParams) (*MeilisearchSearchResult, error) {
	if err := s.ensureItemsIndex(ctx); err != nil {
		return nil, err
	}
	mode := NormalizeSearchMode(params.SearchMode)
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.CropLength <= 0 {
		params.CropLength = 18
	}

	payload := map[string]any{
		"q": params.Query,
		"filter": params.Filters,
		"limit": params.Limit,
		"offset": params.Offset,
		"attributesToHighlight": []string{
			"title",
			"translated_title",
			"summary",
			"facts_text",
			"content_text",
		},
		"attributesToCrop": []string{
			"summary",
			"facts_text",
			"content_text",
		},
		"cropLength": params.CropLength,
		"highlightPreTag": "<mark>",
		"highlightPostTag": "</mark>",
		"showMatchesPosition": true,
	}
	if mode == "and" {
		payload["matchingStrategy"] = "all"
	} else {
		payload["matchingStrategy"] = "last"
	}

	var raw struct {
		Hits []struct {
			ID         string         `json:"id"`
			Formatted  map[string]any `json:"_formatted"`
			RawTitle   string         `json:"title"`
			RawSummary string         `json:"summary"`
		} `json:"hits"`
		EstimatedTotalHits int `json:"estimatedTotalHits"`
		TotalHits          int `json:"totalHits"`
	}
	if err := s.doJSON(ctx, http.MethodPost, "/indexes/"+s.itemsIndex+"/search", payload, &raw, false); err != nil {
		return nil, err
	}

	result := &MeilisearchSearchResult{
		Hits:  make([]MeilisearchSearchHit, 0, len(raw.Hits)),
		Total: raw.EstimatedTotalHits,
		Mode:  mode,
	}
	if result.Total == 0 {
		result.Total = raw.TotalHits
	}
	for _, hit := range raw.Hits {
		snippets := buildSearchSnippets(hit.Formatted)
		result.Hits = append(result.Hits, MeilisearchSearchHit{
			ItemID:         hit.ID,
			SearchSnippets: snippets,
		})
	}
	return result, nil
}

func buildSearchSnippets(formatted map[string]any) []model.ItemSearchSnippet {
	fieldOrder := []struct {
		Input string
		Label string
	}{
		{Input: "translated_title", Label: "title"},
		{Input: "title", Label: "title"},
		{Input: "summary", Label: "summary"},
		{Input: "facts_text", Label: "facts"},
		{Input: "content_text", Label: "content"},
	}

	var snippets []model.ItemSearchSnippet
	seen := map[string]struct{}{}
	for _, field := range fieldOrder {
		raw, ok := formatted[field.Input].(string)
		if !ok {
			continue
		}
		raw = strings.TrimSpace(raw)
		if raw == "" || !strings.Contains(raw, "<mark>") {
			continue
		}
		key := field.Label + ":" + raw
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		snippets = append(snippets, model.ItemSearchSnippet{
			Field:       field.Label,
			SnippetHTML: raw,
		})
		if len(snippets) >= 3 {
			break
		}
	}
	return snippets
}

func (s *MeilisearchService) doJSON(ctx context.Context, method, path string, body any, out any, allowConflict bool) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = strings.NewReader(string(b))
	}

	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.masterKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.masterKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if allowConflict && resp.StatusCode == http.StatusConflict {
		return nil
	}
	if resp.StatusCode >= http.StatusBadRequest {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("meilisearch %s %s status=%d body=%s", method, path, resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}
	if out == nil {
		io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func QuoteMeilisearchFilter(value string) string {
	return strconv.Quote(strings.TrimSpace(value))
}

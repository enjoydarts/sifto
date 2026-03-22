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
	"unicode"
	"unicode/utf8"

	"github.com/enjoydarts/sifto/api/internal/model"
)

type MeilisearchService struct {
	baseURL          string
	masterKey        string
	itemsIndex       string
	suggestionsIndex string
	client           *http.Client
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
	suggestionsIndex := strings.TrimSpace(os.Getenv("MEILISEARCH_SUGGESTIONS_INDEX"))
	if suggestionsIndex == "" {
		suggestionsIndex = "search_suggestions"
	}

	return &MeilisearchService{
		baseURL:          baseURL,
		masterKey:        strings.TrimSpace(os.Getenv("MEILISEARCH_MASTER_KEY")),
		itemsIndex:       itemsIndex,
		suggestionsIndex: suggestionsIndex,
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

func (s *MeilisearchService) SuggestionsIndexName() string {
	if s == nil {
		return ""
	}
	return s.suggestionsIndex
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
	return s.ensureIndex(ctx, s.itemsIndex, settings)
}

func (s *MeilisearchService) ensureSuggestionsIndex(ctx context.Context) error {
	settings := map[string]any{
		"searchableAttributes": []string{
			"label",
			"normalized",
		},
		"filterableAttributes": []string{
			"user_id",
			"kind",
			"source_id",
			"topic",
		},
		"sortableAttributes": []string{
			"score",
			"article_count",
			"updated_at",
		},
	}
	return s.ensureIndex(ctx, s.suggestionsIndex, settings)
}

func (s *MeilisearchService) ensureIndex(ctx context.Context, indexName string, settings map[string]any) error {
	if !s.Enabled() {
		return fmt.Errorf("meilisearch not configured")
	}

	body := map[string]any{
		"uid":        indexName,
		"primaryKey": "id",
	}
	if err := s.doJSON(ctx, http.MethodPost, "/indexes", body, nil, true); err != nil {
		return err
	}
	return s.doJSON(ctx, http.MethodPatch, "/indexes/"+indexName+"/settings", settings, nil, false)
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

func (s *MeilisearchService) UpsertSearchSuggestionDocuments(ctx context.Context, docs []model.SearchSuggestionDocument) error {
	if len(docs) == 0 {
		return nil
	}
	if err := s.ensureSuggestionsIndex(ctx); err != nil {
		return err
	}
	return s.doJSON(ctx, http.MethodPost, "/indexes/"+s.suggestionsIndex+"/documents", docs, nil, false)
}

func (s *MeilisearchService) DeleteSearchSuggestionDocuments(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return s.doJSON(ctx, http.MethodPost, "/indexes/"+s.suggestionsIndex+"/documents/delete-batch", ids, nil, false)
}

func (s *MeilisearchService) DeleteSearchSuggestionDocumentsByFilter(ctx context.Context, filter string) error {
	if !s.Enabled() || strings.TrimSpace(filter) == "" {
		return nil
	}
	body := map[string]any{
		"filter": strings.TrimSpace(filter),
	}
	return s.doJSON(ctx, http.MethodPost, "/indexes/"+s.suggestionsIndex+"/documents/delete", body, nil, false)
}

type MeilisearchSearchParams struct {
	Query      string
	SearchMode string
	Filters    []string
	Offset     int
	Limit      int
	CropLength int
}

type MeilisearchSearchHit struct {
	ItemID         string
	SearchSnippets []model.ItemSearchSnippet
}

type MeilisearchSearchResult struct {
	Hits  []MeilisearchSearchHit
	Total int
	Mode  string
}

type meilisearchSearchPage struct {
	Hits        []MeilisearchSearchHit
	Total       int
	Mode        string
	RawHitCount int
}

type MeilisearchSuggestionParams struct {
	Query   string
	UserID  string
	Limit   int
	Filters []string
}

type MeilisearchSuggestionHit struct {
	ID           string
	Kind         string
	Label        string
	ItemID       *string
	SourceID     *string
	Topic        *string
	ArticleCount *int
	Score        int
}

type MeilisearchSuggestionResult struct {
	Hits []MeilisearchSuggestionHit
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
	if requiresContiguousJapaneseMatch(params.Query) {
		return s.searchItemsStrict(ctx, params)
	}
	page, err := s.searchItemsPage(ctx, params)
	if err != nil {
		return nil, err
	}
	return &MeilisearchSearchResult{
		Hits:  page.Hits,
		Total: page.Total,
		Mode:  page.Mode,
	}, nil
}

func (s *MeilisearchService) searchItemsStrict(ctx context.Context, params MeilisearchSearchParams) (*MeilisearchSearchResult, error) {
	pageSize := params.Limit
	if pageSize <= 0 {
		pageSize = 20
	}
	filteredHits := make([]MeilisearchSearchHit, 0, pageSize)
	rawOffset := 0
	rawTotal := 0
	mode := NormalizeSearchMode(params.SearchMode)
	batchSize := strictSearchBatchSize(pageSize)

	for {
		batchParams := params
		batchParams.Offset = rawOffset
		batchParams.Limit = batchSize

		page, err := s.searchItemsPage(ctx, batchParams)
		if err != nil {
			return nil, err
		}
		mode = page.Mode
		rawTotal = page.Total
		filteredHits = append(filteredHits, page.Hits...)

		if page.RawHitCount == 0 || rawOffset+page.RawHitCount >= rawTotal {
			break
		}
		rawOffset += page.RawHitCount
	}

	offset := params.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > len(filteredHits) {
		offset = len(filteredHits)
	}
	end := offset + pageSize
	if end > len(filteredHits) {
		end = len(filteredHits)
	}

	return &MeilisearchSearchResult{
		Hits:  filteredHits[offset:end],
		Total: len(filteredHits),
		Mode:  mode,
	}, nil
}

func strictSearchBatchSize(pageSize int) int {
	if pageSize <= 0 {
		return 50
	}
	size := pageSize * 4
	if size < 50 {
		size = 50
	}
	if size > 200 {
		size = 200
	}
	return size
}

func (s *MeilisearchService) searchItemsPage(ctx context.Context, params MeilisearchSearchParams) (*meilisearchSearchPage, error) {
	mode := NormalizeSearchMode(params.SearchMode)
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.CropLength <= 0 {
		params.CropLength = 18
	}

	payload := map[string]any{
		"q":      params.Query,
		"filter": params.Filters,
		"limit":  params.Limit,
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
		"cropLength":          params.CropLength,
		"highlightPreTag":     "<mark>",
		"highlightPostTag":    "</mark>",
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

	result := &meilisearchSearchPage{
		Hits:        make([]MeilisearchSearchHit, 0, len(raw.Hits)),
		Total:       raw.EstimatedTotalHits,
		Mode:        mode,
		RawHitCount: len(raw.Hits),
	}
	if result.Total == 0 {
		result.Total = raw.TotalHits
	}
	for _, hit := range raw.Hits {
		snippets := buildSearchSnippetsForQuery(params.Query, hit.Formatted)
		if requiresContiguousJapaneseMatch(params.Query) && len(snippets) == 0 {
			continue
		}
		result.Hits = append(result.Hits, MeilisearchSearchHit{
			ItemID:         hit.ID,
			SearchSnippets: snippets,
		})
	}
	return result, nil
}

func (s *MeilisearchService) SearchSuggestions(ctx context.Context, params MeilisearchSuggestionParams) (*MeilisearchSuggestionResult, error) {
	if err := s.ensureSuggestionsIndex(ctx); err != nil {
		return nil, err
	}
	if params.Limit <= 0 {
		params.Limit = 10
	}

	filters := make([]string, 0, len(params.Filters)+1)
	if strings.TrimSpace(params.UserID) != "" {
		filters = append(filters, "user_id = "+QuoteMeilisearchFilter(params.UserID))
	}
	filters = append(filters, params.Filters...)

	payload := map[string]any{
		"q":      params.Query,
		"filter": filters,
		"limit":  params.Limit,
		"sort": []string{
			"score:desc",
			"article_count:desc",
			"updated_at:desc",
		},
	}

	var raw struct {
		Hits []struct {
			ID           string  `json:"id"`
			Kind         string  `json:"kind"`
			Label        string  `json:"label"`
			ItemID       *string `json:"item_id"`
			SourceID     *string `json:"source_id"`
			Topic        *string `json:"topic"`
			ArticleCount *int    `json:"article_count"`
			Score        int     `json:"score"`
		} `json:"hits"`
	}
	if err := s.doJSON(ctx, http.MethodPost, "/indexes/"+s.suggestionsIndex+"/search", payload, &raw, false); err != nil {
		return nil, err
	}

	result := &MeilisearchSuggestionResult{
		Hits: make([]MeilisearchSuggestionHit, 0, len(raw.Hits)),
	}
	for _, hit := range raw.Hits {
		result.Hits = append(result.Hits, MeilisearchSuggestionHit{
			ID:           hit.ID,
			Kind:         hit.Kind,
			Label:        hit.Label,
			ItemID:       hit.ItemID,
			SourceID:     hit.SourceID,
			Topic:        hit.Topic,
			ArticleCount: hit.ArticleCount,
			Score:        hit.Score,
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

func buildSearchSnippetsForQuery(query string, formatted map[string]any) []model.ItemSearchSnippet {
	snippets := buildSearchSnippets(formatted)
	if !requiresContiguousJapaneseMatch(query) {
		return snippets
	}
	normalizedQuery := normalizeSearchSuggestionText(query)
	if normalizedQuery == "" {
		return snippets
	}
	filtered := make([]model.ItemSearchSnippet, 0, len(snippets))
	for _, snippet := range snippets {
		normalizedSnippet := normalizeSearchSuggestionText(stripSearchHighlightTags(snippet.SnippetHTML))
		if strings.Contains(normalizedSnippet, normalizedQuery) {
			filtered = append(filtered, snippet)
		}
	}
	return filtered
}

func requiresContiguousJapaneseMatch(query string) bool {
	normalizedQuery := normalizeSearchSuggestionText(query)
	if normalizedQuery == "" || strings.Contains(normalizedQuery, " ") || utf8.RuneCountInString(normalizedQuery) < 2 {
		return false
	}
	for _, r := range normalizedQuery {
		if unicode.In(r, unicode.Hiragana, unicode.Katakana, unicode.Han) {
			return true
		}
	}
	return false
}

func stripSearchHighlightTags(input string) string {
	input = strings.ReplaceAll(input, "<mark>", "")
	input = strings.ReplaceAll(input, "</mark>", "")
	return input
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

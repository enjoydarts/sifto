package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func TestNewMeilisearchServiceFromEnv(t *testing.T) {
	t.Setenv("MEILISEARCH_URL", "http://meilisearch:7700")
	t.Setenv("MEILISEARCH_MASTER_KEY", "change-me")
	t.Setenv("MEILISEARCH_ITEMS_INDEX", "")
	t.Setenv("MEILISEARCH_SUGGESTIONS_INDEX", "")

	svc, err := NewMeilisearchServiceFromEnv()
	if err != nil {
		t.Fatalf("NewMeilisearchServiceFromEnv returned error: %v", err)
	}
	if svc == nil {
		t.Fatal("NewMeilisearchServiceFromEnv returned nil service")
	}
	if got := svc.ItemsIndexName(); got != "items" {
		t.Fatalf("ItemsIndexName = %q, want %q", got, "items")
	}
	if got := svc.SuggestionsIndexName(); got != "search_suggestions" {
		t.Fatalf("SuggestionsIndexName = %q, want %q", got, "search_suggestions")
	}
}

func TestNewMeilisearchServiceFromEnvCustomIndex(t *testing.T) {
	t.Setenv("MEILISEARCH_URL", "http://meilisearch:7700")
	t.Setenv("MEILISEARCH_MASTER_KEY", "change-me")
	t.Setenv("MEILISEARCH_ITEMS_INDEX", "items_dev")
	t.Setenv("MEILISEARCH_SUGGESTIONS_INDEX", "search_suggestions_dev")

	svc, err := NewMeilisearchServiceFromEnv()
	if err != nil {
		t.Fatalf("NewMeilisearchServiceFromEnv returned error: %v", err)
	}
	if got := svc.ItemsIndexName(); got != "items_dev" {
		t.Fatalf("ItemsIndexName = %q, want %q", got, "items_dev")
	}
	if got := svc.SuggestionsIndexName(); got != "search_suggestions_dev" {
		t.Fatalf("SuggestionsIndexName = %q, want %q", got, "search_suggestions_dev")
	}
}

func TestNewMeilisearchServiceFromEnvRequiresURL(t *testing.T) {
	t.Setenv("MEILISEARCH_URL", "")
	t.Setenv("MEILISEARCH_MASTER_KEY", "")
	t.Setenv("MEILISEARCH_ITEMS_INDEX", "")
	t.Setenv("MEILISEARCH_SUGGESTIONS_INDEX", "")

	svc, err := NewMeilisearchServiceFromEnv()
	if err == nil {
		t.Fatal("NewMeilisearchServiceFromEnv error = nil, want non-nil")
	}
	if svc != nil {
		t.Fatal("NewMeilisearchServiceFromEnv service = non-nil, want nil")
	}
}

func TestBuildSearchSnippetsForQueryFiltersWeakCompactJapaneseMatches(t *testing.T) {
	formatted := map[string]any{
		"summary":      "…施設の近くでのドローン攻撃により<mark>イ</mark>ンフラへの影響が発生し…",
		"facts_text":   "…ドローン攻撃が<mark>イ</mark>ンフラに影響を与えた。…",
		"content_text": "…大きな障害が発生して<mark>い</mark>ることをAWS…",
	}

	got := buildSearchSnippetsForQuery("イラン", formatted)
	if len(got) != 0 {
		t.Fatalf("buildSearchSnippetsForQuery returned %d snippets, want 0", len(got))
	}
}

func TestBuildSearchSnippetsForQueryKeepsContiguousCompactJapaneseMatches(t *testing.T) {
	formatted := map[string]any{
		"title":      "<mark>イラン</mark>戦争で複数のAWSデータセンターが損傷",
		"summary":    "<mark>イラン</mark>の無人機攻撃により、UAEとバーレーンにある3つのAWSデータセンター…",
		"facts_text": "<mark>イラン</mark>の無人機攻撃により、UAEとバーレーンにある3つのAWSデータセンター…",
	}

	got := buildSearchSnippetsForQuery("イラン", formatted)
	if len(got) != 3 {
		t.Fatalf("buildSearchSnippetsForQuery returned %d snippets, want 3", len(got))
	}
}

func TestBuildSearchSnippetsForQueryPreservesNonCompactQueries(t *testing.T) {
	formatted := map[string]any{
		"summary": "<mark>Cloud</mark>-native platforms improve <mark>security</mark> posture",
	}

	got := buildSearchSnippetsForQuery("cloud security", formatted)
	if len(got) != 1 {
		t.Fatalf("buildSearchSnippetsForQuery returned %d snippets, want 1", len(got))
	}
}

func TestBuildSearchSnippetsForQueryIncludesNoteAndHighlightFields(t *testing.T) {
	formatted := map[string]any{
		"note_text":      "Personal note about <mark>fallback</mark> strategy",
		"highlight_text": "Quoted line mentioning <mark>reliability</mark>",
	}

	got := buildSearchSnippetsForQuery("fallback reliability", formatted)
	if len(got) != 2 {
		t.Fatalf("buildSearchSnippetsForQuery returned %d snippets, want 2", len(got))
	}
	if got[0].Field != "note" {
		t.Fatalf("got[0].Field = %q, want %q", got[0].Field, "note")
	}
	if got[1].Field != "highlight" {
		t.Fatalf("got[1].Field = %q, want %q", got[1].Field, "highlight")
	}
}

func TestEnsureItemsIndexIncludesNoteAndHighlightSettings(t *testing.T) {
	var patchBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/indexes":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"taskUid":1}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/indexes/items/settings":
			if err := json.NewDecoder(r.Body).Decode(&patchBody); err != nil {
				t.Fatalf("decode settings: %v", err)
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"taskUid":2}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	svc := &MeilisearchService{
		baseURL:    server.URL,
		itemsIndex: "items",
		client:     server.Client(),
	}
	if err := svc.ensureItemsIndex(context.Background()); err != nil {
		t.Fatalf("ensureItemsIndex returned error: %v", err)
	}

	searchable, ok := patchBody["searchableAttributes"].([]any)
	if !ok {
		t.Fatalf("searchableAttributes type = %T", patchBody["searchableAttributes"])
	}
	var foundNote, foundHighlight bool
	for _, raw := range searchable {
		if raw == "note_text" {
			foundNote = true
		}
		if raw == "highlight_text" {
			foundHighlight = true
		}
	}
	if !foundNote || !foundHighlight {
		t.Fatalf("searchableAttributes = %#v, want note_text and highlight_text", searchable)
	}
}

func TestEnsureItemsIndexIncludesEffectiveOtherGenreLabelSearchableAttribute(t *testing.T) {
	var patchBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/indexes":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"taskUid":1}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/indexes/items/settings":
			if err := json.NewDecoder(r.Body).Decode(&patchBody); err != nil {
				t.Fatalf("decode settings: %v", err)
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"taskUid":2}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	svc := &MeilisearchService{
		baseURL:    server.URL,
		itemsIndex: "items",
		client:     server.Client(),
	}
	if err := svc.ensureItemsIndex(context.Background()); err != nil {
		t.Fatalf("ensureItemsIndex returned error: %v", err)
	}

	searchable, ok := patchBody["searchableAttributes"].([]any)
	if !ok {
		t.Fatalf("searchableAttributes type = %T", patchBody["searchableAttributes"])
	}
	found := false
	for _, raw := range searchable {
		if raw == "effective_other_genre_label" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("searchableAttributes = %#v, want effective_other_genre_label", searchable)
	}
}

func TestEnsureItemsIndexIncludesEffectiveGenreFilterableAttribute(t *testing.T) {
	var patchBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/indexes":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"taskUid":1}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/indexes/items/settings":
			if err := json.NewDecoder(r.Body).Decode(&patchBody); err != nil {
				t.Fatalf("decode settings: %v", err)
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"taskUid":2}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	svc := &MeilisearchService{
		baseURL:    server.URL,
		itemsIndex: "items",
		client:     server.Client(),
	}
	if err := svc.ensureItemsIndex(context.Background()); err != nil {
		t.Fatalf("ensureItemsIndex returned error: %v", err)
	}

	filterable, ok := patchBody["filterableAttributes"].([]any)
	if !ok {
		t.Fatalf("filterableAttributes type = %T", patchBody["filterableAttributes"])
	}
	found := false
	for _, raw := range filterable {
		if raw == "effective_genre" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("filterableAttributes = %#v, want effective_genre", filterable)
	}
}

func TestGenreCountsFromMapSkipsEmptyGenre(t *testing.T) {
	got := genreCountsFromMap(map[string]int{
		"":         4,
		"analysis": 3,
		"news":     5,
	})

	want := []model.GenreCount{
		{Genre: "news", Count: 5},
		{Genre: "analysis", Count: 3},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("genreCountsFromMap = %#v, want %#v", got, want)
	}
}

func TestSearchItemsStrictJapaneseMatchRepaginatesFilteredResults(t *testing.T) {
	var offsets []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/indexes" {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"taskUid":1}`))
			return
		}
		if r.Method == http.MethodPatch && r.URL.Path == "/indexes/items/settings" {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"taskUid":2}`))
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/indexes/items/search" {
			var payload struct {
				Offset int `json:"offset"`
				Limit  int `json:"limit"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			offsets = append(offsets, payload.Offset)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"hits": [
					{"id":"valid-1","_formatted":{"title":"<mark>イラン</mark>戦争 1"}},
					{"id":"false-1","_formatted":{"summary":"…<mark>イ</mark>ンフラ…"}},
					{"id":"valid-2","_formatted":{"summary":"…<mark>イラン</mark>情勢…"}},
					{"id":"false-2","_formatted":{"content_text":"…発生して<mark>い</mark>る…"}},
					{"id":"valid-3","_formatted":{"facts_text":"…<mark>イラン</mark>関連…"}}
				],
				"estimatedTotalHits": 5
			}`))
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	svc := &MeilisearchService{
		baseURL:    server.URL,
		itemsIndex: "items",
		client:     server.Client(),
	}

	got, err := svc.SearchItems(context.Background(), MeilisearchSearchParams{
		Query:      "イラン",
		Offset:     2,
		Limit:      2,
		CropLength: 18,
	})
	if err != nil {
		t.Fatalf("SearchItems returned error: %v", err)
	}
	if got.Total != 3 {
		t.Fatalf("SearchItems total = %d, want 3", got.Total)
	}
	if len(got.Hits) != 1 {
		t.Fatalf("SearchItems hits = %d, want 1", len(got.Hits))
	}
	if got.Hits[0].ItemID != "valid-3" {
		t.Fatalf("SearchItems hit[0] = %q, want %q", got.Hits[0].ItemID, "valid-3")
	}
	if len(offsets) == 0 || offsets[0] != 0 {
		t.Fatalf("SearchItems first offset = %v, want first request from offset 0", offsets)
	}
}

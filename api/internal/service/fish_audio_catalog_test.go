package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFishAudioCatalogFetchModelsCapsResultSetAndFiltersJapanese(t *testing.T) {
	t.Setenv("FISH_AUDIO_MAX_MODELS", "3")

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if got := r.URL.Query().Get("language"); got != "ja" {
			t.Fatalf("language = %q, want ja", got)
		}
		if got := r.URL.Query().Get("sort_by"); got != "task_count" {
			t.Fatalf("sort_by = %q, want task_count", got)
		}
		page := r.URL.Query().Get("page_number")
		var items []map[string]any
		switch page {
		case "1":
			items = []map[string]any{
				{"_id": "ja-1", "title": "JA 1", "languages": []string{"ja"}, "author": map[string]any{"nickname": "A"}},
				{"_id": "en-1", "title": "EN 1", "languages": []string{"en"}, "author": map[string]any{"nickname": "B"}},
				{"_id": "ja-2", "title": "JA 2", "languages": []string{"ja-JP"}, "author": map[string]any{"nickname": "C"}},
			}
		case "2":
			items = []map[string]any{
				{"_id": "ja-3", "title": "JA 3", "languages": []string{"日本語"}, "author": map[string]any{"nickname": "D"}},
				{"_id": "ja-4", "title": "JA 4", "languages": []string{"ja"}, "author": map[string]any{"nickname": "E"}},
			}
		default:
			items = []map[string]any{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total": 500,
			"items": items,
		})
	}))
	defer server.Close()

	t.Setenv("FISH_AUDIO_MODELS_API_URL", server.URL)

	svc := NewFishAudioCatalogService()
	models, err := svc.FetchModels(context.Background())
	if err != nil {
		t.Fatalf("FetchModels() error = %v", err)
	}
	if len(models) != 3 {
		t.Fatalf("len(models) = %d, want 3", len(models))
	}
	if models[0].ModelID != "ja-1" || models[1].ModelID != "ja-2" || models[2].ModelID != "ja-3" {
		t.Fatalf("model ids = %#v, want ja-1, ja-2, ja-3", []string{models[0].ModelID, models[1].ModelID, models[2].ModelID})
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}

func TestFishAudioCatalogBrowseModelsUsesSortAndQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("language"); got != "ja" {
			t.Fatalf("language = %q, want ja", got)
		}
		if got := r.URL.Query().Get("sort_by"); got != "created_at" {
			t.Fatalf("sort_by = %q, want created_at", got)
		}
		if got := r.URL.Query().Get("title"); got != "calm" {
			t.Fatalf("title = %q, want calm", got)
		}
		if got := r.URL.Query().Get("page_number"); got != "2" {
			t.Fatalf("page_number = %q, want 2", got)
		}
		if got := r.URL.Query().Get("page_size"); got != "12" {
			t.Fatalf("page_size = %q, want 12", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total": 25,
			"items": []map[string]any{
				{"_id": "ja-1", "title": "Calm JP", "languages": []string{"ja"}},
				{"_id": "en-1", "title": "Calm EN", "languages": []string{"en"}},
			},
		})
	}))
	defer server.Close()

	t.Setenv("FISH_AUDIO_MODELS_API_URL", server.URL)

	svc := NewFishAudioCatalogService()
	result, err := svc.BrowseModels(context.Background(), FishAudioBrowseParams{
		Sort:     FishAudioBrowseSortLatest,
		Query:    "calm",
		Page:     2,
		PageSize: 12,
	})
	if err != nil {
		t.Fatalf("BrowseModels() error = %v", err)
	}
	if result.Page != 2 || result.PageSize != 12 || result.Total != 25 || !result.HasMore {
		t.Fatalf("unexpected result %#v", result)
	}
	if len(result.Items) != 1 || result.Items[0].ModelID != "ja-1" {
		t.Fatalf("unexpected items %#v", result.Items)
	}
}

func TestFishAudioCatalogBrowseModelsUsesTrendingScoreSort(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("sort_by"); got != "score" {
			t.Fatalf("sort_by = %q, want score", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total": 1,
			"items": []map[string]any{
				{"_id": "ja-1", "title": "Trend JP", "languages": []string{"ja"}},
			},
		})
	}))
	defer server.Close()

	t.Setenv("FISH_AUDIO_MODELS_API_URL", server.URL)

	svc := NewFishAudioCatalogService()
	result, err := svc.BrowseModels(context.Background(), FishAudioBrowseParams{
		Sort: FishAudioBrowseSortTrending,
	})
	if err != nil {
		t.Fatalf("BrowseModels() error = %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ModelID != "ja-1" {
		t.Fatalf("unexpected items %#v", result.Items)
	}
}

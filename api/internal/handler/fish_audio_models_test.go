package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
)

type fakeFishAudioModelFetcher struct {
	browse func(context.Context, service.FishAudioBrowseParams) (*service.FishAudioBrowseResult, error)
}

func (f *fakeFishAudioModelFetcher) BrowseModels(ctx context.Context, params service.FishAudioBrowseParams) (*service.FishAudioBrowseResult, error) {
	return f.browse(ctx, params)
}

func TestFishAudioModelsHandlerBrowse(t *testing.T) {
	fetcher := &fakeFishAudioModelFetcher{
		browse: func(ctx context.Context, params service.FishAudioBrowseParams) (*service.FishAudioBrowseResult, error) {
			if params.Sort != service.FishAudioBrowseSortTrending {
				t.Fatalf("sort = %q, want trending", params.Sort)
			}
			if params.Query != "calm" {
				t.Fatalf("query = %q, want calm", params.Query)
			}
			if params.Page != 2 || params.PageSize != 12 {
				t.Fatalf("page params = %#v", params)
			}
			return &service.FishAudioBrowseResult{
				Items: []repository.FishAudioModelSnapshot{
					{ModelID: "fish-1", Title: "Calm JP", Description: "desc", TagsJSON: []byte(`["female","calm"]`), SamplesJSON: []byte(`[{"audio":"https://example.com/sample.mp3","text":"hello"}]`)},
				},
				Page:     2,
				PageSize: 12,
				Total:    30,
				HasMore:  true,
			}, nil
		},
	}
	handler := NewFishAudioModelsHandler(fetcher)
	req := httptest.NewRequest(http.MethodGet, "/api/fish-models/browse?sort=trending&query=calm&page=2&page_size=12", nil)
	rec := httptest.NewRecorder()

	handler.Browse(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Items    []map[string]any `json:"items"`
		Page     int              `json:"page"`
		PageSize int              `json:"page_size"`
		Total    int              `json:"total"`
		HasMore  bool             `json:"has_more"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if resp.Page != 2 || resp.PageSize != 12 || resp.Total != 30 || !resp.HasMore {
		t.Fatalf("unexpected browse response %#v", resp)
	}
	if len(resp.Items) != 1 || resp.Items[0]["_id"] != "fish-1" {
		t.Fatalf("unexpected items %#v", resp.Items)
	}
	tags, ok := resp.Items[0]["tags"].([]any)
	if !ok || len(tags) != 2 {
		t.Fatalf("unexpected tags %#v", resp.Items[0]["tags"])
	}
	tagMap, ok := tags[0].(map[string]any)
	if !ok || tagMap["name"] != "female" {
		t.Fatalf("unexpected first tag %#v", tags[0])
	}
	samples, ok := resp.Items[0]["samples"].([]any)
	if !ok || len(samples) != 1 {
		t.Fatalf("unexpected samples %#v", resp.Items[0]["samples"])
	}
	sampleMap, ok := samples[0].(map[string]any)
	if !ok || sampleMap["audio_url"] != "https://example.com/sample.mp3" {
		t.Fatalf("unexpected sample payload %#v", samples[0])
	}
}

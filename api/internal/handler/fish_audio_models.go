package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
)

type fishAudioCatalogFetcher interface {
	BrowseModels(ctx context.Context, params service.FishAudioBrowseParams) (*service.FishAudioBrowseResult, error)
}

type FishAudioModelsHandler struct {
	service fishAudioCatalogFetcher
}

func NewFishAudioModelsHandler(svc fishAudioCatalogFetcher) *FishAudioModelsHandler {
	return &FishAudioModelsHandler{service: svc}
}

func (h *FishAudioModelsHandler) Browse(w http.ResponseWriter, r *http.Request) {
	rawSort := strings.TrimSpace(r.URL.Query().Get("sort"))
	if err := service.ValidateFishAudioBrowseSort(rawSort); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	page, err := parseFishAudioBrowseInt(r, "page", 1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	pageSize, err := parseFishAudioBrowseInt(r, "page_size", 24)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.service.BrowseModels(r.Context(), service.FishAudioBrowseParams{
		Sort:     service.FishAudioBrowseSort(rawSort),
		Query:    r.URL.Query().Get("query"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{
		"items":     fishModelPayloads(result.Items),
		"page":      result.Page,
		"page_size": result.PageSize,
		"total":     result.Total,
		"has_more":  result.HasMore,
	})
}

type fishModelPayload struct {
	ID          string           `json:"_id"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	CoverImage  string           `json:"cover_image,omitempty"`
	Tags        []map[string]any `json:"tags"`
	Languages   []string         `json:"languages"`
	Visibility  string           `json:"visibility"`
	LikeCount   int              `json:"like_count"`
	TaskCount   int              `json:"task_count"`
	Author      map[string]any   `json:"author,omitempty"`
	Samples     []map[string]any `json:"samples"`
	FetchedAt   time.Time        `json:"fetched_at"`
	UpdatedAt   *time.Time       `json:"updated_at,omitempty"`
}

func fishModelPayloads(models []repository.FishAudioModelSnapshot) []fishModelPayload {
	out := make([]fishModelPayload, 0, len(models))
	for _, snapshot := range models {
		out = append(out, fishModelPayloadFromSnapshot(snapshot))
	}
	return out
}

func fishModelPayloadFromSnapshot(snapshot repository.FishAudioModelSnapshot) fishModelPayload {
	var rawTags []any
	var samples []map[string]any
	var languages []string
	_ = json.Unmarshal(snapshot.TagsJSON, &rawTags)
	_ = json.Unmarshal(snapshot.SamplesJSON, &samples)
	_ = json.Unmarshal(snapshot.LanguageCodesJSON, &languages)
	author := map[string]any{}
	if snapshot.AuthorName != "" {
		author["name"] = snapshot.AuthorName
	}
	if snapshot.AuthorAvatar != "" {
		author["avatar_url"] = snapshot.AuthorAvatar
	}
	return fishModelPayload{
		ID:          snapshot.ModelID,
		Title:       snapshot.Title,
		Description: snapshot.Description,
		CoverImage:  snapshot.CoverImage,
		Tags:        normalizeFishTagPayloads(rawTags),
		Languages:   defaultFishStringSlice(languages),
		Visibility:  snapshot.Visibility,
		LikeCount:   snapshot.LikeCount,
		TaskCount:   snapshot.TaskCount,
		Author:      author,
		Samples:     normalizeFishSamplePayloads(samples),
		FetchedAt:   snapshot.FetchedAt,
		UpdatedAt:   snapshot.UpdatedAtRemote,
	}
}

func defaultFishMapSlice(v []map[string]any) []map[string]any {
	if v == nil {
		return make([]map[string]any, 0)
	}
	return v
}

func defaultFishStringSlice(v []string) []string {
	if v == nil {
		return make([]string, 0)
	}
	return v
}

func normalizeFishSamplePayloads(samples []map[string]any) []map[string]any {
	if samples == nil {
		return make([]map[string]any, 0)
	}
	out := make([]map[string]any, 0, len(samples))
	for _, sample := range samples {
		if sample == nil {
			continue
		}
		next := make(map[string]any, len(sample)+1)
		for key, value := range sample {
			next[key] = value
		}
		if _, ok := next["audio_url"]; !ok {
			if audio, ok := next["audio"].(string); ok && strings.TrimSpace(audio) != "" {
				next["audio_url"] = strings.TrimSpace(audio)
			}
		}
		out = append(out, next)
	}
	return out
}

func normalizeFishTagPayloads(tags []any) []map[string]any {
	if tags == nil {
		return make([]map[string]any, 0)
	}
	out := make([]map[string]any, 0, len(tags))
	for _, tag := range tags {
		switch v := tag.(type) {
		case string:
			name := strings.TrimSpace(v)
			if name == "" {
				continue
			}
			out = append(out, map[string]any{"name": name})
		case map[string]any:
			name, _ := v["name"].(string)
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			next := make(map[string]any, len(v)+1)
			for key, value := range v {
				next[key] = value
			}
			next["name"] = name
			out = append(out, next)
		}
	}
	return out
}

func parseFishAudioBrowseInt(r *http.Request, key string, fallback int) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return v, nil
}

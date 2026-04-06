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

	"github.com/enjoydarts/sifto/api/internal/repository"
)

type FishAudioCatalogService struct {
	http *http.Client
}

func NewFishAudioCatalogService() *FishAudioCatalogService {
	return &FishAudioCatalogService{
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

type fishAudioListResponse struct {
	Total      int                    `json:"total"`
	TotalCount int                    `json:"total_count"`
	Items      []fishAudioRemoteModel `json:"items"`
	Data       []fishAudioRemoteModel `json:"data"`
	Models     []fishAudioRemoteModel `json:"models"`
}

type fishAudioRemoteModel struct {
	ID          string         `json:"_id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	CoverImage  string         `json:"cover_image"`
	Visibility  string         `json:"visibility"`
	TrainMode   string         `json:"train_mode"`
	LikeCount   int            `json:"like_count"`
	MarkCount   int            `json:"mark_count"`
	SharedCount int            `json:"shared_count"`
	TaskCount   int            `json:"task_count"`
	Languages   []string       `json:"languages"`
	Tags        []any          `json:"tags"`
	Samples     []any          `json:"samples"`
	Author      map[string]any `json:"author"`
	CreatedAt   *time.Time     `json:"created_at"`
	UpdatedAt   *time.Time     `json:"updated_at"`
}

func (s *FishAudioCatalogService) FetchModels(ctx context.Context) ([]repository.FishAudioModelSnapshot, error) {
	const pageLimit = 100
	const defaultMaxModels = 1000
	page := 1
	out := make([]repository.FishAudioModelSnapshot, 0, pageLimit)
	maxModels := fishAudioMaxModels(defaultMaxModels)
	for {
		payload, items, err := s.listModelsPage(ctx, page, pageLimit, "", "task_count")
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if !fishAudioSupportsJapanese(item.Languages) {
				continue
			}
			out = append(out, normalizeFishAudioModel(item, payload.fetchedAt))
			if len(out) >= maxModels {
				return out[:maxModels], nil
			}
		}
		if len(items) == 0 || (payload.total > 0 && page*pageLimit >= payload.total) || page >= 100 {
			break
		}
		page++
	}
	return out, nil
}

type fishAudioListPage struct {
	total     int
	fetchedAt time.Time
}

func (s *FishAudioCatalogService) listModelsPage(ctx context.Context, page, limit int, query, sortBy string) (*fishAudioListPage, []fishAudioRemoteModel, error) {
	reqURL, err := fishAudioModelsURL(page, limit, query, sortBy)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, nil, err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		resp.Body.Close()
		return nil, nil, fmt.Errorf("fish audio models api status %d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload fishAudioListResponse
	err = json.NewDecoder(resp.Body).Decode(&payload)
	resp.Body.Close()
	if err != nil {
		return nil, nil, err
	}
	items := payload.Items
	if len(items) == 0 {
		items = payload.Data
	}
	if len(items) == 0 {
		items = payload.Models
	}
	total := payload.Total
	if total <= 0 {
		total = payload.TotalCount
	}
	return &fishAudioListPage{
		total:     total,
		fetchedAt: time.Now().UTC(),
	}, items, nil
}

func fishAudioModelsURL(page, limit int, query, sortBy string) (string, error) {
	base := strings.TrimSpace(os.Getenv("FISH_AUDIO_MODELS_API_URL"))
	if base == "" {
		base = "https://api.fish.audio/model"
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("page_size", strconv.Itoa(limit))
	q.Set("page_number", strconv.Itoa(page))
	q.Set("language", "ja")
	if strings.TrimSpace(sortBy) != "" {
		q.Set("sort_by", strings.TrimSpace(sortBy))
	}
	if strings.TrimSpace(query) != "" {
		q.Set("title", strings.TrimSpace(query))
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func fishAudioMaxModels(defaultValue int) int {
	raw := strings.TrimSpace(os.Getenv("FISH_AUDIO_MAX_MODELS"))
	if raw == "" {
		return defaultValue
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return defaultValue
	}
	return v
}

func fishAudioSupportsJapanese(languages []string) bool {
	for _, language := range languages {
		switch strings.ToLower(strings.TrimSpace(language)) {
		case "ja", "ja-jp", "japanese", "jp", "日本語":
			return true
		}
	}
	return false
}

func normalizeFishAudioModel(item fishAudioRemoteModel, fetchedAt time.Time) repository.FishAudioModelSnapshot {
	authorName := ""
	authorAvatar := ""
	if item.Author != nil {
		authorName = strings.TrimSpace(fishAudioStringValue(item.Author["name"]))
		if authorName == "" {
			authorName = strings.TrimSpace(fishAudioStringValue(item.Author["nickname"]))
		}
		authorAvatar = strings.TrimSpace(fishAudioStringValue(item.Author["avatar"]))
		if authorAvatar == "" {
			authorAvatar = strings.TrimSpace(fishAudioStringValue(item.Author["avatar_url"]))
		}
	}
	languagesJSON, _ := json.Marshal(item.Languages)
	tagsJSON, _ := json.Marshal(item.Tags)
	samplesJSON, _ := json.Marshal(item.Samples)
	metadataJSON, _ := json.Marshal(item)
	return repository.FishAudioModelSnapshot{
		ModelID:           strings.TrimSpace(item.ID),
		Title:             strings.TrimSpace(item.Title),
		Description:       strings.TrimSpace(item.Description),
		CoverImage:        strings.TrimSpace(item.CoverImage),
		Visibility:        strings.TrimSpace(item.Visibility),
		TrainMode:         strings.TrimSpace(item.TrainMode),
		AuthorName:        authorName,
		AuthorAvatar:      authorAvatar,
		LanguageCodesJSON: languagesJSON,
		TagsJSON:          tagsJSON,
		SamplesJSON:       samplesJSON,
		MetadataJSON:      metadataJSON,
		LikeCount:         item.LikeCount,
		MarkCount:         item.MarkCount,
		SharedCount:       item.SharedCount,
		TaskCount:         item.TaskCount,
		SampleCount:       len(item.Samples),
		CreatedAtRemote:   item.CreatedAt,
		UpdatedAtRemote:   item.UpdatedAt,
		FetchedAt:         fetchedAt,
	}
}

func fishAudioStringValue(v any) string {
	s, _ := v.(string)
	return s
}

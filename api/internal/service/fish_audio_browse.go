package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

type FishAudioBrowseSort string

const (
	FishAudioBrowseSortRecommended FishAudioBrowseSort = "recommended"
	FishAudioBrowseSortTrending    FishAudioBrowseSort = "trending"
	FishAudioBrowseSortLatest      FishAudioBrowseSort = "latest"
)

type FishAudioBrowseParams struct {
	Sort     FishAudioBrowseSort
	Query    string
	Page     int
	PageSize int
}

type FishAudioBrowseResult struct {
	Items    []repository.FishAudioModelSnapshot
	Total    int
	Page     int
	PageSize int
	HasMore  bool
}

func normalizeFishAudioBrowseSort(raw string) FishAudioBrowseSort {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case string(FishAudioBrowseSortTrending):
		return FishAudioBrowseSortTrending
	case string(FishAudioBrowseSortLatest):
		return FishAudioBrowseSortLatest
	default:
		return FishAudioBrowseSortRecommended
	}
}

func normalizeFishAudioBrowseParams(params FishAudioBrowseParams) FishAudioBrowseParams {
	params.Sort = normalizeFishAudioBrowseSort(string(params.Sort))
	params.Query = strings.TrimSpace(params.Query)
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 24
	}
	if params.PageSize > 60 {
		params.PageSize = 60
	}
	return params
}

func (s *FishAudioCatalogService) BrowseModels(ctx context.Context, params FishAudioBrowseParams) (*FishAudioBrowseResult, error) {
	params = normalizeFishAudioBrowseParams(params)
	payload, items, err := s.listModelsPage(ctx, params.Page, params.PageSize, params.Query, fishAudioBrowseSortParam(params.Sort))
	if err != nil {
		return nil, err
	}
	out := make([]repository.FishAudioModelSnapshot, 0, len(items))
	fetchedAt := payload.fetchedAt
	for _, item := range items {
		if !fishAudioSupportsJapanese(item.Languages) {
			continue
		}
		out = append(out, normalizeFishAudioModel(item, fetchedAt))
	}
	total := payload.total
	if total < 0 {
		total = len(out)
	}
	hasMore := false
	if total > 0 {
		hasMore = params.Page*params.PageSize < total
	} else {
		hasMore = len(items) >= params.PageSize
	}
	return &FishAudioBrowseResult{
		Items:    out,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
		HasMore:  hasMore,
	}, nil
}

func fishAudioBrowseSortParam(sort FishAudioBrowseSort) string {
	switch normalizeFishAudioBrowseSort(string(sort)) {
	case FishAudioBrowseSortTrending:
		return "score"
	case FishAudioBrowseSortLatest:
		return "created_at"
	default:
		return "task_count"
	}
}

func ValidateFishAudioBrowseSort(raw string) error {
	sort := strings.TrimSpace(strings.ToLower(raw))
	if sort == "" {
		return nil
	}
	switch FishAudioBrowseSort(sort) {
	case FishAudioBrowseSortRecommended, FishAudioBrowseSortTrending, FishAudioBrowseSortLatest:
		return nil
	default:
		return fmt.Errorf("invalid sort")
	}
}

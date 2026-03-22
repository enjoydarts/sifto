package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
)

type ItemSearchService struct {
	search *MeilisearchService
	repo   *repository.ItemRepo
}

func NewItemSearchService(search *MeilisearchService, repo *repository.ItemRepo) *ItemSearchService {
	return &ItemSearchService{search: search, repo: repo}
}

type ItemSearchQuery struct {
	UserID       string
	Query        string
	SearchMode   string
	Status       *string
	SourceID     *string
	Topic        *string
	UnreadOnly   bool
	ReadOnly     bool
	FavoriteOnly bool
	LaterOnly    bool
	Page         int
	PageSize     int
}

func (s *ItemSearchService) Search(ctx context.Context, q ItemSearchQuery) (*model.ItemListResponse, error) {
	if s == nil || s.search == nil || s.repo == nil {
		return nil, fmt.Errorf("item search service not configured")
	}
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}

	searchResp, err := s.search.SearchItems(ctx, MeilisearchSearchParams{
		Query:      strings.TrimSpace(q.Query),
		SearchMode: q.SearchMode,
		Filters:    buildItemSearchFilters(q),
		Offset:     (q.Page - 1) * q.PageSize,
		Limit:      q.PageSize,
		CropLength: 18,
	})
	if err != nil {
		return nil, err
	}

	itemIDs := make([]string, 0, len(searchResp.Hits))
	snippetsByID := make(map[string][]model.ItemSearchSnippet, len(searchResp.Hits))
	for _, hit := range searchResp.Hits {
		if strings.TrimSpace(hit.ItemID) == "" {
			continue
		}
		itemIDs = append(itemIDs, hit.ItemID)
		snippetsByID[hit.ItemID] = hit.SearchSnippets
	}

	items, err := s.repo.LoadByIDsPreservingOrder(ctx, q.UserID, itemIDs)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].SearchSnippets = snippetsByID[items[i].ID]
		items[i].SearchMatchCount = len(items[i].SearchSnippets)
	}

	mode := searchResp.Mode
	return &model.ItemListResponse{
		Items:      items,
		Page:       q.Page,
		PageSize:   q.PageSize,
		Total:      searchResp.Total,
		HasNext:    (q.Page*q.PageSize) < searchResp.Total,
		Sort:       "relevance",
		Status:     q.Status,
		SourceID:   q.SourceID,
		SearchMode: &mode,
	}, nil
}

func buildItemSearchFilters(q ItemSearchQuery) []string {
	filters := []string{
		"user_id = " + QuoteMeilisearchFilter(q.UserID),
	}
	status := ""
	if q.Status != nil {
		status = strings.TrimSpace(*q.Status)
	}
	switch status {
	case "deleted":
		filters = append(filters, "is_deleted = true")
	case "pending":
		filters = append(filters, "is_deleted = false")
		filters = append(filters, `(status = "new" OR status = "fetched" OR status = "facts_extracted" OR status = "failed")`)
	case "":
		filters = append(filters, "is_deleted = false")
	default:
		filters = append(filters, "is_deleted = false")
		filters = append(filters, "status = "+QuoteMeilisearchFilter(status))
	}
	if q.SourceID != nil && strings.TrimSpace(*q.SourceID) != "" {
		filters = append(filters, "source_id = "+QuoteMeilisearchFilter(*q.SourceID))
	}
	if q.Topic != nil && strings.TrimSpace(*q.Topic) != "" {
		filters = append(filters, "topics = "+QuoteMeilisearchFilter(*q.Topic))
	}
	if q.UnreadOnly {
		filters = append(filters, "is_read = false")
	}
	if q.ReadOnly {
		filters = append(filters, "is_read = true")
	}
	if q.FavoriteOnly {
		filters = append(filters, "is_favorite = true")
	}
	if q.LaterOnly {
		filters = append(filters, "is_later = true")
	}
	return filters
}

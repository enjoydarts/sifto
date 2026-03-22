package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/enjoydarts/sifto/api/internal/model"
)

const (
	searchSuggestionLimit       = 10
	searchSuggestionArticleCap  = 6
	searchSuggestionSourceCap   = 2
	searchSuggestionTopicCap    = 2
	searchSuggestionQueryMinLen = 2
)

type SearchSuggestionService struct {
	search *MeilisearchService
}

func NewSearchSuggestionService(search *MeilisearchService) *SearchSuggestionService {
	return &SearchSuggestionService{search: search}
}

type SearchSuggestionQuery struct {
	UserID string
	Query  string
	Limit  int
}

func (s *SearchSuggestionService) Search(ctx context.Context, q SearchSuggestionQuery) (*model.SearchSuggestionResponse, error) {
	if s == nil || s.search == nil {
		return nil, fmt.Errorf("search suggestion service not configured")
	}
	query := strings.TrimSpace(q.Query)
	if utf8.RuneCountInString(query) < searchSuggestionQueryMinLen {
		return &model.SearchSuggestionResponse{Items: []model.SearchSuggestionItem{}}, nil
	}
	limit := q.Limit
	if limit <= 0 || limit > searchSuggestionLimit {
		limit = searchSuggestionLimit
	}

	raw, err := s.search.SearchSuggestions(ctx, MeilisearchSuggestionParams{
		Query:  query,
		UserID: q.UserID,
		Limit:  32,
	})
	if err != nil {
		return nil, err
	}

	items := distributeSearchSuggestionHits(raw.Hits, limit)
	return &model.SearchSuggestionResponse{Items: items}, nil
}

func distributeSearchSuggestionHits(hits []MeilisearchSuggestionHit, limit int) []model.SearchSuggestionItem {
	if limit <= 0 {
		limit = searchSuggestionLimit
	}
	articles := make([]model.SearchSuggestionItem, 0, len(hits))
	sources := make([]model.SearchSuggestionItem, 0, len(hits))
	topics := make([]model.SearchSuggestionItem, 0, len(hits))
	seen := map[string]struct{}{}

	for _, hit := range hits {
		if strings.TrimSpace(hit.ID) == "" || strings.TrimSpace(hit.Label) == "" {
			continue
		}
		if _, exists := seen[hit.ID]; exists {
			continue
		}
		seen[hit.ID] = struct{}{}
		item := model.SearchSuggestionItem{
			Kind:         hit.Kind,
			Label:        hit.Label,
			ItemID:       hit.ItemID,
			SourceID:     hit.SourceID,
			Topic:        hit.Topic,
			ArticleCount: hit.ArticleCount,
		}
		switch hit.Kind {
		case "source":
			sources = append(sources, item)
		case "topic":
			topics = append(topics, item)
		case "article":
			articles = append(articles, item)
		}
	}

	out := make([]model.SearchSuggestionItem, 0, limit)
	appendCap := func(list []model.SearchSuggestionItem, cap int) int {
		added := 0
		for _, item := range list {
			if len(out) >= limit || added >= cap {
				break
			}
			out = append(out, item)
			added++
		}
		return added
	}

	sourcesAdded := appendCap(sources, searchSuggestionSourceCap)
	topicsAdded := appendCap(topics, searchSuggestionTopicCap)
	articleCap := searchSuggestionArticleCap + (searchSuggestionSourceCap - sourcesAdded) + (searchSuggestionTopicCap - topicsAdded)
	appendCap(articles, articleCap)

	if len(out) >= limit {
		return out[:limit]
	}

	exists := map[string]struct{}{}
	for _, item := range out {
		key := item.Kind + ":" + item.Label
		exists[key] = struct{}{}
	}
	for _, bucket := range [][]model.SearchSuggestionItem{sources, topics, articles} {
		for _, item := range bucket {
			if len(out) >= limit {
				return out
			}
			key := item.Kind + ":" + item.Label
			if _, ok := exists[key]; ok {
				continue
			}
			exists[key] = struct{}{}
			out = append(out, item)
		}
	}

	return out
}

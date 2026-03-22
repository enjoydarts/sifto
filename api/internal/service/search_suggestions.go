package service

import (
	"context"
	"fmt"
	"strings"
	"unicode"
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

	items := distributeSearchSuggestionHits(query, raw.Hits, limit)
	return &model.SearchSuggestionResponse{Items: items}, nil
}

func distributeSearchSuggestionHits(query string, hits []MeilisearchSuggestionHit, limit int) []model.SearchSuggestionItem {
	if limit <= 0 {
		limit = searchSuggestionLimit
	}
	normalizedQuery := normalizeSearchSuggestionText(query)
	if normalizedQuery == "" {
		return []model.SearchSuggestionItem{}
	}
	articles := make([]model.SearchSuggestionItem, 0, len(hits))
	sources := make([]model.SearchSuggestionItem, 0, len(hits))
	topics := make([]model.SearchSuggestionItem, 0, len(hits))
	seen := map[string]struct{}{}

	for _, hit := range hits {
		if strings.TrimSpace(hit.ID) == "" || strings.TrimSpace(hit.Label) == "" {
			continue
		}
		if searchSuggestionMatchRank(normalizedQuery, hit.Label) == 0 {
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

func searchSuggestionMatchRank(normalizedQuery, label string) int {
	normalizedLabel := normalizeSearchSuggestionText(label)
	if normalizedQuery == "" || normalizedLabel == "" {
		return 0
	}
	switch {
	case normalizedLabel == normalizedQuery:
		return 3
	case strings.HasPrefix(normalizedLabel, normalizedQuery):
		return 2
	case strings.Contains(normalizedLabel, normalizedQuery):
		return 1
	default:
		return 0
	}
}

func normalizeSearchSuggestionText(input string) string {
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(input))
	lastSpace := false
	for _, r := range input {
		if unicode.IsSpace(r) {
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
			continue
		}
		lastSpace = false
		b.WriteRune(foldSearchSuggestionRune(r))
	}
	return strings.TrimSpace(b.String())
}

func foldSearchSuggestionRune(r rune) rune {
	if r >= 'ァ' && r <= 'ヶ' {
		return r - 0x60
	}
	if r == 'ヴ' {
		return 'ゔ'
	}
	return unicode.ToLower(r)
}

package service

import "testing"

func TestDistributeSearchSuggestionHitsAppliesKindCaps(t *testing.T) {
	hits := []MeilisearchSuggestionHit{
		{ID: "source:1", Kind: "source", Label: "Source A"},
		{ID: "source:2", Kind: "source", Label: "Source B"},
		{ID: "source:3", Kind: "source", Label: "Source C"},
		{ID: "topic:1", Kind: "topic", Label: "Topic A"},
		{ID: "topic:2", Kind: "topic", Label: "Topic B"},
		{ID: "topic:3", Kind: "topic", Label: "Topic C"},
		{ID: "article:1", Kind: "article", Label: "Article 1"},
		{ID: "article:2", Kind: "article", Label: "Article 2"},
		{ID: "article:3", Kind: "article", Label: "Article 3"},
		{ID: "article:4", Kind: "article", Label: "Article 4"},
		{ID: "article:5", Kind: "article", Label: "Article 5"},
		{ID: "article:6", Kind: "article", Label: "Article 6"},
		{ID: "article:7", Kind: "article", Label: "Article 7"},
	}

	got := distributeSearchSuggestionHits(hits, 10)
	if len(got) != 10 {
		t.Fatalf("len(got) = %d, want 10", len(got))
	}

	counts := map[string]int{}
	for _, item := range got {
		counts[item.Kind]++
	}
	if counts["source"] != 2 {
		t.Fatalf("source count = %d, want 2", counts["source"])
	}
	if counts["topic"] != 2 {
		t.Fatalf("topic count = %d, want 2", counts["topic"])
	}
	if counts["article"] != 6 {
		t.Fatalf("article count = %d, want 6", counts["article"])
	}
}

func TestDistributeSearchSuggestionHitsSpillsUnusedSlotsIntoArticles(t *testing.T) {
	hits := []MeilisearchSuggestionHit{
		{ID: "source:1", Kind: "source", Label: "Source A"},
		{ID: "topic:1", Kind: "topic", Label: "Topic A"},
		{ID: "article:1", Kind: "article", Label: "Article 1"},
		{ID: "article:2", Kind: "article", Label: "Article 2"},
		{ID: "article:3", Kind: "article", Label: "Article 3"},
		{ID: "article:4", Kind: "article", Label: "Article 4"},
		{ID: "article:5", Kind: "article", Label: "Article 5"},
		{ID: "article:6", Kind: "article", Label: "Article 6"},
		{ID: "article:7", Kind: "article", Label: "Article 7"},
		{ID: "article:8", Kind: "article", Label: "Article 8"},
	}

	got := distributeSearchSuggestionHits(hits, 10)
	if len(got) != 10 {
		t.Fatalf("len(got) = %d, want 10", len(got))
	}

	counts := map[string]int{}
	for _, item := range got {
		counts[item.Kind]++
	}
	if counts["source"] != 1 {
		t.Fatalf("source count = %d, want 1", counts["source"])
	}
	if counts["topic"] != 1 {
		t.Fatalf("topic count = %d, want 1", counts["topic"])
	}
	if counts["article"] != 8 {
		t.Fatalf("article count = %d, want 8", counts["article"])
	}
}

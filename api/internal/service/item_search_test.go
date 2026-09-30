package service

import (
	"strings"
	"testing"
)

func TestBuildItemSearchFiltersNormalizesUncategorizedGenre(t *testing.T) {
	genre := "  untagged  "

	filters := buildItemSearchFilters(ItemSearchQuery{
		UserID: "user-1",
		Genre:  &genre,
	}, true)

	found := false
	for _, filter := range filters {
		if strings.Contains(filter, `effective_genre = "uncategorized"`) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("buildItemSearchFilters = %#v, want uncategorized effective_genre filter", filters)
	}
}

func TestBuildItemSearchFiltersExpandsCanonicalTopicForLegacyDocuments(t *testing.T) {
	topic := "LLM"
	filters := buildItemSearchFilters(ItemSearchQuery{UserID: "user-1", Topic: &topic}, true)
	joined := strings.Join(filters, " ")
	if !strings.Contains(joined, `topics = "LLM"`) || !strings.Contains(joined, `topics = "大規模言語モデル"`) {
		t.Fatalf("filters = %#v, want canonical and legacy topic values", filters)
	}
}

func TestBuildItemSearchFiltersNormalizesLegacyFreeformGenreToOther(t *testing.T) {
	genre := "Observability"

	filters := buildItemSearchFilters(ItemSearchQuery{
		UserID: "user-1",
		Genre:  &genre,
	}, true)

	found := false
	for _, filter := range filters {
		if strings.Contains(filter, `effective_genre = "other"`) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("buildItemSearchFilters = %#v, want other effective_genre filter", filters)
	}
}

func TestBuildItemSearchFiltersNormalizesLegacyAgentGenreToAI(t *testing.T) {
	genre := "agent"

	filters := buildItemSearchFilters(ItemSearchQuery{
		UserID: "user-1",
		Genre:  &genre,
	}, true)

	found := false
	for _, filter := range filters {
		if strings.Contains(filter, `effective_genre = "ai"`) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("buildItemSearchFilters = %#v, want ai effective_genre filter", filters)
	}
}

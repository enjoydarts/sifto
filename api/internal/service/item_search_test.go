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

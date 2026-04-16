package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestItemSearchDocumentJSONOmitsEmptyEffectiveGenre(t *testing.T) {
	doc := ItemSearchDocument{
		ID:       "item-1",
		UserID:   "user-1",
		SourceID: "source-1",
		Status:   "summarized",
		Title:    "Title",
		Summary:  "Summary",
	}

	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(raw), `"effective_genre"`) {
		t.Fatalf("marshaled doc = %s, want effective_genre omitted when empty", raw)
	}
	if strings.Contains(string(raw), `"effective_other_genre_label"`) {
		t.Fatalf("marshaled doc = %s, want effective_other_genre_label omitted when empty", raw)
	}
}

package handler

import (
	"strings"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func TestBuildFavoritesMarkdownIncludesNoteAndHighlights(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	title := "Favorite title"
	noteTime := time.Date(2026, 3, 15, 2, 0, 0, 0, time.UTC)
	markdown := buildFavoritesMarkdown([]model.FavoriteExportItem{
		{
			ID:          "item-1",
			URL:         "https://example.com/post",
			Title:       &title,
			FavoritedAt: noteTime,
			Note: &model.ItemNote{
				ID:        "note-1",
				UserID:    "u1",
				ItemID:    "item-1",
				Content:   "残しておきたいメモ",
				UpdatedAt: noteTime,
			},
			Highlights: []model.ItemHighlight{
				{ID: "h1", UserID: "u1", ItemID: "item-1", QuoteText: "残した引用", Section: "body"},
			},
		},
	}, now, "all favorites")

	if !strings.Contains(markdown, "### Personal Note") {
		t.Fatalf("markdown missing note heading:\n%s", markdown)
	}
	if !strings.Contains(markdown, "残しておきたいメモ") {
		t.Fatalf("markdown missing note content:\n%s", markdown)
	}
	if !strings.Contains(markdown, "### Highlights") {
		t.Fatalf("markdown missing highlights heading:\n%s", markdown)
	}
	if !strings.Contains(markdown, "残した引用") {
		t.Fatalf("markdown missing highlight quote:\n%s", markdown)
	}
}

package service

import (
	"strings"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func TestObsidianExportIncludesPersonalNote(t *testing.T) {
	item := model.FavoriteExportItem{
		ID:          "item-1",
		URL:         "https://example.com/post",
		FavoritedAt: time.Date(2026, 3, 15, 1, 0, 0, 0, time.UTC),
		Note: &model.ItemNote{
			ID:      "note-1",
			UserID:  "u1",
			ItemID:  "item-1",
			Content: "自分向けメモ",
			Tags:    []string{"ai", "weekly"},
		},
	}

	content, _, _ := BuildObsidianFavoriteMarkdown(item, model.ObsidianExportSettings{})
	markdown := string(content)

	if !strings.Contains(markdown, "## Personal Note") {
		t.Fatalf("markdown missing personal note heading:\n%s", markdown)
	}
	if !strings.Contains(markdown, "自分向けメモ") {
		t.Fatalf("markdown missing note content:\n%s", markdown)
	}
	if !strings.Contains(markdown, "- ai") || !strings.Contains(markdown, "- weekly") {
		t.Fatalf("markdown missing note tags:\n%s", markdown)
	}
}

func TestObsidianExportIncludesHighlights(t *testing.T) {
	item := model.FavoriteExportItem{
		ID:          "item-1",
		URL:         "https://example.com/post",
		FavoritedAt: time.Date(2026, 3, 15, 1, 0, 0, 0, time.UTC),
		Highlights: []model.ItemHighlight{
			{
				ID:         "h1",
				UserID:     "u1",
				ItemID:     "item-1",
				QuoteText:  "重要な一文",
				AnchorText: "一文",
				Section:    "summary",
			},
		},
	}

	content, _, _ := BuildObsidianFavoriteMarkdown(item, model.ObsidianExportSettings{})
	markdown := string(content)

	if !strings.Contains(markdown, "## Highlights") {
		t.Fatalf("markdown missing highlights heading:\n%s", markdown)
	}
	if !strings.Contains(markdown, "重要な一文") {
		t.Fatalf("markdown missing highlight quote:\n%s", markdown)
	}
	if !strings.Contains(markdown, "summary") {
		t.Fatalf("markdown missing highlight section:\n%s", markdown)
	}
}

func TestObsidianExportHandlesMissingNoteAndHighlight(t *testing.T) {
	item := model.FavoriteExportItem{
		ID:          "item-1",
		URL:         "https://example.com/post",
		FavoritedAt: time.Date(2026, 3, 15, 1, 0, 0, 0, time.UTC),
	}

	content, _, _ := BuildObsidianFavoriteMarkdown(item, model.ObsidianExportSettings{})
	markdown := string(content)

	if strings.Contains(markdown, "## Personal Note") {
		t.Fatalf("markdown unexpectedly included personal note heading:\n%s", markdown)
	}
	if strings.Contains(markdown, "## Highlights") {
		t.Fatalf("markdown unexpectedly included highlights heading:\n%s", markdown)
	}
}

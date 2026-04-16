package repository

import (
	"strings"
	"testing"
)

func TestItemSearchDocumentsQueryCastsSourceUserIDForLegacyNoteAndHighlightTables(t *testing.T) {
	query := itemSearchDocumentsQuery(`AND i.id = $1`)

	if !strings.Contains(query, "n.user_id = s.user_id::text") {
		t.Fatalf("item search documents query must cast sources.user_id to text for item_notes join")
	}
	if !strings.Contains(query, "ih.user_id = s.user_id::text") {
		t.Fatalf("item search documents query must cast sources.user_id to text for item_highlights subquery")
	}
}

func TestItemSearchDocumentsQueryIncludesNoteAndHighlightFields(t *testing.T) {
	query := itemSearchDocumentsQuery(`AND i.id = $1`)

	if !strings.Contains(query, "COALESCE(n.content, '') AS note_text") {
		t.Fatalf("item search documents query must select note_text")
	}
	if !strings.Contains(query, "COALESCE(h.highlight_text, '') AS highlight_text") {
		t.Fatalf("item search documents query must select highlight_text")
	}
}

func TestItemSearchDocumentsQueryIncludesEffectiveGenre(t *testing.T) {
	query := itemSearchDocumentsQuery(`AND i.id = $1`)

	if !strings.Contains(query, "AS effective_genre") {
		t.Fatalf("item search documents query must select effective_genre")
	}
	if !strings.Contains(query, "'uncategorized'") {
		t.Fatalf("item search documents query must compute an uncategorized fallback")
	}
	if !strings.Contains(query, "NULLIF(BTRIM(i.user_genre), '')") || !strings.Contains(query, "NULLIF(BTRIM(sm.genre), '')") {
		t.Fatalf("item search documents query must compute effective_genre from user_genre then summary genre")
	}
}

func TestItemSearchDocumentsQueryIncludesEffectiveOtherGenreLabel(t *testing.T) {
	query := itemSearchDocumentsQuery(`AND i.id = $1`)

	if !strings.Contains(query, "AS effective_other_genre_label") {
		t.Fatalf("item search documents query must select effective_other_genre_label")
	}
	if !strings.Contains(query, "i.user_other_genre_label") {
		t.Fatalf("item search documents query must consider manual other label")
	}
	if !strings.Contains(query, "sm.other_genre_label") {
		t.Fatalf("item search documents query must consider summary other label")
	}
}

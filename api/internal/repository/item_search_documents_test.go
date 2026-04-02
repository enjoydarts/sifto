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

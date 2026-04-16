package repository

import (
	"strings"
	"testing"
)

func testGenreStrPtr(value string) *string {
	return &value
}

func TestEffectiveGenreExprFallsBackToUncategorized(t *testing.T) {
	expr := effectiveGenreExpr("i", "sm")

	if !strings.Contains(expr, "'uncategorized'") {
		t.Fatalf("effectiveGenreExpr = %q, want uncategorized fallback", expr)
	}
}

func TestAppendItemGenreFilterNormalizesUntaggedAlias(t *testing.T) {
	genre := "  untagged  "

	query, args := appendItemGenreFilter("WHERE s.user_id = $1", []any{"user-1"}, &genre, "i", "sm")

	if !strings.Contains(query, effectiveGenreExpr("i", "sm")+" = $2") {
		t.Fatalf("appendItemGenreFilter query = %q, want effective genre predicate", query)
	}
	if got, ok := args[1].(string); !ok || got != "uncategorized" {
		t.Fatalf("appendItemGenreFilter args = %#v, want normalized uncategorized genre", args)
	}
}

func TestNormalizeGenreInputRecognizesTaxonomyAndOtherLabels(t *testing.T) {
	tests := []struct {
		name      string
		genre     *string
		label     *string
		wantGenre *string
		wantLabel *string
	}{
		{
			name:      "taxonomy key kept",
			genre:     testGenreStrPtr("AI"),
			wantGenre: testGenreStrPtr("ai"),
		},
		{
			name:      "untagged alias becomes uncategorized",
			genre:     testGenreStrPtr("  untagged "),
			wantGenre: testGenreStrPtr("uncategorized"),
		},
		{
			name:      "explicit other keeps label",
			genre:     testGenreStrPtr("other"),
			label:     testGenreStrPtr("Chip packaging"),
			wantGenre: testGenreStrPtr("other"),
			wantLabel: testGenreStrPtr("Chip packaging"),
		},
		{
			name:      "legacy freeform becomes other with derived label",
			genre:     testGenreStrPtr("Observability"),
			wantGenre: testGenreStrPtr("other"),
			wantLabel: testGenreStrPtr("Observability"),
		},
		{
			name:  "blank values clear both",
			genre: testGenreStrPtr(" "),
			label: testGenreStrPtr(" "),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotGenre, gotLabel := normalizeGenreInput(tt.genre, tt.label)
			if !stringPtrEqual(gotGenre, tt.wantGenre) {
				t.Fatalf("genre = %#v, want %#v", gotGenre, tt.wantGenre)
			}
			if !stringPtrEqual(gotLabel, tt.wantLabel) {
				t.Fatalf("label = %#v, want %#v", gotLabel, tt.wantLabel)
			}
		})
	}
}

func TestEffectiveOtherGenreLabelExprUsesManualLabelBeforeSummaryLabel(t *testing.T) {
	expr := effectiveOtherGenreLabelExpr("i", "sm")

	if !strings.Contains(expr, "i.user_other_genre_label") {
		t.Fatalf("effectiveOtherGenreLabelExpr = %q, want user_other_genre_label reference", expr)
	}
	if !strings.Contains(expr, "sm.other_genre_label") {
		t.Fatalf("effectiveOtherGenreLabelExpr = %q, want summary other_genre_label reference", expr)
	}
	if !strings.Contains(expr, "'other'") {
		t.Fatalf("effectiveOtherGenreLabelExpr = %q, want other-genre guard", expr)
	}
}

func stringPtrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

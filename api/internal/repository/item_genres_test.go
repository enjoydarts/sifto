package repository

import (
	"strings"
	"testing"
)

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

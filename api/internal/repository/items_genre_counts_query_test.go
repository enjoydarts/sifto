package repository

import (
	"os"
	"strings"
	"testing"
)

func TestListGenreCountsQueryUsesPositionalGroupBy(t *testing.T) {
	src, err := os.ReadFile("items.go")
	if err != nil {
		t.Fatalf("ReadFile(items.go): %v", err)
	}
	text := string(src)

	if strings.Contains(text, "GROUP BY genre") {
		t.Fatalf("items.go contains GROUP BY genre; want positional GROUP BY for effective genre expression")
	}
	if !strings.Contains(text, "GROUP BY 1") {
		t.Fatalf("items.go missing GROUP BY 1 for genre counts query")
	}
}

func TestAppendSummaryTopicMatchExpandsCanonicalTopic(t *testing.T) {
	match, args := appendSummaryTopicMatch([]any{"user-id"}, "AI", "sm")
	if !strings.Contains(match, "sm.topics") || !strings.Contains(match, "sm.genre = ANY($3::text[])") {
		t.Fatalf("match = %q", match)
	}
	if len(args) != 3 {
		t.Fatalf("args = %#v, want user, labels, genres", args)
	}
}

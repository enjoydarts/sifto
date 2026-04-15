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

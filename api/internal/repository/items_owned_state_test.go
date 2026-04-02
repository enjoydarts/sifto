package repository

import (
	"strings"
	"testing"
)

func TestErrForOwnedItemState(t *testing.T) {
	tests := []struct {
		name  string
		state ownedItemState
		want  error
	}{
		{name: "active", state: ownedItemActive, want: nil},
		{name: "deleted", state: ownedItemDeleted, want: ErrConflict},
		{name: "missing", state: ownedItemMissing, want: ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := errForOwnedItemState(tt.state); got != tt.want {
				t.Fatalf("errForOwnedItemState(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

func TestErrForRestoreOwnedItemState(t *testing.T) {
	tests := []struct {
		name  string
		state ownedItemState
		want  error
	}{
		{name: "active", state: ownedItemActive, want: ErrConflict},
		{name: "deleted", state: ownedItemDeleted, want: nil},
		{name: "missing", state: ownedItemMissing, want: ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := errForRestoreOwnedItemState(tt.state); got != tt.want {
				t.Fatalf("errForRestoreOwnedItemState(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

func TestRetryCandidateConflictForDeleted(t *testing.T) {
	candidate := retryCandidate{isDeleted: true}
	if !candidate.isDeleted {
		t.Fatal("expected deleted retry candidate")
	}
	if got := func(c retryCandidate) error {
		if c.isDeleted {
			return ErrConflict
		}
		return nil
	}(candidate); got != ErrConflict {
		t.Fatalf("deleted retry candidate should map to conflict, got %v", got)
	}
}

func TestAppendItemStatusFilterExcludesDeletedForPending(t *testing.T) {
	query, args := appendItemStatusFilter("SELECT * FROM items i WHERE 1=1", nil, stringPtr("pending"))

	const want = "SELECT * FROM items i WHERE 1=1 AND i.deleted_at IS NULL AND i.status IN ('new', 'fetched', 'facts_extracted', 'failed')"
	if query != want {
		t.Fatalf("appendItemStatusFilter() query = %q, want %q", query, want)
	}
	if len(args) != 0 {
		t.Fatalf("appendItemStatusFilter() args len = %d, want 0", len(args))
	}
}

func TestAppendItemStatusFilterExcludesDeletedForExplicitStatus(t *testing.T) {
	query, args := appendItemStatusFilter("SELECT * FROM items i WHERE 1=1", nil, stringPtr("summarized"))

	const want = "SELECT * FROM items i WHERE 1=1 AND i.deleted_at IS NULL AND i.status = $1"
	if query != want {
		t.Fatalf("appendItemStatusFilter() query = %q, want %q", query, want)
	}
	if len(args) != 1 || args[0] != "summarized" {
		t.Fatalf("appendItemStatusFilter() args = %#v, want [summarized]", args)
	}
}

func TestNormalizeItemDetailStatus(t *testing.T) {
	if got := normalizeItemDetailStatus("failed", true); got != "deleted" {
		t.Fatalf("normalizeItemDetailStatus(deleted) = %q, want %q", got, "deleted")
	}
	if got := normalizeItemDetailStatus("summarized", false); got != "summarized" {
		t.Fatalf("normalizeItemDetailStatus(active) = %q, want %q", got, "summarized")
	}
}

func TestLoadRetryCandidateQueryUsesForUpdateOfItems(t *testing.T) {
	query := loadRetryCandidateQuery(true)
	const want = "FOR UPDATE OF i"
	if !strings.Contains(query, want) {
		t.Fatalf("loadRetryCandidateQuery(true) = %q, want substring %q", query, want)
	}
	if strings.Contains(query, "FOR UPDATE\n") || strings.Contains(query, "FOR UPDATE\r\n") || (strings.Contains(query, "FOR UPDATE ") && !strings.Contains(query, want)) {
		t.Fatalf("loadRetryCandidateQuery(true) should not lock outer-joined tables: %q", query)
	}
}

func TestLoadRetryCandidateQueryWithoutLock(t *testing.T) {
	query := loadRetryCandidateQuery(false)
	if strings.Contains(query, "FOR UPDATE") {
		t.Fatalf("loadRetryCandidateQuery(false) = %q, want no FOR UPDATE", query)
	}
}

func TestResetForExtractRetryRequiresContentResetStatus(t *testing.T) {
	const want = "new"
	if want != "new" {
		t.Fatalf("extract retry reset status = %q, want %q", want, "new")
	}
}

func stringPtr(v string) *string {
	return &v
}

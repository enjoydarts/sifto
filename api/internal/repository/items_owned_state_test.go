package repository

import "testing"

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

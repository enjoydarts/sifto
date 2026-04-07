package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

func TestNormalizeBulkItemIDs(t *testing.T) {
	got := normalizeBulkItemIDs([]string{" item-1 ", "", "item-2", "item-1", "   ", "item-3"})
	want := []string{"item-1", "item-2", "item-3"}

	if len(got) != len(want) {
		t.Fatalf("len(normalizeBulkItemIDs()) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeBulkItemIDs()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRunRetryBulk(t *testing.T) {
	result := runRetryBulk(
		context.Background(),
		[]string{"item-1", "item-2", "item-3", "item-4"},
		func(_ context.Context, itemID string) (retryBulkCandidate, error) {
			switch itemID {
			case "item-1":
				return retryBulkCandidate{ID: "item-1", SourceID: "source-1", URL: "https://example.com/1"}, nil
			case "item-2":
				return retryBulkCandidate{}, repository.ErrConflict
			case "item-3":
				return retryBulkCandidate{ID: "item-3", SourceID: "source-3", URL: "https://example.com/3"}, nil
			default:
				return retryBulkCandidate{}, repository.ErrNotFound
			}
		},
		func(_ context.Context, item retryBulkCandidate) error {
			if item.ID == "item-3" {
				return errors.New("publisher down")
			}
			return nil
		},
	)

	if result.QueuedCount != 1 {
		t.Fatalf("QueuedCount = %d, want 1", result.QueuedCount)
	}
	if result.SkippedCount != 3 {
		t.Fatalf("SkippedCount = %d, want 3", result.SkippedCount)
	}
	if len(result.ItemIDs) != 4 {
		t.Fatalf("len(ItemIDs) = %d, want 4", len(result.ItemIDs))
	}
}

func TestRunDeleteBulk(t *testing.T) {
	result := runDeleteBulk(
		context.Background(),
		[]string{"item-1", "item-2", "item-3"},
		func(_ context.Context, itemID string) error {
			switch itemID {
			case "item-1":
				return nil
			case "item-2":
				return repository.ErrConflict
			default:
				return repository.ErrNotFound
			}
		},
	)

	if result.UpdatedCount != 1 {
		t.Fatalf("UpdatedCount = %d, want 1", result.UpdatedCount)
	}
	if result.SkippedCount != 2 {
		t.Fatalf("SkippedCount = %d, want 2", result.SkippedCount)
	}
	if len(result.ItemIDs) != 3 {
		t.Fatalf("len(ItemIDs) = %d, want 3", len(result.ItemIDs))
	}
}

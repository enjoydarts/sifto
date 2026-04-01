package repository

import (
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapPromptTemplateVersionWriteError(t *testing.T) {
	t.Parallel()

	if got := mapPromptTemplateVersionWriteError(pgx.ErrNoRows); got != ErrInvalidState {
		t.Fatalf("mapPromptTemplateVersionWriteError(ErrNoRows) = %v, want %v", got, ErrInvalidState)
	}

	if got := mapPromptTemplateVersionWriteError(&pgconn.PgError{Code: "23505"}); got != ErrConflict {
		t.Fatalf("mapPromptTemplateVersionWriteError(unique violation) = %v, want %v", got, ErrConflict)
	}

	if got := mapPromptTemplateVersionWriteError(&pgconn.PgError{Code: "22001"}); got != nil {
		t.Fatalf("mapPromptTemplateVersionWriteError(non-mapped error) = %v, want nil", got)
	}
}

func TestShouldReplacePromptExperimentArms(t *testing.T) {
	t.Parallel()

	if shouldReplacePromptExperimentArms(nil) {
		t.Fatal("nil arms should keep existing arm configuration")
	}

	if !shouldReplacePromptExperimentArms([]PromptExperimentArmInput{}) {
		t.Fatal("empty arms slice should clear existing arm configuration")
	}

	if !shouldReplacePromptExperimentArms([]PromptExperimentArmInput{{VersionID: "v1", Weight: 100}}) {
		t.Fatal("non-empty arms slice should replace existing arm configuration")
	}
}

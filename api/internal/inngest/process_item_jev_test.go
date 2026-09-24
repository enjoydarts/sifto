package inngest

import (
	"context"
	"errors"
	"testing"

	"github.com/enjoydarts/sifto/api/internal/service"
)

func TestExecuteJevPrecheckWithoutConfiguredClientUsesExistingFlow(t *testing.T) {
	userID := "user-1"
	called := false
	accepted := executeJevPrecheck(context.Background(), processItemDeps{}, jevPrecheckConfig{
		UserID: &userID,
		Evaluate: func(context.Context, string) (*service.JevEvaluation, error) {
			called = true
			return nil, nil
		},
	})
	if accepted {
		t.Fatal("accepted = true, want false")
	}
	if called {
		t.Fatal("Jev evaluation called without configured client")
	}
}

func TestClassifyJevEscalationUsesStableEnums(t *testing.T) {
	if got := classifyJevEscalation(context.DeadlineExceeded); got != service.JevEscalationTimeout {
		t.Fatalf("deadline reason = %q", got)
	}
	if got := classifyJevEscalation(errors.New("Jev HTTP status=503")); got != service.JevEscalationHTTPError {
		t.Fatalf("http reason = %q", got)
	}
	if got := classifyJevEscalation(errors.New("Jev request: connection refused")); got != service.JevEscalationHTTPError {
		t.Fatalf("request reason = %q", got)
	}
	if got := classifyJevEscalation(errors.New("decode Jev response")); got != service.JevEscalationSchemaError {
		t.Fatalf("schema reason = %q", got)
	}
}

package inngest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/service"
)

func TestConditionalQualityGateRoutes(t *testing.T) {
	result := func(decision service.JevDecision, reason service.JevEscalationReason) *jevPrecheckStepResult {
		return &jevPrecheckStepResult{Gate: service.JevGateResult{Decision: decision, EscalationReason: reason}}
	}
	accepted := result(service.JevDecisionAccepted, "")
	for _, tc := range []struct {
		name     string
		jev, d1  *jevPrecheckStepResult
		provider string
		called   bool
	}{
		{"Jev accepted", accepted, accepted, "jev", false},
		{"confidence rescued", result(service.JevDecisionEscalated, service.JevEscalationLowConfidence), accepted, "d1", true},
		{"dimension rescued", result(service.JevDecisionEscalated, service.JevEscalationLowDimensionScore), accepted, "d1", true},
		{"D1 escalates", result(service.JevDecisionEscalated, service.JevEscalationLowConfidence), result(service.JevDecisionEscalated, service.JevEscalationLowDimensionScore), "", true},
		{"D1 error", result(service.JevDecisionEscalated, service.JevEscalationLowConfidence), result(service.JevDecisionError, service.JevEscalationTimeout), "", true},
		{"D1 missing", result(service.JevDecisionEscalated, service.JevEscalationLowConfidence), nil, "", true},
		{"D1 skipped", result(service.JevDecisionEscalated, service.JevEscalationLowConfidence), &jevPrecheckStepResult{Skipped: true}, "", true},
		{"Jev error", result(service.JevDecisionError, service.JevEscalationLowConfidence), accepted, "", false},
		{"Jev missing", nil, accepted, "", false},
		{"Jev skipped", &jevPrecheckStepResult{Skipped: true}, accepted, "", false},
		{"critical", result(service.JevDecisionEscalated, service.JevEscalationCriticalDimensionLow), accepted, "", false},
		{"unsupported", result(service.JevDecisionEscalated, service.JevEscalationUnsupportedClaimRisk), accepted, "", false},
		{"contradiction", result(service.JevDecisionEscalated, service.JevEscalationContradictionRisk), accepted, "", false},
		{"numeric", result(service.JevDecisionEscalated, service.JevEscalationEntityNumericMismatchRisk), accepted, "", false},
		{"aggregate", result(service.JevDecisionEscalated, service.JevEscalationLowScore), accepted, "", false},
		{"unknown", result(service.JevDecisionEscalated, "future_reason"), accepted, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			provider, used := selectQualityGate(tc.jev, func() *jevPrecheckStepResult { calls++; return tc.d1 })
			if provider != tc.provider || used != tc.called || (calls == 1) != tc.called || calls > 1 {
				t.Fatalf("provider=%q used=%v calls=%d; want provider=%q called=%v", provider, used, calls, tc.provider, tc.called)
			}
		})
	}
}

func TestD1GateUsesOneRequestAndTenSecondDeadline(t *testing.T) {
	key, userID := "test-key", "user"
	for _, failure := range []error{context.DeadlineExceeded, &service.JevHTTPError{Provider: "D1", StatusCode: 429}, &service.JevHTTPError{Provider: "D1", StatusCode: 503}} {
		calls := 0
		result, err := runQualityPrecheckStep(context.Background(), stubD1ShadowKeyProvider{key: &key}, jevPrecheckConfig{
			Provider: "d1", UserID: &userID, Timeout: d1GateTimeout, StepName: "d1-gate-test",
			Evaluate: func(ctx context.Context, gotKey string) (*service.JevEvaluation, error) {
				calls++
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > 10*time.Second || gotKey != key {
					t.Fatalf("deadline=%v key matches=%v", ok, gotKey == key)
				}
				return nil, failure
			},
		})
		if err != nil || calls != 1 || result.Gate.Decision != service.JevDecisionError {
			t.Fatalf("result=%#v err=%v calls=%d", result, err, calls)
		}
	}
}

func TestD1GateTimeoutCancelsEvaluation(t *testing.T) {
	key, userID := "test-key", "user"
	started := time.Now()
	result, err := runQualityPrecheckStep(context.Background(), stubD1ShadowKeyProvider{key: &key}, jevPrecheckConfig{
		Provider: "d1", UserID: &userID, Timeout: 20 * time.Millisecond, StepName: "d1-timeout-test",
		Evaluate: func(ctx context.Context, _ string) (*service.JevEvaluation, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	})
	if err != nil || result.EscalationReason != "timeout" || time.Since(started) > time.Second {
		t.Fatalf("result=%#v err=%v elapsed=%s", result, err, time.Since(started))
	}
}

func TestD1GateWithoutKeyNeverEvaluates(t *testing.T) {
	userID := "user"
	for _, keys := range []stubD1ShadowKeyProvider{{}, {err: errors.New("key lookup failed")}} {
		result, err := runQualityPrecheckStep(context.Background(), keys, jevPrecheckConfig{
			Provider: "d1", UserID: &userID, Timeout: d1GateTimeout, StepName: "d1-key-test",
			Evaluate: func(context.Context, string) (*service.JevEvaluation, error) {
				t.Fatal("evaluated without key")
				return nil, nil
			},
		})
		if err != nil || (!result.Skipped && result.Gate.Decision != service.JevDecisionError) {
			t.Fatalf("result=%#v err=%v", result, err)
		}
	}
}

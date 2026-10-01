package inngest

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/service"
)

func TestD1ShadowRecoversFromTimeoutWithoutChangingCandidate(t *testing.T) {
	calls := 0
	var waits []time.Duration
	config := jevPrecheckConfig{StepName: "d1-test", Catalog: service.JevCatalog{GatePolicy: service.JevGatePolicy{Version: "test"}}, Evaluate: func(ctx context.Context, key string) (*service.JevEvaluation, error) {
		if key != "key" {
			t.Fatalf("key = %q", key)
		}
		calls++
		if calls == 1 {
			return nil, fmt.Errorf("D1 request: %w", context.DeadlineExceeded)
		}
		return &service.JevEvaluation{Dimensions: map[string]service.JevDimension{"coverage": {Score: 1, Confidence: 1}}}, nil
	}}
	result, err := runD1ShadowChecks(context.Background(), config, "key", func(_ context.Context, _ string, d time.Duration) { waits = append(waits, d) })
	if err != nil || result.Gate.Decision != service.JevDecisionAccepted || calls != 2 || len(waits) != 1 || waits[0] != time.Minute {
		t.Fatalf("result=%#v err=%v calls=%d waits=%v", result, err, calls, waits)
	}
}

func TestD1ShadowBoundsRetriesAndKeepsFinalTimeout(t *testing.T) {
	calls := 0
	var waits []time.Duration
	config := jevPrecheckConfig{StepName: "d1-test", Evaluate: func(context.Context, string) (*service.JevEvaluation, error) {
		calls++
		return nil, context.DeadlineExceeded
	}}
	result, err := runD1ShadowChecks(context.Background(), config, "key", func(_ context.Context, _ string, d time.Duration) { waits = append(waits, d) })
	if err != nil || calls != 3 || len(waits) != 2 || waits[0] != time.Minute || waits[1] != 2*time.Minute || result.EscalationReason != "timeout" {
		t.Fatalf("result=%#v err=%v calls=%d waits=%v", result, err, calls, waits)
	}
}

func TestD1ShadowDoesNotRetryPermanentErrors(t *testing.T) {
	for _, failure := range []error{&service.JevHTTPError{Provider: "D1", StatusCode: 401}, errors.New("invalid D1 response"), context.Canceled} {
		calls := 0
		config := jevPrecheckConfig{StepName: "d1-test", Evaluate: func(context.Context, string) (*service.JevEvaluation, error) {
			calls++
			return nil, failure
		}}
		_, err := runD1ShadowChecks(context.Background(), config, "key", func(context.Context, string, time.Duration) { t.Fatal("permanent error retried") })
		if err != nil || calls != 1 {
			t.Fatalf("error=%v calls=%d err=%v", failure, calls, err)
		}
	}
}

func TestD1ShadowHonorsRateLimitCooldown(t *testing.T) {
	calls := 0
	config := jevPrecheckConfig{StepName: "d1-test", Evaluate: func(context.Context, string) (*service.JevEvaluation, error) {
		calls++
		if calls == 1 {
			return nil, &service.JevHTTPError{Provider: "D1", StatusCode: 429, RetryAfter: 3 * time.Minute}
		}
		return &service.JevEvaluation{Dimensions: map[string]service.JevDimension{"coverage": {Score: 1}}}, nil
	}}
	_, err := runD1ShadowChecks(context.Background(), config, "key", func(_ context.Context, _ string, d time.Duration) {
		if d != 3*time.Minute {
			t.Fatalf("cooldown = %s", d)
		}
	})
	if err != nil || calls != 2 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestD1ShadowStopsWhenContextCanceledDuringCooldown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	config := jevPrecheckConfig{StepName: "d1-test", Evaluate: func(context.Context, string) (*service.JevEvaluation, error) {
		calls++
		return nil, context.DeadlineExceeded
	}}
	_, err := runD1ShadowChecks(ctx, config, "key", func(context.Context, string, time.Duration) { cancel() })
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

type stubD1ShadowKeyProvider struct {
	key *string
	err error
}

func (s stubD1ShadowKeyProvider) GetAPIKey(context.Context, string, string) (*string, error) {
	return s.key, s.err
}

func TestD1ShadowEnqueuesCandidateWithoutWaitingForEvaluation(t *testing.T) {
	key := "secret"
	data := service.D1ShadowEventData{ItemID: "item", UserID: "user", Kind: "facts", Content: "body", Facts: []string{"fact"}}
	sends := 0
	queued, err := enqueueD1Shadow(context.Background(), stubD1ShadowKeyProvider{key: &key}, data, func(_ context.Context, got service.D1ShadowEventData) error {
		sends++
		if got.ItemID != data.ItemID || got.Content != "body" || got.Facts[0] != "fact" {
			t.Fatalf("candidate = %#v", got)
		}
		return nil
	})
	if err != nil || !queued || sends != 1 {
		t.Fatalf("queued=%v sends=%d err=%v", queued, sends, err)
	}
}

func TestD1ShadowDoesNotEnqueueWithoutKey(t *testing.T) {
	queued, err := enqueueD1Shadow(context.Background(), stubD1ShadowKeyProvider{}, service.D1ShadowEventData{}, func(context.Context, service.D1ShadowEventData) error {
		t.Fatal("queued without key")
		return nil
	})
	if queued || err != nil {
		t.Fatalf("queued=%v err=%v", queued, err)
	}
}

func TestD1ShadowReportsEnqueueFailure(t *testing.T) {
	key := "secret"
	failure := errors.New("event service unavailable")
	queued, err := enqueueD1Shadow(context.Background(), stubD1ShadowKeyProvider{key: &key}, service.D1ShadowEventData{}, func(context.Context, service.D1ShadowEventData) error { return failure })
	if queued || !errors.Is(err, failure) {
		t.Fatalf("queued=%v err=%v", queued, err)
	}
}

func TestD1RetryDelayOnlyRetriesTransientFailures(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want time.Duration
	}{
		{&service.JevHTTPError{StatusCode: 429}, 2 * time.Minute},
		{&service.JevHTTPError{StatusCode: 503}, time.Minute},
		{&service.JevHTTPError{StatusCode: 400}, 0},
		{&service.JevHTTPError{StatusCode: 403}, 0},
		{errors.New("invalid D1 answer"), 0},
		{context.Canceled, 0},
	} {
		if got := d1RetryDelay(tc.err, 0); got != tc.want {
			t.Fatalf("error=%v delay=%s want=%s", tc.err, got, tc.want)
		}
	}
}

func TestD1ShadowSkipsKeyRemovedWhileQueued(t *testing.T) {
	config := jevPrecheckConfig{StepName: "d1-test", Evaluate: func(context.Context, string) (*service.JevEvaluation, error) {
		return nil, errD1KeyNotConfigured
	}}
	result, err := runD1ShadowChecks(context.Background(), config, "", func(context.Context, string, time.Duration) { t.Fatal("removed key retried") })
	if err != nil || !result.Skipped {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestD1ShadowPreservesKeyConfigurationError(t *testing.T) {
	config := jevPrecheckConfig{StepName: "d1-test", Evaluate: func(context.Context, string) (*service.JevEvaluation, error) {
		return nil, fmt.Errorf("%w: decrypt failed", errD1KeyLookup)
	}}
	result, err := runD1ShadowChecks(context.Background(), config, "", func(context.Context, string, time.Duration) { t.Fatal("configuration error retried") })
	if err != nil || result.EscalationReason != "configuration_error" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

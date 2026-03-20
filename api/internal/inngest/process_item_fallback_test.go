package inngest

import "testing"

func TestIsTransientLLMWorkerError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "rate limit", err: assertErr("worker /summarize: status 500 detail=summarize failed: openrouter chat.completions failed status=429 body={\"error\":{\"message\":\"Rate limit reached\"}}"), want: true},
		{name: "provider 502", err: assertErr("worker /extract-facts: status 500 detail=extract_facts failed: openrouter chat.completions failed: empty choices body={\"error\":{\"message\":\"Provider returned error\",\"code\":502}}"), want: true},
		{name: "timeout", err: assertErr("worker /summarize: context deadline exceeded"), want: true},
		{name: "temporary overload", err: assertErr("worker /summarize: upstream provider overload"), want: true},
		{name: "deprecated model 404", err: assertErr("worker /extract-facts: status 500 detail=extract_facts failed: openrouter chat.completions failed status=404 body={\"error\":{\"message\":\"Hunter Alpha was a stealth model revealed on March 18th as an early testing version of MiMo-V2-Pro. Find it here: https://openrouter.ai/xiaomi/mimo-v2-pro\",\"code\":404}}"), want: true},
		{name: "structured parse failed with snippet", err: assertErr("worker /summarize: status 500 detail=summarize failed: openrouter summarize parse failed: response_snippet="), want: true},
		{name: "parse failed", err: assertErr("worker /extract-facts: status 500 detail=openrouter extract_facts parse failed"), want: false},
		{name: "capability missing", err: assertErr("model missing required capability for facts"), want: false},
	}
	for _, tt := range tests {
		if got := isTransientLLMWorkerError(tt.err); got != tt.want {
			t.Fatalf("%s: isTransientLLMWorkerError(%v) = %v, want %v", tt.name, tt.err, got, tt.want)
		}
	}
}

func TestCanUseLLMFallbackForAttempt(t *testing.T) {
	tests := []struct {
		name          string
		primaryModel  *string
		fallbackModel *string
		err           error
		want          bool
	}{
		{
			name:          "transient and different fallback",
			primaryModel:  strptr("openrouter::google/gemini-2.5-flash"),
			fallbackModel: strptr("openrouter::openai/gpt-oss-120b"),
			err:           assertErr("worker /summarize: status 500 detail=summarize failed: openrouter chat.completions failed status=429"),
			want:          true,
		},
		{
			name:          "same resolved model does not fallback",
			primaryModel:  strptr("openrouter::openai/gpt-oss-120b"),
			fallbackModel: strptr("openrouter::openai/gpt-oss-120b"),
			err:           assertErr("worker /summarize: status 500 detail=summarize failed: openrouter chat.completions failed status=429"),
			want:          false,
		},
		{
			name:          "non transient error does not fallback",
			primaryModel:  strptr("openrouter::google/gemini-2.5-flash"),
			fallbackModel: strptr("openrouter::openai/gpt-oss-120b"),
			err:           assertErr("worker /summarize: status 500 detail=summarize failed: parse failed"),
			want:          false,
		},
		{
			name:          "structured parse failure with snippet falls back",
			primaryModel:  strptr("openrouter::google/gemini-2.5-flash"),
			fallbackModel: strptr("openrouter::openai/gpt-oss-120b"),
			err:           assertErr("worker /summarize: status 500 detail=summarize failed: openrouter summarize parse failed: response_snippet="),
			want:          true,
		},
	}
	for _, tt := range tests {
		if got := canUseLLMFallbackForAttempt(tt.primaryModel, tt.fallbackModel, tt.err); got != tt.want {
			t.Fatalf("%s: canUseLLMFallbackForAttempt(%v, %v, %v) = %v, want %v", tt.name, tt.primaryModel, tt.fallbackModel, tt.err, got, tt.want)
		}
	}
}

func TestExecutionFailedModel(t *testing.T) {
	resolved := "openrouter::google/gemini-2.5-flash"
	runtimeModel := "openrouter::openai/gpt-oss-120b"

	tests := []struct {
		name     string
		runtime  *llmRuntime
		resolved *string
		want     *string
	}{
		{
			name:     "prefers runtime model",
			runtime:  &llmRuntime{Model: &runtimeModel},
			resolved: &resolved,
			want:     &runtimeModel,
		},
		{
			name:     "falls back to resolved model",
			runtime:  nil,
			resolved: &resolved,
			want:     &resolved,
		},
		{
			name:     "empty model returns nil",
			runtime:  &llmRuntime{Model: strptr(" ")},
			resolved: nil,
			want:     nil,
		},
	}

	for _, tt := range tests {
		got := executionFailedModel(tt.runtime, tt.resolved)
		switch {
		case tt.want == nil && got != nil:
			t.Fatalf("%s: got %v, want nil", tt.name, *got)
		case tt.want != nil && got == nil:
			t.Fatalf("%s: got nil, want %v", tt.name, *tt.want)
		case tt.want != nil && got != nil && *got != *tt.want:
			t.Fatalf("%s: got %v, want %v", tt.name, *got, *tt.want)
		}
	}
}

func TestShouldRetryExtractBody(t *testing.T) {
	if !shouldRetryExtractBody(0, assertErr("worker /extract-body: status 422 detail=Failed to extract body")) {
		t.Fatal("first extract-body failure should retry")
	}
	if !shouldRetryExtractBody(1, assertErr("worker /extract-body: status 422 detail=Failed to extract body")) {
		t.Fatal("second extract-body failure should retry")
	}
	if shouldRetryExtractBody(2, assertErr("worker /extract-body: status 422 detail=Failed to extract body")) {
		t.Fatal("third extract-body failure should stop retrying")
	}
	if shouldRetryExtractBody(0, nil) {
		t.Fatal("nil error should not retry")
	}
}

func assertErr(msg string) error { return transientErr(msg) }

func strptr(v string) *string { return &v }

type transientErr string

func (e transientErr) Error() string { return string(e) }

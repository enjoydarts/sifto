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
		{name: "parse failed", err: assertErr("worker /extract-facts: status 500 detail=openrouter extract_facts parse failed"), want: false},
		{name: "capability missing", err: assertErr("model missing required capability for facts"), want: false},
	}
	for _, tt := range tests {
		if got := isTransientLLMWorkerError(tt.err); got != tt.want {
			t.Fatalf("%s: isTransientLLMWorkerError(%v) = %v, want %v", tt.name, tt.err, got, tt.want)
		}
	}
}

func assertErr(msg string) error { return transientErr(msg) }

type transientErr string

func (e transientErr) Error() string { return string(e) }

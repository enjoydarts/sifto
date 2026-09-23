package inngest

import "testing"

func TestProcessItemStatusIsTerminal(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{status: "summarized", want: true},
		{status: "new", want: false},
		{status: "fetched", want: false},
		{status: "facts_extracted", want: false},
		{status: "failed", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := processItemStatusIsTerminal(tt.status); got != tt.want {
				t.Fatalf("processItemStatusIsTerminal(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

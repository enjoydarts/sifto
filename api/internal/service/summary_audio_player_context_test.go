package service

import (
	"context"
	"testing"
)

func TestSummaryAudioRequestContextIgnoresParentCancellation(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	detached := SummaryAudioRequestContext(parent)

	cancel()

	select {
	case <-detached.Done():
		t.Fatal("detached context should not be canceled when parent is canceled")
	default:
	}
	if err := detached.Err(); err != nil {
		t.Fatalf("detached context err = %v, want nil", err)
	}
}

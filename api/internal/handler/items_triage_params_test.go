package handler

import (
	"net/url"
	"testing"
)

func TestBuildTriageQueueParamsExcludesReadItems(t *testing.T) {
	params, err := buildTriageQueueParams(url.Values{
		"window":           []string{"24h"},
		"size":             []string{"20"},
		"diversify_topics": []string{"true"},
		"exclude_later":    []string{"true"},
	})
	if err != nil {
		t.Fatalf("buildTriageQueueParams returned error: %v", err)
	}
	if !params.ExcludeRead {
		t.Fatalf("ExcludeRead = false, want true")
	}
}

func TestBuildTriageQueueParamsSupportsAllMode(t *testing.T) {
	params, err := buildTriageQueueParams(url.Values{
		"mode": []string{"all"},
	})
	if err != nil {
		t.Fatalf("buildTriageQueueParams returned error: %v", err)
	}
	if params.Window != "all" {
		t.Fatalf("Window = %q, want all", params.Window)
	}
	if !params.ExcludeRead {
		t.Fatalf("ExcludeRead = false, want true")
	}
}

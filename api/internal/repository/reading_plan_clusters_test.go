package repository

import (
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func TestShouldClusterTriageRejectsBroadTopicMatch(t *testing.T) {
	now := time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC)
	seed := model.Item{
		ID:            "seed",
		SourceID:      "source-a",
		SummaryTopics: []string{"OpenAI", "GPT-5"},
		CreatedAt:     now,
	}
	cand := model.Item{
		ID:            "cand",
		SourceID:      "source-b",
		SummaryTopics: []string{"OpenAI", "launch"},
		CreatedAt:     now.Add(8 * time.Hour),
	}

	if got := shouldClusterTriage(seed, cand, 0.61, []string{"OpenAI announces GPT-5 coding agent"}, []string{"OpenAI hires new policy lead"}); got {
		t.Fatalf("shouldClusterTriage returned true for same-company different-event case")
	}
}

func TestShouldClusterTriageAcceptsSameEventWithSharedFacts(t *testing.T) {
	now := time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC)
	seed := model.Item{
		ID:            "seed",
		SourceID:      "source-a",
		SummaryTopics: []string{"OpenAI", "GPT-5"},
		CreatedAt:     now,
	}
	cand := model.Item{
		ID:            "cand",
		SourceID:      "source-b",
		SummaryTopics: []string{"OpenAI", "GPT-5"},
		CreatedAt:     now.Add(90 * time.Minute),
	}

	if got := shouldClusterTriage(seed, cand, 0.61, []string{"OpenAI launches GPT-5 coding agent", "Available in ChatGPT and API"}, []string{"OpenAI launches GPT-5 coding agent", "Rolls out to ChatGPT and API users"}); !got {
		t.Fatalf("shouldClusterTriage returned false for same-event coverage")
	}
}

func TestBuildTriageClustersUsesTriageSpecificRule(t *testing.T) {
	now := time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC)
	scoreA := 0.92
	scoreB := 0.87
	scoreC := 0.84
	items := []model.Item{
		{ID: "a", SourceID: "source-a", SummaryTopics: []string{"OpenAI", "GPT-5"}, CreatedAt: now, SummaryScore: &scoreA},
		{ID: "b", SourceID: "source-b", SummaryTopics: []string{"OpenAI", "GPT-5"}, CreatedAt: now.Add(30 * time.Minute), SummaryScore: &scoreB},
		{ID: "c", SourceID: "source-c", SummaryTopics: []string{"OpenAI", "policy"}, CreatedAt: now.Add(45 * time.Minute), SummaryScore: &scoreC},
	}
	embByID := map[string][]float64{
		"a": {1, 0, 0},
		"b": {0.61, 0.79, 0},
		"c": {0.61, 0, 0.79},
	}
	factsByID := map[string][]string{
		"a": {"OpenAI launches GPT-5 coding agent"},
		"b": {"OpenAI launches GPT-5 coding agent"},
		"c": {"OpenAI hires policy lead"},
	}

	clusters := buildTriageClusters(items, embByID, factsByID, nil)
	if len(clusters) != 1 {
		t.Fatalf("len(clusters) = %d, want 1", len(clusters))
	}
	if got := len(clusters[0].Items); got != 2 {
		t.Fatalf("len(clusters[0].Items) = %d, want 2", got)
	}
	if clusters[0].Representative.ID != "a" {
		t.Fatalf("representative = %q, want a", clusters[0].Representative.ID)
	}
}

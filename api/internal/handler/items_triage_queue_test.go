package handler

import (
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func TestBuildTriageQueueEntriesMixesBundlesAndSingletons(t *testing.T) {
	now := time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC)
	item1 := model.Item{ID: "item-1", CreatedAt: now}
	item2 := model.Item{ID: "item-2", CreatedAt: now.Add(-time.Minute)}
	item3 := model.Item{ID: "item-3", CreatedAt: now.Add(-2 * time.Minute)}

	resp := &model.ReadingPlanResponse{
		Items: []model.Item{item1, item2, item3},
		Clusters: []model.ReadingPlanCluster{
			{
				ID:             "bundle-1",
				Label:          "topic",
				Size:           2,
				MaxSimilarity:  0.82,
				Representative: item1,
				Items:          []model.Item{item1, item2},
			},
		},
	}

	entries := buildTriageQueueEntries(resp)
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].EntryType != "bundle" {
		t.Fatalf("entries[0].EntryType = %q, want bundle", entries[0].EntryType)
	}
	if entries[0].Bundle == nil || entries[0].Bundle.Size != 2 {
		t.Fatalf("entries[0].Bundle = %#v, want size=2 bundle", entries[0].Bundle)
	}
	if entries[1].EntryType != "item" {
		t.Fatalf("entries[1].EntryType = %q, want item", entries[1].EntryType)
	}
	if entries[1].Item == nil || entries[1].Item.ID != "item-3" {
		t.Fatalf("entries[1].Item = %#v, want item-3", entries[1].Item)
	}
}

func TestBuildTriageQueueEntriesUsesRepresentativeSummaryAndSharedTopics(t *testing.T) {
	now := time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC)
	summary := "Representative summary"
	item1 := model.Item{ID: "item-1", Summary: &summary, SummaryTopics: []string{"OpenAI", "GPT-5"}, CreatedAt: now}
	item2 := model.Item{ID: "item-2", SummaryTopics: []string{"OpenAI", "launch"}, CreatedAt: now.Add(-time.Minute)}

	resp := &model.ReadingPlanResponse{
		Items: []model.Item{item1, item2},
		Clusters: []model.ReadingPlanCluster{
			{
				ID:             "bundle-1",
				Label:          "OpenAI",
				Size:           2,
				MaxSimilarity:  0.91,
				Representative: item1,
				Items:          []model.Item{item1, item2},
			},
		},
	}

	entries := buildTriageQueueEntries(resp)
	if len(entries) != 1 || entries[0].Bundle == nil {
		t.Fatalf("entries = %#v, want single bundle entry", entries)
	}
	if entries[0].Bundle.Summary == nil || *entries[0].Bundle.Summary != summary {
		t.Fatalf("bundle summary = %#v, want %q", entries[0].Bundle.Summary, summary)
	}
	if len(entries[0].Bundle.SharedTopics) != 1 || entries[0].Bundle.SharedTopics[0] != "OpenAI" {
		t.Fatalf("shared topics = %#v, want [OpenAI]", entries[0].Bundle.SharedTopics)
	}
}

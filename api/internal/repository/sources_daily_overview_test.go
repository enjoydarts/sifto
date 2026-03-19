package repository

import (
	"testing"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func TestBuildSourcesDailyOverviewAggregatesCounts(t *testing.T) {
	rows := []model.SourceDailyStats{
		{
			SourceID: "a",
			DailyCounts: []model.SourceDailyCount{
				{Day: "2026-03-01", Count: 2},
				{Day: "2026-03-02", Count: 0},
				{Day: "2026-03-03", Count: 4},
			},
		},
		{
			SourceID: "b",
			DailyCounts: []model.SourceDailyCount{
				{Day: "2026-03-01", Count: 1},
				{Day: "2026-03-02", Count: 3},
				{Day: "2026-03-03", Count: 0},
			},
		},
	}

	got := BuildSourcesDailyOverview(rows)
	if got.Last30DaysTotal != 10 {
		t.Fatalf("Last30DaysTotal = %d, want 10", got.Last30DaysTotal)
	}
	if got.TodayCount != 4 {
		t.Fatalf("TodayCount = %d, want 4", got.TodayCount)
	}
	if got.YesterdayCount != 3 {
		t.Fatalf("YesterdayCount = %d, want 3", got.YesterdayCount)
	}
	if got.ActiveDays30d != 3 {
		t.Fatalf("ActiveDays30d = %d, want 3", got.ActiveDays30d)
	}
	if got.AvgItemsPerActiveDay30 != (10.0 / 3.0) {
		t.Fatalf("AvgItemsPerActiveDay30 = %f, want %f", got.AvgItemsPerActiveDay30, 10.0/3.0)
	}
	if len(got.DailyCounts) != 3 {
		t.Fatalf("len(DailyCounts) = %d, want 3", len(got.DailyCounts))
	}
	if got.DailyCounts[0].Count != 3 || got.DailyCounts[1].Count != 3 || got.DailyCounts[2].Count != 4 {
		t.Fatalf("DailyCounts = %+v, want [3 3 4]", got.DailyCounts)
	}
}

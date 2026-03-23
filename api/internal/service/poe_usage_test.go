package service

import (
	"testing"
	"time"
)

func TestNormalizePoeUsageTimestampSupportsSecondsMillisAndMicros(t *testing.T) {
	tests := []struct {
		name  string
		input int64
		want  time.Time
	}{
		{
			name:  "seconds",
			input: 1704825600,
			want:  time.Unix(1704825600, 0).UTC(),
		},
		{
			name:  "milliseconds",
			input: 1704825600123,
			want:  time.UnixMilli(1704825600123).UTC(),
		},
		{
			name:  "microseconds",
			input: 1704825600123456,
			want:  time.UnixMicro(1704825600123456).UTC(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizePoeUsageTimestamp(tc.input); !got.Equal(tc.want) {
				t.Fatalf("normalizePoeUsageTimestamp(%d) = %s, want %s", tc.input, got.Format(time.RFC3339Nano), tc.want.Format(time.RFC3339Nano))
			}
		})
	}
}

func TestSummarizePoeUsageBuildsTotalsAndModelRollups(t *testing.T) {
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
	entries := []PoeUsageEntry{
		{
			QueryID:    "q1",
			BotName:    "Claude-Sonnet-4.5",
			CreatedAt:  now.Add(-2 * time.Hour),
			CostUSD:    1.25,
			CostPoints: 1200,
			UsageType:  "API",
			ChatName:   "",
			RawCostUSD: "1.25",
			Breakdown:  map[string]string{"Total": "1200 points"},
		},
		{
			QueryID:    "q2",
			BotName:    "Claude-Sonnet-4.5",
			CreatedAt:  now.Add(-4 * time.Hour),
			CostUSD:    0.75,
			CostPoints: 800,
			UsageType:  "Chat",
			ChatName:   "Compare",
			RawCostUSD: "0.75",
			Breakdown:  map[string]string{"Total": "800 points"},
		},
		{
			QueryID:    "q3",
			BotName:    "GPT-5",
			CreatedAt:  now.Add(-26 * time.Hour),
			CostUSD:    0.5,
			CostPoints: 400,
			UsageType:  "API",
			ChatName:   "",
			RawCostUSD: "0.50",
			Breakdown:  map[string]string{"Total": "400 points"},
		},
	}

	summary, byModel := summarizePoeUsage(entries)

	if summary.EntryCount != 3 {
		t.Fatalf("EntryCount = %d, want 3", summary.EntryCount)
	}
	if summary.TotalCostPoints != 2400 {
		t.Fatalf("TotalCostPoints = %d, want 2400", summary.TotalCostPoints)
	}
	if summary.TotalCostUSD != 2.5 {
		t.Fatalf("TotalCostUSD = %v, want 2.5", summary.TotalCostUSD)
	}
	if summary.APIEntryCount != 2 {
		t.Fatalf("APIEntryCount = %d, want 2", summary.APIEntryCount)
	}
	if summary.ChatEntryCount != 1 {
		t.Fatalf("ChatEntryCount = %d, want 1", summary.ChatEntryCount)
	}
	if summary.LatestEntryAt == nil || !summary.LatestEntryAt.Equal(now.Add(-2*time.Hour)) {
		t.Fatalf("LatestEntryAt = %v, want %v", summary.LatestEntryAt, now.Add(-2*time.Hour))
	}

	if len(byModel) != 2 {
		t.Fatalf("len(byModel) = %d, want 2", len(byModel))
	}
	if byModel[0].BotName != "Claude-Sonnet-4.5" {
		t.Fatalf("top model = %q, want Claude-Sonnet-4.5", byModel[0].BotName)
	}
	if byModel[0].EntryCount != 2 {
		t.Fatalf("top model EntryCount = %d, want 2", byModel[0].EntryCount)
	}
	if byModel[0].TotalCostPoints != 2000 {
		t.Fatalf("top model TotalCostPoints = %d, want 2000", byModel[0].TotalCostPoints)
	}
	if byModel[0].TotalCostUSD != 2.0 {
		t.Fatalf("top model TotalCostUSD = %v, want 2.0", byModel[0].TotalCostUSD)
	}
}

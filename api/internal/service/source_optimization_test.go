package service

import "testing"

func TestSourceOptimizationClassifiesKeep(t *testing.T) {
	rec := ClassifySourceOptimization(SourceOptimizationMetrics{
		UnreadBacklog:        6,
		ReadRate:             0.62,
		FavoriteRate:         0.18,
		NotificationOpenRate: 0.35,
		AverageSummaryScore:  0.74,
	})
	if rec.Recommendation != "keep" {
		t.Fatalf("recommendation = %s, want keep", rec.Recommendation)
	}
}

func TestSourceOptimizationClassifiesPrune(t *testing.T) {
	rec := ClassifySourceOptimization(SourceOptimizationMetrics{
		UnreadBacklog:        80,
		ReadRate:             0.05,
		FavoriteRate:         0.0,
		NotificationOpenRate: 0.01,
		AverageSummaryScore:  0.18,
	})
	if rec.Recommendation != "prune" {
		t.Fatalf("recommendation = %s, want prune", rec.Recommendation)
	}
}

func TestSourceOptimizationClassifiesMute(t *testing.T) {
	rec := ClassifySourceOptimization(SourceOptimizationMetrics{
		UnreadBacklog:        40,
		ReadRate:             0.18,
		FavoriteRate:         0.02,
		NotificationOpenRate: 0.0,
		AverageSummaryScore:  0.52,
	})
	if rec.Recommendation != "mute" {
		t.Fatalf("recommendation = %s, want mute", rec.Recommendation)
	}
}

func TestSourceOptimizationClassifiesPromote(t *testing.T) {
	rec := ClassifySourceOptimization(SourceOptimizationMetrics{
		UnreadBacklog:        4,
		ReadRate:             0.88,
		FavoriteRate:         0.24,
		NotificationOpenRate: 0.41,
		AverageSummaryScore:  0.82,
	})
	if rec.Recommendation != "promote" {
		t.Fatalf("recommendation = %s, want promote", rec.Recommendation)
	}
}

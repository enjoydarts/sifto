package service

import (
	"sort"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

type ReviewSchedule struct {
	Stage string
	DueAt time.Time
}

func BuildReviewSchedules(base time.Time) []ReviewSchedule {
	return []ReviewSchedule{
		{Stage: "d1", DueAt: base.Add(24 * time.Hour)},
		{Stage: "d7", DueAt: base.Add(7 * 24 * time.Hour)},
		{Stage: "d30", DueAt: base.Add(30 * 24 * time.Hour)},
	}
}

func RankReviewQueue(items []model.ReviewQueueItem) []model.ReviewQueueItem {
	return RankReviewQueueAt(items, time.Now())
}

func RankReviewQueueAt(items []model.ReviewQueueItem, now time.Time) []model.ReviewQueueItem {
	out := append([]model.ReviewQueueItem(nil), items...)
	sort.SliceStable(out, func(i, j int) bool {
		return reviewQueueScore(out[i], now) > reviewQueueScore(out[j], now)
	})
	return out
}

func reviewQueueScore(item model.ReviewQueueItem, now time.Time) float64 {
	score := 0.0
	switch strings.TrimSpace(item.SourceSignal) {
	case "favorite":
		score += 6
	case "note":
		score += 4
	case "insight":
		score += 3
	default:
		score += 1
	}
	if item.Item.IsFavorite {
		score += 3
	}
	for _, label := range item.ReasonLabels {
		if strings.TrimSpace(label) == "note" {
			score += 2
		}
	}
	if item.LastSurfacedAt != nil {
		hours := now.Sub(*item.LastSurfacedAt).Hours()
		if hours < 6 {
			score -= 5
		} else if hours < 24 {
			score -= 2
		}
	}
	return score
}

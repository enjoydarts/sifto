package service

import (
	"sort"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

type WeeklyReviewInputs struct {
	ReadCount       int
	NoteCount       int
	InsightCount    int
	FavoriteCount   int
	Topics          []model.WeeklyReviewTopic
	MissedHighValue []model.Item
}

func BuildWeeklyReviewSnapshot(userID, weekStart, weekEnd string, inputs WeeklyReviewInputs) model.WeeklyReviewSnapshot {
	topics := append([]model.WeeklyReviewTopic(nil), inputs.Topics...)
	sort.SliceStable(topics, func(i, j int) bool {
		return topics[i].Count > topics[j].Count
	})
	if len(topics) > 5 {
		topics = topics[:5]
	}
	missed := append([]model.Item(nil), inputs.MissedHighValue...)
	if len(missed) > 5 {
		missed = missed[:5]
	}
	return model.WeeklyReviewSnapshot{
		UserID:          userID,
		WeekStart:       weekStart,
		WeekEnd:         weekEnd,
		ReadCount:       inputs.ReadCount,
		NoteCount:       inputs.NoteCount,
		InsightCount:    inputs.InsightCount,
		FavoriteCount:   inputs.FavoriteCount,
		DominantTopics:  topics,
		MissedHighValue: missed,
		CreatedAt:       time.Now(),
	}
}

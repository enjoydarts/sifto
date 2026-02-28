package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/minoru-kitayama/sifto/api/internal/model"
	"github.com/minoru-kitayama/sifto/api/internal/repository"
	"github.com/minoru-kitayama/sifto/api/internal/timeutil"
)

func GreetingByHour(now time.Time) string {
	hour := now.Hour()
	if hour < 11 {
		return "おはようございます"
	}
	if hour < 18 {
		return "こんにちは"
	}
	return "こんばんは"
}

func BuildBriefingToday(
	ctx context.Context,
	itemRepo *repository.ItemRepo,
	streakRepo *repository.ReadingStreakRepo,
	userID string,
	targetDate time.Time,
	size int,
) (*model.BriefingTodayResponse, error) {
	if size < 1 {
		size = 12
	}
	if size > 30 {
		size = 30
	}
	const streakTarget = 3
	start := timeutil.StartOfDayJST(targetDate)
	dateStr := start.Format("2006-01-02")

	plan, err := itemRepo.ReadingPlan(ctx, userID, repository.ReadingPlanParams{
		Window:          "today_jst",
		Size:            size,
		DiversifyTopics: true,
		ExcludeRead:     true,
	})
	if err != nil {
		return nil, err
	}
	stats, err := itemRepo.Stats(ctx, userID)
	if err != nil {
		return nil, err
	}

	yRead := 0
	ySkipped := 0
	streak := 0
	todayRead := 0
	yesterday := start.AddDate(0, 0, -1).Format("2006-01-02")
	if streakRepo != nil {
		if _, streakDays, _, err := streakRepo.GetByUserAndDate(ctx, userID, yesterday); err == nil {
			streak = streakDays
		}
		if readCount, _, _, err := streakRepo.GetByUserAndDate(ctx, userID, dateStr); err == nil {
			todayRead = readCount
		}
	}
	if readCount, unreadCount, err := itemRepo.CountSummarizedReadUnreadOnDateJST(ctx, userID, yesterday); err == nil {
		yRead = readCount
		ySkipped = unreadCount
	}

	items := make([]model.Item, len(plan.Items))
	copy(items, plan.Items)
	sort.SliceStable(items, func(i, j int) bool {
		si := -1.0
		sj := -1.0
		if items[i].SummaryScore != nil {
			si = *items[i].SummaryScore
		}
		if items[j].SummaryScore != nil {
			sj = *items[j].SummaryScore
		}
		if si != sj {
			return si > sj
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	highlight, err := itemRepo.HighlightItems24h(ctx, userID, 0.85, 3)
	if err != nil {
		highlight = nil
	}
	if len(highlight) == 0 {
		highlightCount := minInt(3, len(items))
		highlight = make([]model.Item, 0, highlightCount)
		highlight = append(highlight, items[:highlightCount]...)
	}

	summaryItemIDs := make([]string, 0, len(plan.Clusters)*3)
	for _, c := range plan.Clusters {
		for i, it := range c.Items {
			if i >= 3 {
				break
			}
			summaryItemIDs = append(summaryItemIDs, it.ID)
		}
	}
	summaryMap, err := itemRepo.SummariesByItemIDs(ctx, userID, summaryItemIDs)
	if err != nil {
		summaryMap = map[string]string{}
	}

	clusters := make([]model.BriefingCluster, 0, len(plan.Clusters))
	for _, c := range plan.Clusters {
		var maxScore *float64
		if c.Representative.SummaryScore != nil {
			v := *c.Representative.SummaryScore
			maxScore = &v
		}
		clusters = append(clusters, model.BriefingCluster{
			ID:       c.ID,
			Label:    c.Label,
			Summary:  buildClusterSummary(c.Items, summaryMap),
			MaxScore: maxScore,
			Topics:   c.Representative.SummaryTopics,
			Items:    c.Items,
		})
	}

	streakRemaining := streakTarget - todayRead
	if streakRemaining < 0 {
		streakRemaining = 0
	}
	streakAtRisk := streak > 0 && streakRemaining > 0 && timeutil.NowJST().Hour() >= 18

	return &model.BriefingTodayResponse{
		Date:           dateStr,
		Greeting:       GreetingByHour(timeutil.NowJST()),
		Status:         "ready",
		HighlightItems: highlight,
		Clusters:       clusters,
		Stats: model.BriefingStats{
			TotalUnread:         stats.Unread,
			TodayHighlightCount: len(plan.Items),
			YesterdayRead:       yRead,
			YesterdaySkipped:    ySkipped,
			StreakDays:          streak,
			TodayReadCount:      todayRead,
			StreakTarget:        streakTarget,
			StreakRemaining:     streakRemaining,
			StreakAtRisk:        streakAtRisk,
		},
	}, nil
}

func buildClusterSummary(items []model.Item, summaryMap map[string]string) string {
	if len(items) == 0 {
		return ""
	}
	lines := make([]string, 0, minInt(2, len(items)))
	for i, it := range items {
		if i >= 2 {
			break
		}
		title := strings.TrimSpace(coalesceTitle(it))
		summary := strings.TrimSpace(summaryMap[it.ID])
		if summary == "" {
			continue
		}
		summary = truncateRunes(summary, 120)
		if title != "" {
			lines = append(lines, title+": "+summary)
		} else {
			lines = append(lines, summary)
		}
	}
	return strings.Join(lines, " / ")
}

func coalesceTitle(it model.Item) string {
	if it.TranslatedTitle != nil && strings.TrimSpace(*it.TranslatedTitle) != "" {
		return *it.TranslatedTitle
	}
	if it.Title != nil && strings.TrimSpace(*it.Title) != "" {
		return *it.Title
	}
	return it.URL
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	rs := []rune(s)
	if len(rs) <= max {
		return s
	}
	return string(rs[:max]) + "..."
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

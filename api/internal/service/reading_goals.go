package service

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

const maxActiveReadingGoals = 7

var errReadingGoalLimitExceeded = errors.New("active reading goals limit exceeded")

type ReadingGoalInput struct {
	Title       string
	Description string
	Priority    int
	DueDate     string
}

func NormalizeReadingGoalInput(in ReadingGoalInput) (model.ReadingGoal, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return model.ReadingGoal{}, errors.New("title is required")
	}
	description := strings.TrimSpace(in.Description)
	priority := in.Priority
	if priority < 1 || priority > 5 {
		priority = 3
	}
	var dueDate *time.Time
	if raw := strings.TrimSpace(in.DueDate); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return model.ReadingGoal{}, errors.New("invalid due_date")
		}
		dueDate = &parsed
	}
	return model.ReadingGoal{
		Title:       title,
		Description: description,
		Priority:    priority,
		Status:      "active",
		DueDate:     dueDate,
	}, nil
}

func CanActivateAnotherReadingGoal(goals []model.ReadingGoal, current *model.ReadingGoal) error {
	activeCount := 0
	for _, goal := range goals {
		if goal.Status == "active" {
			activeCount++
		}
	}
	if current != nil && current.Status == "active" {
		return nil
	}
	if activeCount >= maxActiveReadingGoals {
		return errReadingGoalLimitExceeded
	}
	return nil
}

func RankTodayQueueItems(candidates []model.TodayQueueCandidate, goals []model.ReadingGoal, limit int, now time.Time) []model.TodayQueueItem {
	if limit <= 0 {
		limit = 5
	}
	ranked := make([]model.TodayQueueItem, 0, len(candidates))
	for _, candidate := range candidates {
		matchedGoals := matchReadingGoals(candidate.Item, goals)
		reasons := make([]string, 0, 3)
		score := 0.0
		if candidate.Item.PersonalScore != nil {
			score += *candidate.Item.PersonalScore
		}
		if len(matchedGoals) > 0 {
			score += 0.15 + float64(matchedGoals[0].Priority)*0.01
			reasons = append(reasons, "priority goal")
		}
		if candidate.Item.CreatedAt.After(now.Add(-6 * time.Hour)) {
			score += 0.05
			reasons = append(reasons, "fresh")
		}
		if candidate.LastSkippedAt != nil && candidate.LastSkippedAt.After(now.Add(-24*time.Hour)) {
			score -= 0.15
		}
		ranked = append(ranked, model.TodayQueueItem{
			Item:                    candidate.Item,
			EstimatedReadingMinutes: estimateReadingMinutes(candidate.Item),
			ReasonLabels:            reasons,
			MatchedGoals:            matchedGoals,
		})
		ranked[len(ranked)-1].Item.PersonalScore = &score
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		si := 0.0
		sj := 0.0
		if ranked[i].Item.PersonalScore != nil {
			si = *ranked[i].Item.PersonalScore
		}
		if ranked[j].Item.PersonalScore != nil {
			sj = *ranked[j].Item.PersonalScore
		}
		if si != sj {
			return si > sj
		}
		return ranked[i].Item.CreatedAt.After(ranked[j].Item.CreatedAt)
	})

	selected := make([]model.TodayQueueItem, 0, min(limit, len(ranked)))
	seenTopics := map[string]struct{}{}
	remainingDistinctCandidates := func(start int) bool {
		for idx := start; idx < len(ranked); idx++ {
			if !sharesSeenTopic(ranked[idx].Item.SummaryTopics, seenTopics) {
				return true
			}
		}
		return false
	}
	for idx, item := range ranked {
		if len(selected) >= limit {
			break
		}
		if sharesSeenTopic(item.Item.SummaryTopics, seenTopics) && remainingDistinctCandidates(idx+1) {
			continue
		}
		if len(item.ReasonLabels) == 0 {
			item.ReasonLabels = append(item.ReasonLabels, "attention")
		}
		selected = append(selected, item)
		for _, topic := range item.Item.SummaryTopics {
			normalized := strings.ToLower(strings.TrimSpace(topic))
			if normalized != "" {
				seenTopics[normalized] = struct{}{}
			}
		}
	}
	for _, item := range ranked {
		if len(selected) >= limit {
			break
		}
		already := false
		for _, picked := range selected {
			if picked.Item.ID == item.Item.ID {
				already = true
				break
			}
		}
		if already {
			continue
		}
		selected = append(selected, item)
	}
	return selected
}

func matchReadingGoals(item model.Item, goals []model.ReadingGoal) []model.ReadingGoal {
	matched := make([]model.ReadingGoal, 0, len(goals))
	title := strings.ToLower(strings.TrimSpace(derefString(item.Title)))
	reason := strings.ToLower(strings.TrimSpace(derefString(item.RecommendationReason)))
	for _, goal := range goals {
		if goal.Status != "active" {
			continue
		}
		needle := strings.ToLower(strings.TrimSpace(goal.Title))
		if needle == "" {
			continue
		}
		if strings.Contains(title, needle) || strings.Contains(reason, needle) || topicsContain(item.SummaryTopics, needle) {
			matched = append(matched, goal)
		}
	}
	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].Priority != matched[j].Priority {
			return matched[i].Priority > matched[j].Priority
		}
		return matched[i].UpdatedAt.After(matched[j].UpdatedAt)
	})
	return matched
}

func topicsContain(topics []string, needle string) bool {
	for _, topic := range topics {
		if strings.Contains(strings.ToLower(strings.TrimSpace(topic)), needle) {
			return true
		}
	}
	return false
}

func sharesSeenTopic(topics []string, seen map[string]struct{}) bool {
	for _, topic := range topics {
		if _, ok := seen[strings.ToLower(strings.TrimSpace(topic))]; ok {
			return true
		}
	}
	return false
}

func estimateReadingMinutes(item model.Item) int {
	textLength := len(strings.TrimSpace(derefString(item.ContentText)))
	if textLength == 0 {
		textLength = len(strings.TrimSpace(derefString(item.Summary)))
	}
	if textLength == 0 {
		textLength = len(strings.Join(item.SummaryTopics, " "))
	}
	if textLength == 0 {
		return 1
	}
	minutes := textLength / 900
	if minutes < 1 {
		minutes = 1
	}
	if minutes > 12 {
		minutes = 12
	}
	return minutes
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

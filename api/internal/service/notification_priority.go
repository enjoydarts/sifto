package service

type NotificationPriorityInput struct {
	ItemScore           float64
	GoalMatch           bool
	RecentNotifications int
	DuplicateDigestRisk bool
	Sensitivity         string
	DailyCap            int
	ThemeWeight         float64
}

type NotificationPriorityDecision struct {
	Route  string
	Reason string
}

func RouteNotificationPriority(in NotificationPriorityInput) NotificationPriorityDecision {
	score := in.ItemScore
	if in.GoalMatch {
		score += 0.18 * in.ThemeWeight
	}
	if in.DuplicateDigestRisk {
		score -= 0.12
	}
	score -= float64(in.RecentNotifications) * 0.08
	if in.DailyCap > 0 && in.RecentNotifications >= in.DailyCap {
		score -= 0.25
	}
	switch in.Sensitivity {
	case "high":
		score += 0.08
	case "low":
		score -= 0.08
	}
	if score >= 0.78 {
		return NotificationPriorityDecision{Route: "send_now", Reason: "high score and timing"}
	}
	if score >= 0.38 {
		return NotificationPriorityDecision{Route: "hold_for_briefing", Reason: "worth surfacing in briefing"}
	}
	return NotificationPriorityDecision{Route: "suppress", Reason: "low priority after caps and duplication"}
}

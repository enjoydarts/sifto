package service

import "testing"

func TestNotificationPriorityRoutesImmediate(t *testing.T) {
	out := RouteNotificationPriority(NotificationPriorityInput{
		ItemScore:           0.91,
		GoalMatch:           true,
		RecentNotifications: 0,
		DuplicateDigestRisk: false,
		Sensitivity:         "high",
		DailyCap:            3,
		ThemeWeight:         1.2,
	})
	if out.Route != "send_now" {
		t.Fatalf("route = %s, want send_now", out.Route)
	}
}

func TestNotificationPriorityRoutesHoldForBriefing(t *testing.T) {
	out := RouteNotificationPriority(NotificationPriorityInput{
		ItemScore:           0.62,
		GoalMatch:           false,
		RecentNotifications: 1,
		DuplicateDigestRisk: true,
		Sensitivity:         "medium",
		DailyCap:            3,
		ThemeWeight:         1.0,
	})
	if out.Route != "hold_for_briefing" {
		t.Fatalf("route = %s, want hold_for_briefing", out.Route)
	}
}

func TestNotificationPrioritySuppressesLowValue(t *testing.T) {
	out := RouteNotificationPriority(NotificationPriorityInput{
		ItemScore:           0.21,
		GoalMatch:           false,
		RecentNotifications: 4,
		DuplicateDigestRisk: true,
		Sensitivity:         "low",
		DailyCap:            2,
		ThemeWeight:         0.8,
	})
	if out.Route != "suppress" {
		t.Fatalf("route = %s, want suppress", out.Route)
	}
}

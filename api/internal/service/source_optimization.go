package service

type SourceOptimizationMetrics struct {
	UnreadBacklog        int
	ReadRate             float64
	FavoriteRate         float64
	NotificationOpenRate float64
	AverageSummaryScore  float64
}

type SourceOptimizationDecision struct {
	Recommendation string
	Reason         string
}

func ClassifySourceOptimization(m SourceOptimizationMetrics) SourceOptimizationDecision {
	if m.ReadRate >= 0.75 && m.FavoriteRate >= 0.15 && m.AverageSummaryScore >= 0.75 {
		return SourceOptimizationDecision{Recommendation: "promote", Reason: "high engagement and high value"}
	}
	if m.UnreadBacklog >= 60 && m.ReadRate <= 0.10 && m.AverageSummaryScore <= 0.25 {
		return SourceOptimizationDecision{Recommendation: "prune", Reason: "large backlog with low engagement"}
	}
	if m.UnreadBacklog >= 30 && m.ReadRate <= 0.20 && m.NotificationOpenRate <= 0.05 {
		return SourceOptimizationDecision{Recommendation: "mute", Reason: "backlog is growing and notifications are ignored"}
	}
	return SourceOptimizationDecision{Recommendation: "keep", Reason: "source is still useful"}
}

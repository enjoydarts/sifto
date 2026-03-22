package repository

import (
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func TestCalcPersonalScoreDetailedIncludesBreakdown(t *testing.T) {
	now := time.Now()
	profile := &model.UserPreferenceProfile{
		UserID: "u1",
		LearnedWeights: map[string]float64{
			"importance":    0.50,
			"novelty":       0.20,
			"actionability": 0.15,
			"reliability":   0.10,
			"relevance":     0.05,
		},
		TopicInterests:   map[string]float64{"kubernetes": 1.0, "container": 0.8},
		PrefEmbedding:    []float64{0.5, 0.5, 0.5},
		SourceAffinities: map[string]float64{"src1": 0.9},
		FeedbackCount:    31,
		ComputedAt:       &now,
	}
	item := PersonalScoreInput{
		ScoreBreakdown: &model.ItemSummaryScoreBreakdown{
			Importance:    ptr(0.9),
			Novelty:       ptr(0.4),
			Actionability: ptr(0.7),
			Reliability:   ptr(0.6),
			Relevance:     ptr(0.5),
		},
		Topics:    []string{"kubernetes", "container"},
		Embedding: []float64{0.5, 0.4, 0.6},
		SourceID:  "src1",
	}

	result := CalcPersonalScoreDetailed(item, profile)

	if result.Score <= 0 || result.Score > 1 {
		t.Fatalf("score = %f, want within (0,1]", result.Score)
	}
	if result.Breakdown == nil {
		t.Fatal("breakdown should be present for active profile")
	}
	if got := result.Breakdown.DominantDimension; got == nil || *got != "importance" {
		t.Fatalf("dominant_dimension = %v, want importance", got)
	}
	if len(result.Breakdown.MatchedTopics) != 2 {
		t.Fatalf("matched_topics = %v, want 2 topics", result.Breakdown.MatchedTopics)
	}
	if result.Breakdown.LearnedWeightScore.Weight <= result.Breakdown.SourceAffinity.Weight {
		t.Fatalf("learned weight should have stronger configured weight: %+v", result.Breakdown)
	}
}

func TestCalcPersonalScoreDetailedColdStartOmitsBreakdown(t *testing.T) {
	result := CalcPersonalScoreDetailed(PersonalScoreInput{SummaryScore: ptr(0.7)}, &model.UserPreferenceProfile{
		UserID:        "u1",
		FeedbackCount: 2,
	})

	if result.Score != 0.7 {
		t.Fatalf("score = %f, want 0.7", result.Score)
	}
	if result.Breakdown != nil {
		t.Fatalf("breakdown = %#v, want nil", result.Breakdown)
	}
	if result.Reason != "attention" {
		t.Fatalf("reason = %q, want attention", result.Reason)
	}
}

package repository

import (
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func TestCalcPersonalScore_FullProfile(t *testing.T) {
	now := time.Now()
	profile := &model.UserPreferenceProfile{
		UserID: "u1",
		LearnedWeights: map[string]float64{
			"importance":    0.40,
			"novelty":       0.25,
			"actionability": 0.15,
			"reliability":   0.15,
			"relevance":     0.05,
		},
		TopicInterests: map[string]float64{
			"go":   1.0,
			"rust": 0.8,
		},
		PrefEmbedding:    []float64{0.5, 0.5, 0.5},
		SourceAffinities: map[string]float64{"src1": 0.9},
		FeedbackCount:    50,
		ReadCount:        200,
		ComputedAt:       &now,
	}

	item := PersonalScoreInput{
		SummaryScore: ptr(0.85),
		ScoreBreakdown: &model.ItemSummaryScoreBreakdown{
			Importance:    ptr(0.9),
			Novelty:       ptr(0.7),
			Actionability: ptr(0.5),
			Reliability:   ptr(0.8),
			Relevance:     ptr(0.6),
		},
		Topics:    []string{"Go", "Rust"},
		Embedding: []float64{0.6, 0.4, 0.5},
		SourceID:  "src1",
	}

	score, reason := CalcPersonalScore(item, profile)

	if score <= 0 || score > 1 {
		t.Errorf("score should be in (0, 1], got %f", score)
	}
	if reason == "" {
		t.Error("reason should not be empty")
	}
}

func TestCalcPersonalScore_ColdStart(t *testing.T) {
	profile := &model.UserPreferenceProfile{
		UserID:        "u1",
		FeedbackCount: 5,
	}

	summaryScore := 0.75
	item := PersonalScoreInput{
		SummaryScore: &summaryScore,
	}

	score, reason := CalcPersonalScore(item, profile)

	if score != summaryScore {
		t.Errorf("cold start should return summary_score (%f), got %f", summaryScore, score)
	}
	if reason != "attention" {
		t.Errorf("cold start reason should be 'attention', got '%s'", reason)
	}
}

func TestCalcPersonalScore_NilProfile(t *testing.T) {
	summaryScore := 0.65
	item := PersonalScoreInput{
		SummaryScore: &summaryScore,
	}

	score, reason := CalcPersonalScore(item, nil)

	if score != summaryScore {
		t.Errorf("nil profile should return summary_score (%f), got %f", summaryScore, score)
	}
	if reason != "attention" {
		t.Errorf("nil profile reason should be 'attention', got '%s'", reason)
	}
}

func TestCalcPersonalScore_EmptyTopics(t *testing.T) {
	now := time.Now()
	profile := &model.UserPreferenceProfile{
		UserID: "u1",
		LearnedWeights: map[string]float64{
			"importance": 0.5,
			"novelty":    0.5,
		},
		TopicInterests:   map[string]float64{"go": 1.0},
		SourceAffinities: map[string]float64{},
		FeedbackCount:    30,
		ReadCount:        100,
		ComputedAt:       &now,
	}

	item := PersonalScoreInput{
		SummaryScore: ptr(0.7),
		ScoreBreakdown: &model.ItemSummaryScoreBreakdown{
			Importance: ptr(0.8),
			Novelty:    ptr(0.6),
		},
		Topics:   []string{},
		SourceID: "unknown-source",
	}

	score, reason := CalcPersonalScore(item, profile)

	if score < 0 || score > 1 {
		t.Errorf("score should be in [0, 1], got %f", score)
	}
	if reason == "" {
		t.Error("reason should not be empty")
	}
}

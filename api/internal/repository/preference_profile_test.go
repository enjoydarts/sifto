package repository

import (
	"math"
	"testing"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func ptr(f float64) *float64 { return &f }

// --- computeLearnedWeights ---

func TestComputeLearnedWeights_BasicCase(t *testing.T) {
	positive := []model.ItemSummaryScoreBreakdown{
		{Importance: ptr(0.9), Novelty: ptr(0.8), Actionability: ptr(0.3), Reliability: ptr(0.7), Relevance: ptr(0.6)},
		{Importance: ptr(0.8), Novelty: ptr(0.7), Actionability: ptr(0.4), Reliability: ptr(0.6), Relevance: ptr(0.5)},
	}
	negative := []model.ItemSummaryScoreBreakdown{
		{Importance: ptr(0.3), Novelty: ptr(0.5), Actionability: ptr(0.6), Reliability: ptr(0.4), Relevance: ptr(0.3)},
	}

	w := computeLearnedWeights(positive, negative, 30)

	// With feedbackCount=30, confidence=1.0, so fully learned weights
	var sum float64
	for _, v := range w {
		sum += v
	}
	if math.Abs(sum-1.0) > 0.01 {
		t.Errorf("weights should sum to ~1.0, got %f", sum)
	}

	// importance diff: avg(0.9,0.8)-0.3=0.55, novelty diff: avg(0.8,0.7)-0.5=0.25
	// importance should be the largest weight
	if w["importance"] <= w["novelty"] {
		t.Errorf("importance (%f) should be > novelty (%f)", w["importance"], w["novelty"])
	}
	// actionability diff: avg(0.3,0.4)-0.6 = -0.25 → clamped to 0
	if w["actionability"] != 0 {
		t.Errorf("actionability should be 0 (negative diff clamped), got %f", w["actionability"])
	}
}

func TestComputeLearnedWeights_ColdStart(t *testing.T) {
	positive := []model.ItemSummaryScoreBreakdown{
		{Importance: ptr(0.9), Novelty: ptr(0.2), Actionability: ptr(0.1), Reliability: ptr(0.1), Relevance: ptr(0.1)},
	}
	negative := []model.ItemSummaryScoreBreakdown{
		{Importance: ptr(0.1), Novelty: ptr(0.1), Actionability: ptr(0.1), Reliability: ptr(0.1), Relevance: ptr(0.1)},
	}

	w := computeLearnedWeights(positive, negative, 5)

	// confidence = 5/30 ≈ 0.167, so heavily blended with defaults
	defaultImp := model.DefaultScoreWeights["importance"]
	// The blended value should be close to defaults
	if math.Abs(w["importance"]-defaultImp) > 0.2 {
		t.Errorf("cold start importance (%f) should be near default (%f)", w["importance"], defaultImp)
	}
	// reliability has default 0.17, so it should still be well above 0
	if w["reliability"] < 0.1 {
		t.Errorf("cold start reliability (%f) should be significant due to default blending", w["reliability"])
	}
}

func TestComputeLearnedWeights_AllDiffNegative(t *testing.T) {
	positive := []model.ItemSummaryScoreBreakdown{
		{Importance: ptr(0.1), Novelty: ptr(0.1), Actionability: ptr(0.1), Reliability: ptr(0.1), Relevance: ptr(0.1)},
	}
	negative := []model.ItemSummaryScoreBreakdown{
		{Importance: ptr(0.9), Novelty: ptr(0.9), Actionability: ptr(0.9), Reliability: ptr(0.9), Relevance: ptr(0.9)},
	}

	w := computeLearnedWeights(positive, negative, 30)

	// All diffs negative → should fallback to defaults
	for k, v := range model.DefaultScoreWeights {
		if math.Abs(w[k]-v) > 1e-9 {
			t.Errorf("expected default weight for %s=%f, got %f", k, v, w[k])
		}
	}
}

// --- computeTopicInterests ---

func TestComputeTopicInterests_BasicCase(t *testing.T) {
	actions := []topicAction{
		{Topics: []string{"Go", "Rust"}, Signal: 1.0, DaysAgo: 0},
		{Topics: []string{"Go", "Python"}, Signal: 1.0, DaysAgo: 15},
		{Topics: []string{"Rust"}, Signal: 0.5, DaysAgo: 5},
	}

	interests := computeTopicInterests(actions)

	if len(interests) != 3 {
		t.Fatalf("expected 3 topics, got %d", len(interests))
	}
	// "go" should be highest (signal=1.0 at day 0 + signal=1.0 at day 15)
	if interests["go"] < interests["python"] {
		t.Errorf("go (%f) should be >= python (%f)", interests["go"], interests["python"])
	}
	// Max value should be 1.0
	var maxVal float64
	for _, v := range interests {
		if v > maxVal {
			maxVal = v
		}
	}
	if math.Abs(maxVal-1.0) > 1e-9 {
		t.Errorf("max interest should be 1.0, got %f", maxVal)
	}
}

func TestComputeTopicInterests_EmptyTopics(t *testing.T) {
	actions := []topicAction{
		{Topics: []string{}, Signal: 1.0, DaysAgo: 0},
		{Topics: []string{" ", ""}, Signal: 1.0, DaysAgo: 0},
	}

	interests := computeTopicInterests(actions)
	if len(interests) != 0 {
		t.Errorf("expected empty interests, got %v", interests)
	}
}

func TestComputeTopicInterests_TimeDecay(t *testing.T) {
	actions := []topicAction{
		{Topics: []string{"recent"}, Signal: 1.0, DaysAgo: 0},
		{Topics: []string{"old"}, Signal: 1.0, DaysAgo: 60},
	}

	interests := computeTopicInterests(actions)

	// recent: decay = 0.5^0 = 1.0
	// old:    decay = 0.5^2 = 0.25
	if interests["recent"] <= interests["old"] {
		t.Errorf("recent (%f) should be > old (%f)", interests["recent"], interests["old"])
	}
	// recent should be 1.0 (it's the max)
	if math.Abs(interests["recent"]-1.0) > 1e-9 {
		t.Errorf("recent should be 1.0, got %f", interests["recent"])
	}
	// old should be ~0.25
	if math.Abs(interests["old"]-0.25) > 0.01 {
		t.Errorf("old should be ~0.25, got %f", interests["old"])
	}
}

func TestComputeTopicInterests_PreservesNegativeSignals(t *testing.T) {
	actions := []topicAction{
		{Topics: []string{"AI"}, Signal: 1.0, DaysAgo: 0},
		{Topics: []string{"Celebrity"}, Signal: -1.0, DaysAgo: 0},
	}

	interests := computeTopicInterests(actions)
	if interests["ai"] != 1.0 {
		t.Fatalf("ai should normalize to 1.0, got %f", interests["ai"])
	}
	if interests["celebrity"] >= 0 {
		t.Fatalf("celebrity should remain negative, got %f", interests["celebrity"])
	}
}

func TestPreferenceProfileIncludesNewSignals(t *testing.T) {
	actions := []topicAction{
		{Topics: []string{"AI"}, Signal: 2.0, DaysAgo: 0},    // note / insight
		{Topics: []string{"AI"}, Signal: 1.5, DaysAgo: 1},    // review done
		{Topics: []string{"Infra"}, Signal: 0.4, DaysAgo: 0}, // weak signal
	}

	interests := computeTopicInterests(actions)
	if interests["ai"] <= interests["infra"] {
		t.Fatalf("ai (%f) should be > infra (%f)", interests["ai"], interests["infra"])
	}
}

func TestPreferenceProfileStatusAndConfidence(t *testing.T) {
	if got, want := preferenceStatusFromFeedbackCount(0), "cold_start"; got != want {
		t.Fatalf("status(0) = %q, want %q", got, want)
	}
	if got, want := preferenceStatusFromFeedbackCount(10), "learning"; got != want {
		t.Fatalf("status(10) = %q, want %q", got, want)
	}
	if got, want := preferenceStatusFromFeedbackCount(30), "active"; got != want {
		t.Fatalf("status(30) = %q, want %q", got, want)
	}
	if got := preferenceConfidenceFromFeedbackCount(15); math.Abs(got-0.5) > 1e-9 {
		t.Fatalf("confidence(15) = %f, want 0.5", got)
	}
	if got := preferenceConfidenceFromFeedbackCount(99); got != 1 {
		t.Fatalf("confidence should clamp to 1, got %f", got)
	}
}

func TestTopTopicsWithSignalCounts(t *testing.T) {
	topics := topTopicsWithSignalCounts(
		map[string]float64{"go": 1.0, "rust": 0.8, "ml": 0.3},
		[]topicAction{
			{Topics: []string{"Go", "Rust"}, Signal: 1.0},
			{Topics: []string{"Go"}, Signal: 1.0},
			{Topics: []string{"ML"}, Signal: 1.0},
		},
		2,
	)
	if len(topics) != 2 {
		t.Fatalf("len(topics) = %d, want 2", len(topics))
	}
	if topics[0].Topic != "go" || topics[0].SignalCount != 2 {
		t.Fatalf("top topic = %+v, want go with 2 signals", topics[0])
	}
	if topics[1].Topic != "rust" {
		t.Fatalf("second topic = %+v, want rust", topics[1])
	}
}

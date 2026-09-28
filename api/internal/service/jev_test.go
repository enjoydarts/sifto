package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func passingJevDimensions() map[string]JevDimension {
	return map[string]JevDimension{
		"source_support":     {Score: 0.95, Confidence: 0.96},
		"contradiction_free": {Score: 0.95, Confidence: 0.96},
		"coverage":           {Score: 0.90, Confidence: 0.95},
		"inference_control":  {Score: 0.90, Confidence: 0.94},
		"specificity":        {Score: 0.90, Confidence: 0.93},
	}
}

func passingJevSignals() map[string]JevSignal {
	return map[string]JevSignal{
		"has_unsupported_fact": {Probability: 0.02},
		"has_contradiction":    {Probability: 0.01},
	}
}

func TestJevClientRejectsAnswersWithUnexpectedDimensionNames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"jev-20260915","answers":{"source_support":{"type":"score","score":4,"confidence":1},"contradiction_free":{"type":"score","score":4,"confidence":1},"coverage":{"type":"score","score":4,"confidence":1},"inference_control":{"type":"score","score":4,"confidence":1},"unexpected":{"type":"score","score":4,"confidence":1},"has_unsupported_fact":{"type":"noul","noul":0},"has_contradiction":{"type":"noul","noul":0}},"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer server.Close()

	catalog := JevCatalog{DefaultModel: "jev-latest", Models: []JevModelConfig{{ID: "jev-latest", MatchPrefix: []string{"jev-"}, InputPerMTokUSD: 0.042, PricingSource: "test"}}}
	client := NewJevClient(server.URL, server.Client(), catalog)
	_, err := client.EvaluateFacts(context.Background(), "secret", "title", "body", []string{"fact"})
	if err == nil || !strings.Contains(err.Error(), "missing Jev answer specificity") {
		t.Fatalf("error = %v", err)
	}
}

func TestEvaluateJevGateAcceptsOnlyAllThresholds(t *testing.T) {
	policy := JevGatePolicy{Version: "v1", AggregateThreshold: 0.90, MinimumDimensionScore: 0.80, MinimumConfidence: 0.50, ConfidenceScoreMargin: 0.10, CriticalDimensionScore: 0.90, SignalThresholds: map[string]float64{"has_unsupported_fact": 0.10, "has_contradiction": 0.05}}
	result := EvaluateJevGate(passingJevDimensions(), passingJevSignals(), []string{"source_support", "contradiction_free"}, policy)
	if result.Decision != JevDecisionAccepted || result.PolicyVersion != "v1" || result.EscalationReason != "" {
		t.Fatalf("result = %#v", result)
	}
}

func TestEvaluateJevGateReturnsStableEscalationReasons(t *testing.T) {
	policy := JevGatePolicy{Version: "v1", AggregateThreshold: 0.90, MinimumDimensionScore: 0.80, MinimumConfidence: 0.50, ConfidenceScoreMargin: 0.10, CriticalDimensionScore: 0.90}
	cases := []struct {
		name string
		edit func(map[string]JevDimension)
		want JevEscalationReason
	}{
		{"critical", func(v map[string]JevDimension) { d := v["source_support"]; d.Score = 0.89; v["source_support"] = d }, JevEscalationCriticalDimensionLow},
		{"confidence near boundary", func(v map[string]JevDimension) {
			d := v["coverage"]
			d.Score = 0.85
			d.Confidence = 0.49
			v["coverage"] = d
		}, JevEscalationLowConfidence},
		{"dimension", func(v map[string]JevDimension) { d := v["coverage"]; d.Score = 0.79; v["coverage"] = d }, JevEscalationLowDimensionScore},
		{"aggregate", func(v map[string]JevDimension) {
			for k, d := range v {
				if k == "source_support" || k == "contradiction_free" {
					d.Score = 0.90
				} else {
					d.Score = 0.86
				}
				v[k] = d
			}
		}, JevEscalationLowScore},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dims := passingJevDimensions()
			tc.edit(dims)
			got := EvaluateJevGate(dims, nil, []string{"source_support", "contradiction_free"}, policy)
			if got.Decision != JevDecisionEscalated || got.EscalationReason != tc.want {
				t.Fatalf("result = %#v, want reason %q", got, tc.want)
			}
		})
	}
}

func TestEvaluateJevGateDoesNotEscalateLowConfidenceForClearlyPassingScore(t *testing.T) {
	policy := JevGatePolicy{Version: "v4", AggregateThreshold: 0.75, MinimumDimensionScore: 0.75, MinimumConfidence: 0.50, ConfidenceScoreMargin: 0.10, CriticalDimensionScore: 0.75}
	dimensions := passingJevDimensions()
	dimension := dimensions["coverage"]
	dimension.Confidence = 0.20
	dimension.Score = 0.90
	dimensions["coverage"] = dimension

	got := EvaluateJevGate(dimensions, nil, []string{"source_support", "contradiction_free"}, policy)
	if got.Decision != JevDecisionAccepted {
		t.Fatalf("clearly passing score should not be rejected solely for low confidence: %#v", got)
	}
}

func TestEvaluateJevGateEscalatesCriticalSignalsWithStableReasons(t *testing.T) {
	policy := JevGatePolicy{Version: "v4", AggregateThreshold: 0.75, MinimumDimensionScore: 0.75, MinimumConfidence: 0.50, ConfidenceScoreMargin: 0.10, CriticalDimensionScore: 0.75, SignalThresholds: map[string]float64{"has_unsupported_fact": 0.10, "has_unsupported_claim": 0.10, "has_contradiction": 0.05, "has_entity_numeric_mismatch": 0.10}}
	cases := []struct {
		name   string
		signal string
		want   JevEscalationReason
	}{
		{"unsupported fact", "has_unsupported_fact", JevEscalationUnsupportedClaimRisk},
		{"unsupported claim", "has_unsupported_claim", JevEscalationUnsupportedClaimRisk},
		{"contradiction", "has_contradiction", JevEscalationContradictionRisk},
		{"entity or number mismatch", "has_entity_numeric_mismatch", JevEscalationEntityNumericMismatchRisk},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			signals := map[string]JevSignal{tc.signal: {Probability: policy.SignalThresholds[tc.signal]}}
			got := EvaluateJevGate(passingJevDimensions(), signals, []string{"source_support", "contradiction_free"}, policy)
			if got.Decision != JevDecisionEscalated || got.EscalationReason != tc.want {
				t.Fatalf("result = %#v, want reason %q", got, tc.want)
			}
		})
	}
}

func TestEvaluateJevGateUsesDeterministicCriticalSignalPriority(t *testing.T) {
	policy := JevGatePolicy{Version: "v4", AggregateThreshold: 0.75, MinimumDimensionScore: 0.75, MinimumConfidence: 0.50, ConfidenceScoreMargin: 0.10, CriticalDimensionScore: 0.75, SignalThresholds: map[string]float64{"has_unsupported_claim": 0.50, "has_contradiction": 0.50}}
	signals := map[string]JevSignal{"has_unsupported_claim": {Probability: 0.90}, "has_contradiction": {Probability: 0.90}}

	got := EvaluateJevGate(passingJevDimensions(), signals, []string{"source_support", "contradiction_free"}, policy)
	if got.EscalationReason != JevEscalationContradictionRisk {
		t.Fatalf("result = %#v, want deterministic contradiction priority", got)
	}
}

func TestJevClientSendsScoresAndUsesConcreteModelPricing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("Authorization = %q", got)
		}
		var body struct {
			Model     string                     `json:"model"`
			Questions map[string]json.RawMessage `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Model != "jev-latest" || len(body.Questions) != 7 {
			t.Fatalf("body = %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"jev-20260915","answers":{"source_support":{"type":"score","score":3.8,"confidence":0.96,"legend":{"0":"bad","4":"good"},"probabilities":{"4":0.9}},"contradiction_free":{"type":"score","score":3.8,"confidence":0.96,"legend":{"0":"bad","4":"good"},"probabilities":{"4":0.9}},"coverage":{"type":"score","score":3.6,"confidence":0.95,"legend":{"0":"bad","4":"good"},"probabilities":{"4":0.9}},"inference_control":{"type":"score","score":3.6,"confidence":0.94,"legend":{"0":"bad","4":"good"},"probabilities":{"4":0.9}},"specificity":{"type":"score","score":3.6,"confidence":0.93,"legend":{"0":"bad","4":"good"},"probabilities":{"4":0.9}},"has_unsupported_fact":{"type":"noul","noul":0.03},"has_contradiction":{"type":"noul","noul":0.01}},"usage":{"input_tokens":1000000,"output_tokens":5}}`))
	}))
	defer server.Close()

	catalog := JevCatalog{DefaultModel: "jev-latest", Models: []JevModelConfig{{ID: "jev-latest", MatchExact: []string{"jev-latest"}, MatchPrefix: []string{"jev-"}, InputPerMTokUSD: 0.042, PricingSource: "typesafe_jev_2026_09"}}}
	client := NewJevClient(server.URL, server.Client(), catalog)
	got, err := client.EvaluateFacts(context.Background(), "secret", "title", "body", []string{"fact"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != "jev-20260915" || got.Usage.EstimatedCostUSD != 0.042 || got.Usage.PricingSource != "typesafe_jev_2026_09" {
		t.Fatalf("result = %#v", got)
	}
	if got.Signals["has_unsupported_fact"].Probability != 0.03 || got.Signals["has_contradiction"].Probability != 0.01 {
		t.Fatalf("signals = %#v", got.Signals)
	}
}

func TestJevQuestionsUseDimensionSpecificRubricsAndNoulSignals(t *testing.T) {
	facts := factsJevQuestions()
	faithfulness := faithfulnessJevQuestions()
	if len(facts) != 7 || len(faithfulness) != 8 {
		t.Fatalf("question counts: facts=%d faithfulness=%d", len(facts), len(faithfulness))
	}

	sourceSupport := facts["source_support"].(map[string]any)
	coverage := facts["coverage"].(map[string]any)
	sourceCriteria := sourceSupport["criteria"].([]string)
	coverageCriteria := coverage["criteria"].([]string)
	if sourceCriteria[0] == coverageCriteria[0] || sourceCriteria[4] == coverageCriteria[4] {
		t.Fatalf("rubrics must describe each dimension independently: source=%#v coverage=%#v", sourceCriteria, coverageCriteria)
	}
	for _, name := range []string{"has_unsupported_fact", "has_contradiction"} {
		question := facts[name].(map[string]any)
		if question["type"] != "noul" || question["instructions"] == "" {
			t.Fatalf("%s question = %#v", name, question)
		}
	}
	for _, name := range []string{"has_unsupported_claim", "has_contradiction", "has_entity_numeric_mismatch"} {
		question := faithfulness[name].(map[string]any)
		if question["type"] != "noul" || question["instructions"] == "" {
			t.Fatalf("%s question = %#v", name, question)
		}
	}
}

func TestJevClientRejectsInvalidNoulProbability(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"jev-20260915","answers":{"source_support":{"type":"score","score":4,"confidence":1},"contradiction_free":{"type":"score","score":4,"confidence":1},"coverage":{"type":"score","score":4,"confidence":1},"inference_control":{"type":"score","score":4,"confidence":1},"specificity":{"type":"score","score":4,"confidence":1},"has_unsupported_fact":{"type":"noul","noul":1.1},"has_contradiction":{"type":"noul","noul":0}},"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer server.Close()

	catalog := JevCatalog{DefaultModel: "jev-latest", Models: []JevModelConfig{{ID: "jev-latest", MatchPrefix: []string{"jev-"}, InputPerMTokUSD: 0.042, PricingSource: "test"}}}
	client := NewJevClient(server.URL, server.Client(), catalog)
	_, err := client.EvaluateFacts(context.Background(), "secret", "title", "body", []string{"fact"})
	if err == nil || !strings.Contains(err.Error(), "invalid Jev noul answer has_unsupported_fact") {
		t.Fatalf("error = %v", err)
	}
}

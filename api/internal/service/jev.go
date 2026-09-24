package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

type JevDecision string
type JevEscalationReason string

const (
	JevDecisionAccepted  JevDecision = "accepted"
	JevDecisionEscalated JevDecision = "escalated"
	JevDecisionError     JevDecision = "error"

	JevEscalationLowScore             JevEscalationReason = "low_score"
	JevEscalationLowDimensionScore    JevEscalationReason = "low_dimension_score"
	JevEscalationLowConfidence        JevEscalationReason = "low_confidence"
	JevEscalationCriticalDimensionLow JevEscalationReason = "critical_dimension_low"
	JevEscalationTimeout              JevEscalationReason = "timeout"
	JevEscalationHTTPError            JevEscalationReason = "http_error"
	JevEscalationSchemaError          JevEscalationReason = "schema_error"
	JevEscalationConfigurationError   JevEscalationReason = "configuration_error"
)

type JevGatePolicy struct {
	Version                string  `json:"version"`
	AggregateThreshold     float64 `json:"aggregate_threshold"`
	MinimumDimensionScore  float64 `json:"minimum_dimension_score"`
	MinimumConfidence      float64 `json:"minimum_confidence"`
	CriticalDimensionScore float64 `json:"critical_dimension_score"`
}

type JevDimension struct {
	Score         float64            `json:"score"`
	RawScore      float64            `json:"raw_score"`
	Confidence    float64            `json:"confidence"`
	Legend        map[string]any     `json:"legend,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
}

type JevGateResult struct {
	Decision          JevDecision         `json:"decision"`
	EscalationReason  JevEscalationReason `json:"escalation_reason,omitempty"`
	PolicyVersion     string              `json:"gate_policy_version"`
	AggregateScore    float64             `json:"aggregate_score"`
	MinimumScore      float64             `json:"minimum_score"`
	MinimumConfidence float64             `json:"minimum_confidence"`
}

type JevUsage struct {
	InputTokens      int     `json:"input_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
	PricingSource    string  `json:"pricing_source"`
}

type JevEvaluation struct {
	RequestedModel string                  `json:"requested_model"`
	Model          string                  `json:"model"`
	Dimensions     map[string]JevDimension `json:"dimensions"`
	Usage          JevUsage                `json:"usage"`
	LatencyMS      int64                   `json:"latency_ms"`
}

func EvaluateJevGate(dimensions map[string]JevDimension, critical []string, policy JevGatePolicy) JevGateResult {
	result := JevGateResult{Decision: JevDecisionEscalated, PolicyVersion: policy.Version, MinimumScore: 1, MinimumConfidence: 1}
	if len(dimensions) == 0 {
		result.EscalationReason = JevEscalationSchemaError
		result.MinimumScore = 0
		result.MinimumConfidence = 0
		return result
	}
	criticalSet := make(map[string]struct{}, len(critical))
	for _, name := range critical {
		criticalSet[name] = struct{}{}
	}
	criticalLow := false
	for name, dimension := range dimensions {
		result.AggregateScore += dimension.Score
		result.MinimumScore = math.Min(result.MinimumScore, dimension.Score)
		result.MinimumConfidence = math.Min(result.MinimumConfidence, dimension.Confidence)
		if _, ok := criticalSet[name]; ok && dimension.Score < policy.CriticalDimensionScore {
			criticalLow = true
		}
	}
	result.AggregateScore /= float64(len(dimensions))
	switch {
	case criticalLow:
		result.EscalationReason = JevEscalationCriticalDimensionLow
	case result.MinimumConfidence < policy.MinimumConfidence:
		result.EscalationReason = JevEscalationLowConfidence
	case result.MinimumScore < policy.MinimumDimensionScore:
		result.EscalationReason = JevEscalationLowDimensionScore
	case result.AggregateScore < policy.AggregateThreshold:
		result.EscalationReason = JevEscalationLowScore
	default:
		result.Decision = JevDecisionAccepted
	}
	return result
}

type JevClient struct {
	baseURL string
	http    *http.Client
	catalog JevCatalog
}

func NewJevClient(baseURL string, client *http.Client, catalog JevCatalog) *JevClient {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &JevClient{baseURL: strings.TrimRight(baseURL, "/"), http: client, catalog: catalog}
}

func NewJevClientFromCatalog(catalog JevCatalog) *JevClient {
	return NewJevClient("https://api.typesafe.ai", &http.Client{Timeout: 10 * time.Second}, catalog)
}

func (c *JevClient) EvaluateFacts(ctx context.Context, apiKey, title, content string, facts []string) (*JevEvaluation, error) {
	state := map[string]any{"title": title, "source_content": content, "extracted_facts": facts}
	return c.evaluate(ctx, apiKey, state, factsJevQuestions())
}

func (c *JevClient) EvaluateFaithfulness(ctx context.Context, apiKey, title string, facts []string, summary string) (*JevEvaluation, error) {
	state := map[string]any{"title": title, "facts": facts, "summary": summary}
	return c.evaluate(ctx, apiKey, state, faithfulnessJevQuestions())
}

func (c *JevClient) evaluate(ctx context.Context, apiKey string, state any, questions map[string]any) (*JevEvaluation, error) {
	requestedModel := c.catalog.DefaultModel
	payload, err := json.Marshal(map[string]any{"model": requestedModel, "state": state, "questions": questions})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/systemone", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	req.Header.Set("Content-Type", "application/json")
	started := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Jev request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("Jev HTTP status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var decoded struct {
		Model   string `json:"model"`
		Answers map[string]struct {
			Type          string             `json:"type"`
			Score         float64            `json:"score"`
			Confidence    float64            `json:"confidence"`
			Legend        map[string]any     `json:"legend"`
			Probabilities map[string]float64 `json:"probabilities"`
		} `json:"answers"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode Jev response: %w", err)
	}
	if strings.TrimSpace(decoded.Model) == "" || len(decoded.Answers) != len(questions) {
		return nil, fmt.Errorf("invalid Jev response")
	}
	dimensions := make(map[string]JevDimension, len(decoded.Answers))
	for name, answer := range decoded.Answers {
		if answer.Type != "score" || answer.Score < 0 || answer.Score > 4 || answer.Confidence < 0 || answer.Confidence > 1 {
			return nil, fmt.Errorf("invalid Jev score answer %s", name)
		}
		dimensions[name] = JevDimension{Score: answer.Score / 4, RawScore: answer.Score, Confidence: answer.Confidence, Legend: answer.Legend, Probabilities: answer.Probabilities}
	}
	pricing, ok := c.catalog.ResolvePricing(decoded.Model, requestedModel)
	if !ok {
		return nil, fmt.Errorf("Jev pricing not configured for model %s", decoded.Model)
	}
	cost := float64(decoded.Usage.InputTokens)*pricing.InputPerMTokUSD/1_000_000 + float64(decoded.Usage.OutputTokens)*pricing.OutputPerMTokUSD/1_000_000
	return &JevEvaluation{RequestedModel: requestedModel, Model: decoded.Model, Dimensions: dimensions, Usage: JevUsage{InputTokens: decoded.Usage.InputTokens, OutputTokens: decoded.Usage.OutputTokens, EstimatedCostUSD: cost, PricingSource: pricing.PricingSource}, LatencyMS: time.Since(started).Milliseconds()}, nil
}

func scoreQuestion(instructions string) map[string]any {
	return map[string]any{"type": "score", "instructions": instructions, "criteria": []string{"重大な不適合", "大きな問題がある", "一部問題がある", "ほぼ適合", "完全に適合"}}
}

func factsJevQuestions() map[string]any {
	return map[string]any{
		"source_support":     scoreQuestion("抽出された各factがsource_contentに直接支持されている程度を評価する"),
		"contradiction_free": scoreQuestion("抽出factsがsource_contentと矛盾していない程度を評価する"),
		"coverage":           scoreQuestion("抽出factsがsource_contentの重要な主張を網羅している程度を評価する"),
		"inference_control":  scoreQuestion("抽出factsに推測や因果の飛躍がない程度を評価する"),
		"specificity":        scoreQuestion("抽出factsが具体的で有用でありプレースホルダーでない程度を評価する"),
	}
}

func faithfulnessJevQuestions() map[string]any {
	return map[string]any{
		"facts_support":           scoreQuestion("summaryの各主張がfactsに直接支持されている程度を評価する"),
		"contradiction_free":      scoreQuestion("summaryがfactsと矛盾していない程度を評価する"),
		"coverage":                scoreQuestion("summaryが主要factsを網羅している程度を評価する"),
		"inference_control":       scoreQuestion("summaryに誇張、推測、因果の追加がない程度を評価する"),
		"entity_numeric_accuracy": scoreQuestion("summaryの固有名詞、数値、日時がfactsと一致する程度を評価する"),
	}
}

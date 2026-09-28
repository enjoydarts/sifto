package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

	JevEscalationLowScore                  JevEscalationReason = "low_score"
	JevEscalationLowDimensionScore         JevEscalationReason = "low_dimension_score"
	JevEscalationLowConfidence             JevEscalationReason = "low_confidence"
	JevEscalationCriticalDimensionLow      JevEscalationReason = "critical_dimension_low"
	JevEscalationTimeout                   JevEscalationReason = "timeout"
	JevEscalationHTTPError                 JevEscalationReason = "http_error"
	JevEscalationSchemaError               JevEscalationReason = "schema_error"
	JevEscalationConfigurationError        JevEscalationReason = "configuration_error"
	JevEscalationUnsupportedClaimRisk      JevEscalationReason = "unsupported_claim_risk"
	JevEscalationContradictionRisk         JevEscalationReason = "contradiction_risk"
	JevEscalationEntityNumericMismatchRisk JevEscalationReason = "entity_numeric_mismatch_risk"
)

type JevGatePolicy struct {
	Version                string             `json:"version"`
	AggregateThreshold     float64            `json:"aggregate_threshold"`
	MinimumDimensionScore  float64            `json:"minimum_dimension_score"`
	MinimumConfidence      float64            `json:"minimum_confidence"`
	ConfidenceScoreMargin  float64            `json:"confidence_score_margin"`
	MinPassingProbability  float64            `json:"min_passing_probability"`
	CriticalDimensionScore float64            `json:"critical_dimension_score"`
	SignalThresholds       map[string]float64 `json:"signal_thresholds"`
}

type JevDimension struct {
	Score         float64            `json:"score"`
	RawScore      float64            `json:"raw_score"`
	Confidence    float64            `json:"confidence"`
	Legend        map[string]any     `json:"legend,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
}

type JevSignal struct {
	Probability float64 `json:"probability"`
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
	Signals        map[string]JevSignal    `json:"signals"`
	Usage          JevUsage                `json:"usage"`
	LatencyMS      int64                   `json:"latency_ms"`
}

func EvaluateJevGate(dimensions map[string]JevDimension, signals map[string]JevSignal, critical []string, policy JevGatePolicy) JevGateResult {
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
	lowConfidenceNearBoundary := false
	for name, dimension := range dimensions {
		result.AggregateScore += dimension.Score
		result.MinimumScore = math.Min(result.MinimumScore, dimension.Score)
		result.MinimumConfidence = math.Min(result.MinimumConfidence, dimension.Confidence)
		if _, ok := criticalSet[name]; ok && dimension.Score < policy.CriticalDimensionScore {
			criticalLow = true
		}
		if dimension.Confidence < policy.MinimumConfidence && dimension.Score < policy.MinimumDimensionScore+policy.ConfidenceScoreMargin && !passingRubricProbabilityAtLeast(dimension.Probabilities, policy.MinPassingProbability) {
			lowConfidenceNearBoundary = true
		}
	}
	result.AggregateScore /= float64(len(dimensions))
	criticalSignals := []struct {
		Names  []string
		Reason JevEscalationReason
	}{
		{Names: []string{"has_contradiction"}, Reason: JevEscalationContradictionRisk},
		{Names: []string{"has_entity_numeric_mismatch"}, Reason: JevEscalationEntityNumericMismatchRisk},
		{Names: []string{"has_unsupported_fact", "has_unsupported_claim"}, Reason: JevEscalationUnsupportedClaimRisk},
	}
	for _, criticalSignal := range criticalSignals {
		for _, name := range criticalSignal.Names {
			signal, present := signals[name]
			threshold, configured := policy.SignalThresholds[name]
			if !present || !configured || signal.Probability < threshold {
				continue
			}
			result.EscalationReason = criticalSignal.Reason
			return result
		}
	}
	switch {
	case criticalLow:
		result.EscalationReason = JevEscalationCriticalDimensionLow
	case result.MinimumScore < policy.MinimumDimensionScore:
		result.EscalationReason = JevEscalationLowDimensionScore
	case result.AggregateScore < policy.AggregateThreshold:
		result.EscalationReason = JevEscalationLowScore
	case lowConfidenceNearBoundary:
		result.EscalationReason = JevEscalationLowConfidence
	default:
		result.Decision = JevDecisionAccepted
	}
	return result
}

func passingRubricProbabilityAtLeast(probabilities map[string]float64, threshold float64) bool {
	if threshold <= 0 {
		return false
	}
	passThree, hasThree := probabilities["3"]
	passFour, hasFour := probabilities["4"]
	if !hasThree || !hasFour || !validJevProbabilities(probabilities) {
		return false
	}
	total := 0.0
	for _, probability := range probabilities {
		total += probability
	}
	return total >= 0.95 && total <= 1.05 && passThree+passFour >= threshold
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
		return nil, fmt.Errorf("Jev HTTP status=%d", resp.StatusCode)
	}
	var decoded struct {
		Model   string                     `json:"model"`
		Answers map[string]json.RawMessage `json:"answers"`
		Usage   struct {
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
	for name := range questions {
		if _, ok := decoded.Answers[name]; !ok {
			return nil, fmt.Errorf("missing Jev answer %s", name)
		}
	}
	dimensions := make(map[string]JevDimension)
	signals := make(map[string]JevSignal)
	for name, raw := range decoded.Answers {
		question, ok := questions[name].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid Jev question %s", name)
		}
		var answer struct {
			Type          string             `json:"type"`
			Score         float64            `json:"score"`
			Confidence    float64            `json:"confidence"`
			Legend        map[string]any     `json:"legend"`
			Probabilities map[string]float64 `json:"probabilities"`
			Noul          float64            `json:"noul"`
		}
		if err := json.Unmarshal(raw, &answer); err != nil {
			return nil, fmt.Errorf("decode Jev answer %s: %w", name, err)
		}
		switch question["type"] {
		case "score":
			if answer.Type != "score" || answer.Score < 0 || answer.Score > 4 || answer.Confidence < 0 || answer.Confidence > 1 || !validJevProbabilities(answer.Probabilities) {
				return nil, fmt.Errorf("invalid Jev score answer %s", name)
			}
			dimensions[name] = JevDimension{Score: answer.Score / 4, RawScore: answer.Score, Confidence: answer.Confidence, Legend: answer.Legend, Probabilities: answer.Probabilities}
		case "noul":
			if answer.Type != "noul" || answer.Noul < 0 || answer.Noul > 1 {
				return nil, fmt.Errorf("invalid Jev noul answer %s", name)
			}
			signals[name] = JevSignal{Probability: answer.Noul}
		default:
			return nil, fmt.Errorf("unsupported Jev question type for %s", name)
		}
	}
	pricing, ok := c.catalog.ResolvePricing(decoded.Model, requestedModel)
	if !ok {
		return nil, fmt.Errorf("Jev pricing not configured for model %s", decoded.Model)
	}
	cost := float64(decoded.Usage.InputTokens)*pricing.InputPerMTokUSD/1_000_000 + float64(decoded.Usage.OutputTokens)*pricing.OutputPerMTokUSD/1_000_000
	return &JevEvaluation{RequestedModel: requestedModel, Model: decoded.Model, Dimensions: dimensions, Signals: signals, Usage: JevUsage{InputTokens: decoded.Usage.InputTokens, OutputTokens: decoded.Usage.OutputTokens, EstimatedCostUSD: cost, PricingSource: pricing.PricingSource}, LatencyMS: time.Since(started).Milliseconds()}, nil
}

func validJevProbabilities(probabilities map[string]float64) bool {
	for _, probability := range probabilities {
		if probability < 0 || probability > 1 {
			return false
		}
	}
	return true
}

func scoreQuestion(instructions string, criteria ...string) map[string]any {
	return map[string]any{"type": "score", "instructions": instructions, "criteria": criteria}
}

func noulQuestion(instructions, trueCriteria, falseCriteria string) map[string]any {
	return map[string]any{"type": "noul", "instructions": instructions, "criteria": map[string]string{"true": trueCriteria, "false": falseCriteria}}
}

func factsJevQuestions() map[string]any {
	return map[string]any{
		"source_support": scoreQuestion("抽出された各factがsource_contentに根拠を持つ程度を評価する。意味を保つ言い換えは支持とみなすが、新しい主張や主体の取り違えは支持とみなさない。",
			"重要なfactに捏造または根拠のない主張がある", "複数のfactが直接支持されない、または主体・条件が変わっている", "一部に曖昧な帰属や本文からは確定できない主張がある", "全ての重要なfactが支持され、軽微な言い換えや省略だけである", "全てのfactが本文に明確かつ直接支持され、新しい主張がない"),
		"contradiction_free": scoreQuestion("抽出factsとsource_contentの間に矛盾がない程度を評価する。数値、日時、主体、肯否、比較方向の反転を重視する。",
			"本文と正反対の重要主張がある", "重大な数値・主体・肯否の矛盾がある", "限定条件や時点の不一致があり解釈を変える", "実質的な矛盾はなく、表現上の小差だけである", "全てのfactが本文と完全に整合する"),
		"coverage": scoreQuestion("抽出factsがsource_contentの主要な結論、重要な根拠、重要な数値をどの程度網羅するか評価する。全ての細部を要求しない。",
			"記事の中心的な結論を欠く", "主要論点の複数を欠き全体像が変わる", "中心は捉えるが重要な論点または数値を一部欠く", "主要論点をほぼ網羅し軽微な省略だけである", "主要な結論・根拠・数値を過不足なく網羅する"),
		"inference_control": scoreQuestion("抽出factsがsource_contentを越えた推測、因果関係、一般化を加えていない程度を評価する。明示内容から必然的な言い換えは許容する。",
			"本文にない重大な推測や因果を事実としている", "複数の飛躍や過度な一般化がある", "限定的だが本文から確定できない解釈がある", "必然的な言い換えの範囲で、推測はほぼない", "明示情報だけで構成され推測・因果の追加がない"),
		"specificity": scoreQuestion("抽出factsが具体的で単独でも意味を持ち、曖昧なプレースホルダーや重複に偏っていない程度を評価する。",
			"大半が曖昧・空疎・重複で利用できない", "具体性を欠くfactが多く対象や内容が分からない", "有用だが一部に曖昧さまたは重複がある", "ほぼ全て具体的で軽微な曖昧さだけである", "全て具体的・簡潔で独立して理解できる"),
		"has_unsupported_fact": noulQuestion("source_contentに直接または明確に支持されない重要なfactが1件でも存在する確率を答える。単なる言い換えは含めない。", "重要な根拠なしfactが少なくとも1件ある", "全ての重要なfactが本文に支持される"),
		"has_contradiction":    noulQuestion("source_contentと矛盾するfactが1件でも存在する確率を答える。数値、日時、主体、肯否、比較方向を確認する。", "本文と矛盾するfactが少なくとも1件ある", "本文と矛盾するfactはない"),
	}
}

func faithfulnessJevQuestions() map[string]any {
	return map[string]any{
		"facts_support": scoreQuestion("summaryの各主張がfactsに支持される程度を評価する。自然な要約・統合は許容するが、factsにない情報追加は支持とみなさない。",
			"中心的主張がfactsにない", "複数の重要主張が支持されない", "一部に曖昧な帰属または支持不足がある", "全ての重要主張が支持され軽微な言い換えだけである", "全主張がfactsに明確かつ直接支持される"),
		"contradiction_free": scoreQuestion("summaryとfactsの間に矛盾がない程度を評価する。数値、日時、主体、肯否、比較方向の反転を重視する。",
			"中心的内容がfactsと正反対である", "重大な数値・主体・肯否の矛盾がある", "限定条件や時点の不一致があり解釈を変える", "実質的な矛盾はなく表現上の小差だけである", "summary全体がfactsと完全に整合する"),
		"coverage": scoreQuestion("summaryが主要factsを読者が理解できる範囲で網羅する程度を評価する。要約なので細部の省略は許容する。",
			"主要な結論を欠き記事の意味が分からない", "複数の主要factsを欠き全体像が変わる", "中心は捉えるが重要なfactを一部欠く", "主要factsをほぼ網羅し要約として妥当な省略だけである", "主要factsを簡潔かつ十分に網羅する"),
		"inference_control": scoreQuestion("summaryがfactsを越えた誇張、推測、因果関係、断定を追加していない程度を評価する。自然な接続や必然的な含意は許容する。",
			"重大な推測や因果を事実として追加している", "複数の誇張・断定・一般化がある", "限定的だがfactsから確定できない解釈がある", "自然な要約の範囲で推測はほぼない", "factsの範囲内で慎重かつ正確に記述する"),
		"entity_numeric_accuracy": scoreQuestion("summary内の固有名詞、数値、単位、割合、日時がfactsと一致する程度を評価する。該当情報がない場合は一致とみなす。",
			"重要な固有名詞または数値が誤っている", "複数の名称・数値・単位に不一致がある", "軽微だが意味に影響し得る不一致がある", "全てほぼ一致し丸め等の無害な差だけである", "全ての固有名詞・数値・日時が正確に一致する"),
		"has_unsupported_claim":       noulQuestion("factsに直接または明確に支持されない重要なsummaryの主張が1件でも存在する確率を答える。自然な要約や必然的な含意は含めない。", "重要な支持なし主張が少なくとも1件ある", "全ての重要主張がfactsに支持される"),
		"has_contradiction":           noulQuestion("factsと矛盾するsummaryの主張が1件でも存在する確率を答える。", "factsと矛盾する主張が少なくとも1件ある", "factsと矛盾する主張はない"),
		"has_entity_numeric_mismatch": noulQuestion("factsと一致しない固有名詞、数値、単位、割合、日時がsummaryに1件でも存在する確率を答える。", "固有名詞または数値等の不一致が少なくとも1件ある", "固有名詞と数値等は全て一致する"),
	}
}

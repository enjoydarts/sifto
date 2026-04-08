package repository

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

type PersonalScoreResult struct {
	Score     float64
	Reason    string
	Breakdown *model.PersonalScoreBreakdown
}

// PersonalScoreInput holds the item-level data needed for personal scoring.
type PersonalScoreInput struct {
	SummaryScore   *float64
	ScoreBreakdown *model.ItemSummaryScoreBreakdown
	Topics         []string
	Embedding      []float64
	SourceID       string
	PublishedAt    *time.Time
	FetchedAt      *time.Time
	CreatedAt      time.Time
}

// CalcPersonalScore computes a personal relevance score for an item given a user profile.
// Returns the score and a reason string explaining the dominant signal.
func CalcPersonalScore(item PersonalScoreInput, profile *model.UserPreferenceProfile) (float64, string) {
	result := CalcPersonalScoreDetailed(item, profile)
	return result.Score, result.Reason
}

func CalcPersonalScoreDetailed(item PersonalScoreInput, profile *model.UserPreferenceProfile) PersonalScoreResult {
	base := 0.0
	if item.SummaryScore != nil {
		base = *item.SummaryScore
	}
	recency := calcRecencyDecay(item)

	// Cold start: not enough feedback
	if profile == nil || profile.FeedbackCount < 10 {
		return PersonalScoreResult{
			Score:     base,
			Reason:    "attention",
			Breakdown: nil,
		}
	}

	// Component weights
	alpha := 0.32 // learned weight score
	baseWeight := 0.18
	beta := 0.14    // topic relevance
	gamma := 0.16   // embedding similarity
	delta := 0.10   // source affinity
	epsilon := 0.10 // recency

	hasEmbedding := len(item.Embedding) > 0 && len(profile.PrefEmbedding) > 0 && len(item.Embedding) == len(profile.PrefEmbedding)

	// Re-normalize if embedding is unavailable
	if !hasEmbedding {
		total := alpha + baseWeight + beta + delta + epsilon
		alpha = alpha / total
		baseWeight = baseWeight / total
		beta = beta / total
		gamma = 0
		delta = delta / total
		epsilon = epsilon / total
	}

	lwScore := calcLearnedWeightScore(item.ScoreBreakdown, profile.LearnedWeights)
	topicRel := calcTopicRelevance(item.Topics, profile.TopicInterests)
	var embSim float64
	if hasEmbedding {
		embSim = calcEmbeddingSimilarity(item.Embedding, profile.PrefEmbedding)
	}
	srcAff := calcSourceAffinity(item.SourceID, profile.SourceAffinities)

	score := alpha*lwScore + baseWeight*base + beta*topicRel + gamma*embSim + delta*srcAff + epsilon*recency
	score *= 0.36 + 0.64*recency

	// Clamp to [0, 1]
	score = clamp01(score)

	reason := determineReason(item, profile, embSim, topicRel, srcAff, recency)
	return PersonalScoreResult{
		Score:  score,
		Reason: reason,
		Breakdown: &model.PersonalScoreBreakdown{
			LearnedWeightScore:  model.PersonalScoreComponent{Value: clamp01(lwScore), Weight: alpha},
			TopicRelevance:      model.PersonalScoreComponent{Value: clamp01(topicRel), Weight: beta},
			EmbeddingSimilarity: model.PersonalScoreComponent{Value: clamp01(embSim), Weight: gamma},
			SourceAffinity:      model.PersonalScoreComponent{Value: clamp01(srcAff), Weight: delta},
			RecencyDecay:        model.PersonalScoreComponent{Value: recency, Weight: epsilon},
			MatchedTopics:       matchedTopics(item.Topics, profile.TopicInterests),
			DominantDimension:   dominantDimension(item.ScoreBreakdown, profile.LearnedWeights),
		},
	}
}

// calcLearnedWeightScore computes a weighted sum of breakdown dimensions.
func calcLearnedWeightScore(bd *model.ItemSummaryScoreBreakdown, weights map[string]float64) float64 {
	if bd == nil || len(weights) == 0 {
		return 0.5
	}
	m := breakdownToMap(*bd)
	var sum, wSum float64
	for k, w := range weights {
		if v, ok := m[k]; ok {
			sum += v * w
			wSum += w
		}
	}
	if wSum <= 0 {
		return 0.5
	}
	return sum / wSum
}

// calcTopicRelevance computes the average interest of the item's topics.
func calcTopicRelevance(topics []string, interests map[string]float64) float64 {
	if len(topics) == 0 || len(interests) == 0 {
		return 0.5
	}
	var sum float64
	var count int
	for _, t := range topics {
		norm := strings.ToLower(strings.TrimSpace(t))
		if norm == "" {
			continue
		}
		if v, ok := interests[norm]; ok {
			sum += v
			count++
		}
	}
	if count == 0 {
		return 0.5
	}
	return clamp01(0.5 + 0.5*(sum/float64(count)))
}

// calcEmbeddingSimilarity computes dot product similarity between embeddings.
func calcEmbeddingSimilarity(a, b []float64) float64 {
	return dotProduct(a, b)
}

// calcSourceAffinity returns the affinity for a source, defaulting to 0.5.
func calcSourceAffinity(sourceID string, affinities map[string]float64) float64 {
	if affinities == nil {
		return 0.5
	}
	if v, ok := affinities[sourceID]; ok {
		return clamp01(0.5 + 0.5*v)
	}
	return 0.5
}

func calcRecencyDecay(item PersonalScoreInput) float64 {
	timestamp := item.PublishedAt
	if timestamp == nil || timestamp.IsZero() {
		timestamp = item.FetchedAt
	}
	if (timestamp == nil || timestamp.IsZero()) && !item.CreatedAt.IsZero() {
		timestamp = &item.CreatedAt
	}
	if timestamp == nil || timestamp.IsZero() {
		return 0.5
	}
	ageHours := time.Since(*timestamp).Hours()
	if ageHours <= 0 {
		return 1
	}
	if ageHours <= 6 {
		return 1
	}
	decay := math.Exp(-math.Ln2 * ageHours / 72.0)
	if decay < 0.02 {
		return 0.02
	}
	return decay
}

// determineReason selects the most informative reason for the score.
func determineReason(item PersonalScoreInput, profile *model.UserPreferenceProfile, embSim, topicRel, srcAff, recency float64) string {
	if recency >= 0.92 {
		return "recency"
	}
	if embSim > 0.7 {
		return "embedding_similarity"
	}
	if topicRel > 0.8 {
		// Find highest-interest topic
		bestTopic := ""
		bestVal := 0.0
		for _, t := range item.Topics {
			norm := strings.ToLower(strings.TrimSpace(t))
			if v, ok := profile.TopicInterests[norm]; ok && v > bestVal {
				bestVal = v
				bestTopic = norm
			}
		}
		if bestTopic != "" {
			return "topic:" + bestTopic
		}
	}
	if srcAff > 0.7 {
		return "source_affinity"
	}

	// Find the breakdown dimension contributing the most
	if bestKey := dominantDimension(item.ScoreBreakdown, profile.LearnedWeights); bestKey != nil {
		return "weight:" + *bestKey
	}

	return "attention"
}

func dominantDimension(bd *model.ItemSummaryScoreBreakdown, weights map[string]float64) *string {
	if bd == nil || len(weights) == 0 {
		return nil
	}
	m := breakdownToMap(*bd)
	bestKey := ""
	bestContrib := -1.0
	for k, w := range weights {
		if v, ok := m[k]; ok {
			contrib := v * w
			if contrib > bestContrib {
				bestContrib = contrib
				bestKey = k
			}
		}
	}
	if bestKey == "" || bestContrib <= 0 {
		return nil
	}
	return &bestKey
}

func matchedTopics(topics []string, interests map[string]float64) []string {
	if len(topics) == 0 || len(interests) == 0 {
		return nil
	}
	out := make([]string, 0, len(topics))
	seen := make(map[string]struct{}, len(topics))
	for _, topic := range topics {
		norm := strings.ToLower(strings.TrimSpace(topic))
		if norm == "" {
			continue
		}
		if _, ok := interests[norm]; !ok {
			continue
		}
		if _, ok := seen[norm]; ok {
			continue
		}
		seen[norm] = struct{}{}
		out = append(out, norm)
	}
	sort.Strings(out)
	return out
}

// clamp01 clamps a value to [0, 1]. Kept for potential future use.
func clamp01(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}

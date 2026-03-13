package repository

import (
	"math"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/model"
)

// PersonalScoreInput holds the item-level data needed for personal scoring.
type PersonalScoreInput struct {
	SummaryScore   *float64
	ScoreBreakdown *model.ItemSummaryScoreBreakdown
	Topics         []string
	Embedding      []float64
	SourceID       string
}

// CalcPersonalScore computes a personal relevance score for an item given a user profile.
// Returns the score and a reason string explaining the dominant signal.
func CalcPersonalScore(item PersonalScoreInput, profile *model.UserPreferenceProfile) (float64, string) {
	base := 0.0
	if item.SummaryScore != nil {
		base = *item.SummaryScore
	}

	// Cold start: not enough feedback
	if profile == nil || profile.FeedbackCount < 10 {
		return base, "attention"
	}

	// Component weights
	alpha := 0.50 // learned weight score
	beta := 0.20  // topic relevance
	gamma := 0.18 // embedding similarity
	delta := 0.12 // source affinity

	hasEmbedding := len(item.Embedding) > 0 && len(profile.PrefEmbedding) > 0 && len(item.Embedding) == len(profile.PrefEmbedding)

	// Re-normalize if embedding is unavailable
	if !hasEmbedding {
		total := alpha + beta + delta
		alpha = alpha / total
		beta = beta / total
		gamma = 0
		delta = delta / total
	}

	lwScore := calcLearnedWeightScore(item.ScoreBreakdown, profile.LearnedWeights)
	topicRel := calcTopicRelevance(item.Topics, profile.TopicInterests)
	var embSim float64
	if hasEmbedding {
		embSim = calcEmbeddingSimilarity(item.Embedding, profile.PrefEmbedding)
	}
	srcAff := calcSourceAffinity(item.SourceID, profile.SourceAffinities)

	score := alpha*lwScore + beta*topicRel + gamma*embSim + delta*srcAff

	// Clamp to [0, 1]
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	reason := determineReason(item, profile, embSim, topicRel, srcAff)
	return score, reason
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
	return sum / float64(count)
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
		return v
	}
	return 0.5
}

// determineReason selects the most informative reason for the score.
func determineReason(item PersonalScoreInput, profile *model.UserPreferenceProfile, embSim, topicRel, srcAff float64) string {
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
	if item.ScoreBreakdown != nil && len(profile.LearnedWeights) > 0 {
		m := breakdownToMap(*item.ScoreBreakdown)
		bestKey := ""
		bestContrib := -1.0
		for k, w := range profile.LearnedWeights {
			if v, ok := m[k]; ok {
				contrib := v * w
				if contrib > bestContrib {
					bestContrib = contrib
					bestKey = k
				}
			}
		}
		if bestKey != "" && bestContrib > 0 {
			return "weight:" + bestKey
		}
	}

	return "attention"
}

// clamp01 clamps a value to [0, 1]. Kept for potential future use.
func clamp01(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}

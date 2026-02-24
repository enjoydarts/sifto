package repository

import (
	"context"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minoru-kitayama/sifto/api/internal/model"
)

type feedbackPreferenceProfile struct {
	prefEmbedding []float64
	embeddingDims int
}

func loadFeedbackPreferenceProfile(ctx context.Context, db *pgxpool.Pool, userID string) (*feedbackPreferenceProfile, error) {
	profile := &feedbackPreferenceProfile{}
	embeddingRows, err := db.Query(ctx, `
		SELECT ie.dimensions, ie.embedding,
		       (
		         CASE
		           WHEN fb.rating > 0 THEN 1.0
		           WHEN fb.rating < 0 THEN -1.0
		           ELSE 0.0
		         END
		         + CASE WHEN fb.is_favorite THEN 0.7 ELSE 0.0 END
		       )::double precision AS signal
		FROM item_feedbacks fb
		JOIN items i ON i.id = fb.item_id
		JOIN sources s ON s.id = i.source_id
		JOIN item_embeddings ie ON ie.item_id = i.id
		WHERE fb.user_id = $1
		  AND s.user_id = $1
		  AND (fb.rating <> 0 OR fb.is_favorite = true)
		ORDER BY fb.updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer embeddingRows.Close()

	var sum []float64
	var sumAbs float64
	var dims int
	for embeddingRows.Next() {
		var rowDims int
		var vec []float64
		var signal float64
		if err := embeddingRows.Scan(&rowDims, &vec, &signal); err != nil {
			return nil, err
		}
		if signal == 0 || rowDims <= 0 || len(vec) != rowDims {
			continue
		}
		if dims == 0 {
			dims = rowDims
			sum = make([]float64, dims)
		}
		if rowDims != dims {
			continue
		}
		for i := range vec {
			sum[i] += vec[i] * signal
		}
		if signal < 0 {
			sumAbs += -signal
		} else {
			sumAbs += signal
		}
	}
	if err := embeddingRows.Err(); err != nil {
		return nil, err
	}
	if dims > 0 && sumAbs > 0 {
		profile.prefEmbedding = make([]float64, dims)
		for i := range sum {
			profile.prefEmbedding[i] = sum[i] / sumAbs
		}
		profile.embeddingDims = dims
	}

	return profile, nil
}

func itemPreferenceAdjustedScore(item model.Item, profile *feedbackPreferenceProfile) float64 {
	base := 0.0
	if item.SummaryScore != nil {
		base = *item.SummaryScore
	}
	if profile == nil {
		if item.IsFavorite {
			return base + 0.12
		}
		return base
	}
	adj := base
	if item.IsFavorite {
		adj += 0.12
	}
	return adj
}

func itemPreferenceAdjustedScoreWithEmbedding(item model.Item, profile *feedbackPreferenceProfile, embeddingBiasByItemID map[string]float64) float64 {
	adj := itemPreferenceAdjustedScore(item, profile)
	if embeddingBiasByItemID != nil {
		if v, ok := embeddingBiasByItemID[item.ID]; ok {
			adj += v * 0.12
		}
	}
	return adj
}

func sortItemsByPreference(items []model.Item, profile *feedbackPreferenceProfile, embeddingBiasByItemID map[string]float64) {
	sort.SliceStable(items, func(i, j int) bool {
		ai := itemPreferenceAdjustedScoreWithEmbedding(items[i], profile, embeddingBiasByItemID)
		aj := itemPreferenceAdjustedScoreWithEmbedding(items[j], profile, embeddingBiasByItemID)
		if ai != aj {
			return ai > aj
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
}

func digestPreferenceAdjustedScore(d model.DigestItemDetail, profile *feedbackPreferenceProfile) float64 {
	item := d.Item
	item.SummaryScore = d.Summary.Score
	item.SummaryTopics = d.Summary.Topics
	if item.CreatedAt.IsZero() && !d.Summary.SummarizedAt.IsZero() {
		item.CreatedAt = d.Summary.SummarizedAt
	}
	return itemPreferenceAdjustedScore(item, profile)
}

func sortDigestItemsByPreference(items []model.DigestItemDetail, profile *feedbackPreferenceProfile, embeddingBiasByItemID map[string]float64) {
	sort.SliceStable(items, func(i, j int) bool {
		ai := digestPreferenceAdjustedScore(items[i], profile)
		if embeddingBiasByItemID != nil {
			ai += embeddingBiasByItemID[items[i].Item.ID] * 0.12
		}
		aj := digestPreferenceAdjustedScore(items[j], profile)
		if embeddingBiasByItemID != nil {
			aj += embeddingBiasByItemID[items[j].Item.ID] * 0.12
		}
		if ai != aj {
			return ai > aj
		}
		ti := digestRecency(items[i])
		tj := digestRecency(items[j])
		return ti.After(tj)
	})
}

func loadEmbeddingBiasByItemID(ctx context.Context, db *pgxpool.Pool, itemIDs []string, profile *feedbackPreferenceProfile) (map[string]float64, error) {
	if profile == nil || profile.embeddingDims <= 0 || len(profile.prefEmbedding) == 0 || len(itemIDs) == 0 {
		return nil, nil
	}
	rows, err := db.Query(ctx, `
		SELECT item_id, embedding
		FROM item_embeddings
		WHERE dimensions = $2
		  AND item_id = ANY($1::uuid[])`,
		itemIDs, profile.embeddingDims)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]float64, len(itemIDs))
	for rows.Next() {
		var itemID string
		var emb []float64
		if err := rows.Scan(&itemID, &emb); err != nil {
			return nil, err
		}
		if len(emb) != len(profile.prefEmbedding) {
			continue
		}
		out[itemID] = dotProduct(profile.prefEmbedding, emb)
	}
	return out, rows.Err()
}

func dotProduct(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}
	var sum float64
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}

func digestRecency(d model.DigestItemDetail) time.Time {
	if d.Item.PublishedAt != nil {
		return *d.Item.PublishedAt
	}
	if !d.Summary.SummarizedAt.IsZero() {
		return d.Summary.SummarizedAt
	}
	return d.Item.CreatedAt
}

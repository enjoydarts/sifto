package repository

import (
	"context"
	"math"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minoru-kitayama/sifto/api/internal/model"
)

func loadItemEmbeddingsByID(ctx context.Context, db *pgxpool.Pool, itemIDs []string) (map[string][]float64, error) {
	if len(itemIDs) == 0 {
		return nil, nil
	}
	rows, err := db.Query(ctx, `
		SELECT item_id, embedding
		FROM item_embeddings
		WHERE item_id = ANY($1::uuid[])`, itemIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string][]float64, len(itemIDs))
	for rows.Next() {
		var itemID string
		var emb []float64
		if err := rows.Scan(&itemID, &emb); err != nil {
			return nil, err
		}
		if len(emb) == 0 {
			continue
		}
		out[itemID] = emb
	}
	return out, rows.Err()
}

func (r *ItemRepo) readingPlanClustersByEmbeddings(ctx context.Context, items []model.Item, selectedItemIDs []string) ([]model.ReadingPlanCluster, error) {
	if len(items) < 2 {
		return nil, nil
	}
	selectedSet := make(map[string]struct{}, len(selectedItemIDs))
	for _, id := range selectedItemIDs {
		selectedSet[id] = struct{}{}
	}
	itemIDs := make([]string, 0, len(items))
	for _, it := range items {
		itemIDs = append(itemIDs, it.ID)
	}
	embByID, err := loadItemEmbeddingsByID(ctx, r.db, itemIDs)
	if err != nil {
		return nil, err
	}
	if len(embByID) < 2 {
		return nil, nil
	}

	used := make([]bool, len(items))
	clusters := make([]model.ReadingPlanCluster, 0, len(items)/2)
	for i := range items {
		if used[i] {
			continue
		}
		seed := items[i]
		seedEmb, ok := embByID[seed.ID]
		if !ok || len(seedEmb) == 0 {
			continue
		}
		used[i] = true
		members := []model.Item{seed}
		maxSim := 0.0
		for j := i + 1; j < len(items); j++ {
			if used[j] {
				continue
			}
			cand := items[j]
			cEmb, ok := embByID[cand.ID]
			if !ok || len(cEmb) == 0 {
				continue
			}
			sim := cosineSimilarity(seedEmb, cEmb)
			if shouldClusterReadingPlan(seed, cand, sim) {
				used[j] = true
				members = append(members, cand)
				if sim > maxSim {
					maxSim = sim
				}
			}
		}
		filtered := members
		if len(selectedSet) > 0 {
			filtered = make([]model.Item, 0, len(members))
			for _, m := range members {
				if _, ok := selectedSet[m.ID]; ok {
					filtered = append(filtered, m)
				}
			}
		}
		if len(filtered) < 2 {
			continue
		}
		sort.SliceStable(filtered, func(a, b int) bool {
			as := -1.0
			if filtered[a].SummaryScore != nil {
				as = *filtered[a].SummaryScore
			}
			bs := -1.0
			if filtered[b].SummaryScore != nil {
				bs = *filtered[b].SummaryScore
			}
			if as != bs {
				return as > bs
			}
			return filtered[a].CreatedAt.After(filtered[b].CreatedAt)
		})
		clusters = append(clusters, model.ReadingPlanCluster{
			ID:             filtered[0].ID,
			Label:          readingPlanClusterLabel(filtered[0]),
			Size:           len(filtered),
			MaxSimilarity:  maxSim,
			Representative: filtered[0],
			Items:          filtered,
		})
	}

	sort.SliceStable(clusters, func(i, j int) bool {
		if clusters[i].Size != clusters[j].Size {
			return clusters[i].Size > clusters[j].Size
		}
		if clusters[i].MaxSimilarity != clusters[j].MaxSimilarity {
			return clusters[i].MaxSimilarity > clusters[j].MaxSimilarity
		}
		return clusters[i].Representative.CreatedAt.After(clusters[j].Representative.CreatedAt)
	})
	return clusters, nil
}

func shouldClusterReadingPlan(seed, cand model.Item, similarity float64) bool {
	if similarity >= 0.84 {
		return true
	}
	if similarity < 0.68 {
		return false
	}
	return hasTopicOverlap(seed.SummaryTopics, cand.SummaryTopics)
}

func hasTopicOverlap(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(a))
	for _, v := range a {
		if v == "" {
			continue
		}
		set[v] = struct{}{}
	}
	for _, v := range b {
		if _, ok := set[v]; ok {
			return true
		}
	}
	return false
}

func readingPlanClusterLabel(it model.Item) string {
	for _, t := range it.SummaryTopics {
		if t != "" {
			return t
		}
	}
	if it.Title != nil && *it.Title != "" {
		return *it.Title
	}
	return "Related"
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

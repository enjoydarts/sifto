package repository

import (
	"context"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func (r *ItemRepo) LoadByIDsPreservingOrder(ctx context.Context, userID string, itemIDs []string) ([]model.Item, error) {
	if len(itemIDs) == 0 {
		return []model.Item{}, nil
	}

	rows, err := r.db.Query(ctx, `
		WITH ranked_ids AS (
			SELECT item_id, ord
			FROM unnest($2::uuid[]) WITH ORDINALITY AS ids(item_id, ord)
		)
		SELECT i.id, i.source_id, s.title AS source_title, i.url, i.title, i.thumbnail_url, COALESCE(sm.summary, i.content_text) AS content_text, i.status, i.processing_error,
		       fc.final_result AS facts_check_result,
		       sfc.final_result AS faithfulness_result,
		       (ir.item_id IS NOT NULL) AS is_read,
		       COALESCE(fb.is_favorite, false) AS is_favorite,
		       COALESCE(fb.rating, 0) AS feedback_rating,
		       sm.score, sm.personal_score, sm.personal_score_reason, COALESCE(sm.topics, '{}'::text[]), sm.translated_title,
		       i.user_genre, `+effectiveGenreExpr("i", "sm")+` AS genre,
		       i.published_at, i.fetched_at, i.created_at, i.updated_at
		FROM ranked_ids rid
		JOIN items i ON i.id = rid.item_id
		JOIN sources s ON s.id = i.source_id
		LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
		LEFT JOIN item_feedbacks fb ON fb.item_id = i.id AND fb.user_id = $1
		LEFT JOIN item_summaries sm ON sm.item_id = i.id
		LEFT JOIN item_facts_checks fc ON fc.item_id = i.id
		LEFT JOIN summary_faithfulness_checks sfc ON sfc.item_id = i.id
		WHERE s.user_id = $1
		ORDER BY rid.ord`,
		userID, itemIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanItemsWithGenres(rows)
}

package repository

import (
	"context"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func (r *ItemRepo) Stats(ctx context.Context, userID string) (*model.ItemStatsResponse, error) {
	rows, err := r.db.Query(ctx, `
		SELECT i.status,
		       COUNT(*)::int AS total,
		       COALESCE(SUM(CASE WHEN ir.item_id IS NOT NULL THEN 1 ELSE 0 END), 0)::int AS read_count
		FROM items i
		JOIN sources s ON s.id = i.source_id
		LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
		WHERE s.user_id = $1
		  AND i.deleted_at IS NULL
		GROUP BY i.status`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	resp := &model.ItemStatsResponse{ByStatus: map[string]int{}}
	for rows.Next() {
		var status string
		var total, readCount int
		if err := rows.Scan(&status, &total, &readCount); err != nil {
			return nil, err
		}
		resp.ByStatus[status] = total
		resp.Total += total
		resp.Read += readCount
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	resp.Unread = resp.Total - resp.Read
	return resp, nil
}

func (r *ItemRepo) CountNewOnDateJST(ctx context.Context, userID, date string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM items i
		JOIN sources s ON s.id = i.source_id
		WHERE s.user_id = $1
		  AND i.deleted_at IS NULL
		  AND (COALESCE(i.published_at, i.created_at) AT TIME ZONE 'Asia/Tokyo')::date = $2::date`,
		userID, date,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *ItemRepo) CountReadOnDateJST(ctx context.Context, userID, date string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM item_reads ir
		JOIN items i ON i.id = ir.item_id
		JOIN sources s ON s.id = i.source_id
		WHERE s.user_id = $1
		  AND i.deleted_at IS NULL
		  AND (ir.read_at AT TIME ZONE 'Asia/Tokyo')::date = $2::date`,
		userID, date,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *ItemRepo) ReadActivityInRangeJST(ctx context.Context, userID string, from, to string) (readCount int, activeDays int, err error) {
	err = r.db.QueryRow(ctx, `
		WITH reads AS (
			SELECT (ir.read_at AT TIME ZONE 'Asia/Tokyo')::date AS day_jst
			FROM item_reads ir
			JOIN items i ON i.id = ir.item_id
			JOIN sources s ON s.id = i.source_id
			WHERE s.user_id = $1
			  AND i.deleted_at IS NULL
			  AND (ir.read_at AT TIME ZONE 'Asia/Tokyo')::date >= $2::date
			  AND (ir.read_at AT TIME ZONE 'Asia/Tokyo')::date <= $3::date
		)
		SELECT COUNT(*)::int AS read_count, COUNT(DISTINCT day_jst)::int AS active_days
		FROM reads`,
		userID, from, to,
	).Scan(&readCount, &activeDays)
	if err != nil {
		return 0, 0, err
	}
	return readCount, activeDays, nil
}

func (r *ItemRepo) HighlightItems24h(ctx context.Context, userID string, minScore float64, limit int) ([]model.Item, error) {
	if limit <= 0 {
		limit = 3
	}
	if limit > 30 {
		limit = 30
	}
	rows, err := r.db.Query(ctx, `
		SELECT i.id, i.source_id, s.title AS source_title, i.url, i.title, i.thumbnail_url, NULL::text AS content_text, i.status, i.processing_error,
		       fc.final_result AS facts_check_result,
		       sfc.final_result AS faithfulness_result,
		       (ir.item_id IS NOT NULL) AS is_read,
		       COALESCE(fb.is_favorite, false) AS is_favorite,
		       COALESCE(fb.rating, 0) AS feedback_rating,
		       sm.score, sm.personal_score, sm.personal_score_reason, COALESCE(sm.topics, '{}'::text[]), sm.translated_title,
		       i.published_at, i.fetched_at, i.created_at, i.updated_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		JOIN item_summaries sm ON sm.item_id = i.id
		LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
		LEFT JOIN item_feedbacks fb ON fb.item_id = i.id AND fb.user_id = $1
		LEFT JOIN item_facts_checks fc ON fc.item_id = i.id
		LEFT JOIN summary_faithfulness_checks sfc ON sfc.item_id = i.id
		WHERE s.user_id = $1
		  AND i.deleted_at IS NULL
		  AND i.status = 'summarized'
		  AND `+briefingEffectiveTimeSQL+` >= NOW() - INTERVAL '24 hours'
		  AND COALESCE(sm.score, 0) >= $2
		ORDER BY sm.score DESC NULLS LAST, i.created_at DESC
		LIMIT $3`,
		userID, minScore, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItems(rows)
}

func (r *ItemRepo) SummariesByItemIDs(ctx context.Context, userID string, itemIDs []string) (map[string]string, error) {
	if len(itemIDs) == 0 {
		return map[string]string{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT i.id, sm.summary
		FROM items i
		JOIN sources s ON s.id = i.source_id
		JOIN item_summaries sm ON sm.item_id = i.id
		WHERE s.user_id = $1
		  AND i.deleted_at IS NULL
		  AND i.id = ANY($2::uuid[])`,
		userID, itemIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string, len(itemIDs))
	for rows.Next() {
		var itemID string
		var summary string
		if err := rows.Scan(&itemID, &summary); err != nil {
			return nil, err
		}
		out[itemID] = summary
	}
	return out, rows.Err()
}

func (r *ItemRepo) CountSummarizedOnDateJST(ctx context.Context, userID, date string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM items i
		JOIN sources s ON s.id = i.source_id
		WHERE s.user_id = $1
		  AND i.deleted_at IS NULL
		  AND i.status = 'summarized'
		  AND (COALESCE(i.published_at, i.created_at) AT TIME ZONE 'Asia/Tokyo')::date = $2::date`,
		userID, date,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *ItemRepo) CountSummarizedReadUnreadOnDateJST(ctx context.Context, userID, date string) (readCount int, unreadCount int, err error) {
	err = r.db.QueryRow(ctx, `
		SELECT
		  COALESCE(SUM(CASE WHEN ir.item_id IS NOT NULL THEN 1 ELSE 0 END), 0)::int AS read_count,
		  COALESCE(SUM(CASE WHEN ir.item_id IS NULL THEN 1 ELSE 0 END), 0)::int AS unread_count
		FROM items i
		JOIN sources s ON s.id = i.source_id
		LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
		WHERE s.user_id = $1
		  AND i.status = 'summarized'
		  AND (COALESCE(i.published_at, i.created_at) AT TIME ZONE 'Asia/Tokyo')::date = $2::date`,
		userID, date,
	).Scan(&readCount, &unreadCount)
	if err != nil {
		return 0, 0, err
	}
	return readCount, unreadCount, nil
}

func (r *ItemRepo) FavoriteExportItems(ctx context.Context, userID string, days, limit int) ([]model.FavoriteExportItem, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	query := `
		SELECT i.id, i.url, i.title, sm.translated_title, s.title,
		       sm.summary, COALESCE(sm.topics, '{}'::text[]), sm.score,
		       COALESCE(i.published_at, i.created_at) AS published_at,
		       fb.updated_at
		FROM item_feedbacks fb
		JOIN items i ON i.id = fb.item_id
		JOIN sources s ON s.id = i.source_id
		LEFT JOIN item_summaries sm ON sm.item_id = i.id
		WHERE fb.user_id = $1
		  AND s.user_id = $1
		  AND i.deleted_at IS NULL
		  AND fb.is_favorite = true`
	args := []any{userID}
	if days > 0 {
		args = append(args, days)
		query += ` AND fb.updated_at >= NOW() - ($` + itoa(len(args)) + `::int * INTERVAL '1 day')`
	}
	args = append(args, limit)
	query += ` ORDER BY fb.updated_at DESC, sm.score DESC NULLS LAST, i.created_at DESC LIMIT $` + itoa(len(args))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.FavoriteExportItem, 0, limit)
	for rows.Next() {
		var item model.FavoriteExportItem
		if err := rows.Scan(
			&item.ID,
			&item.URL,
			&item.Title,
			&item.TranslatedTitle,
			&item.SourceTitle,
			&item.Summary,
			&item.Topics,
			&item.SummaryScore,
			&item.PublishedAt,
			&item.FavoritedAt,
		); err != nil {
			return nil, err
		}
		if llm, err := loadLatestItemLLMUsage(ctx, r.db, item.ID, "summary"); err == nil {
			item.SummaryLLM = llm
		}
		if llm, err := loadLatestItemLLMUsage(ctx, r.db, item.ID, "facts"); err == nil {
			item.FactsLLM = llm
		}
		if emb, err := loadLatestItemEmbeddingModel(ctx, r.db, item.ID); err == nil {
			item.EmbeddingModel = emb
		}
		if note, highlights, err := r.GetByItem(ctx, userID, item.ID); err == nil {
			item.Note = note
			item.Highlights = highlights
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

package repository

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minoru-kitayama/sifto/api/internal/model"
)

type ItemRepo struct{ db *pgxpool.Pool }

func NewItemRepo(db *pgxpool.Pool) *ItemRepo { return &ItemRepo{db} }

type ItemListParams struct {
	Status   *string
	SourceID *string
	UnreadOnly bool
	Sort     string // newest | score
	Page     int
	PageSize int
}

type ReadingPlanParams struct {
	Window         string // 24h | today_jst | 7d
	Size            int
	DiversifyTopics bool
	ExcludeRead    bool
}

func (r *ItemRepo) List(ctx context.Context, userID string, status, sourceID *string, limit int) ([]model.Item, error) {
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}
	query := `
		SELECT i.id, i.source_id, i.url, i.title, i.content_text, i.status,
		       (ir.item_id IS NOT NULL) AS is_read,
		       sm.score, COALESCE(sm.topics, '{}'::text[]),
		       i.published_at, i.fetched_at, i.created_at, i.updated_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
		LEFT JOIN item_summaries sm ON sm.item_id = i.id
		WHERE s.user_id = $1`
	args := []any{userID}

	if status != nil {
		args = append(args, *status)
		query += ` AND i.status = $` + itoa(len(args))
	}
	if sourceID != nil {
		args = append(args, *sourceID)
		query += ` AND i.source_id = $` + itoa(len(args))
	}
	if status != nil && *status == "summarized" {
		query += ` ORDER BY sm.score DESC NULLS LAST, i.created_at DESC LIMIT ` + itoa(limit)
	} else {
		query += ` ORDER BY i.created_at DESC LIMIT ` + itoa(limit)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.Item
	for rows.Next() {
		var it model.Item
		if err := rows.Scan(&it.ID, &it.SourceID, &it.URL, &it.Title, &it.ContentText,
			&it.Status, &it.IsRead, &it.SummaryScore, &it.SummaryTopics, &it.PublishedAt, &it.FetchedAt, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, nil
}

func (r *ItemRepo) ListPage(ctx context.Context, userID string, p ItemListParams) (*model.ItemListResponse, error) {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.PageSize <= 0 {
		p.PageSize = 20
	}
	if p.PageSize > 200 {
		p.PageSize = 200
	}
	if p.Sort != "score" {
		p.Sort = "newest"
	}

	baseWhere := ` FROM items i
		JOIN sources s ON s.id = i.source_id
		WHERE s.user_id = $1`
	args := []any{userID}
	if p.Status != nil {
		args = append(args, *p.Status)
		baseWhere += ` AND i.status = $` + itoa(len(args))
	}
	if p.SourceID != nil {
		args = append(args, *p.SourceID)
		baseWhere += ` AND i.source_id = $` + itoa(len(args))
	}
	if p.UnreadOnly {
		baseWhere += ` AND NOT EXISTS (
			SELECT 1 FROM item_reads ir2
			WHERE ir2.item_id = i.id AND ir2.user_id = $1
		)`
	}

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*)`+baseWhere, args...).Scan(&total); err != nil {
		return nil, err
	}

	offset := (p.Page - 1) * p.PageSize
	args = append(args, p.PageSize, offset)
	limitArg := `$` + itoa(len(args)-1)
	offsetArg := `$` + itoa(len(args))

	orderBy := ` ORDER BY i.created_at DESC`
	if p.Sort == "score" {
		orderBy = ` ORDER BY sm.score DESC NULLS LAST, i.created_at DESC`
	}

	rows, err := r.db.Query(ctx, `
		SELECT i.id, i.source_id, i.url, i.title, i.content_text, i.status,
		       (ir.item_id IS NOT NULL) AS is_read,
		       sm.score, COALESCE(sm.topics, '{}'::text[]),
		       i.published_at, i.fetched_at, i.created_at, i.updated_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
		LEFT JOIN item_summaries sm ON sm.item_id = i.id
		WHERE s.user_id = $1`+
		func() string {
			q := ""
			if p.Status != nil {
				q += ` AND i.status = $2`
			}
			if p.SourceID != nil {
				if p.Status != nil {
					q += ` AND i.source_id = $3`
				} else {
					q += ` AND i.source_id = $2`
				}
			}
			if p.UnreadOnly {
				q += ` AND ir.item_id IS NULL`
			}
			return q
		}()+
		orderBy+` LIMIT `+limitArg+` OFFSET `+offsetArg,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items, err := scanItems(rows)
	if err != nil {
		return nil, err
	}
	return &model.ItemListResponse{
		Items:    items,
		Page:     p.Page,
		PageSize: p.PageSize,
		Total:    total,
		HasNext:  offset+len(items) < total,
		Sort:     p.Sort,
		Status:   p.Status,
		SourceID: p.SourceID,
	}, nil
}

func (r *ItemRepo) ReadingPlan(ctx context.Context, userID string, p ReadingPlanParams) (*model.ReadingPlanResponse, error) {
	if p.Size <= 0 {
		p.Size = 15
	}
	if p.Size > 100 {
		p.Size = 100
	}
	if p.Window == "" {
		p.Window = "24h"
	}
	// Pull a sufficiently large candidate pool, then diversify in Go.
	candidateLimit := 2000
	filterSQL := ``
	switch p.Window {
	case "today_jst":
		filterSQL = ` AND (COALESCE(i.published_at, i.created_at) AT TIME ZONE 'Asia/Tokyo')::date = (NOW() AT TIME ZONE 'Asia/Tokyo')::date`
	case "7d":
		filterSQL = ` AND COALESCE(i.published_at, i.created_at) >= NOW() - INTERVAL '7 days'`
	default:
		p.Window = "24h"
		filterSQL = ` AND COALESCE(i.published_at, i.created_at) >= NOW() - INTERVAL '24 hours'`
	}
	if p.ExcludeRead {
		filterSQL += ` AND ir.item_id IS NULL`
	}

	var poolCount int
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM items i
		JOIN sources s ON s.id = i.source_id
		LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
		WHERE s.user_id = $1
		  AND i.status = 'summarized'`+filterSQL, userID).Scan(&poolCount); err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT i.id, i.source_id, i.url, i.title, i.content_text, i.status,
		       (ir.item_id IS NOT NULL) AS is_read,
		       sm.score, COALESCE(sm.topics, '{}'::text[]),
		       i.published_at, i.fetched_at, i.created_at, i.updated_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
		LEFT JOIN item_summaries sm ON sm.item_id = i.id
		WHERE s.user_id = $1
		  AND i.status = 'summarized'`+filterSQL+`
		ORDER BY sm.score DESC NULLS LAST, i.created_at DESC
		LIMIT $2`, userID, candidateLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	candidates, err := scanItems(rows)
	if err != nil {
		return nil, err
	}

	selected := make([]model.Item, 0, p.Size)
	if p.DiversifyTopics {
		seen := map[string]struct{}{}
		for _, it := range candidates {
			if len(selected) >= p.Size {
				break
			}
			key := firstTopicKey(it.SummaryTopics)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			selected = append(selected, it)
		}
	}
	if !p.DiversifyTopics || len(selected) < p.Size {
		exists := map[string]struct{}{}
		for _, it := range selected {
			exists[it.ID] = struct{}{}
		}
		for _, it := range candidates {
			if len(selected) >= p.Size {
				break
			}
			if _, ok := exists[it.ID]; ok {
				continue
			}
			selected = append(selected, it)
			exists[it.ID] = struct{}{}
		}
	}

	topics, err := r.readingPlanTopics(ctx, userID, p)
	if err != nil {
		return nil, err
	}

	return &model.ReadingPlanResponse{
		Items:           selected,
		Window:          p.Window,
		Size:            p.Size,
		DiversifyTopics: p.DiversifyTopics,
		ExcludeRead:     p.ExcludeRead,
		SourcePoolCount: poolCount,
		Topics:          topics,
	}, nil
}

func (r *ItemRepo) readingPlanTopics(ctx context.Context, userID string, p ReadingPlanParams) ([]model.ReadingPlanTopic, error) {
	filterSQL := ``
	switch p.Window {
	case "today_jst":
		filterSQL = ` AND (COALESCE(i.published_at, i.created_at) AT TIME ZONE 'Asia/Tokyo')::date = (NOW() AT TIME ZONE 'Asia/Tokyo')::date`
	case "7d":
		filterSQL = ` AND COALESCE(i.published_at, i.created_at) >= NOW() - INTERVAL '7 days'`
	default:
		filterSQL = ` AND COALESCE(i.published_at, i.created_at) >= NOW() - INTERVAL '24 hours'`
	}
	if p.ExcludeRead {
		filterSQL += ` AND ir.item_id IS NULL`
	}

	rows, err := r.db.Query(ctx, `
		WITH base AS (
			SELECT COALESCE(NULLIF(BTRIM(t.topic), ''), '__untagged__') AS topic_key, sm.score
			FROM items i
			JOIN sources s ON s.id = i.source_id
			LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
			JOIN item_summaries sm ON sm.item_id = i.id
			CROSS JOIN LATERAL unnest(
				CASE
					WHEN COALESCE(array_length(sm.topics, 1), 0) = 0 THEN ARRAY['__untagged__']::text[]
					ELSE sm.topics
				END
			) AS t(topic)
			WHERE s.user_id = $1
			  AND i.status = 'summarized'`+filterSQL+`
		)
		SELECT topic_key, COUNT(*)::int, MAX(score)::double precision
		FROM base
		GROUP BY topic_key
		ORDER BY COUNT(*) DESC, MAX(score) DESC NULLS LAST, topic_key ASC
		LIMIT 12`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.ReadingPlanTopic
	for rows.Next() {
		var v model.ReadingPlanTopic
		if err := rows.Scan(&v.Topic, &v.Count, &v.MaxScore); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *ItemRepo) Stats(ctx context.Context, userID string) (*model.ItemStatsResponse, error) {
	rows, err := r.db.Query(ctx, `
		SELECT i.status,
		       COUNT(*)::int AS total,
		       COALESCE(SUM(CASE WHEN ir.item_id IS NOT NULL THEN 1 ELSE 0 END), 0)::int AS read_count
		FROM items i
		JOIN sources s ON s.id = i.source_id
		LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
		WHERE s.user_id = $1
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

func firstTopicKey(topics []string) string {
	for _, t := range topics {
		if t != "" {
			return t
		}
	}
	return "__untagged__"
}

type itemRowScanner interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanItems(rows itemRowScanner) ([]model.Item, error) {
	var items []model.Item
	for rows.Next() {
		var it model.Item
		if err := rows.Scan(&it.ID, &it.SourceID, &it.URL, &it.Title, &it.ContentText,
			&it.Status, &it.IsRead, &it.SummaryScore, &it.SummaryTopics, &it.PublishedAt, &it.FetchedAt, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (r *ItemRepo) GetDetail(ctx context.Context, id, userID string) (*model.ItemDetail, error) {
	var d model.ItemDetail
	err := r.db.QueryRow(ctx, `
		SELECT i.id, i.source_id, i.url, i.title, i.content_text, i.status,
		       EXISTS (
		           SELECT 1 FROM item_reads ir
		           WHERE ir.item_id = i.id AND ir.user_id = $2
		       ) AS is_read,
		       i.published_at, i.fetched_at, i.created_at, i.updated_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		WHERE i.id = $1 AND s.user_id = $2`, id, userID,
	).Scan(&d.ID, &d.SourceID, &d.URL, &d.Title, &d.ContentText,
		&d.Status, &d.IsRead, &d.PublishedAt, &d.FetchedAt, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, mapDBError(err)
	}

	// facts
	var f model.ItemFacts
	err = r.db.QueryRow(ctx, `
		SELECT id, item_id, facts, extracted_at FROM item_facts WHERE item_id = $1`, id,
	).Scan(&f.ID, &f.ItemID, &f.Facts, &f.ExtractedAt)
	if err == nil {
		d.Facts = &f
	}

	// summary
	var s model.ItemSummary
	err = r.db.QueryRow(ctx, `
		SELECT id, item_id, summary, topics, score, score_breakdown, score_reason, score_policy_version, summarized_at
		FROM item_summaries WHERE item_id = $1`, id,
	).Scan(&s.ID, &s.ItemID, &s.Summary, &s.Topics, &s.Score,
		scoreBreakdownScanner{dst: &s.ScoreBreakdown}, &s.ScoreReason, &s.ScorePolicyVersion, &s.SummarizedAt)
	if err == nil {
		d.Summary = &s
	}

	return &d, nil
}

func (r *ItemRepo) ListRelated(ctx context.Context, id, userID string, limit int) ([]model.RelatedItem, error) {
	if limit <= 0 {
		limit = 6
	}
	if limit > 20 {
		limit = 20
	}

	rows, err := r.db.Query(ctx, `
		WITH target AS (
			SELECT ie.embedding AS emb, ie.dimensions AS dims
			FROM item_embeddings ie
			JOIN items ti ON ti.id = ie.item_id
			JOIN sources ts ON ts.id = ti.source_id
			WHERE ie.item_id = $1
			  AND ts.user_id = $2
		)
		SELECT i.id, i.source_id, i.url, i.title,
		       sm.summary, COALESCE(sm.topics, '{}'::text[]), sm.score,
		       COALESCE(
		         (
		           SELECT SUM(tv * cv)
		           FROM unnest(t.emb) WITH ORDINALITY AS tval(tv, idx)
		           JOIN unnest(ie.embedding) WITH ORDINALITY AS cval(cv, idx) USING (idx)
		         ),
		         0
		       )::double precision AS similarity,
		       i.published_at, i.created_at
		FROM target t
		JOIN item_embeddings ie ON ie.item_id <> $1 AND ie.dimensions = t.dims
		JOIN items i ON i.id = ie.item_id
		JOIN sources s ON s.id = i.source_id
		LEFT JOIN item_summaries sm ON sm.item_id = i.id
		WHERE s.user_id = $2
		  AND i.status = 'summarized'
		ORDER BY similarity DESC, COALESCE(i.published_at, i.created_at) DESC
		LIMIT $3`, id, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.RelatedItem
	for rows.Next() {
		var v model.RelatedItem
		if err := rows.Scan(
			&v.ID, &v.SourceID, &v.URL, &v.Title,
			&v.Summary, &v.Topics, &v.SummaryScore,
			&v.Similarity, &v.PublishedAt, &v.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *ItemRepo) GetForRetry(ctx context.Context, id, userID string) (*model.Item, error) {
	var it model.Item
	err := r.db.QueryRow(ctx, `
		SELECT i.id, i.source_id, i.url, i.title, i.content_text, i.status,
		       FALSE AS is_read,
		       i.published_at, i.fetched_at, i.created_at, i.updated_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		WHERE i.id = $1 AND s.user_id = $2`, id, userID,
	).Scan(&it.ID, &it.SourceID, &it.URL, &it.Title, &it.ContentText,
		&it.Status, &it.IsRead, &it.PublishedAt, &it.FetchedAt, &it.CreatedAt, &it.UpdatedAt)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &it, nil
}

func (r *ItemRepo) ListFailedForRetry(ctx context.Context, userID string, sourceID *string) ([]model.Item, error) {
	query := `
		SELECT i.id, i.source_id, i.url, i.title, i.content_text, i.status,
		       FALSE AS is_read,
		       i.published_at, i.fetched_at, i.created_at, i.updated_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		WHERE s.user_id = $1 AND i.status = 'failed'`
	args := []any{userID}
	if sourceID != nil {
		args = append(args, *sourceID)
		query += ` AND i.source_id = $2`
	}
	query += ` ORDER BY i.updated_at DESC LIMIT 500`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.Item
	for rows.Next() {
		var it model.Item
		if err := rows.Scan(&it.ID, &it.SourceID, &it.URL, &it.Title, &it.ContentText,
			&it.Status, &it.IsRead, &it.PublishedAt, &it.FetchedAt, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, nil
}

func (r *ItemRepo) UpsertFromFeed(ctx context.Context, sourceID, url string, title *string) (string, bool, error) {
	var id string
	var created bool
	err := r.db.QueryRow(ctx, `
		INSERT INTO items (source_id, url, title)
		VALUES ($1, $2, $3)
		ON CONFLICT (source_id, url) DO NOTHING
		RETURNING id, true`,
		sourceID, url, title,
	).Scan(&id, &created)
	if err != nil {
		// conflict - already exists
		err2 := r.db.QueryRow(ctx, `SELECT id FROM items WHERE source_id = $1 AND url = $2`, sourceID, url).Scan(&id)
		return id, false, err2
	}
	return id, true, nil
}

func (r *ItemRepo) MarkRead(ctx context.Context, userID, itemID string) error {
	if err := r.ensureOwned(ctx, userID, itemID); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO item_reads (user_id, item_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, item_id) DO UPDATE
		SET read_at = NOW()`,
		userID, itemID,
	)
	return err
}

func (r *ItemRepo) MarkUnread(ctx context.Context, userID, itemID string) error {
	if err := r.ensureOwned(ctx, userID, itemID); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx, `DELETE FROM item_reads WHERE user_id = $1 AND item_id = $2`, userID, itemID)
	return err
}

func (r *ItemRepo) ensureOwned(ctx context.Context, userID, itemID string) error {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM items i
			JOIN sources s ON s.id = i.source_id
			WHERE i.id = $1 AND s.user_id = $2
		)`,
		itemID, userID,
	).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

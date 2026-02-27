package repository

import (
	"context"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minoru-kitayama/sifto/api/internal/model"
)

type ItemRepo struct{ db *pgxpool.Pool }

func NewItemRepo(db *pgxpool.Pool) *ItemRepo { return &ItemRepo{db} }

type ItemListParams struct {
	Status       *string
	SourceID     *string
	Topic        *string
	UnreadOnly   bool
	FavoriteOnly bool
	Sort         string // newest | score
	Page         int
	PageSize     int
}

type ReadingPlanParams struct {
	Window          string // 24h | today_jst | 7d
	Size            int
	DiversifyTopics bool
	ExcludeRead     bool
}

func (r *ItemRepo) List(ctx context.Context, userID string, status, sourceID *string, limit int) ([]model.Item, error) {
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}
	query := `
		SELECT i.id, i.source_id, i.url, i.title, i.thumbnail_url, NULL::text AS content_text, i.status,
		       (ir.item_id IS NOT NULL) AS is_read,
		       COALESCE(fb.is_favorite, false) AS is_favorite,
		       COALESCE(fb.rating, 0) AS feedback_rating,
		       sm.score, COALESCE(sm.topics, '{}'::text[]),
		       i.published_at, i.fetched_at, i.created_at, i.updated_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
		LEFT JOIN item_feedbacks fb ON fb.item_id = i.id AND fb.user_id = $1
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
		if err := rows.Scan(&it.ID, &it.SourceID, &it.URL, &it.Title, &it.ThumbnailURL, &it.ContentText,
			&it.Status, &it.IsRead, &it.IsFavorite, &it.FeedbackRating, &it.SummaryScore, &it.SummaryTopics, &it.PublishedAt, &it.FetchedAt, &it.CreatedAt, &it.UpdatedAt); err != nil {
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
	if p.Topic != nil && *p.Topic != "" {
		args = append(args, *p.Topic)
		baseWhere += ` AND EXISTS (
			SELECT 1
			FROM item_summaries smt
			WHERE smt.item_id = i.id
			  AND $` + itoa(len(args)) + `::text = ANY(COALESCE(smt.topics, '{}'::text[]))
		)`
	}
	if p.UnreadOnly {
		baseWhere += ` AND NOT EXISTS (
			SELECT 1 FROM item_reads ir2
			WHERE ir2.item_id = i.id AND ir2.user_id = $1
		)`
	}
	if p.FavoriteOnly {
		baseWhere += ` AND EXISTS (
			SELECT 1 FROM item_feedbacks fb2
			WHERE fb2.item_id = i.id AND fb2.user_id = $1 AND fb2.is_favorite = true
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
		SELECT i.id, i.source_id, i.url, i.title, i.thumbnail_url, NULL::text AS content_text, i.status,
		       (ir.item_id IS NOT NULL) AS is_read,
		       COALESCE(fb.is_favorite, false) AS is_favorite,
		       COALESCE(fb.rating, 0) AS feedback_rating,
		       sm.score, COALESCE(sm.topics, '{}'::text[]),
		       i.published_at, i.fetched_at, i.created_at, i.updated_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
		LEFT JOIN item_feedbacks fb ON fb.item_id = i.id AND fb.user_id = $1
		LEFT JOIN item_summaries sm ON sm.item_id = i.id
		WHERE s.user_id = $1`+
		func() string {
			q := ""
			nextIdx := 2
			if p.Status != nil {
				q += ` AND i.status = $` + itoa(nextIdx)
				nextIdx++
			}
			if p.SourceID != nil {
				q += ` AND i.source_id = $` + itoa(nextIdx)
				nextIdx++
			}
			if p.Topic != nil && *p.Topic != "" {
				q += ` AND EXISTS (
					SELECT 1 FROM item_summaries smt
					WHERE smt.item_id = i.id
					  AND $` + itoa(nextIdx) + `::text = ANY(COALESCE(smt.topics, '{}'::text[]))
				)`
				nextIdx++
			}
			if p.UnreadOnly {
				q += ` AND ir.item_id IS NULL`
			}
			if p.FavoriteOnly {
				q += ` AND COALESCE(fb.is_favorite, false) = true`
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
		SELECT i.id, i.source_id, i.url, i.title, i.thumbnail_url, NULL::text AS content_text, i.status,
		       (ir.item_id IS NOT NULL) AS is_read,
		       COALESCE(fb.is_favorite, false) AS is_favorite,
		       COALESCE(fb.rating, 0) AS feedback_rating,
		       sm.score, COALESCE(sm.topics, '{}'::text[]),
		       i.published_at, i.fetched_at, i.created_at, i.updated_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
		LEFT JOIN item_feedbacks fb ON fb.item_id = i.id AND fb.user_id = $1
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
	profile, err := loadFeedbackPreferenceProfile(ctx, r.db, userID)
	if err != nil {
		return nil, err
	}
	candidateIDs := make([]string, 0, len(candidates))
	for _, it := range candidates {
		candidateIDs = append(candidateIDs, it.ID)
	}
	embeddingBiasByItemID, err := loadEmbeddingBiasByItemID(ctx, r.db, candidateIDs, profile)
	if err != nil {
		return nil, err
	}
	sortItemsByPreference(candidates, profile, embeddingBiasByItemID)
	candidateEmbByItemID, err := loadItemEmbeddingsByID(ctx, r.db, candidateIDs)
	if err != nil {
		return nil, err
	}

	selected := selectItemsByMMR(candidates, p.Size, p.DiversifyTopics, profile, embeddingBiasByItemID, candidateEmbByItemID)
	topics, err := r.readingPlanTopics(ctx, userID, p)
	if err != nil {
		return nil, err
	}
	selectedIDs := make([]string, 0, len(selected))
	for _, it := range selected {
		selectedIDs = append(selectedIDs, it.ID)
	}
	clusters, err := r.readingPlanClustersByEmbeddings(ctx, candidates, selectedIDs)
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
		Clusters:        clusters,
	}, nil
}

// ClusterItemsByEmbeddings clusters arbitrary items using the same embeddings-based
// logic as ReadingPlan (without filtering by selected IDs).
func (r *ItemRepo) ClusterItemsByEmbeddings(ctx context.Context, items []model.Item) ([]model.ReadingPlanCluster, error) {
	return r.readingPlanClustersByEmbeddings(ctx, items, nil)
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

func (r *ItemRepo) TopicTrends(ctx context.Context, userID string, limit int) ([]model.TopicTrend, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	rows, err := r.db.Query(ctx, `
		WITH base AS (
			SELECT COALESCE(NULLIF(BTRIM(t.topic), ''), '__untagged__') AS topic_key,
			       COALESCE(sm.score, 0)::double precision AS score,
			       COALESCE(i.published_at, i.created_at) AS ts
			FROM items i
			JOIN sources s ON s.id = i.source_id
			JOIN item_summaries sm ON sm.item_id = i.id
			CROSS JOIN LATERAL unnest(
				CASE
					WHEN COALESCE(array_length(sm.topics, 1), 0) = 0 THEN ARRAY['__untagged__']::text[]
					ELSE sm.topics
				END
			) AS t(topic)
			WHERE s.user_id = $1
			  AND i.status = 'summarized'
			  AND COALESCE(i.published_at, i.created_at) >= NOW() - INTERVAL '48 hours'
		)
		SELECT topic_key,
		       COUNT(*) FILTER (WHERE ts >= NOW() - INTERVAL '24 hours')::int AS count_24h,
		       COUNT(*) FILTER (WHERE ts < NOW() - INTERVAL '24 hours')::int AS count_prev_24h,
		       MAX(score) FILTER (WHERE ts >= NOW() - INTERVAL '24 hours')::double precision AS max_score_24h
		FROM base
		GROUP BY topic_key
		HAVING COUNT(*) FILTER (WHERE ts >= NOW() - INTERVAL '24 hours') > 0
		ORDER BY
		  (COUNT(*) FILTER (WHERE ts >= NOW() - INTERVAL '24 hours')
		   - COUNT(*) FILTER (WHERE ts < NOW() - INTERVAL '24 hours')) DESC,
		  COUNT(*) FILTER (WHERE ts >= NOW() - INTERVAL '24 hours') DESC,
		  topic_key ASC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.TopicTrend
	for rows.Next() {
		var v model.TopicTrend
		if err := rows.Scan(&v.Topic, &v.Count24h, &v.CountPrev24h, &v.MaxScore24h); err != nil {
			return nil, err
		}
		v.Delta = v.Count24h - v.CountPrev24h
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *ItemRepo) PositiveFeedbackTopics(ctx context.Context, userID string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 12
	}
	if limit > 50 {
		limit = 50
	}
	rows, err := r.db.Query(ctx, `
		WITH weighted AS (
			SELECT COALESCE(NULLIF(BTRIM(t.topic), ''), '') AS topic,
			       (
			         CASE WHEN fb.rating > 0 THEN 2 ELSE 0 END
			         + CASE WHEN fb.is_favorite THEN 3 ELSE 0 END
			       )::int AS w
			FROM item_feedbacks fb
			JOIN items i ON i.id = fb.item_id
			JOIN sources s ON s.id = i.source_id
			JOIN item_summaries sm ON sm.item_id = i.id
			CROSS JOIN LATERAL unnest(COALESCE(sm.topics, '{}'::text[])) AS t(topic)
			WHERE fb.user_id = $1
			  AND s.user_id = $1
			  AND (fb.rating > 0 OR fb.is_favorite = true)
		)
		SELECT topic
		FROM weighted
		WHERE topic <> ''
		GROUP BY topic
		ORDER BY SUM(w) DESC, COUNT(*) DESC, topic ASC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0, limit)
	for rows.Next() {
		var topic string
		if err := rows.Scan(&topic); err != nil {
			return nil, err
		}
		topic = strings.TrimSpace(topic)
		if topic == "" {
			continue
		}
		out = append(out, topic)
	}
	return out, rows.Err()
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
		if err := rows.Scan(&it.ID, &it.SourceID, &it.URL, &it.Title, &it.ThumbnailURL, &it.ContentText,
			&it.Status, &it.IsRead, &it.IsFavorite, &it.FeedbackRating, &it.SummaryScore, &it.SummaryTopics, &it.PublishedAt, &it.FetchedAt, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (r *ItemRepo) GetDetail(ctx context.Context, id, userID string) (*model.ItemDetail, error) {
	var d model.ItemDetail
	err := r.db.QueryRow(ctx, `
		SELECT i.id, i.source_id, i.url, i.title, i.thumbnail_url, i.content_text, i.status,
		       EXISTS (
		           SELECT 1 FROM item_reads ir
		           WHERE ir.item_id = i.id AND ir.user_id = $2
		       ) AS is_read, i.processing_error,
		       i.published_at, i.fetched_at, i.created_at, i.updated_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		WHERE i.id = $1 AND s.user_id = $2`, id, userID,
	).Scan(&d.ID, &d.SourceID, &d.URL, &d.Title, &d.ThumbnailURL, &d.ContentText,
		&d.Status, &d.IsRead, &d.ProcessingError, &d.PublishedAt, &d.FetchedAt, &d.CreatedAt, &d.UpdatedAt)
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
	if d.Summary != nil {
		var llm model.ItemSummaryLLM
		err = r.db.QueryRow(ctx, `
			SELECT provider, model, pricing_source, created_at
			FROM llm_usage_logs
			WHERE item_id = $1
			  AND purpose = 'summary'
			ORDER BY created_at DESC
			LIMIT 1`, id,
		).Scan(&llm.Provider, &llm.Model, &llm.PricingSource, &llm.CreatedAt)
		if err == nil {
			d.SummaryLLM = &llm
		}
	}

	// feedback (optional)
	fb, err := r.GetFeedback(ctx, userID, id)
	if err == nil {
		d.Feedback = fb
	}

	return &d, nil
}

func (r *ItemRepo) GetFeedback(ctx context.Context, userID, itemID string) (*model.ItemFeedback, error) {
	var fb model.ItemFeedback
	err := r.db.QueryRow(ctx, `
		SELECT user_id, item_id, rating, is_favorite, updated_at
		FROM item_feedbacks
		WHERE user_id = $1 AND item_id = $2`,
		userID, itemID,
	).Scan(&fb.UserID, &fb.ItemID, &fb.Rating, &fb.IsFavorite, &fb.UpdatedAt)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &fb, nil
}

func (r *ItemRepo) UpsertFeedback(ctx context.Context, userID, itemID string, rating int, isFavorite bool) (*model.ItemFeedback, error) {
	if err := r.ensureOwned(ctx, userID, itemID); err != nil {
		return nil, err
	}
	var fb model.ItemFeedback
	err := r.db.QueryRow(ctx, `
		INSERT INTO item_feedbacks (user_id, item_id, rating, is_favorite)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, item_id) DO UPDATE SET
		  rating = EXCLUDED.rating,
		  is_favorite = EXCLUDED.is_favorite,
		  updated_at = NOW()
		RETURNING user_id, item_id, rating, is_favorite, updated_at`,
		userID, itemID, rating, isFavorite,
	).Scan(&fb.UserID, &fb.ItemID, &fb.Rating, &fb.IsFavorite, &fb.UpdatedAt)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &fb, nil
}

func (r *ItemRepo) ListRelated(ctx context.Context, id, userID string, limit int) ([]model.RelatedItem, error) {
	if limit <= 0 {
		limit = 6
	}
	if limit > 20 {
		limit = 20
	}
	const minSimilarity = 0.35
	fetchLimit := limit * 5
	if fetchLimit < 30 {
		fetchLimit = 30
	}
	if fetchLimit > 120 {
		fetchLimit = 120
	}

	rows, err := r.db.Query(ctx, `
		WITH target AS (
			SELECT ie.embedding AS emb, ie.dimensions AS dims, ti.source_id AS target_source_id
			FROM item_embeddings ie
			JOIN items ti ON ti.id = ie.item_id
			JOIN sources ts ON ts.id = ti.source_id
			WHERE ie.item_id = $1
			  AND ts.user_id = $2
		), scored AS (
			SELECT i.id, i.source_id, i.url, i.title,
			       sm.summary, COALESCE(sm.topics, '{}'::text[]) AS topics, sm.score,
			       COALESCE(
			         (
			           SELECT SUM(tv * cv)
			           FROM unnest(t.emb) WITH ORDINALITY AS tval(tv, idx)
			           JOIN unnest(ie.embedding) WITH ORDINALITY AS cval(cv, idx) USING (idx)
			         ),
			         0
			       )::double precision AS similarity,
			       (i.source_id = t.target_source_id) AS is_same_source,
			       i.published_at, i.created_at
			FROM target t
			JOIN item_embeddings ie ON ie.item_id <> $1 AND ie.dimensions = t.dims
			JOIN items i ON i.id = ie.item_id
			JOIN sources s ON s.id = i.source_id
			LEFT JOIN item_summaries sm ON sm.item_id = i.id
			WHERE s.user_id = $2
			  AND i.status = 'summarized'
		)
		SELECT id, source_id, url, title,
		       summary, topics, score, similarity, published_at, created_at
		FROM scored
		WHERE similarity >= $4
		ORDER BY is_same_source ASC, similarity DESC, COALESCE(published_at, created_at) DESC
		LIMIT $3`, id, userID, fetchLimit, minSimilarity)
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
		SELECT i.id, i.source_id, i.url, i.title, i.thumbnail_url, i.content_text, i.status,
		       FALSE AS is_read,
		       FALSE AS is_favorite,
		       0 AS feedback_rating,
		       i.published_at, i.fetched_at, i.created_at, i.updated_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		WHERE i.id = $1 AND s.user_id = $2`, id, userID,
	).Scan(&it.ID, &it.SourceID, &it.URL, &it.Title, &it.ThumbnailURL, &it.ContentText,
		&it.Status, &it.IsRead, &it.IsFavorite, &it.FeedbackRating, &it.PublishedAt, &it.FetchedAt, &it.CreatedAt, &it.UpdatedAt)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &it, nil
}

func (r *ItemRepo) ListFailedForRetry(ctx context.Context, userID string, sourceID *string) ([]model.Item, error) {
	query := `
		SELECT i.id, i.source_id, i.url, i.title, i.thumbnail_url, i.content_text, i.status,
		       FALSE AS is_read,
		       FALSE AS is_favorite,
		       0 AS feedback_rating,
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
		if err := rows.Scan(&it.ID, &it.SourceID, &it.URL, &it.Title, &it.ThumbnailURL, &it.ContentText,
			&it.Status, &it.IsRead, &it.IsFavorite, &it.FeedbackRating, &it.PublishedAt, &it.FetchedAt, &it.CreatedAt, &it.UpdatedAt); err != nil {
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

func (r *ItemRepo) Delete(ctx context.Context, itemID, userID string) error {
	if err := r.ensureOwned(ctx, userID, itemID); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx, `DELETE FROM items WHERE id = $1`, itemID)
	return err
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

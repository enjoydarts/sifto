package repository

import (
	"context"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
	"github.com/jackc/pgx/v5"
)

type topicPulseRow struct {
	TopicKey string
	Day      *string
	Count    *int
	DayMax   *float64
	Total    int
	Delta    int
	TopMax   *float64
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
			  AND i.deleted_at IS NULL
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

func (r *ItemRepo) TopicPulse(ctx context.Context, userID string, days, limit int) ([]model.TopicPulseItem, error) {
	out, err := r.topicPulseFromDailyAggregate(ctx, userID, days, limit)
	if err != nil {
		return nil, err
	}
	if len(out) > 0 {
		return out, nil
	}
	return r.topicPulseFromLiveData(ctx, userID, days, limit)
}

func (r *ItemRepo) topicPulseFromDailyAggregate(ctx context.Context, userID string, days, limit int) ([]model.TopicPulseItem, error) {
	if days <= 0 {
		days = 7
	}
	if days > 30 {
		days = 30
	}
	if limit <= 0 {
		limit = 12
	}
	if limit > 50 {
		limit = 50
	}

	rows, err := r.db.Query(ctx, `
		WITH totals AS (
			SELECT topic_key,
			       SUM(count)::int AS total,
			       COALESCE(SUM(count) FILTER (WHERE day_jst = (NOW() AT TIME ZONE 'Asia/Tokyo')::date), 0)::int AS today_count,
			       COALESCE(SUM(count) FILTER (WHERE day_jst = (NOW() AT TIME ZONE 'Asia/Tokyo')::date - 1), 0)::int AS prev_count,
			       MAX(max_score)::double precision AS max_score
			FROM topic_pulse_daily
			WHERE user_id = $1
			  AND day_jst >= ((NOW() AT TIME ZONE 'Asia/Tokyo')::date - ($2::int - 1))
			GROUP BY topic_key
			ORDER BY SUM(count) DESC, MAX(max_score) DESC NULLS LAST, topic_key ASC
			LIMIT $3
		)
		SELECT t.topic_key,
		       d.day_jst::text,
		       d.count,
		       d.max_score,
		       t.total,
		       (t.today_count - t.prev_count)::int AS delta,
		       t.max_score
		FROM totals t
		LEFT JOIN topic_pulse_daily d
		  ON d.user_id = $1
		 AND d.topic_key = t.topic_key
		 AND d.day_jst >= ((NOW() AT TIME ZONE 'Asia/Tokyo')::date - ($2::int - 1))
		ORDER BY t.total DESC, t.topic_key ASC, d.day_jst ASC`,
		userID, days, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTopicPulseRows(rows, days, limit)
}

func (r *ItemRepo) topicPulseFromLiveData(ctx context.Context, userID string, days, limit int) ([]model.TopicPulseItem, error) {
	if days <= 0 {
		days = 7
	}
	if days > 30 {
		days = 30
	}
	if limit <= 0 {
		limit = 12
	}
	if limit > 50 {
		limit = 50
	}

	rows, err := r.db.Query(ctx, `
		WITH base AS (
			SELECT COALESCE(NULLIF(BTRIM(t.topic), ''), '__untagged__') AS topic_key,
			       COALESCE(sm.score, 0)::double precision AS score,
			       (COALESCE(i.published_at, i.created_at) AT TIME ZONE 'Asia/Tokyo')::date AS day_jst
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
			  AND i.deleted_at IS NULL
			  AND i.status = 'summarized'
			  AND COALESCE(i.published_at, i.created_at) >= NOW() - make_interval(days => $2::int)
		),
		daily AS (
			SELECT topic_key, day_jst, COUNT(*)::int AS cnt, MAX(score)::double precision AS max_score
			FROM base
			GROUP BY topic_key, day_jst
		),
		totals AS (
			SELECT topic_key,
			       SUM(cnt)::int AS total,
			       COALESCE(SUM(cnt) FILTER (WHERE day_jst = (NOW() AT TIME ZONE 'Asia/Tokyo')::date), 0)::int AS today_count,
			       COALESCE(SUM(cnt) FILTER (WHERE day_jst = (NOW() AT TIME ZONE 'Asia/Tokyo')::date - 1), 0)::int AS prev_count,
			       MAX(max_score)::double precision AS max_score
			FROM daily
			GROUP BY topic_key
			ORDER BY SUM(cnt) DESC, MAX(max_score) DESC NULLS LAST, topic_key ASC
			LIMIT $3
		)
		SELECT t.topic_key,
		       d.day_jst::text,
		       d.cnt,
		       d.max_score,
		       t.total,
		       (t.today_count - t.prev_count)::int AS delta,
		       t.max_score
		FROM totals t
		LEFT JOIN daily d ON d.topic_key = t.topic_key
		ORDER BY t.total DESC, t.topic_key ASC, d.day_jst ASC`,
		userID, days, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTopicPulseRows(rows, days, limit)
}

func scanTopicPulseRows(rows pgx.Rows, days, limit int) ([]model.TopicPulseItem, error) {
	bucketByTopic := map[string]*model.TopicPulseItem{}
	pointMapByTopic := map[string]map[string]model.TopicPulsePoint{}
	order := make([]string, 0, limit)
	for rows.Next() {
		var row topicPulseRow
		if err := rows.Scan(&row.TopicKey, &row.Day, &row.Count, &row.DayMax, &row.Total, &row.Delta, &row.TopMax); err != nil {
			return nil, err
		}
		item, ok := bucketByTopic[row.TopicKey]
		if !ok {
			item = &model.TopicPulseItem{
				Topic:    row.TopicKey,
				Total:    row.Total,
				Delta:    row.Delta,
				MaxScore: row.TopMax,
			}
			bucketByTopic[row.TopicKey] = item
			pointMapByTopic[row.TopicKey] = map[string]model.TopicPulsePoint{}
			order = append(order, row.TopicKey)
		}
		if row.Day != nil && row.Count != nil {
			pointMapByTopic[row.TopicKey][*row.Day] = model.TopicPulsePoint{
				Date:     *row.Day,
				Count:    *row.Count,
				MaxScore: row.DayMax,
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	today := timeutil.StartOfDayJST(timeutil.NowJST())
	start := today.AddDate(0, 0, -(days - 1))
	dates := make([]string, 0, days)
	for d := start; !d.After(today); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d.Format("2006-01-02"))
	}

	out := make([]model.TopicPulseItem, 0, len(order))
	for _, topic := range order {
		item := bucketByTopic[topic]
		if item == nil {
			continue
		}
		points := make([]model.TopicPulsePoint, 0, len(dates))
		for _, date := range dates {
			p, ok := pointMapByTopic[topic][date]
			if !ok {
				p = model.TopicPulsePoint{Date: date, Count: 0}
			}
			points = append(points, p)
		}
		item.Points = points
		out = append(out, *item)
	}
	return out, nil
}

func (r *ItemRepo) RebuildTopicPulseDaily(ctx context.Context, days int) error {
	if days <= 0 {
		days = 35
	}
	cutoff := timeutil.StartOfDayJST(timeutil.NowJST()).AddDate(0, 0, -(days - 1))
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM topic_pulse_daily WHERE day_jst >= $1::date`, cutoff.Format("2006-01-02")); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO topic_pulse_daily (user_id, day_jst, topic_key, count, max_score, updated_at)
		SELECT s.user_id,
		       (COALESCE(i.published_at, i.created_at) AT TIME ZONE 'Asia/Tokyo')::date AS day_jst,
		       COALESCE(NULLIF(BTRIM(t.topic), ''), '__untagged__') AS topic_key,
		       COUNT(*)::int AS count,
		       MAX(COALESCE(sm.score, 0)::double precision) AS max_score,
		       NOW()
		FROM items i
		JOIN sources s ON s.id = i.source_id
		JOIN item_summaries sm ON sm.item_id = i.id
		CROSS JOIN LATERAL unnest(
			CASE
				WHEN COALESCE(array_length(sm.topics, 1), 0) = 0 THEN ARRAY['__untagged__']::text[]
				ELSE sm.topics
			END
		) AS t(topic)
		WHERE i.status = 'summarized'
		  AND COALESCE(i.published_at, i.created_at) >= $1
		GROUP BY s.user_id, day_jst, topic_key`, cutoff); err != nil {
		return err
	}

	return tx.Commit(ctx)
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
			  AND i.deleted_at IS NULL
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
	candidateLimit := fetchLimit * 8
	if candidateLimit < 240 {
		candidateLimit = 240
	}
	if candidateLimit > relatedCandidateLimitMax {
		candidateLimit = relatedCandidateLimitMax
	}

	rows, err := r.db.Query(ctx, `
		WITH target AS (
			SELECT ie.embedding AS emb, ie.dimensions AS dims, ti.source_id AS target_source_id
			FROM item_embeddings ie
			JOIN items ti ON ti.id = ie.item_id
			JOIN sources ts ON ts.id = ti.source_id
			WHERE ie.item_id = $1
			  AND ts.user_id = $2
		), candidate_items AS (
			SELECT i.id, i.source_id, COALESCE(i.published_at, i.created_at) AS effective_published_at
			FROM items i
			JOIN sources s ON s.id = i.source_id
			LEFT JOIN item_summaries sm ON sm.item_id = i.id
			WHERE s.user_id = $2
			  AND i.deleted_at IS NULL
			  AND i.status = 'summarized'
			  AND i.id <> $1
			ORDER BY COALESCE(i.published_at, i.created_at) DESC, sm.score DESC NULLS LAST
			LIMIT $5
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
			       i.published_at, i.created_at,
			       ci.effective_published_at
			FROM target t
			JOIN candidate_items ci ON true
			JOIN item_embeddings ie ON ie.item_id = ci.id AND ie.dimensions = t.dims
			JOIN items i ON i.id = ie.item_id
			LEFT JOIN item_summaries sm ON sm.item_id = i.id
		)
		SELECT id, source_id, url, title,
		       summary, topics, score, similarity, published_at, created_at
		FROM scored
		WHERE similarity >= $4
		ORDER BY is_same_source ASC, similarity DESC, effective_published_at DESC
		LIMIT $3`, id, userID, fetchLimit, minSimilarity, candidateLimit)
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

func (r *ItemRepo) AskCandidatesByEmbedding(
	ctx context.Context,
	userID string,
	queryEmbedding []float64,
	days int,
	unreadOnly bool,
	sourceIDs []string,
	limit int,
) ([]model.AskCandidate, error) {
	if len(queryEmbedding) == 0 {
		return nil, nil
	}
	if days <= 0 {
		days = 30
	}
	if days > 365 {
		days = 365
	}
	if limit <= 0 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}
	fetchLimit := limit * 4
	if fetchLimit < 24 {
		fetchLimit = 24
	}
	if fetchLimit > 80 {
		fetchLimit = 80
	}
	candidateLimit := fetchLimit * 12
	if candidateLimit < 300 {
		candidateLimit = 300
	}
	if candidateLimit > askCandidateLimitMax {
		candidateLimit = askCandidateLimitMax
	}

	query := `
		WITH q AS (
			SELECT $2::double precision[] AS emb, array_length($2::double precision[], 1) AS dims
		), candidate_items AS (
			SELECT i.id, COALESCE(i.published_at, i.created_at) AS effective_published_at, sm.score
			FROM items i
			JOIN sources s ON s.id = i.source_id
			JOIN item_summaries sm ON sm.item_id = i.id
			LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
			WHERE s.user_id = $1
			  AND i.deleted_at IS NULL
			  AND i.status = 'summarized'
			  AND COALESCE(i.published_at, i.created_at) >= NOW() - make_interval(days => $3::int)
	`
	args := []any{userID, queryEmbedding, days}
	if unreadOnly {
		query += ` AND ir.item_id IS NULL`
	}
	if len(sourceIDs) > 0 {
		args = append(args, sourceIDs)
		query += ` AND i.source_id = ANY($4::uuid[])`
	}
	query += `
			ORDER BY COALESCE(i.published_at, i.created_at) DESC, sm.score DESC NULLS LAST
			LIMIT $`
	args = append(args, candidateLimit)
	query += strconv.Itoa(len(args)) + `
		), scored AS (
			SELECT i.id, i.source_id, i.url, i.title, i.thumbnail_url, i.status,
			       (ir.item_id IS NOT NULL) AS is_read,
			       COALESCE(fb.is_favorite, false) AS is_favorite,
			       COALESCE(fb.rating, 0) AS feedback_rating,
			       sm.score, COALESCE(sm.topics, '{}'::text[]) AS topics, sm.translated_title,
			       i.published_at, i.fetched_at, i.created_at, i.updated_at,
			       sm.summary,
			       COALESCE(f.facts, '[]'::jsonb) AS facts,
			       ci.effective_published_at,
			       COALESCE(
			         (
			           SELECT SUM(qv * cv)
			           FROM unnest(q.emb) WITH ORDINALITY AS qvals(qv, idx)
			           JOIN unnest(ie.embedding) WITH ORDINALITY AS cvals(cv, idx) USING (idx)
			         ),
			         0
			       )::double precision AS similarity
			FROM q
			JOIN candidate_items ci ON true
			JOIN item_embeddings ie ON ie.item_id = ci.id AND ie.dimensions = q.dims
			JOIN items i ON i.id = ie.item_id
			JOIN item_summaries sm ON sm.item_id = i.id
			LEFT JOIN item_facts f ON f.item_id = i.id
			LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
			LEFT JOIN item_feedbacks fb ON fb.item_id = i.id AND fb.user_id = $1
	`
	query += `
		)
		SELECT id, source_id, url, title, thumbnail_url, status,
		       is_read, is_favorite, feedback_rating,
		       score, topics, translated_title,
		       published_at, fetched_at, created_at, updated_at,
		       summary, facts, similarity
		FROM scored
		WHERE similarity > 0.15
		ORDER BY similarity DESC, score DESC NULLS LAST, effective_published_at DESC
		LIMIT $`
	args = append(args, fetchLimit)
	query += strconv.Itoa(len(args))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.AskCandidate, 0, fetchLimit)
	for rows.Next() {
		var v model.AskCandidate
		if err := rows.Scan(
			&v.ID, &v.SourceID, &v.URL, &v.Title, &v.ThumbnailURL, &v.Status,
			&v.IsRead, &v.IsFavorite, &v.FeedbackRating,
			&v.SummaryScore, &v.SummaryTopics, &v.TranslatedTitle,
			&v.PublishedAt, &v.FetchedAt, &v.CreatedAt, &v.UpdatedAt,
			&v.Summary, jsonStringArrayScanner{dst: &v.Facts}, &v.Similarity,
		); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return selectAskCandidatesByMMR(out, limit), nil
}

func selectAskCandidatesByMMR(candidates []model.AskCandidate, limit int) []model.AskCandidate {
	if limit <= 0 || len(candidates) == 0 {
		return nil
	}
	if len(candidates) <= limit {
		return candidates
	}

	remaining := make([]model.AskCandidate, len(candidates))
	copy(remaining, candidates)
	selected := make([]model.AskCandidate, 0, limit)
	sourceCounts := map[string]int{}
	topicCounts := map[string]int{}

	bestIdx := 0
	bestScore := -1e9
	for i, item := range remaining {
		score := askCandidateBaseScore(item)
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	first := remaining[bestIdx]
	selected = append(selected, first)
	sourceCounts[first.SourceID]++
	topicCounts[firstTopicKey(first.SummaryTopics)]++
	remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)

	for len(selected) < limit && len(remaining) > 0 {
		bestIdx = 0
		bestScore = -1e9
		for i, item := range remaining {
			base := askCandidateBaseScore(item)
			sourcePenalty := math.Min(0.16, 0.06*float64(sourceCounts[item.SourceID]))
			topicPenalty := math.Min(0.20, 0.08*float64(topicCounts[firstTopicKey(item.SummaryTopics)]))
			dupPenalty := maxAskTopicOverlap(item, selected)
			score := base - sourcePenalty - topicPenalty - 0.18*dupPenalty
			if score > bestScore {
				bestScore = score
				bestIdx = i
			}
		}
		chosen := remaining[bestIdx]
		selected = append(selected, chosen)
		sourceCounts[chosen.SourceID]++
		topicCounts[firstTopicKey(chosen.SummaryTopics)]++
		remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
	}

	return selected
}

func askCandidateBaseScore(item model.AskCandidate) float64 {
	score := item.Similarity * 0.68
	if item.SummaryScore != nil {
		score += *item.SummaryScore * 0.22
	}
	score += askCandidateRecencyBoost(item) * 0.10
	return score
}

func askCandidateRecencyBoost(item model.AskCandidate) float64 {
	ref := item.CreatedAt
	if item.PublishedAt != nil {
		ref = *item.PublishedAt
	}
	if ref.IsZero() {
		return 0
	}
	hours := time.Since(ref).Hours()
	switch {
	case hours <= 24:
		return 1.0
	case hours <= 72:
		return 0.7
	case hours <= 7*24:
		return 0.4
	default:
		return 0.1
	}
}

func maxAskTopicOverlap(item model.AskCandidate, selected []model.AskCandidate) float64 {
	if len(selected) == 0 {
		return 0
	}
	current := make(map[string]struct{}, len(item.SummaryTopics))
	for _, topic := range item.SummaryTopics {
		t := strings.TrimSpace(strings.ToLower(topic))
		if t != "" {
			current[t] = struct{}{}
		}
	}
	if len(current) == 0 {
		return 0
	}
	maxOverlap := 0.0
	for _, selectedItem := range selected {
		count := 0
		for _, topic := range selectedItem.SummaryTopics {
			if _, ok := current[strings.TrimSpace(strings.ToLower(topic))]; ok {
				count++
			}
		}
		overlap := float64(count) / math.Max(1, float64(len(current)))
		if overlap > maxOverlap {
			maxOverlap = overlap
		}
	}
	return maxOverlap
}

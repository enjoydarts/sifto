package repository

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

const (
	readingPlanCandidateLimit24h = 800
	readingPlanCandidateLimit7d  = 1200
	relatedCandidateLimitMax     = 600
	askCandidateLimitMax         = 1200
)

type ReadingPlanParams struct {
	Window          string // 24h | today_jst | 7d
	Size            int
	DiversifyTopics bool
	ExcludeRead     bool
	ExcludeLater    bool
}

type briefingNavigatorCandidateWindow struct {
	minAge time.Duration
	maxAge time.Duration
}

var briefingNavigatorCandidateWindows = []briefingNavigatorCandidateWindow{
	{minAge: 0, maxAge: 1 * time.Hour},
	{minAge: 1 * time.Hour, maxAge: 12 * time.Hour},
	{minAge: 12 * time.Hour, maxAge: 24 * time.Hour},
}

const briefingEffectiveTimeSQL = "COALESCE(i.fetched_at, i.created_at, i.published_at, i.created_at)"

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
	candidateLimit := readingPlanCandidateLimit24h
	filterSQL := ``
	switch p.Window {
	case "today_jst":
		filterSQL = ` AND (COALESCE(i.published_at, i.created_at) AT TIME ZONE 'Asia/Tokyo')::date = (NOW() AT TIME ZONE 'Asia/Tokyo')::date`
	case "7d":
		candidateLimit = readingPlanCandidateLimit7d
		filterSQL = ` AND COALESCE(i.published_at, i.created_at) >= NOW() - INTERVAL '7 days'`
	default:
		p.Window = "24h"
		filterSQL = ` AND ` + briefingEffectiveTimeSQL + ` >= NOW() - INTERVAL '24 hours'`
	}
	if p.ExcludeRead {
		filterSQL += ` AND ir.item_id IS NULL`
	}
	if p.ExcludeLater {
		filterSQL += ` AND NOT EXISTS (
			SELECT 1 FROM item_laters il
			WHERE il.item_id = i.id AND il.user_id = $1
		)`
	}

	var poolCount int
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM items i
		JOIN sources s ON s.id = i.source_id
		LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
		WHERE s.user_id = $1
		  AND i.deleted_at IS NULL
		  AND i.status = 'summarized'`+filterSQL, userID).Scan(&poolCount); err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT i.id, i.source_id, s.title AS source_title, i.url, i.title, i.thumbnail_url, NULL::text AS content_text, i.status, i.processing_error,
		       fc.final_result AS facts_check_result,
		       sfc.final_result AS faithfulness_result,
		       (ir.item_id IS NOT NULL) AS is_read,
		       COALESCE(fb.is_favorite, false) AS is_favorite,
		       COALESCE(fb.rating, 0) AS feedback_rating,
		       sm.score, sm.score_breakdown, sm.personal_score, sm.personal_score_reason, COALESCE(sm.topics, '{}'::text[]), sm.translated_title,
		       i.published_at, i.fetched_at, i.created_at, i.updated_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
		LEFT JOIN item_feedbacks fb ON fb.item_id = i.id AND fb.user_id = $1
		LEFT JOIN item_summaries sm ON sm.item_id = i.id
		LEFT JOIN item_facts_checks fc ON fc.item_id = i.id
		LEFT JOIN summary_faithfulness_checks sfc ON sfc.item_id = i.id
		WHERE s.user_id = $1
		  AND i.deleted_at IS NULL
		  AND i.status = 'summarized'`+filterSQL+`
		ORDER BY sm.score DESC NULLS LAST, i.created_at DESC
		LIMIT $2`, userID, candidateLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	candidates, err := scanItemsWithBreakdown(rows)
	if err != nil {
		return nil, err
	}

	prefRepo := NewPreferenceProfileRepo(r.db)
	prefProfile, _ := prefRepo.GetProfile(ctx, userID)

	candidateIDs := make([]string, 0, len(candidates))
	for _, it := range candidates {
		candidateIDs = append(candidateIDs, it.ID)
	}
	candidateEmbByItemID, err := loadItemEmbeddingsByID(ctx, r.db, candidateIDs)
	if err != nil {
		return nil, err
	}

	for i := range candidates {
		input := PersonalScoreInput{
			SummaryScore:   candidates[i].SummaryScore,
			ScoreBreakdown: candidates[i].SummaryScoreBreakdown,
			Topics:         candidates[i].SummaryTopics,
			Embedding:      candidateEmbByItemID[candidates[i].ID],
			SourceID:       candidates[i].SourceID,
		}
		score, reason := CalcPersonalScore(input, prefProfile)
		candidates[i].PersonalScore = &score
		candidates[i].PersonalScoreReason = &reason
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		si, sj := 0.0, 0.0
		if candidates[i].PersonalScore != nil {
			si = *candidates[i].PersonalScore
		}
		if candidates[j].PersonalScore != nil {
			sj = *candidates[j].PersonalScore
		}
		if si != sj {
			return si > sj
		}
		return candidates[i].CreatedAt.After(candidates[j].CreatedAt)
	})

	selected := selectItemsByMMR(candidates, p.Size, p.DiversifyTopics, candidateEmbByItemID)
	for i := range selected {
		if selected[i].PersonalScoreReason != nil && *selected[i].PersonalScoreReason != "attention" {
			selected[i].RecommendationReason = selected[i].PersonalScoreReason
		} else {
			reason := itemRecommendationReason(selected[i], nil)
			selected[i].RecommendationReason = &reason
		}
	}
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

func (r *ItemRepo) ClusterItemsByEmbeddings(ctx context.Context, items []model.Item) ([]model.ReadingPlanCluster, error) {
	return r.readingPlanClustersByEmbeddings(ctx, items, nil)
}

func (r *ItemRepo) BriefingClusters24h(ctx context.Context, userID string, limit int) ([]model.ReadingPlanCluster, error) {
	if limit <= 0 {
		limit = 16
	}
	if limit > 40 {
		limit = 40
	}
	const candidateLimit = 800
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
		  AND ir.item_id IS NULL
		  AND NOT EXISTS (
			SELECT 1 FROM item_laters il
			WHERE il.user_id = $1
			  AND il.item_id = i.id
		  )
		ORDER BY sm.score DESC NULLS LAST, `+briefingEffectiveTimeSQL+` DESC
		LIMIT $2`,
		userID, candidateLimit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	candidates, err := scanItems(rows)
	if err != nil {
		return nil, err
	}
	clusters, err := r.briefingClustersByEmbeddings(ctx, candidates)
	if err != nil {
		return nil, err
	}
	if len(clusters) > limit {
		clusters = clusters[:limit]
	}
	return clusters, nil
}

func (r *ItemRepo) readingPlanTopics(ctx context.Context, userID string, p ReadingPlanParams) ([]model.ReadingPlanTopic, error) {
	filterSQL := ``
	switch p.Window {
	case "today_jst":
		filterSQL = ` AND (COALESCE(i.published_at, i.created_at) AT TIME ZONE 'Asia/Tokyo')::date = (NOW() AT TIME ZONE 'Asia/Tokyo')::date`
	case "7d":
		filterSQL = ` AND COALESCE(i.published_at, i.created_at) >= NOW() - INTERVAL '7 days'`
	default:
		filterSQL = ` AND ` + briefingEffectiveTimeSQL + ` >= NOW() - INTERVAL '24 hours'`
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
			  AND i.deleted_at IS NULL
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

func (r *ItemRepo) BriefingNavigatorCandidates24h(ctx context.Context, userID string, limit int) ([]model.BriefingNavigatorCandidate, error) {
	if limit <= 0 {
		limit = 12
	}
	if limit > 24 {
		limit = 24
	}
	out := make([]model.BriefingNavigatorCandidate, 0, limit)
	for _, window := range briefingNavigatorCandidateWindows {
		remaining := limit - len(out)
		if remaining <= 0 {
			break
		}
		batch, err := r.briefingNavigatorCandidatesByWindow(ctx, userID, remaining, window)
		if err != nil {
			return nil, err
		}
		out = mergeBriefingNavigatorCandidates(limit, out, batch)
	}
	return out, nil
}

func (r *ItemRepo) AINavigatorBriefCandidatesInWindow(ctx context.Context, userID string, start, end time.Time, limit int) ([]model.BriefingNavigatorCandidate, error) {
	if limit <= 0 {
		limit = 24
	}
	if limit > 24 {
		limit = 24
	}
	rows, err := r.db.Query(ctx, `
		SELECT i.id,
		       i.title,
		       sm.translated_title,
		       s.title AS source_title,
		       sm.summary,
		       COALESCE(sm.topics, '{}'::text[]) AS topics,
		       sm.score,
		       `+briefingEffectiveTimeSQL+` AS published_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		JOIN item_summaries sm ON sm.item_id = i.id
		LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
		WHERE s.user_id = $1
		  AND i.deleted_at IS NULL
		  AND i.status = 'summarized'
		  AND ir.item_id IS NULL
		  AND NOT EXISTS (
			SELECT 1 FROM item_laters il
			WHERE il.user_id = $1
			  AND il.item_id = i.id
		  )
		  AND NULLIF(BTRIM(sm.summary), '') IS NOT NULL
		  AND `+briefingEffectiveTimeSQL+` >= $2
		  AND `+briefingEffectiveTimeSQL+` < $3
		ORDER BY RANDOM()
		LIMIT $4
	`, userID, start, end, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.BriefingNavigatorCandidate, 0, limit)
	for rows.Next() {
		var row model.BriefingNavigatorCandidate
		if err := rows.Scan(
			&row.ItemID,
			&row.Title,
			&row.TranslatedTitle,
			&row.SourceTitle,
			&row.Summary,
			&row.Topics,
			&row.Score,
			&row.PublishedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *ItemRepo) briefingNavigatorCandidatesByWindow(
	ctx context.Context,
	userID string,
	limit int,
	window briefingNavigatorCandidateWindow,
) ([]model.BriefingNavigatorCandidate, error) {
	query, args := briefingNavigatorCandidatesWindowQuery(userID, limit, window)
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.BriefingNavigatorCandidate, 0, limit)
	for rows.Next() {
		var row model.BriefingNavigatorCandidate
		if err := rows.Scan(
			&row.ItemID,
			&row.Title,
			&row.TranslatedTitle,
			&row.SourceTitle,
			&row.Summary,
			&row.Topics,
			&row.Score,
			&row.PublishedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func briefingNavigatorCandidatesWindowQuery(
	userID string,
	limit int,
	window briefingNavigatorCandidateWindow,
) (string, []any) {
	query := `
		SELECT i.id,
		       i.title,
		       sm.translated_title,
		       s.title AS source_title,
		       sm.summary,
		       COALESCE(sm.topics, '{}'::text[]) AS topics,
		       sm.score,
		       ` + briefingEffectiveTimeSQL + ` AS published_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		JOIN item_summaries sm ON sm.item_id = i.id
		LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
		WHERE s.user_id = $1
		  AND i.deleted_at IS NULL
		  AND i.status = 'summarized'
		  AND ir.item_id IS NULL
		  AND NOT EXISTS (
			SELECT 1 FROM item_laters il
			WHERE il.user_id = $1
			  AND il.item_id = i.id
		  )
		  AND NULLIF(BTRIM(sm.summary), '') IS NOT NULL
		  AND ` + briefingEffectiveTimeSQL + ` >= NOW() - make_interval(secs => $2::int)`
	args := []any{userID, int(window.maxAge.Seconds())}
	if window.minAge > 0 {
		args = append(args, int(window.minAge.Seconds()))
		query += `
		  AND ` + briefingEffectiveTimeSQL + ` < NOW() - make_interval(secs => $3::int)`
	}
	args = append(args, limit)
	query += `
		ORDER BY RANDOM()
		LIMIT $` + itoa(len(args))
	return query, args
}

func mergeBriefingNavigatorCandidates(
	limit int,
	groups ...[]model.BriefingNavigatorCandidate,
) []model.BriefingNavigatorCandidate {
	if limit <= 0 {
		return nil
	}
	out := make([]model.BriefingNavigatorCandidate, 0, limit)
	seen := make(map[string]struct{}, limit)
	for _, group := range groups {
		for _, candidate := range group {
			itemID := strings.TrimSpace(candidate.ItemID)
			if itemID == "" {
				continue
			}
			if _, ok := seen[itemID]; ok {
				continue
			}
			seen[itemID] = struct{}{}
			out = append(out, candidate)
			if len(out) >= limit {
				return out
			}
		}
	}
	return out
}

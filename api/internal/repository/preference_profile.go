package repository

import (
	"context"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

// scoreKeys defines the canonical order of score breakdown dimensions.
var scoreKeys = []string{"importance", "novelty", "actionability", "reliability", "relevance"}

// breakdownToMap converts an ItemSummaryScoreBreakdown to a map.
func breakdownToMap(b model.ItemSummaryScoreBreakdown) map[string]float64 {
	m := make(map[string]float64, len(scoreKeys))
	if b.Importance != nil {
		m["importance"] = *b.Importance
	}
	if b.Novelty != nil {
		m["novelty"] = *b.Novelty
	}
	if b.Actionability != nil {
		m["actionability"] = *b.Actionability
	}
	if b.Reliability != nil {
		m["reliability"] = *b.Reliability
	}
	if b.Relevance != nil {
		m["relevance"] = *b.Relevance
	}
	return m
}

// avgBreakdown computes the per-dimension average of a slice of breakdowns.
func avgBreakdown(breakdowns []model.ItemSummaryScoreBreakdown) map[string]float64 {
	if len(breakdowns) == 0 {
		return make(map[string]float64)
	}
	sum := make(map[string]float64, len(scoreKeys))
	count := make(map[string]int, len(scoreKeys))
	for _, b := range breakdowns {
		m := breakdownToMap(b)
		for k, v := range m {
			sum[k] += v
			count[k]++
		}
	}
	avg := make(map[string]float64, len(scoreKeys))
	for _, k := range scoreKeys {
		if count[k] > 0 {
			avg[k] = sum[k] / float64(count[k])
		}
	}
	return avg
}

// computeLearnedWeights derives per-dimension weights from positive/negative examples.
func computeLearnedWeights(positive, negative []model.ItemSummaryScoreBreakdown, feedbackCount int) map[string]float64 {
	posAvg := avgBreakdown(positive)
	negAvg := avgBreakdown(negative)

	// diff = positive - negative; clamp to >= 0
	diff := make(map[string]float64, len(scoreKeys))
	allNonPositive := true
	for _, k := range scoreKeys {
		d := posAvg[k] - negAvg[k]
		if d > 0 {
			diff[k] = d
			allNonPositive = false
		}
	}

	// Fallback to defaults if all diffs <= 0
	if allNonPositive {
		out := make(map[string]float64, len(model.DefaultScoreWeights))
		for k, v := range model.DefaultScoreWeights {
			out[k] = v
		}
		return out
	}

	// Normalize diff so they sum to 1.0
	var total float64
	for _, v := range diff {
		total += v
	}
	learned := make(map[string]float64, len(scoreKeys))
	for _, k := range scoreKeys {
		if total > 0 {
			learned[k] = diff[k] / total
		}
	}

	// Blend with defaults using confidence
	confidence := math.Min(float64(feedbackCount)/30.0, 1.0)
	blended := make(map[string]float64, len(scoreKeys))
	for _, k := range scoreKeys {
		blended[k] = confidence*learned[k] + (1.0-confidence)*model.DefaultScoreWeights[k]
	}

	return blended
}

// topicAction represents a user action associated with topics.
type topicAction struct {
	Topics  []string
	Signal  float64
	DaysAgo float64
}

// computeTopicInterests derives topic interest scores from user actions with time decay.
func computeTopicInterests(actions []topicAction) map[string]float64 {
	raw := make(map[string]float64)
	for _, a := range actions {
		decay := math.Pow(0.5, a.DaysAgo/30.0)
		for _, t := range a.Topics {
			norm := strings.ToLower(strings.TrimSpace(t))
			if norm == "" {
				continue
			}
			raw[norm] += a.Signal * decay
		}
	}

	// Normalize by max absolute value so dislikes remain negative.
	var maxAbs float64
	for _, v := range raw {
		abs := math.Abs(v)
		if abs > maxAbs {
			maxAbs = abs
		}
	}
	if maxAbs <= 0 {
		return raw
	}
	out := make(map[string]float64, len(raw))
	for k, v := range raw {
		out[k] = v / maxAbs
	}
	return out
}

// PreferenceProfileRepo provides CRUD and computation for user preference profiles.
type PreferenceProfileRepo struct {
	db *pgxpool.Pool
}

// NewPreferenceProfileRepo creates a new PreferenceProfileRepo.
func NewPreferenceProfileRepo(db *pgxpool.Pool) *PreferenceProfileRepo {
	return &PreferenceProfileRepo{db: db}
}

// BuildProfileForUser builds a UserPreferenceProfile by aggregating feedback,
// reading history, embeddings, and source affinity data.
func (r *PreferenceProfileRepo) BuildProfileForUser(ctx context.Context, userID string) (*model.UserPreferenceProfile, error) {
	positive, negative, feedbackCount, err := r.loadFeedbackBreakdowns(ctx, userID)
	if err != nil {
		return nil, err
	}

	learnedWeights := computeLearnedWeights(positive, negative, feedbackCount)

	actions, err := r.loadTopicActions(ctx, userID)
	if err != nil {
		return nil, err
	}
	topicInterests := computeTopicInterests(actions)

	prefEmb, _, err := r.loadPrefEmbedding(ctx, userID)
	if err != nil {
		return nil, err
	}

	sourceAffinities, err := r.loadSourceAffinities(ctx, userID)
	if err != nil {
		return nil, err
	}

	readCount, err := r.countReads(ctx, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &model.UserPreferenceProfile{
		UserID:           userID,
		LearnedWeights:   learnedWeights,
		TopicInterests:   topicInterests,
		PrefEmbedding:    prefEmb,
		SourceAffinities: sourceAffinities,
		FeedbackCount:    feedbackCount,
		ReadCount:        readCount,
		ComputedAt:       &now,
	}, nil
}

// UpsertProfile inserts or updates a user preference profile.
func (r *PreferenceProfileRepo) UpsertProfile(ctx context.Context, profile *model.UserPreferenceProfile) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_preference_profiles (
			user_id, learned_weights, topic_interests, pref_embedding,
			source_affinities, feedback_count, read_count, computed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id) DO UPDATE SET
			learned_weights = EXCLUDED.learned_weights,
			topic_interests = EXCLUDED.topic_interests,
			pref_embedding = EXCLUDED.pref_embedding,
			source_affinities = EXCLUDED.source_affinities,
			feedback_count = EXCLUDED.feedback_count,
			read_count = EXCLUDED.read_count,
			computed_at = EXCLUDED.computed_at`,
		profile.UserID,
		profile.LearnedWeights,
		profile.TopicInterests,
		profile.PrefEmbedding,
		profile.SourceAffinities,
		profile.FeedbackCount,
		profile.ReadCount,
		profile.ComputedAt,
	)
	return err
}

// GetProfile retrieves a user preference profile by user ID.
func (r *PreferenceProfileRepo) GetProfile(ctx context.Context, userID string) (*model.UserPreferenceProfile, error) {
	var p model.UserPreferenceProfile
	err := r.db.QueryRow(ctx, `
		SELECT user_id, learned_weights, topic_interests, pref_embedding,
		       source_affinities, feedback_count, read_count, computed_at
		FROM user_preference_profiles
		WHERE user_id = $1`, userID,
	).Scan(
		&p.UserID,
		&p.LearnedWeights,
		&p.TopicInterests,
		&p.PrefEmbedding,
		&p.SourceAffinities,
		&p.FeedbackCount,
		&p.ReadCount,
		&p.ComputedAt,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &p, nil
}

func (r *PreferenceProfileRepo) DeleteProfile(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM user_preference_profiles WHERE user_id = $1`, userID)
	return err
}

func (r *PreferenceProfileRepo) GetProfileView(ctx context.Context, userID string) (*model.PreferenceProfileResponse, error) {
	profile, err := r.GetProfile(ctx, userID)
	if err != nil {
		if err != ErrNotFound {
			return nil, err
		}
		profile = &model.UserPreferenceProfile{
			UserID:           userID,
			LearnedWeights:   cloneDefaultWeights(),
			TopicInterests:   map[string]float64{},
			SourceAffinities: map[string]float64{},
		}
		readCount, readErr := r.countReads(ctx, userID)
		if readErr != nil {
			return nil, readErr
		}
		profile.ReadCount = readCount
	}
	if len(profile.LearnedWeights) == 0 {
		profile.LearnedWeights = cloneDefaultWeights()
	}
	if profile.TopicInterests == nil {
		profile.TopicInterests = map[string]float64{}
	}
	if profile.SourceAffinities == nil {
		profile.SourceAffinities = map[string]float64{}
	}

	actions, err := r.loadTopicActions(ctx, userID)
	if err != nil {
		log.Printf("GetProfileView loadTopicActions failed user_id=%s err=%v", userID, err)
		return nil, err
	}
	topTopics := topTopicsWithSignalCounts(profile.TopicInterests, actions, 10)

	topSources, err := r.loadTopSourceAffinitiesWithTitles(ctx, userID, profile.SourceAffinities, 5)
	if err != nil {
		log.Printf("GetProfileView loadTopSourceAffinitiesWithTitles failed user_id=%s err=%v", userID, err)
		return nil, err
	}

	readingPattern, err := r.loadReadingPattern(ctx, userID)
	if err != nil {
		log.Printf("GetProfileView loadReadingPattern failed user_id=%s err=%v", userID, err)
		return nil, err
	}

	learnedWeights := make(map[string]model.PreferenceProfileWeight, len(scoreKeys))
	for _, key := range scoreKeys {
		current := profile.LearnedWeights[key]
		def := model.DefaultScoreWeights[key]
		learnedWeights[key] = model.PreferenceProfileWeight{
			Value:   current,
			Default: def,
			Delta:   current - def,
		}
	}

	return &model.PreferenceProfileResponse{
		Status:         preferenceStatusFromFeedbackCount(profile.FeedbackCount),
		Confidence:     preferenceConfidenceFromFeedbackCount(profile.FeedbackCount),
		FeedbackCount:  profile.FeedbackCount,
		ReadCount:      profile.ReadCount,
		ComputedAt:     profile.ComputedAt,
		LearnedWeights: learnedWeights,
		TopTopics:      topTopics,
		TopSources:     topSources,
		ReadingPattern: readingPattern,
	}, nil
}

func (r *PreferenceProfileRepo) GetProfileSummary(ctx context.Context, userID string) (*model.PreferenceProfileSummaryResponse, error) {
	view, err := r.GetProfileView(ctx, userID)
	if err != nil {
		return nil, err
	}
	topLimit := len(view.TopTopics)
	if topLimit > 3 {
		topLimit = 3
	}
	topTopics := make([]string, 0, topLimit)
	for _, topic := range view.TopTopics {
		topTopics = append(topTopics, topic.Topic)
		if len(topTopics) == 3 {
			break
		}
	}

	strongestWeight := ""
	bestWeight := -1.0
	for key, weight := range view.LearnedWeights {
		if weight.Value > bestWeight {
			bestWeight = weight.Value
			strongestWeight = key
		}
	}

	return &model.PreferenceProfileSummaryResponse{
		Status:          view.Status,
		Confidence:      view.Confidence,
		FeedbackCount:   view.FeedbackCount,
		TopTopics:       topTopics,
		StrongestWeight: strongestWeight,
		ComputedAt:      view.ComputedAt,
	}, nil
}

// --- internal helpers ---

func preferenceStatusFromFeedbackCount(feedbackCount int) string {
	switch {
	case feedbackCount >= 30:
		return "active"
	case feedbackCount >= 10:
		return "learning"
	default:
		return "cold_start"
	}
}

func preferenceConfidenceFromFeedbackCount(feedbackCount int) float64 {
	return math.Min(float64(feedbackCount)/30.0, 1.0)
}

func cloneDefaultWeights() map[string]float64 {
	out := make(map[string]float64, len(model.DefaultScoreWeights))
	for key, value := range model.DefaultScoreWeights {
		out[key] = value
	}
	return out
}

func topTopicsWithSignalCounts(interests map[string]float64, actions []topicAction, limit int) []model.PreferenceProfileTopic {
	if limit <= 0 || len(interests) == 0 {
		return nil
	}
	counts := make(map[string]int, len(interests))
	for _, action := range actions {
		seen := map[string]struct{}{}
		for _, topic := range action.Topics {
			norm := strings.ToLower(strings.TrimSpace(topic))
			if norm == "" {
				continue
			}
			if _, ok := seen[norm]; ok {
				continue
			}
			seen[norm] = struct{}{}
			counts[norm]++
		}
	}

	out := make([]model.PreferenceProfileTopic, 0, len(interests))
	for topic, score := range interests {
		if score <= 0 {
			continue
		}
		out = append(out, model.PreferenceProfileTopic{
			Topic:       topic,
			Score:       score,
			SignalCount: counts[topic],
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			if out[i].SignalCount == out[j].SignalCount {
				return out[i].Topic < out[j].Topic
			}
			return out[i].SignalCount > out[j].SignalCount
		}
		return out[i].Score > out[j].Score
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (r *PreferenceProfileRepo) loadTopSourceAffinitiesWithTitles(ctx context.Context, userID string, affinities map[string]float64, limit int) ([]model.PreferenceProfileSource, error) {
	if limit <= 0 || len(affinities) == 0 {
		return nil, nil
	}
	type scoredSource struct {
		sourceID string
		score    float64
	}
	sources := make([]scoredSource, 0, len(affinities))
	for sourceID, score := range affinities {
		if score <= 0 {
			continue
		}
		sources = append(sources, scoredSource{sourceID: sourceID, score: score})
	}
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].score == sources[j].score {
			return sources[i].sourceID < sources[j].sourceID
		}
		return sources[i].score > sources[j].score
	})
	if len(sources) > limit {
		sources = sources[:limit]
	}

	sourceIDs := make([]string, 0, len(sources))
	for _, source := range sources {
		sourceIDs = append(sourceIDs, source.sourceID)
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, title
		FROM sources
		WHERE user_id = $1
		  AND id::text = ANY($2)`, userID, sourceIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	titles := make(map[string]string, len(sourceIDs))
	for rows.Next() {
		var id string
		var title *string
		if err := rows.Scan(&id, &title); err != nil {
			return nil, err
		}
		if title != nil {
			titles[id] = *title
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]model.PreferenceProfileSource, 0, len(sources))
	for _, source := range sources {
		out = append(out, model.PreferenceProfileSource{
			SourceID:    source.sourceID,
			SourceTitle: titles[source.sourceID],
			Score:       source.score,
		})
	}
	return out, nil
}

func (r *PreferenceProfileRepo) loadReadingPattern(ctx context.Context, userID string) (model.PreferenceProfileReadingPattern, error) {
	var out model.PreferenceProfileReadingPattern
	err := r.db.QueryRow(ctx, `
		WITH scoped AS (
			SELECT
				COALESCE(sm.score, 0)::double precision AS score,
				(ir.item_id IS NOT NULL) AS is_read,
				COALESCE(fb.is_favorite, false) AS is_favorite,
				EXISTS (
					SELECT 1
					FROM item_notes n
					WHERE n.user_id = $2
					  AND n.item_id = i.id
				) AS has_note
			FROM items i
			JOIN sources s ON s.id = i.source_id
			LEFT JOIN item_summaries sm ON sm.item_id = i.id
			LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1::uuid
			LEFT JOIN item_feedbacks fb ON fb.item_id = i.id AND fb.user_id = $1::uuid
			WHERE s.user_id = $1::uuid
			  AND i.deleted_at IS NULL
			  AND i.status = 'summarized'
			  AND COALESCE(i.published_at, i.created_at) >= NOW() - INTERVAL '90 days'
		)
		SELECT
			COALESCE(AVG(score) FILTER (WHERE is_read), 0),
			COALESCE(AVG(score) FILTER (WHERE NOT is_read), 0),
			COALESCE(AVG(CASE WHEN is_favorite THEN 1.0 ELSE 0.0 END), 0),
			COALESCE(AVG(CASE WHEN has_note THEN 1.0 ELSE 0.0 END), 0)
		FROM scoped`, userID, userID,
	).Scan(&out.AvgScoreRead, &out.AvgScoreSkipped, &out.FavoriteRate, &out.NoteRate)
	return out, err
}

func (r *PreferenceProfileRepo) loadFeedbackBreakdowns(ctx context.Context, userID string) (positive, negative []model.ItemSummaryScoreBreakdown, feedbackCount int, err error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			(
				CASE
					WHEN fb.rating > 0 THEN 1.0
					WHEN fb.rating < 0 THEN -1.2
					ELSE 0.0
				END
				+ CASE WHEN fb.is_favorite = true THEN 1.4 ELSE 0.0 END
				+ CASE WHEN il.item_id IS NOT NULL THEN 0.45 ELSE 0.0 END
				+ CASE WHEN ir.item_id IS NOT NULL THEN 0.2 ELSE 0.0 END
				+ CASE WHEN i.deleted_at IS NOT NULL THEN -0.7 ELSE 0.0 END
			)::double precision AS signal,
			isb.score_breakdown
		FROM items i
		JOIN sources s ON s.id = i.source_id
		JOIN item_summaries isb ON isb.item_id = i.id
		LEFT JOIN item_feedbacks fb ON fb.item_id = i.id AND fb.user_id = $1
		LEFT JOIN item_laters il ON il.item_id = i.id AND il.user_id = $1
		LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
		WHERE s.user_id = $1
		  AND (
		    fb.rating <> 0
		    OR fb.is_favorite = true
		    OR il.item_id IS NOT NULL
		    OR ir.item_id IS NOT NULL
		    OR i.deleted_at IS NOT NULL
		  )
		  AND isb.score_breakdown IS NOT NULL`, userID)
	if err != nil {
		return nil, nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var signal float64
		var bd *model.ItemSummaryScoreBreakdown
		if err := rows.Scan(&signal, scoreBreakdownScanner{dst: &bd}); err != nil {
			return nil, nil, 0, err
		}
		if bd == nil || signal == 0 {
			continue
		}
		feedbackCount++
		if signal > 0 {
			positive = append(positive, *bd)
		}
		if signal < 0 {
			negative = append(negative, *bd)
		}
	}
	return positive, negative, feedbackCount, rows.Err()
}

func (r *PreferenceProfileRepo) loadTopicActions(ctx context.Context, userID string) ([]topicAction, error) {
	rows, err := r.db.Query(ctx, `
		WITH actions AS (
			SELECT
				isb.topics,
				(
					CASE
						WHEN i.deleted_at IS NOT NULL THEN -0.7
						WHEN fb.is_favorite = true THEN 2.0
						WHEN fb.rating > 0 THEN 1.0
						WHEN fb.rating < 0 THEN -1.2
						WHEN il.item_id IS NOT NULL THEN 0.45
						WHEN ir.item_id IS NOT NULL THEN 0.2
						ELSE 0.0
					END
				)::double precision AS signal,
				COALESCE(fb.updated_at, il.created_at, ir.read_at, i.deleted_at, i.created_at) AS acted_at
			FROM items i
			JOIN sources s ON s.id = i.source_id
			JOIN item_summaries isb ON isb.item_id = i.id
			LEFT JOIN item_feedbacks fb ON fb.item_id = i.id AND fb.user_id = $1::uuid
			LEFT JOIN item_laters il ON il.item_id = i.id AND il.user_id = $1::uuid
			LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1::uuid
			WHERE s.user_id = $1::uuid
			  AND isb.topics IS NOT NULL
			  AND array_length(isb.topics, 1) > 0
			  AND COALESCE(fb.updated_at, il.created_at, ir.read_at, i.deleted_at, i.created_at) >= NOW() - INTERVAL '90 days'
			  AND (fb.item_id IS NOT NULL OR il.item_id IS NOT NULL OR ir.item_id IS NOT NULL OR i.deleted_at IS NOT NULL)
			UNION ALL
			SELECT isb.topics, 2.0::double precision AS signal, n.updated_at AS acted_at
			FROM item_notes n
			JOIN items i ON i.id = n.item_id
			JOIN sources s ON s.id = i.source_id AND s.user_id::text = n.user_id
			JOIN item_summaries isb ON isb.item_id = i.id
			WHERE n.user_id = $2
			  AND i.deleted_at IS NULL
			  AND n.updated_at >= NOW() - INTERVAL '90 days'
			UNION ALL
			SELECT isb.topics, 1.8::double precision AS signal, ai.created_at AS acted_at
			FROM ask_insight_items aii
			JOIN ask_insights ai ON ai.id = aii.insight_id AND ai.user_id = $2
			JOIN items i ON i.id = aii.item_id
			JOIN item_summaries isb ON isb.item_id = i.id
			WHERE ai.created_at >= NOW() - INTERVAL '90 days'
			  AND i.deleted_at IS NULL
			UNION ALL
			SELECT isb.topics, 1.5::double precision AS signal, rq.completed_at AS acted_at
			FROM review_queue rq
			JOIN items i ON i.id = rq.item_id
			JOIN item_summaries isb ON isb.item_id = i.id
			WHERE rq.user_id = $2
			  AND i.deleted_at IS NULL
			  AND rq.status = 'done'
			  AND rq.completed_at IS NOT NULL
			  AND rq.completed_at >= NOW() - INTERVAL '90 days'
		)
		SELECT topics, signal, EXTRACT(EPOCH FROM (NOW() - acted_at)) / 86400.0 AS days_ago
		FROM actions`, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []topicAction
	for rows.Next() {
		var a topicAction
		if err := rows.Scan(&a.Topics, &a.Signal, &a.DaysAgo); err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}
	return actions, rows.Err()
}

func (r *PreferenceProfileRepo) loadPrefEmbedding(ctx context.Context, userID string) ([]float64, int, error) {
	fp, err := loadFeedbackPreferenceProfile(ctx, r.db, userID)
	if err != nil {
		return nil, 0, err
	}
	if fp == nil {
		return nil, 0, nil
	}
	return fp.prefEmbedding, fp.embeddingDims, nil
}

func (r *PreferenceProfileRepo) loadSourceAffinities(ctx context.Context, userID string) (map[string]float64, error) {
	rows, err := r.db.Query(ctx, `
		WITH base AS (
			SELECT
				s.id AS source_id,
				COUNT(i.id)::int AS item_count_30d,
				COUNT(ir.item_id)::int AS read_count_30d,
				COUNT(il.item_id)::int AS later_count_30d,
				COUNT(*) FILTER (WHERE i.deleted_at IS NOT NULL)::int AS deleted_count_30d,
				COALESCE(SUM(
					CASE
						WHEN i.deleted_at IS NOT NULL THEN -0.7
						WHEN fb.is_favorite = true THEN 2.0
						WHEN fb.rating > 0 THEN 1.0
						WHEN fb.rating < 0 THEN -1.2
						ELSE 0.0
					END
				), 0)::double precision AS feedback_signal
			FROM sources s
			LEFT JOIN items i
			       ON i.source_id = s.id
			      AND COALESCE(i.published_at, i.created_at) >= NOW() - INTERVAL '30 days'
			LEFT JOIN item_reads ir
			       ON ir.item_id = i.id
			      AND ir.user_id = $1
			LEFT JOIN item_laters il
			       ON il.item_id = i.id
			      AND il.user_id = $1
			LEFT JOIN item_feedbacks fb
			       ON fb.item_id = i.id
			      AND fb.user_id = $1
			WHERE s.user_id = $1
			  AND s.enabled = true
			GROUP BY s.id
		)
		SELECT
			source_id,
			(
				feedback_signal * 0.7
				+ CASE WHEN item_count_30d > 0 THEN (later_count_30d::double precision / item_count_30d::double precision) * 0.9 ELSE 0 END
				+ CASE WHEN item_count_30d > 0 THEN (read_count_30d::double precision / item_count_30d::double precision) * 1.8 ELSE 0 END
				- CASE WHEN item_count_30d > 0 THEN (deleted_count_30d::double precision / item_count_30d::double precision) * 1.3 ELSE 0 END
			)::double precision AS affinity_score
		FROM base
		WHERE item_count_30d > 0`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	raw := make(map[string]float64)
	var maxVal float64
	for rows.Next() {
		var sourceID string
		var score float64
		if err := rows.Scan(&sourceID, &score); err != nil {
			return nil, err
		}
		raw[sourceID] = score
		if abs := math.Abs(score); abs > maxVal {
			maxVal = abs
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Normalize to -1.0..1.0
	if maxVal <= 0 {
		return raw, nil
	}
	out := make(map[string]float64, len(raw))
	for k, v := range raw {
		out[k] = v / maxVal
	}
	return out, nil
}

func (r *PreferenceProfileRepo) countReads(ctx context.Context, userID string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM item_reads ir
		JOIN items i ON i.id = ir.item_id
		WHERE ir.user_id = $1
		  AND i.deleted_at IS NULL`,
		userID,
	).Scan(&n)
	return n, err
}

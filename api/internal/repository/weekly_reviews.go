package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WeeklyReviewRepo struct{ db *pgxpool.Pool }

func NewWeeklyReviewRepo(db *pgxpool.Pool) *WeeklyReviewRepo { return &WeeklyReviewRepo{db: db} }

func weeklyReviewReadCountQuery() string {
	return `
		SELECT COUNT(*)::int
		FROM item_reads ir
		JOIN items i ON i.id = ir.item_id
		JOIN sources s ON s.id = i.source_id
		WHERE ir.user_id = $1
		  AND s.user_id = $1
		  AND i.deleted_at IS NULL
		  AND (ir.read_at AT TIME ZONE 'Asia/Tokyo')::date BETWEEN $2::date AND $3::date`
}

func weeklyReviewTopicsQuery() string {
	return `
		SELECT topic, COUNT(*)::int AS count
		FROM (
			SELECT unnest(COALESCE(sm.topics, '{}'::text[])) AS topic
			FROM item_reads ir
			JOIN items i ON i.id = ir.item_id
			JOIN sources s ON s.id = i.source_id
			LEFT JOIN item_summaries sm ON sm.item_id = i.id
			WHERE ir.user_id = $1
			  AND s.user_id = $1
			  AND i.deleted_at IS NULL
			  AND (ir.read_at AT TIME ZONE 'Asia/Tokyo')::date BETWEEN $2::date AND $3::date
		) t
		WHERE topic <> ''
		GROUP BY topic
		ORDER BY count DESC, topic ASC
		LIMIT 5`
}

func (r *WeeklyReviewRepo) GetLatest(ctx context.Context, userID string) (*model.WeeklyReviewSnapshot, error) {
	var (
		id        string
		weekStart time.Time
		weekEnd   time.Time
		payload   []byte
		createdAt time.Time
	)
	err := r.db.QueryRow(ctx, `
		SELECT id, week_start, week_end, snapshot, created_at
		FROM weekly_review_snapshots
		WHERE user_id = $1
		ORDER BY week_start DESC
		LIMIT 1`, userID,
	).Scan(&id, &weekStart, &weekEnd, &payload, &createdAt)
	if err != nil {
		return nil, mapDBError(err)
	}
	var snapshot model.WeeklyReviewSnapshot
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return nil, err
	}
	snapshot.ID = id
	snapshot.UserID = userID
	snapshot.WeekStart = weekStart.Format("2006-01-02")
	snapshot.WeekEnd = weekEnd.Format("2006-01-02")
	snapshot.CreatedAt = createdAt
	return &snapshot, nil
}

func (r *WeeklyReviewRepo) Upsert(ctx context.Context, userID string, snapshot model.WeeklyReviewSnapshot) (*model.WeeklyReviewSnapshot, error) {
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	var (
		id        string
		createdAt time.Time
	)
	err = r.db.QueryRow(ctx, `
		INSERT INTO weekly_review_snapshots (user_id, week_start, week_end, snapshot)
		VALUES ($1, $2::date, $3::date, $4)
		ON CONFLICT (user_id, week_start) DO UPDATE
		SET week_end = EXCLUDED.week_end,
		    snapshot = EXCLUDED.snapshot
		RETURNING id, created_at`,
		userID, snapshot.WeekStart, snapshot.WeekEnd, payload,
	).Scan(&id, &createdAt)
	if err != nil {
		return nil, err
	}
	snapshot.ID = id
	snapshot.UserID = userID
	snapshot.CreatedAt = createdAt
	return &snapshot, nil
}

func (r *WeeklyReviewRepo) CollectInputs(ctx context.Context, userID, weekStart, weekEnd string) (readCount, noteCount, insightCount, favoriteCount int, topics []model.WeeklyReviewTopic, missed []model.Item, err error) {
	if err = r.db.QueryRow(ctx, weeklyReviewReadCountQuery(), userID, weekStart, weekEnd).Scan(&readCount); err != nil {
		return
	}
	if err = r.db.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM item_notes
		WHERE user_id = $1
		  AND (updated_at AT TIME ZONE 'Asia/Tokyo')::date BETWEEN $2::date AND $3::date`,
		userID, weekStart, weekEnd,
	).Scan(&noteCount); err != nil {
		return
	}
	if err = r.db.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM ask_insights
		WHERE user_id = $1
		  AND (created_at AT TIME ZONE 'Asia/Tokyo')::date BETWEEN $2::date AND $3::date`,
		userID, weekStart, weekEnd,
	).Scan(&insightCount); err != nil {
		return
	}
	if err = r.db.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM item_feedbacks
		WHERE user_id = $1
		  AND is_favorite = true
		  AND (updated_at AT TIME ZONE 'Asia/Tokyo')::date BETWEEN $2::date AND $3::date`,
		userID, weekStart, weekEnd,
	).Scan(&favoriteCount); err != nil {
		return
	}

	topicRows, queryErr := r.db.Query(ctx, weeklyReviewTopicsQuery(), userID, weekStart, weekEnd)
	if queryErr != nil {
		err = queryErr
		return
	}
	defer topicRows.Close()
	for topicRows.Next() {
		var topic model.WeeklyReviewTopic
		if scanErr := topicRows.Scan(&topic.Topic, &topic.Count); scanErr != nil {
			err = scanErr
			return
		}
		topics = append(topics, topic)
	}
	if err = topicRows.Err(); err != nil {
		return
	}

	missedRows, queryErr := r.db.Query(ctx, `
		SELECT i.id, i.source_id, i.url, i.title, i.thumbnail_url, NULL::text AS content_text, i.status,
		       fc.final_result AS facts_check_result, sfc.final_result AS faithfulness_result,
		       false AS is_read, COALESCE(fb.is_favorite, false) AS is_favorite, COALESCE(fb.rating, 0) AS feedback_rating,
		       sm.score, COALESCE(sm.topics, '{}'::text[]), sm.translated_title,
		       i.published_at, i.fetched_at, i.created_at, i.updated_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		LEFT JOIN item_summaries sm ON sm.item_id = i.id
		LEFT JOIN item_feedbacks fb ON fb.item_id = i.id AND fb.user_id = $1
		LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
		LEFT JOIN item_facts_checks fc ON fc.item_id = i.id
		LEFT JOIN summary_faithfulness_checks sfc ON sfc.item_id = i.id
		WHERE s.user_id = $1
		  AND i.deleted_at IS NULL
		  AND ir.item_id IS NULL
		  AND COALESCE(sm.score, 0) >= 0.75
		  AND (i.created_at AT TIME ZONE 'Asia/Tokyo')::date BETWEEN $2::date AND $3::date
		ORDER BY sm.score DESC NULLS LAST, i.created_at DESC
		LIMIT 5`, userID, weekStart, weekEnd)
	if queryErr != nil {
		err = queryErr
		return
	}
	defer missedRows.Close()
	for missedRows.Next() {
		var item model.Item
		if scanErr := missedRows.Scan(
			&item.ID, &item.SourceID, &item.URL, &item.Title, &item.ThumbnailURL, &item.ContentText, &item.Status,
			&item.FactsCheckResult, &item.FaithfulnessResult, &item.IsRead, &item.IsFavorite, &item.FeedbackRating,
			&item.SummaryScore, &item.SummaryTopics, &item.TranslatedTitle, &item.PublishedAt, &item.FetchedAt, &item.CreatedAt, &item.UpdatedAt,
		); scanErr != nil {
			err = scanErr
			return
		}
		missed = append(missed, item)
	}
	err = missedRows.Err()
	return
}

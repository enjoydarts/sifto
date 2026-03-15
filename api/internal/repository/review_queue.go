package repository

import (
	"context"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ReviewQueueRepo struct{ db *pgxpool.Pool }

func NewReviewQueueRepo(db *pgxpool.Pool) *ReviewQueueRepo { return &ReviewQueueRepo{db: db} }

func (r *ReviewQueueRepo) EnqueueDefault(ctx context.Context, userID, itemID, sourceSignal string, base time.Time) error {
	for _, schedule := range buildReviewSchedules(base) {
		_, err := r.db.Exec(ctx, `
			INSERT INTO review_queue (id, user_id, item_id, source_signal, review_stage, review_due_at)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (user_id, item_id, review_stage, status) DO NOTHING`,
			uuid.NewString(), userID, itemID, sourceSignal, schedule.Stage, schedule.DueAt,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

type reviewSchedule struct {
	Stage string
	DueAt time.Time
}

func buildReviewSchedules(base time.Time) []reviewSchedule {
	return []reviewSchedule{
		{Stage: "d1", DueAt: base.Add(24 * time.Hour)},
		{Stage: "d7", DueAt: base.Add(7 * 24 * time.Hour)},
		{Stage: "d30", DueAt: base.Add(30 * 24 * time.Hour)},
	}
}

func (r *ReviewQueueRepo) ListDue(ctx context.Context, userID string, now time.Time, limit int) ([]model.ReviewQueueItem, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := r.db.Query(ctx, `
		SELECT rq.id, rq.user_id, rq.item_id, rq.source_signal, rq.review_stage, rq.status,
		       rq.review_due_at, rq.last_surfaced_at, rq.completed_at, rq.snooze_count, rq.created_at, rq.updated_at,
		       i.id, i.source_id, i.url, i.title, i.thumbnail_url, NULL::text AS content_text, i.status,
		       fc.final_result AS facts_check_result,
		       sfc.final_result AS faithfulness_result,
		       (ir.item_id IS NOT NULL) AS is_read,
		       COALESCE(fb.is_favorite, false) AS is_favorite,
		       COALESCE(fb.rating, 0) AS feedback_rating,
		       sm.score, COALESCE(sm.topics, '{}'::text[]), sm.translated_title,
		       i.published_at, i.fetched_at, i.created_at, i.updated_at,
		       (n.item_id IS NOT NULL) AS has_note
		FROM review_queue rq
		JOIN items i ON i.id = rq.item_id
		JOIN sources s ON s.id = i.source_id AND s.user_id = rq.user_id
		LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = rq.user_id
		LEFT JOIN item_feedbacks fb ON fb.item_id = i.id AND fb.user_id = rq.user_id
		LEFT JOIN item_summaries sm ON sm.item_id = i.id
		LEFT JOIN item_facts_checks fc ON fc.item_id = i.id
		LEFT JOIN summary_faithfulness_checks sfc ON sfc.item_id = i.id
		LEFT JOIN item_notes n ON n.item_id = i.id AND n.user_id = rq.user_id
		WHERE rq.user_id = $1
		  AND rq.status = 'pending'
		  AND rq.review_due_at <= $2
		ORDER BY
		  CASE WHEN COALESCE(fb.is_favorite, false) THEN 0 ELSE 1 END,
		  CASE WHEN n.item_id IS NOT NULL THEN 0 ELSE 1 END,
		  COALESCE(rq.last_surfaced_at, to_timestamp(0)) ASC,
		  rq.review_due_at ASC
		LIMIT $3`,
		userID, now, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []model.ReviewQueueItem{}
	for rows.Next() {
		var v model.ReviewQueueItem
		var hasNote bool
		if err := rows.Scan(
			&v.ID, &v.UserID, &v.ItemID, &v.SourceSignal, &v.ReviewStage, &v.Status,
			&v.ReviewDueAt, &v.LastSurfacedAt, &v.CompletedAt, &v.SnoozeCount, &v.CreatedAt, &v.UpdatedAt,
			&v.Item.ID, &v.Item.SourceID, &v.Item.URL, &v.Item.Title, &v.Item.ThumbnailURL, &v.Item.ContentText, &v.Item.Status,
			&v.Item.FactsCheckResult, &v.Item.FaithfulnessResult, &v.Item.IsRead, &v.Item.IsFavorite, &v.Item.FeedbackRating,
			&v.Item.SummaryScore, &v.Item.SummaryTopics, &v.Item.TranslatedTitle, &v.Item.PublishedAt, &v.Item.FetchedAt, &v.Item.CreatedAt, &v.Item.UpdatedAt,
			&hasNote,
		); err != nil {
			return nil, err
		}
		if v.Item.IsFavorite {
			v.ReasonLabels = append(v.ReasonLabels, "favorite")
		}
		if hasNote {
			v.ReasonLabels = append(v.ReasonLabels, "note")
		}
		if stage := strings.TrimSpace(v.ReviewStage); stage != "" {
			v.ReasonLabels = append(v.ReasonLabels, stage)
		}
		items = append(items, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return items, nil
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	_, _ = r.db.Exec(ctx, `
		UPDATE review_queue
		SET last_surfaced_at = NOW(), updated_at = NOW()
		WHERE user_id = $1
		  AND id = ANY($2::uuid[])`,
		userID, ids,
	)
	return items, nil
}

func (r *ReviewQueueRepo) CountDue(ctx context.Context, userID string, now time.Time) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM review_queue
		WHERE user_id = $1
		  AND status = 'pending'
		  AND review_due_at <= $2`,
		userID, now,
	).Scan(&count)
	return count, err
}

func (r *ReviewQueueRepo) MarkDone(ctx context.Context, userID, queueID string) error {
	cmd, err := r.db.Exec(ctx, `
		UPDATE review_queue
		SET status = 'done', completed_at = NOW(), updated_at = NOW()
		WHERE user_id = $1 AND id = $2`, userID, queueID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ReviewQueueRepo) Snooze(ctx context.Context, userID, queueID string, duration time.Duration) error {
	cmd, err := r.db.Exec(ctx, `
		UPDATE review_queue
		SET review_due_at = review_due_at + $3::interval,
		    snooze_count = snooze_count + 1,
		    updated_at = NOW()
		WHERE user_id = $1 AND id = $2 AND status = 'pending'`,
		userID, queueID, duration.String(),
	)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

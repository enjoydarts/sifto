package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

type ItemBulkJobAction string

const (
	ItemBulkJobActionRetry          ItemBulkJobAction = "retry"
	ItemBulkJobActionRetryFromFacts ItemBulkJobAction = "retry_from_facts"
)

type ItemBulkJobFilters struct {
	Status   string `json:"status"`
	SourceID string `json:"source_id,omitempty"`
	Topic    string `json:"topic,omitempty"`
	Genre    string `json:"genre,omitempty"`
	Query    string `json:"q,omitempty"`
}

type ItemBulkJob struct {
	ID             string
	UserID         string
	Action         ItemBulkJobAction
	Filters        ItemBulkJobFilters
	Status         string
	MatchedCount   int
	ProcessedCount int
	QueuedCount    int
	SkippedCount   int
}

type ItemBulkJobCandidate struct {
	ID       string
	SourceID string
	URL      string
}

func normalizeItemBulkJobAction(action ItemBulkJobAction) (ItemBulkJobAction, error) {
	switch action {
	case ItemBulkJobActionRetry, ItemBulkJobActionRetryFromFacts:
		return action, nil
	default:
		return "", ErrInvalidState
	}
}

func validateItemBulkJobFilters(filters ItemBulkJobFilters) error {
	if strings.TrimSpace(filters.Status) != "pending" {
		return ErrInvalidState
	}
	if strings.TrimSpace(filters.Query) != "" {
		return ErrInvalidState
	}
	return nil
}

func (r *ItemRepo) CreateItemBulkJob(ctx context.Context, userID string, action ItemBulkJobAction, filters ItemBulkJobFilters) (ItemBulkJob, error) {
	action, err := normalizeItemBulkJobAction(action)
	if err != nil {
		return ItemBulkJob{}, err
	}
	if err := validateItemBulkJobFilters(filters); err != nil {
		return ItemBulkJob{}, err
	}
	filterJSON, err := json.Marshal(filters)
	if err != nil {
		return ItemBulkJob{}, err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return ItemBulkJob{}, err
	}
	defer tx.Rollback(ctx)

	var job ItemBulkJob
	err = tx.QueryRow(ctx, `
		INSERT INTO item_bulk_jobs (user_id, action, filters)
		VALUES ($1, $2, $3::jsonb)
		RETURNING id, user_id, action, status
	`, userID, string(action), string(filterJSON)).Scan(&job.ID, &job.UserID, &job.Action, &job.Status)
	if err != nil {
		return ItemBulkJob{}, err
	}
	job.Filters = filters

	status := filters.Status
	params := ItemListParams{Status: &status}
	if strings.TrimSpace(filters.SourceID) != "" {
		sourceID := strings.TrimSpace(filters.SourceID)
		params.SourceID = &sourceID
	}
	if strings.TrimSpace(filters.Topic) != "" {
		topic := strings.TrimSpace(filters.Topic)
		params.Topic = &topic
	}
	if strings.TrimSpace(filters.Genre) != "" {
		genre := strings.TrimSpace(filters.Genre)
		params.Genre = &genre
	}
	joins, where, args := buildItemListFilterParts(userID, params, true)
	args = append(args, job.ID)
	jobIDArg := `$` + itoa(len(args))

	err = tx.QueryRow(ctx, `
		WITH target_items AS (
			SELECT i.id
			FROM items i
			`+joins+`
			WHERE `+where+`
			ORDER BY i.created_at DESC, i.id DESC
		), inserted_rows AS (
			INSERT INTO item_bulk_job_items (job_id, item_id)
			SELECT `+jobIDArg+`, t.id
			FROM target_items t
			ON CONFLICT (job_id, item_id) DO NOTHING
			RETURNING 1
		), counted AS (
			SELECT COUNT(*)::int AS matched_count FROM inserted_rows
		)
		UPDATE item_bulk_jobs j
		SET matched_count = counted.matched_count,
		    queued_count = counted.matched_count,
		    updated_at = NOW()
		FROM counted
		WHERE j.id = `+jobIDArg+`
		RETURNING j.matched_count, j.queued_count
	`, args...).Scan(&job.MatchedCount, &job.QueuedCount)
	if err != nil {
		return ItemBulkJob{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return ItemBulkJob{}, err
	}
	return job, nil
}

func (r *ItemRepo) GetItemBulkJob(ctx context.Context, jobID string) (ItemBulkJob, error) {
	var job ItemBulkJob
	var filterJSON []byte
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, action, filters, status, matched_count, processed_count, queued_count, skipped_count
		FROM item_bulk_jobs
		WHERE id = $1
	`, jobID).Scan(&job.ID, &job.UserID, &job.Action, &filterJSON, &job.Status, &job.MatchedCount, &job.ProcessedCount, &job.QueuedCount, &job.SkippedCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return ItemBulkJob{}, ErrNotFound
	}
	if err != nil {
		return ItemBulkJob{}, err
	}
	if len(filterJSON) > 0 {
		_ = json.Unmarshal(filterJSON, &job.Filters)
	}
	return job, nil
}

func (r *ItemRepo) ClaimItemBulkJobItems(ctx context.Context, jobID string, limit int) ([]ItemBulkJobCandidate, error) {
	if limit <= 0 {
		limit = 50
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE item_bulk_jobs
		SET status = 'running', updated_at = NOW()
		WHERE id = $1 AND status IN ('queued', 'running')
	`, jobID); err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `
		WITH claimed AS (
			SELECT j.item_id
			FROM item_bulk_job_items j
			WHERE j.job_id = $1 AND j.status IN ('queued', 'processing')
			ORDER BY j.position
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		), updated AS (
			UPDATE item_bulk_job_items j
			SET status = 'processing'
			FROM claimed
			WHERE j.job_id = $1 AND j.item_id = claimed.item_id
			RETURNING j.item_id
		)
		SELECT i.id, i.source_id, i.url
		FROM updated u
		JOIN items i ON i.id = u.item_id
		ORDER BY u.item_id
	`, jobID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	candidates := make([]ItemBulkJobCandidate, 0)
	for rows.Next() {
		var item ItemBulkJobCandidate
		if err := rows.Scan(&item.ID, &item.SourceID, &item.URL); err != nil {
			return nil, err
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return candidates, nil
}

func (r *ItemRepo) MarkItemBulkJobItemProcessed(ctx context.Context, jobID string, itemID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE item_bulk_job_items
		SET status = 'processed', error_message = NULL, processed_at = NOW()
		WHERE job_id = $1 AND item_id = $2
	`, jobID, itemID)
	return err
}

func (r *ItemRepo) MarkItemBulkJobItemSkipped(ctx context.Context, jobID string, itemID string, message string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE item_bulk_job_items
		SET status = 'skipped', error_message = $3, processed_at = NOW()
		WHERE job_id = $1 AND item_id = $2
	`, jobID, itemID, strings.TrimSpace(message))
	return err
}

func (r *ItemRepo) RefreshItemBulkJobCounts(ctx context.Context, jobID string) (int, error) {
	var remaining int
	err := r.db.QueryRow(ctx, `
		WITH counts AS (
			SELECT
				COUNT(*) FILTER (WHERE status = 'processed')::int AS processed_count,
				COUNT(*) FILTER (WHERE status IN ('queued', 'processing'))::int AS queued_count,
				COUNT(*) FILTER (WHERE status = 'skipped')::int AS skipped_count
			FROM item_bulk_job_items
			WHERE job_id = $1
		)
		UPDATE item_bulk_jobs j
		SET processed_count = counts.processed_count,
		    queued_count = counts.queued_count,
		    skipped_count = counts.skipped_count,
		    updated_at = NOW()
		FROM counts
		WHERE j.id = $1
		RETURNING j.queued_count
	`, jobID).Scan(&remaining)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return remaining, err
}

func (r *ItemRepo) CompleteItemBulkJob(ctx context.Context, jobID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE item_bulk_jobs
		SET status = 'completed', completed_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, jobID)
	return err
}

func (r *ItemRepo) FailItemBulkJob(ctx context.Context, jobID string, message string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE item_bulk_jobs
		SET status = 'failed', error_message = $2, updated_at = NOW()
		WHERE id = $1
	`, jobID, strings.TrimSpace(message))
	return err
}

package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SearchBackfillRun struct {
	ID               string     `json:"id"`
	RequestedOffset  int        `json:"requested_offset"`
	BatchSize        int        `json:"batch_size"`
	AllItems         bool       `json:"all_items"`
	TotalItems       int        `json:"total_items"`
	QueuedBatches    int        `json:"queued_batches"`
	CompletedBatches int        `json:"completed_batches"`
	FailedBatches    int        `json:"failed_batches"`
	ProcessedItems   int        `json:"processed_items"`
	Status           string     `json:"status"`
	LastError        *string    `json:"last_error,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
}

type SearchBackfillRunRepo struct{ db *pgxpool.Pool }

func NewSearchBackfillRunRepo(db *pgxpool.Pool) *SearchBackfillRunRepo {
	return &SearchBackfillRunRepo{db: db}
}

func (r *SearchBackfillRunRepo) Create(ctx context.Context, offset, batchSize int, allItems bool, totalItems, queuedBatches int) (*SearchBackfillRun, error) {
	status := "queued"
	var startedAt *time.Time
	var finishedAt *time.Time
	if queuedBatches == 0 {
		now := time.Now()
		status = "completed"
		startedAt = &now
		finishedAt = &now
	}

	row := r.db.QueryRow(ctx, `
		INSERT INTO search_backfill_runs (
			requested_offset, batch_size, all_items, total_items, queued_batches,
			status, started_at, finished_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, requested_offset, batch_size, all_items, total_items, queued_batches,
		          completed_batches, failed_batches, processed_items, status, last_error,
		          created_at, updated_at, started_at, finished_at
	`, offset, batchSize, allItems, totalItems, queuedBatches, status, startedAt, finishedAt)

	return scanSearchBackfillRun(row)
}

func (r *SearchBackfillRunRepo) GetByID(ctx context.Context, runID string) (*SearchBackfillRun, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, requested_offset, batch_size, all_items, total_items, queued_batches,
		       completed_batches, failed_batches, processed_items, status, last_error,
		       created_at, updated_at, started_at, finished_at
		FROM search_backfill_runs
		WHERE id = $1
	`, runID)
	return scanSearchBackfillRun(row)
}

func (r *SearchBackfillRunRepo) ListRecent(ctx context.Context, limit int) ([]SearchBackfillRun, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, requested_offset, batch_size, all_items, total_items, queued_batches,
		       completed_batches, failed_batches, processed_items, status, last_error,
		       created_at, updated_at, started_at, finished_at
		FROM search_backfill_runs
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := make([]SearchBackfillRun, 0, limit)
	for rows.Next() {
		run, err := scanSearchBackfillRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, *run)
	}
	return runs, rows.Err()
}

func (r *SearchBackfillRunRepo) DeleteFinished(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM search_backfill_runs
		WHERE status IN ('completed', 'failed', 'partial_failed')
	`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r *SearchBackfillRunRepo) MarkRunning(ctx context.Context, runID string) (*SearchBackfillRun, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE search_backfill_runs
		SET status = CASE WHEN queued_batches > 0 THEN 'running' ELSE status END,
		    started_at = COALESCE(started_at, NOW()),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, requested_offset, batch_size, all_items, total_items, queued_batches,
		          completed_batches, failed_batches, processed_items, status, last_error,
		          created_at, updated_at, started_at, finished_at
	`, runID)
	return scanSearchBackfillRun(row)
}

func (r *SearchBackfillRunRepo) MarkFanoutFailed(ctx context.Context, runID, errText string) (*SearchBackfillRun, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE search_backfill_runs
		SET status = CASE WHEN completed_batches > 0 THEN 'partial_failed' ELSE 'failed' END,
		    last_error = $2,
		    finished_at = NOW(),
		    updated_at = NOW(),
		    started_at = COALESCE(started_at, NOW())
		WHERE id = $1
		RETURNING id, requested_offset, batch_size, all_items, total_items, queued_batches,
		          completed_batches, failed_batches, processed_items, status, last_error,
		          created_at, updated_at, started_at, finished_at
	`, runID, errText)
	return scanSearchBackfillRun(row)
}

func (r *SearchBackfillRunRepo) MarkBatchSucceeded(ctx context.Context, runID string, processedItems int) (*SearchBackfillRun, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE search_backfill_runs
		SET completed_batches = completed_batches + 1,
		    processed_items = processed_items + GREATEST($2, 0),
		    status = CASE
		      WHEN completed_batches + failed_batches + 1 >= queued_batches AND failed_batches > 0 THEN 'partial_failed'
		      WHEN completed_batches + failed_batches + 1 >= queued_batches THEN 'completed'
		      ELSE 'running'
		    END,
		    finished_at = CASE
		      WHEN completed_batches + failed_batches + 1 >= queued_batches THEN NOW()
		      ELSE finished_at
		    END,
		    updated_at = NOW(),
		    started_at = COALESCE(started_at, NOW())
		WHERE id = $1
		RETURNING id, requested_offset, batch_size, all_items, total_items, queued_batches,
		          completed_batches, failed_batches, processed_items, status, last_error,
		          created_at, updated_at, started_at, finished_at
	`, runID, processedItems)
	return scanSearchBackfillRun(row)
}

func (r *SearchBackfillRunRepo) MarkBatchFailed(ctx context.Context, runID, errText string) (*SearchBackfillRun, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE search_backfill_runs
		SET failed_batches = failed_batches + 1,
		    last_error = $2,
		    status = CASE
		      WHEN completed_batches + failed_batches + 1 >= queued_batches AND completed_batches > 0 THEN 'partial_failed'
		      WHEN completed_batches + failed_batches + 1 >= queued_batches THEN 'failed'
		      ELSE 'running'
		    END,
		    finished_at = CASE
		      WHEN completed_batches + failed_batches + 1 >= queued_batches THEN NOW()
		      ELSE finished_at
		    END,
		    updated_at = NOW(),
		    started_at = COALESCE(started_at, NOW())
		WHERE id = $1
		RETURNING id, requested_offset, batch_size, all_items, total_items, queued_batches,
		          completed_batches, failed_batches, processed_items, status, last_error,
		          created_at, updated_at, started_at, finished_at
	`, runID, errText)
	return scanSearchBackfillRun(row)
}

type searchBackfillRunScanner interface {
	Scan(dest ...any) error
}

func scanSearchBackfillRun(row searchBackfillRunScanner) (*SearchBackfillRun, error) {
	var run SearchBackfillRun
	if err := row.Scan(
		&run.ID,
		&run.RequestedOffset,
		&run.BatchSize,
		&run.AllItems,
		&run.TotalItems,
		&run.QueuedBatches,
		&run.CompletedBatches,
		&run.FailedBatches,
		&run.ProcessedItems,
		&run.Status,
		&run.LastError,
		&run.CreatedAt,
		&run.UpdatedAt,
		&run.StartedAt,
		&run.FinishedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		}
		return nil, err
	}
	return &run, nil
}

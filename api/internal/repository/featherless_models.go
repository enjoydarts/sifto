package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FeatherlessModelRepo struct{ db *pgxpool.Pool }

func NewFeatherlessModelRepo(db *pgxpool.Pool) *FeatherlessModelRepo {
	return &FeatherlessModelRepo{db: db}
}

type FeatherlessSyncRun struct {
	ID             string     `json:"id"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	LastProgressAt *time.Time `json:"last_progress_at,omitempty"`
	Status         string     `json:"status"`
	TriggerType    string     `json:"trigger_type"`
	FetchedCount   int        `json:"fetched_count"`
	AcceptedCount  int        `json:"accepted_count"`
	ErrorMessage   *string    `json:"error_message,omitempty"`
}

type FeatherlessModelSnapshot struct {
	ModelID                string    `json:"model_id"`
	DisplayName            string    `json:"display_name"`
	ModelClass             string    `json:"model_class"`
	ContextLength          *int      `json:"context_length,omitempty"`
	MaxCompletionTokens    *int      `json:"max_completion_tokens,omitempty"`
	IsGated                bool      `json:"is_gated"`
	AvailableOnCurrentPlan bool      `json:"available_on_current_plan"`
	FetchedAt              time.Time `json:"fetched_at"`
}

func (r *FeatherlessModelRepo) StartSyncRun(ctx context.Context, triggerType string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO featherless_model_sync_runs (status, trigger_type)
		VALUES ('running', $1)
		RETURNING id`, triggerType,
	).Scan(&id)
	return id, err
}

func (r *FeatherlessModelRepo) FinishSyncRun(ctx context.Context, syncRunID string, fetchedCount, acceptedCount int, errMsg *string) error {
	status := "success"
	if errMsg != nil && *errMsg != "" {
		status = "failed"
	}
	_, err := r.db.Exec(ctx, `
		UPDATE featherless_model_sync_runs
		SET finished_at = NOW(),
		    last_progress_at = COALESCE(last_progress_at, NOW()),
		    status = $2,
		    fetched_count = $3,
		    accepted_count = $4,
		    error_message = $5
		WHERE id = $1`,
		syncRunID, status, fetchedCount, acceptedCount, errMsg,
	)
	return err
}

func (r *FeatherlessModelRepo) InsertSnapshots(ctx context.Context, syncRunID string, fetchedAt time.Time, models []FeatherlessModelSnapshot) error {
	if len(models) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, m := range models {
		if _, err := tx.Exec(ctx, `
			INSERT INTO featherless_model_snapshots (
				sync_run_id, fetched_at, model_id, display_name, model_class,
				context_length, max_completion_tokens, is_gated, available_on_current_plan
			) VALUES (
				$1, $2, $3, $4, $5,
				$6, $7, $8, $9
			)`,
			syncRunID, fetchedAt, m.ModelID, m.DisplayName, m.ModelClass,
			m.ContextLength, m.MaxCompletionTokens, m.IsGated, m.AvailableOnCurrentPlan,
		); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *FeatherlessModelRepo) ListLatestSnapshots(ctx context.Context) ([]FeatherlessModelSnapshot, *FeatherlessSyncRun, error) {
	var run FeatherlessSyncRun
	err := r.db.QueryRow(ctx, `
		SELECT id, started_at, finished_at, last_progress_at, status, trigger_type, fetched_count, accepted_count, error_message
		FROM featherless_model_sync_runs
		ORDER BY started_at DESC
		LIMIT 1`,
	).Scan(&run.ID, &run.StartedAt, &run.FinishedAt, &run.LastProgressAt, &run.Status, &run.TriggerType, &run.FetchedCount, &run.AcceptedCount, &run.ErrorMessage)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT model_id, display_name, model_class, context_length, max_completion_tokens, is_gated, available_on_current_plan, fetched_at
		FROM featherless_model_snapshots
		WHERE sync_run_id = $1
		ORDER BY display_name, model_id`, run.ID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	out := make([]FeatherlessModelSnapshot, 0)
	for rows.Next() {
		var m FeatherlessModelSnapshot
		if err := rows.Scan(
			&m.ModelID, &m.DisplayName, &m.ModelClass, &m.ContextLength, &m.MaxCompletionTokens,
			&m.IsGated, &m.AvailableOnCurrentPlan, &m.FetchedAt,
		); err != nil {
			return nil, nil, err
		}
		out = append(out, m)
	}
	return out, &run, rows.Err()
}

func (r *FeatherlessModelRepo) GetLatestManualRunningSyncRun(ctx context.Context) (*FeatherlessSyncRun, error) {
	var run FeatherlessSyncRun
	err := r.db.QueryRow(ctx, `
		SELECT id, started_at, finished_at, last_progress_at, status, trigger_type, fetched_count, accepted_count, error_message
		FROM featherless_model_sync_runs
		WHERE trigger_type = 'manual' AND status = 'running'
		ORDER BY started_at DESC
		LIMIT 1`,
	).Scan(&run.ID, &run.StartedAt, &run.FinishedAt, &run.LastProgressAt, &run.Status, &run.TriggerType, &run.FetchedCount, &run.AcceptedCount, &run.ErrorMessage)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &run, nil
}

func (r *FeatherlessModelRepo) ListPreviousSuccessfulSnapshots(ctx context.Context, beforeSyncRunID string) ([]FeatherlessModelSnapshot, error) {
	var previousRunID string
	err := r.db.QueryRow(ctx, `
		SELECT id
		FROM featherless_model_sync_runs
		WHERE status = 'success'
		  AND started_at < (SELECT started_at FROM featherless_model_sync_runs WHERE id = $1)
		ORDER BY started_at DESC
		LIMIT 1`, beforeSyncRunID,
	).Scan(&previousRunID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return []FeatherlessModelSnapshot{}, nil
		}
		return nil, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT model_id, display_name, model_class, context_length, max_completion_tokens, is_gated, available_on_current_plan, fetched_at
		FROM featherless_model_snapshots
		WHERE sync_run_id = $1
		ORDER BY display_name, model_id`, previousRunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]FeatherlessModelSnapshot, 0)
	for rows.Next() {
		var m FeatherlessModelSnapshot
		if err := rows.Scan(
			&m.ModelID, &m.DisplayName, &m.ModelClass, &m.ContextLength, &m.MaxCompletionTokens,
			&m.IsGated, &m.AvailableOnCurrentPlan, &m.FetchedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

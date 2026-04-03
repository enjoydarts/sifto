package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type XAIVoiceRepo struct{ db *pgxpool.Pool }

func NewXAIVoiceRepo(db *pgxpool.Pool) *XAIVoiceRepo {
	return &XAIVoiceRepo{db: db}
}

type XAIVoiceSyncRun struct {
	ID             int64      `json:"id"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	LastProgressAt *time.Time `json:"last_progress_at,omitempty"`
	Status         string     `json:"status"`
	TriggerType    string     `json:"trigger_type"`
	FetchedCount   int        `json:"fetched_count"`
	SavedCount     int        `json:"saved_count"`
	ErrorMessage   *string    `json:"error_message,omitempty"`
}

type XAIVoiceSnapshot struct {
	ID           int64     `json:"id"`
	SyncRunID    int64     `json:"sync_run_id"`
	VoiceID      string    `json:"voice_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Language     string    `json:"language"`
	PreviewURL   string    `json:"preview_url"`
	MetadataJSON []byte    `json:"metadata_json"`
	FetchedAt    time.Time `json:"fetched_at"`
}

func (r *XAIVoiceRepo) StartSyncRun(ctx context.Context, triggerType string) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO xai_voice_sync_runs (status, trigger_type)
		VALUES ('running', $1)
		RETURNING id
	`, triggerType).Scan(&id)
	return id, err
}

func (r *XAIVoiceRepo) FinishSyncRun(ctx context.Context, syncRunID int64, fetchedCount, savedCount int, errMsg *string) error {
	status := "success"
	if errMsg != nil && *errMsg != "" {
		status = "failed"
	}
	_, err := r.db.Exec(ctx, `
		UPDATE xai_voice_sync_runs
		SET finished_at = NOW(),
		    last_progress_at = COALESCE(last_progress_at, NOW()),
		    status = $2,
		    fetched_count = $3,
		    saved_count = $4,
		    error_message = $5
		WHERE id = $1
	`, syncRunID, status, fetchedCount, savedCount, errMsg)
	return err
}

func (r *XAIVoiceRepo) FailSyncRun(ctx context.Context, syncRunID int64, errMsg string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE xai_voice_sync_runs
		SET finished_at = NOW(),
		    last_progress_at = COALESCE(last_progress_at, NOW()),
		    status = 'failed',
		    error_message = $2
		WHERE id = $1
	`, syncRunID, errMsg)
	return err
}

func (r *XAIVoiceRepo) InsertSnapshots(ctx context.Context, syncRunID int64, fetchedAt time.Time, voices []XAIVoiceSnapshot) error {
	if len(voices) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, v := range voices {
		metadataJSON := "{}"
		if len(v.MetadataJSON) > 0 {
			metadataJSON = string(v.MetadataJSON)
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO xai_voice_snapshots (
				sync_run_id, voice_id, name, description, language, preview_url, metadata_json, fetched_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7::jsonb, $8
			)
		`, syncRunID, v.VoiceID, v.Name, v.Description, v.Language, v.PreviewURL, metadataJSON, fetchedAt)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *XAIVoiceRepo) ListLatestSnapshots(ctx context.Context) ([]XAIVoiceSnapshot, *XAIVoiceSyncRun, error) {
	var run XAIVoiceSyncRun
	err := r.db.QueryRow(ctx, `
		SELECT id, started_at, finished_at, last_progress_at, status, trigger_type, fetched_count, saved_count, error_message
		FROM xai_voice_sync_runs
		ORDER BY started_at DESC
		LIMIT 1
	`).Scan(&run.ID, &run.StartedAt, &run.FinishedAt, &run.LastProgressAt, &run.Status, &run.TriggerType, &run.FetchedCount, &run.SavedCount, &run.ErrorMessage)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, sync_run_id, voice_id, name, description, language, preview_url, metadata_json, fetched_at
		FROM xai_voice_snapshots
		WHERE sync_run_id = $1
		ORDER BY voice_id ASC, name ASC
	`, run.ID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	out := make([]XAIVoiceSnapshot, 0)
	for rows.Next() {
		var v XAIVoiceSnapshot
		if err := rows.Scan(&v.ID, &v.SyncRunID, &v.VoiceID, &v.Name, &v.Description, &v.Language, &v.PreviewURL, &v.MetadataJSON, &v.FetchedAt); err != nil {
			return nil, nil, err
		}
		out = append(out, v)
	}
	return out, &run, rows.Err()
}

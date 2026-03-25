package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AivisModelRepo struct{ db *pgxpool.Pool }

func NewAivisModelRepo(db *pgxpool.Pool) *AivisModelRepo {
	return &AivisModelRepo{db: db}
}

type AivisSyncRun struct {
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

type AivisModelSnapshot struct {
	AivmModelUUID       string          `json:"aivm_model_uuid"`
	Name                string          `json:"name"`
	Description         string          `json:"description"`
	DetailedDescription string          `json:"detailed_description"`
	Category            string          `json:"category"`
	VoiceTimbre         string          `json:"voice_timbre"`
	Visibility          string          `json:"visibility"`
	IsTagLocked         bool            `json:"is_tag_locked"`
	TotalDownloadCount  int             `json:"total_download_count"`
	LikeCount           int             `json:"like_count"`
	IsLiked             bool            `json:"is_liked"`
	UserJSON            json.RawMessage `json:"user_json"`
	ModelFilesJSON      json.RawMessage `json:"model_files_json"`
	TagsJSON            json.RawMessage `json:"tags_json"`
	SpeakersJSON        json.RawMessage `json:"speakers_json"`
	ModelFileCount      int             `json:"model_file_count"`
	SpeakerCount        int             `json:"speaker_count"`
	StyleCount          int             `json:"style_count"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
	FetchedAt           time.Time       `json:"fetched_at"`
}

func (r *AivisModelRepo) StartSyncRun(ctx context.Context, triggerType string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO aivis_model_sync_runs (status, trigger_type)
		VALUES ('running', $1)
		RETURNING id
	`, triggerType).Scan(&id)
	return id, err
}

func (r *AivisModelRepo) FinishSyncRun(ctx context.Context, syncRunID string, fetchedCount, acceptedCount int, errMsg *string) error {
	status := "success"
	if errMsg != nil && *errMsg != "" {
		status = "failed"
	}
	_, err := r.db.Exec(ctx, `
		UPDATE aivis_model_sync_runs
		SET finished_at = NOW(),
		    last_progress_at = COALESCE(last_progress_at, NOW()),
		    status = $2,
		    fetched_count = $3,
		    accepted_count = $4,
		    error_message = $5
		WHERE id = $1
	`, syncRunID, status, fetchedCount, acceptedCount, errMsg)
	return err
}

func (r *AivisModelRepo) FailSyncRun(ctx context.Context, syncRunID, errMsg string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE aivis_model_sync_runs
		SET finished_at = NOW(),
		    last_progress_at = COALESCE(last_progress_at, NOW()),
		    status = 'failed',
		    error_message = $2
		WHERE id = $1
	`, syncRunID, errMsg)
	return err
}

func (r *AivisModelRepo) InsertSnapshots(ctx context.Context, syncRunID string, fetchedAt time.Time, models []AivisModelSnapshot) error {
	if len(models) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, m := range models {
		_, err := tx.Exec(ctx, `
			INSERT INTO aivis_model_snapshots (
				sync_run_id, fetched_at, aivm_model_uuid, name, description, detailed_description,
				category, voice_timbre, visibility, is_tag_locked, total_download_count,
				like_count, is_liked, user_json, model_files_json, tags_json, speakers_json,
				model_file_count, speaker_count, style_count, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6,
				$7, $8, $9, $10, $11,
				$12, $13, $14, $15, $16, $17,
				$18, $19, $20, $21, $22
			)
		`,
			syncRunID, fetchedAt, m.AivmModelUUID, m.Name, m.Description, m.DetailedDescription,
			m.Category, m.VoiceTimbre, m.Visibility, m.IsTagLocked, m.TotalDownloadCount,
			m.LikeCount, m.IsLiked, m.UserJSON, m.ModelFilesJSON, m.TagsJSON, m.SpeakersJSON,
			m.ModelFileCount, m.SpeakerCount, m.StyleCount, m.CreatedAt, m.UpdatedAt,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *AivisModelRepo) ListLatestSnapshots(ctx context.Context) ([]AivisModelSnapshot, *AivisSyncRun, error) {
	var run AivisSyncRun
	err := r.db.QueryRow(ctx, `
		SELECT id, started_at, finished_at, last_progress_at, status, trigger_type, fetched_count, accepted_count, error_message
		FROM aivis_model_sync_runs
		ORDER BY started_at DESC
		LIMIT 1
	`).Scan(&run.ID, &run.StartedAt, &run.FinishedAt, &run.LastProgressAt, &run.Status, &run.TriggerType, &run.FetchedCount, &run.AcceptedCount, &run.ErrorMessage)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT aivm_model_uuid, name, description, detailed_description, category, voice_timbre, visibility,
		       is_tag_locked, total_download_count, like_count, is_liked, user_json, model_files_json,
		       tags_json, speakers_json, model_file_count, speaker_count, style_count, created_at, updated_at, fetched_at
		FROM aivis_model_snapshots
		WHERE sync_run_id = $1
		ORDER BY total_download_count DESC, like_count DESC, name ASC
	`, run.ID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	out := make([]AivisModelSnapshot, 0)
	for rows.Next() {
		var m AivisModelSnapshot
		if err := rows.Scan(
			&m.AivmModelUUID, &m.Name, &m.Description, &m.DetailedDescription, &m.Category, &m.VoiceTimbre, &m.Visibility,
			&m.IsTagLocked, &m.TotalDownloadCount, &m.LikeCount, &m.IsLiked, &m.UserJSON, &m.ModelFilesJSON,
			&m.TagsJSON, &m.SpeakersJSON, &m.ModelFileCount, &m.SpeakerCount, &m.StyleCount, &m.CreatedAt, &m.UpdatedAt, &m.FetchedAt,
		); err != nil {
			return nil, nil, err
		}
		out = append(out, m)
	}
	return out, &run, rows.Err()
}

func (r *AivisModelRepo) GetLatestManualRunningSyncRun(ctx context.Context) (*AivisSyncRun, error) {
	var run AivisSyncRun
	err := r.db.QueryRow(ctx, `
		SELECT id, started_at, finished_at, last_progress_at, status, trigger_type, fetched_count, accepted_count, error_message
		FROM aivis_model_sync_runs
		WHERE trigger_type = 'manual' AND status = 'running'
		ORDER BY started_at DESC
		LIMIT 1
	`).Scan(&run.ID, &run.StartedAt, &run.FinishedAt, &run.LastProgressAt, &run.Status, &run.TriggerType, &run.FetchedCount, &run.AcceptedCount, &run.ErrorMessage)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &run, nil
}

func (r *AivisModelRepo) ListPreviousSuccessfulSnapshots(ctx context.Context, beforeSyncRunID string) ([]AivisModelSnapshot, error) {
	rows, err := r.db.Query(ctx, `
		SELECT aivm_model_uuid, name, description, detailed_description, category, voice_timbre, visibility,
		       is_tag_locked, total_download_count, like_count, is_liked, user_json, model_files_json,
		       tags_json, speakers_json, model_file_count, speaker_count, style_count, created_at, updated_at, fetched_at
		FROM aivis_model_snapshots
		WHERE sync_run_id = (
			SELECT id
			FROM aivis_model_sync_runs
			WHERE status = 'success' AND id <> $1
			ORDER BY started_at DESC
			LIMIT 1
		)
		ORDER BY total_download_count DESC, like_count DESC, name ASC
	`, beforeSyncRunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AivisModelSnapshot, 0)
	for rows.Next() {
		var m AivisModelSnapshot
		if err := rows.Scan(
			&m.AivmModelUUID, &m.Name, &m.Description, &m.DetailedDescription, &m.Category, &m.VoiceTimbre, &m.Visibility,
			&m.IsTagLocked, &m.TotalDownloadCount, &m.LikeCount, &m.IsLiked, &m.UserJSON, &m.ModelFilesJSON,
			&m.TagsJSON, &m.SpeakersJSON, &m.ModelFileCount, &m.SpeakerCount, &m.StyleCount, &m.CreatedAt, &m.UpdatedAt, &m.FetchedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

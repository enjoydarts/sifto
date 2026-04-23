package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DeepInfraModelRepo struct{ db *pgxpool.Pool }

func NewDeepInfraModelRepo(db *pgxpool.Pool) *DeepInfraModelRepo {
	return &DeepInfraModelRepo{db: db}
}

type DeepInfraSyncRun struct {
	ID                        string     `json:"id"`
	StartedAt                 time.Time  `json:"started_at"`
	FinishedAt                *time.Time `json:"finished_at,omitempty"`
	LastProgressAt            *time.Time `json:"last_progress_at,omitempty"`
	Status                    string     `json:"status"`
	TriggerType               string     `json:"trigger_type"`
	FetchedCount              int        `json:"fetched_count"`
	AcceptedCount             int        `json:"accepted_count"`
	TranslationTargetCount    int        `json:"translation_target_count"`
	TranslationCompletedCount int        `json:"translation_completed_count"`
	TranslationFailedCount    int        `json:"translation_failed_count"`
	LastErrorMessage          *string    `json:"last_error_message,omitempty"`
	ErrorMessage              *string    `json:"error_message,omitempty"`
}

type DeepInfraModelSnapshot struct {
	ModelID             string          `json:"model_id"`
	DisplayName         string          `json:"display_name"`
	ProviderSlug        string          `json:"provider_slug"`
	ReportedType        string          `json:"reported_type"`
	DescriptionEN       *string         `json:"description_en,omitempty"`
	DescriptionJA       *string         `json:"description_ja,omitempty"`
	ContextLength       *int            `json:"context_length,omitempty"`
	MaxTokens           *int            `json:"max_tokens,omitempty"`
	InputPerMTokUSD     *float64        `json:"input_per_mtok_usd,omitempty"`
	OutputPerMTokUSD    *float64        `json:"output_per_mtok_usd,omitempty"`
	CacheReadPerMTokUSD *float64        `json:"cache_read_per_mtok_usd,omitempty"`
	TagsJSON            json.RawMessage `json:"tags_json,omitempty"`
	FetchedAt           time.Time       `json:"fetched_at"`
}

type DeepInfraDescriptionCacheEntry struct {
	ModelID       string
	DescriptionEN *string
	DescriptionJA *string
}

func (r *DeepInfraModelRepo) StartSyncRun(ctx context.Context, triggerType string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO deepinfra_model_sync_runs (status, trigger_type)
		VALUES ('running', $1)
		RETURNING id`, triggerType,
	).Scan(&id)
	return id, err
}

func (r *DeepInfraModelRepo) FinishSyncRun(ctx context.Context, syncRunID string, fetchedCount, acceptedCount int, errMsg *string) error {
	status := "success"
	if errMsg != nil && *errMsg != "" {
		status = "failed"
	}
	_, err := r.db.Exec(ctx, `
		UPDATE deepinfra_model_sync_runs
		SET finished_at = NOW(),
		    last_progress_at = COALESCE(last_progress_at, NOW()),
		    status = $2,
		    fetched_count = $3,
		    accepted_count = $4,
		    error_message = $5,
		    translation_completed_count = translation_target_count
		WHERE id = $1`,
		syncRunID, status, fetchedCount, acceptedCount, errMsg,
	)
	return err
}

func (r *DeepInfraModelRepo) UpdateTranslationProgress(ctx context.Context, syncRunID string, total, completed int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE deepinfra_model_sync_runs
		SET translation_target_count = $2,
		    translation_completed_count = $3,
		    last_progress_at = NOW()
		WHERE id = $1`,
		syncRunID, total, completed,
	)
	return err
}

func (r *DeepInfraModelRepo) RecordTranslationFailure(ctx context.Context, syncRunID, errMsg string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE deepinfra_model_sync_runs
		SET translation_failed_count = translation_failed_count + 1,
		    last_error_message = $2,
		    last_progress_at = NOW()
		WHERE id = $1`,
		syncRunID, errMsg,
	)
	return err
}

func (r *DeepInfraModelRepo) FailSyncRun(ctx context.Context, syncRunID, errMsg string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE deepinfra_model_sync_runs
		SET finished_at = NOW(),
		    last_progress_at = COALESCE(last_progress_at, NOW()),
		    status = 'failed',
		    error_message = $2,
		    last_error_message = $2
		WHERE id = $1`,
		syncRunID, errMsg,
	)
	return err
}

func (r *DeepInfraModelRepo) InsertSnapshots(ctx context.Context, syncRunID string, fetchedAt time.Time, models []DeepInfraModelSnapshot) error {
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
			INSERT INTO deepinfra_model_snapshots (
				sync_run_id, fetched_at, model_id, display_name, provider_slug, reported_type,
				description_en, description_ja, context_length, max_tokens,
				input_per_mtok_usd, output_per_mtok_usd, cache_read_per_mtok_usd, tags_json
			) VALUES (
				$1, $2, $3, $4, $5, $6,
				$7, $8, $9, $10,
				$11, $12, $13, $14
			)`,
			syncRunID, fetchedAt, m.ModelID, m.DisplayName, m.ProviderSlug, m.ReportedType,
			m.DescriptionEN, m.DescriptionJA, m.ContextLength, m.MaxTokens,
			m.InputPerMTokUSD, m.OutputPerMTokUSD, m.CacheReadPerMTokUSD, m.TagsJSON,
		); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *DeepInfraModelRepo) UpdateDescriptionsJA(ctx context.Context, syncRunID string, descriptions map[string]string) error {
	if len(descriptions) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for modelID, descriptionJA := range descriptions {
		if _, err := tx.Exec(ctx, `
			UPDATE deepinfra_model_snapshots
			SET description_ja = $3
			WHERE sync_run_id = $1 AND model_id = $2
		`, syncRunID, modelID, descriptionJA); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *DeepInfraModelRepo) ListLatestSnapshots(ctx context.Context) ([]DeepInfraModelSnapshot, *DeepInfraSyncRun, error) {
	var run DeepInfraSyncRun
	err := r.db.QueryRow(ctx, `
		SELECT id, started_at, finished_at, last_progress_at, status, trigger_type, fetched_count, accepted_count,
		       translation_target_count, translation_completed_count, translation_failed_count, last_error_message, error_message
		FROM deepinfra_model_sync_runs
		ORDER BY started_at DESC
		LIMIT 1`,
	).Scan(&run.ID, &run.StartedAt, &run.FinishedAt, &run.LastProgressAt, &run.Status, &run.TriggerType, &run.FetchedCount, &run.AcceptedCount, &run.TranslationTargetCount, &run.TranslationCompletedCount, &run.TranslationFailedCount, &run.LastErrorMessage, &run.ErrorMessage)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT model_id, display_name, provider_slug, reported_type, description_en, description_ja, context_length, max_tokens,
		       input_per_mtok_usd, output_per_mtok_usd, cache_read_per_mtok_usd, tags_json, fetched_at
		FROM deepinfra_model_snapshots
		WHERE sync_run_id = $1
		ORDER BY provider_slug, display_name, model_id`, run.ID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	out := make([]DeepInfraModelSnapshot, 0)
	for rows.Next() {
		var m DeepInfraModelSnapshot
		if err := rows.Scan(
			&m.ModelID, &m.DisplayName, &m.ProviderSlug, &m.ReportedType, &m.DescriptionEN, &m.DescriptionJA, &m.ContextLength, &m.MaxTokens,
			&m.InputPerMTokUSD, &m.OutputPerMTokUSD, &m.CacheReadPerMTokUSD, &m.TagsJSON, &m.FetchedAt,
		); err != nil {
			return nil, nil, err
		}
		out = append(out, m)
	}
	return out, &run, rows.Err()
}

func (r *DeepInfraModelRepo) GetLatestManualRunningSyncRun(ctx context.Context) (*DeepInfraSyncRun, error) {
	var run DeepInfraSyncRun
	err := r.db.QueryRow(ctx, `
		SELECT id, started_at, finished_at, last_progress_at, status, trigger_type, fetched_count, accepted_count,
		       translation_target_count, translation_completed_count, translation_failed_count, last_error_message, error_message
		FROM deepinfra_model_sync_runs
		WHERE trigger_type = 'manual' AND status = 'running'
		ORDER BY started_at DESC
		LIMIT 1`,
	).Scan(&run.ID, &run.StartedAt, &run.FinishedAt, &run.LastProgressAt, &run.Status, &run.TriggerType, &run.FetchedCount, &run.AcceptedCount, &run.TranslationTargetCount, &run.TranslationCompletedCount, &run.TranslationFailedCount, &run.LastErrorMessage, &run.ErrorMessage)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &run, nil
}

func (r *DeepInfraModelRepo) ListPreviousSuccessfulSnapshots(ctx context.Context, beforeSyncRunID string) ([]DeepInfraModelSnapshot, error) {
	var previousRunID string
	err := r.db.QueryRow(ctx, `
		SELECT id
		FROM deepinfra_model_sync_runs
		WHERE status = 'success'
		  AND started_at < (SELECT started_at FROM deepinfra_model_sync_runs WHERE id = $1)
		ORDER BY started_at DESC
		LIMIT 1`, beforeSyncRunID,
	).Scan(&previousRunID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return []DeepInfraModelSnapshot{}, nil
		}
		return nil, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT model_id, display_name, provider_slug, reported_type, description_en, description_ja, context_length, max_tokens,
		       input_per_mtok_usd, output_per_mtok_usd, cache_read_per_mtok_usd, tags_json, fetched_at
		FROM deepinfra_model_snapshots
		WHERE sync_run_id = $1
		ORDER BY provider_slug, display_name, model_id`, previousRunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]DeepInfraModelSnapshot, 0)
	for rows.Next() {
		var m DeepInfraModelSnapshot
		if err := rows.Scan(
			&m.ModelID, &m.DisplayName, &m.ProviderSlug, &m.ReportedType, &m.DescriptionEN, &m.DescriptionJA, &m.ContextLength, &m.MaxTokens,
			&m.InputPerMTokUSD, &m.OutputPerMTokUSD, &m.CacheReadPerMTokUSD, &m.TagsJSON, &m.FetchedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *DeepInfraModelRepo) ListLatestDescriptionCache(ctx context.Context) (map[string]DeepInfraDescriptionCacheEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT ON (model_id) model_id, description_en, description_ja
		FROM deepinfra_model_snapshots
		WHERE description_en IS NOT NULL
		ORDER BY model_id, fetched_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]DeepInfraDescriptionCacheEntry{}
	for rows.Next() {
		var entry DeepInfraDescriptionCacheEntry
		if err := rows.Scan(&entry.ModelID, &entry.DescriptionEN, &entry.DescriptionJA); err != nil {
			return nil, err
		}
		out[entry.ModelID] = entry
	}
	return out, rows.Err()
}

package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OpenRouterModelRepo struct{ db *pgxpool.Pool }

func NewOpenRouterModelRepo(db *pgxpool.Pool) *OpenRouterModelRepo {
	return &OpenRouterModelRepo{db: db}
}

type OpenRouterSyncRun struct {
	ID                        string     `json:"id"`
	StartedAt                 time.Time  `json:"started_at"`
	FinishedAt                *time.Time `json:"finished_at,omitempty"`
	Status                    string     `json:"status"`
	TriggerType               string     `json:"trigger_type"`
	FetchedCount              int        `json:"fetched_count"`
	AcceptedCount             int        `json:"accepted_count"`
	TranslationTargetCount    int        `json:"translation_target_count"`
	TranslationCompletedCount int        `json:"translation_completed_count"`
	ErrorMessage              *string    `json:"error_message,omitempty"`
}

type OpenRouterModelSnapshot struct {
	ModelID                 string          `json:"model_id"`
	CanonicalSlug           *string         `json:"canonical_slug,omitempty"`
	ProviderSlug            string          `json:"provider_slug"`
	DisplayName             string          `json:"display_name"`
	DescriptionEN           *string         `json:"description_en,omitempty"`
	DescriptionJA           *string         `json:"description_ja,omitempty"`
	ContextLength           *int            `json:"context_length,omitempty"`
	PricingJSON             json.RawMessage `json:"pricing_json"`
	SupportedParametersJSON json.RawMessage `json:"supported_parameters_json"`
	ArchitectureJSON        json.RawMessage `json:"architecture_json"`
	TopProviderJSON         json.RawMessage `json:"top_provider_json"`
	ModalityFlagsJSON       json.RawMessage `json:"modality_flags_json"`
	IsTextGeneration        bool            `json:"is_text_generation"`
	IsActive                bool            `json:"is_active"`
	FetchedAt               time.Time       `json:"fetched_at"`
}

func (r *OpenRouterModelRepo) StartSyncRun(ctx context.Context, triggerType string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO openrouter_model_sync_runs (status, trigger_type)
		VALUES ('running', $1)
		RETURNING id`,
		triggerType,
	).Scan(&id)
	return id, err
}

func (r *OpenRouterModelRepo) FinishSyncRun(ctx context.Context, syncRunID string, fetchedCount, acceptedCount int, errMsg *string) error {
	status := "success"
	if errMsg != nil && *errMsg != "" {
		status = "failed"
	}
	_, err := r.db.Exec(ctx, `
		UPDATE openrouter_model_sync_runs
		SET finished_at = NOW(),
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

func (r *OpenRouterModelRepo) UpdateTranslationProgress(ctx context.Context, syncRunID string, total, completed int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE openrouter_model_sync_runs
		SET translation_target_count = $2,
		    translation_completed_count = $3
		WHERE id = $1`,
		syncRunID, total, completed,
	)
	return err
}

func (r *OpenRouterModelRepo) InsertSnapshots(ctx context.Context, syncRunID string, fetchedAt time.Time, models []OpenRouterModelSnapshot) error {
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
			INSERT INTO openrouter_model_snapshots (
				sync_run_id, fetched_at, model_id, canonical_slug, provider_slug, display_name,
				description_en, description_ja, context_length, pricing_json,
				supported_parameters_json, architecture_json, top_provider_json,
				modality_flags_json, is_text_generation, is_active
			) VALUES (
				$1, $2, $3, $4, $5, $6,
				$7, $8, $9, $10,
				$11, $12, $13,
				$14, $15, $16
			)`,
			syncRunID, fetchedAt, m.ModelID, m.CanonicalSlug, m.ProviderSlug, m.DisplayName,
			m.DescriptionEN, m.DescriptionJA, m.ContextLength, m.PricingJSON,
			m.SupportedParametersJSON, m.ArchitectureJSON, m.TopProviderJSON,
			m.ModalityFlagsJSON, m.IsTextGeneration, m.IsActive,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *OpenRouterModelRepo) UpdateDescriptionsJA(ctx context.Context, syncRunID string, descriptions map[string]string) error {
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
			UPDATE openrouter_model_snapshots
			SET description_ja = $3
			WHERE sync_run_id = $1 AND model_id = $2
		`, syncRunID, modelID, descriptionJA); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *OpenRouterModelRepo) ListLatestSnapshots(ctx context.Context) ([]OpenRouterModelSnapshot, *OpenRouterSyncRun, error) {
	var run OpenRouterSyncRun
	err := r.db.QueryRow(ctx, `
		SELECT id, started_at, finished_at, status, trigger_type, fetched_count, accepted_count,
		       translation_target_count, translation_completed_count, error_message
		FROM openrouter_model_sync_runs
		ORDER BY started_at DESC
		LIMIT 1`,
	).Scan(&run.ID, &run.StartedAt, &run.FinishedAt, &run.Status, &run.TriggerType, &run.FetchedCount, &run.AcceptedCount, &run.TranslationTargetCount, &run.TranslationCompletedCount, &run.ErrorMessage)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT model_id, canonical_slug, provider_slug, display_name, description_en, description_ja,
		       context_length, pricing_json, supported_parameters_json, architecture_json,
		       top_provider_json, modality_flags_json, is_text_generation, is_active, fetched_at
		FROM openrouter_model_snapshots
		WHERE sync_run_id = $1
		ORDER BY provider_slug, display_name`, run.ID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	out := make([]OpenRouterModelSnapshot, 0)
	for rows.Next() {
		var m OpenRouterModelSnapshot
		if err := rows.Scan(
			&m.ModelID, &m.CanonicalSlug, &m.ProviderSlug, &m.DisplayName, &m.DescriptionEN, &m.DescriptionJA,
			&m.ContextLength, &m.PricingJSON, &m.SupportedParametersJSON, &m.ArchitectureJSON,
			&m.TopProviderJSON, &m.ModalityFlagsJSON, &m.IsTextGeneration, &m.IsActive, &m.FetchedAt,
		); err != nil {
			return nil, nil, err
		}
		out = append(out, m)
	}
	return out, &run, rows.Err()
}

func (r *OpenRouterModelRepo) GetLatestManualRunningSyncRun(ctx context.Context) (*OpenRouterSyncRun, error) {
	var run OpenRouterSyncRun
	err := r.db.QueryRow(ctx, `
		SELECT id, started_at, finished_at, status, trigger_type, fetched_count, accepted_count,
		       translation_target_count, translation_completed_count, error_message
		FROM openrouter_model_sync_runs
		WHERE trigger_type = 'manual' AND status = 'running'
		ORDER BY started_at DESC
		LIMIT 1`,
	).Scan(&run.ID, &run.StartedAt, &run.FinishedAt, &run.Status, &run.TriggerType, &run.FetchedCount, &run.AcceptedCount, &run.TranslationTargetCount, &run.TranslationCompletedCount, &run.ErrorMessage)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &run, nil
}

func (r *OpenRouterModelRepo) ListPreviousSuccessfulModelIDs(ctx context.Context, beforeSyncRunID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT s.model_id
		FROM openrouter_model_snapshots s
		WHERE s.sync_run_id = (
			SELECT id
			FROM openrouter_model_sync_runs
			WHERE status = 'success' AND id <> $1
			ORDER BY started_at DESC
			LIMIT 1
		)
	`, beforeSyncRunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var modelID string
		if err := rows.Scan(&modelID); err != nil {
			return nil, err
		}
		out = append(out, modelID)
	}
	return out, rows.Err()
}

func (r *OpenRouterModelRepo) ListModelsByIDsForRun(ctx context.Context, syncRunID string, modelIDs []string) ([]OpenRouterModelSnapshot, error) {
	if len(modelIDs) == 0 {
		return []OpenRouterModelSnapshot{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT model_id, canonical_slug, provider_slug, display_name, description_en, description_ja,
		       context_length, pricing_json, supported_parameters_json, architecture_json,
		       top_provider_json, modality_flags_json, is_text_generation, is_active, fetched_at
		FROM openrouter_model_snapshots
		WHERE sync_run_id = $1 AND model_id = ANY($2::text[])
		ORDER BY provider_slug, display_name
	`, syncRunID, modelIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]OpenRouterModelSnapshot, 0, len(modelIDs))
	for rows.Next() {
		var m OpenRouterModelSnapshot
		if err := rows.Scan(
			&m.ModelID, &m.CanonicalSlug, &m.ProviderSlug, &m.DisplayName, &m.DescriptionEN, &m.DescriptionJA,
			&m.ContextLength, &m.PricingJSON, &m.SupportedParametersJSON, &m.ArchitectureJSON,
			&m.TopProviderJSON, &m.ModalityFlagsJSON, &m.IsTextGeneration, &m.IsActive, &m.FetchedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *OpenRouterModelRepo) InsertNotificationLogs(ctx context.Context, syncRunID string, dayJST time.Time, modelIDs []string) error {
	for _, modelID := range modelIDs {
		if _, err := r.db.Exec(ctx, `
			INSERT INTO openrouter_model_notification_logs (sync_run_id, model_id, day_jst)
			VALUES ($1, $2, $3::date)
			ON CONFLICT (model_id, day_jst) DO NOTHING
		`, syncRunID, modelID, dayJST.Format("2006-01-02")); err != nil {
			return err
		}
	}
	return nil
}

type OpenRouterDescriptionCacheEntry struct {
	ModelID       string
	DescriptionEN *string
	DescriptionJA *string
}

func (r *OpenRouterModelRepo) ListLatestDescriptionCache(ctx context.Context) (map[string]OpenRouterDescriptionCacheEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT ON (model_id) model_id, description_en, description_ja
		FROM openrouter_model_snapshots
		ORDER BY model_id, created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]OpenRouterDescriptionCacheEntry)
	for rows.Next() {
		var entry OpenRouterDescriptionCacheEntry
		if err := rows.Scan(&entry.ModelID, &entry.DescriptionEN, &entry.DescriptionJA); err != nil {
			return nil, err
		}
		out[entry.ModelID] = entry
	}
	return out, rows.Err()
}

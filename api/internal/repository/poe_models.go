package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PoeModelRepo struct{ db *pgxpool.Pool }

func NewPoeModelRepo(db *pgxpool.Pool) *PoeModelRepo {
	return &PoeModelRepo{db: db}
}

type PoeSyncRun struct {
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

type PoeModelSnapshot struct {
	ModelID                          string          `json:"model_id"`
	CanonicalSlug                    *string         `json:"canonical_slug,omitempty"`
	DisplayName                      string          `json:"display_name"`
	OwnedBy                          string          `json:"owned_by"`
	DescriptionEN                    *string         `json:"description_en,omitempty"`
	DescriptionJA                    *string         `json:"description_ja,omitempty"`
	ContextLength                    *int            `json:"context_length,omitempty"`
	PricingJSON                      json.RawMessage `json:"pricing_json"`
	ArchitectureJSON                 json.RawMessage `json:"architecture_json"`
	ModalityFlagsJSON                json.RawMessage `json:"modality_flags_json"`
	IsActive                         bool            `json:"is_active"`
	TransportSupportsOpenAICompat    bool            `json:"transport_supports_openai_compat"`
	TransportSupportsAnthropicCompat bool            `json:"transport_supports_anthropic_compat"`
	PreferredTransport               string          `json:"preferred_transport"`
	FetchedAt                        time.Time       `json:"fetched_at"`
}

func (r *PoeModelRepo) StartSyncRun(ctx context.Context, triggerType string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO poe_model_sync_runs (status, trigger_type)
		VALUES ('running', $1)
		RETURNING id`, triggerType,
	).Scan(&id)
	return id, err
}

func (r *PoeModelRepo) FinishSyncRun(ctx context.Context, syncRunID string, fetchedCount, acceptedCount int, errMsg *string) error {
	status := "success"
	if errMsg != nil && *errMsg != "" {
		status = "failed"
	}
	_, err := r.db.Exec(ctx, `
		UPDATE poe_model_sync_runs
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

func (r *PoeModelRepo) UpdateTranslationProgress(ctx context.Context, syncRunID string, total, completed int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE poe_model_sync_runs
		SET translation_target_count = $2,
		    translation_completed_count = $3,
		    last_progress_at = NOW()
		WHERE id = $1`,
		syncRunID, total, completed,
	)
	return err
}

func (r *PoeModelRepo) RecordTranslationFailure(ctx context.Context, syncRunID, modelID, errMsg string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE poe_model_sync_runs
		SET translation_failed_count = translation_failed_count + 1,
		    last_error_message = $2,
		    last_progress_at = NOW()
		WHERE id = $1`,
		syncRunID, errMsg,
	)
	return err
}

func (r *PoeModelRepo) FailSyncRun(ctx context.Context, syncRunID, errMsg string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE poe_model_sync_runs
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

func (r *PoeModelRepo) InsertSnapshots(ctx context.Context, syncRunID string, fetchedAt time.Time, models []PoeModelSnapshot) error {
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
			INSERT INTO poe_model_snapshots (
				sync_run_id, fetched_at, model_id, canonical_slug, display_name, owned_by,
				description_en, description_ja, context_length, pricing_json, architecture_json,
				modality_flags_json, is_active, transport_supports_openai_compat,
				transport_supports_anthropic_compat, preferred_transport
			) VALUES (
				$1, $2, $3, $4, $5, $6,
				$7, $8, $9, $10, $11,
				$12, $13, $14,
				$15, $16
			)`,
			syncRunID, fetchedAt, m.ModelID, m.CanonicalSlug, m.DisplayName, m.OwnedBy,
			m.DescriptionEN, m.DescriptionJA, m.ContextLength, m.PricingJSON, m.ArchitectureJSON,
			m.ModalityFlagsJSON, m.IsActive, m.TransportSupportsOpenAICompat,
			m.TransportSupportsAnthropicCompat, m.PreferredTransport,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *PoeModelRepo) UpdateDescriptionsJA(ctx context.Context, syncRunID string, descriptions map[string]string) error {
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
			UPDATE poe_model_snapshots
			SET description_ja = $3
			WHERE sync_run_id = $1 AND model_id = $2
		`, syncRunID, modelID, descriptionJA); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *PoeModelRepo) ListLatestSnapshots(ctx context.Context) ([]PoeModelSnapshot, *PoeSyncRun, error) {
	var run PoeSyncRun
	err := r.db.QueryRow(ctx, `
		SELECT id, started_at, finished_at, last_progress_at, status, trigger_type, fetched_count, accepted_count,
		       translation_target_count, translation_completed_count, translation_failed_count, last_error_message, error_message
		FROM poe_model_sync_runs
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
		SELECT model_id, canonical_slug, display_name, owned_by, description_en, description_ja,
		       context_length, pricing_json, architecture_json, modality_flags_json, is_active,
		       transport_supports_openai_compat, transport_supports_anthropic_compat, preferred_transport, fetched_at
		FROM poe_model_snapshots
		WHERE sync_run_id = $1
		ORDER BY owned_by, display_name`, run.ID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	out := make([]PoeModelSnapshot, 0)
	for rows.Next() {
		var m PoeModelSnapshot
		if err := rows.Scan(
			&m.ModelID, &m.CanonicalSlug, &m.DisplayName, &m.OwnedBy, &m.DescriptionEN, &m.DescriptionJA,
			&m.ContextLength, &m.PricingJSON, &m.ArchitectureJSON, &m.ModalityFlagsJSON, &m.IsActive,
			&m.TransportSupportsOpenAICompat, &m.TransportSupportsAnthropicCompat, &m.PreferredTransport, &m.FetchedAt,
		); err != nil {
			return nil, nil, err
		}
		out = append(out, m)
	}
	return out, &run, rows.Err()
}

func (r *PoeModelRepo) GetLatestManualRunningSyncRun(ctx context.Context) (*PoeSyncRun, error) {
	var run PoeSyncRun
	err := r.db.QueryRow(ctx, `
		SELECT id, started_at, finished_at, last_progress_at, status, trigger_type, fetched_count, accepted_count,
		       translation_target_count, translation_completed_count, translation_failed_count, last_error_message, error_message
		FROM poe_model_sync_runs
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

func (r *PoeModelRepo) ListPreviousSuccessfulSnapshots(ctx context.Context, beforeSyncRunID string) ([]PoeModelSnapshot, error) {
	rows, err := r.db.Query(ctx, `
		SELECT model_id, canonical_slug, display_name, owned_by, description_en, description_ja,
		       context_length, pricing_json, architecture_json, modality_flags_json, is_active,
		       transport_supports_openai_compat, transport_supports_anthropic_compat, preferred_transport, fetched_at
		FROM poe_model_snapshots
		WHERE sync_run_id = (
			SELECT id
			FROM poe_model_sync_runs
			WHERE status = 'success' AND id <> $1
			ORDER BY started_at DESC
			LIMIT 1
		)
		ORDER BY owned_by, display_name`, beforeSyncRunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PoeModelSnapshot, 0)
	for rows.Next() {
		var m PoeModelSnapshot
		if err := rows.Scan(
			&m.ModelID, &m.CanonicalSlug, &m.DisplayName, &m.OwnedBy, &m.DescriptionEN, &m.DescriptionJA,
			&m.ContextLength, &m.PricingJSON, &m.ArchitectureJSON, &m.ModalityFlagsJSON, &m.IsActive,
			&m.TransportSupportsOpenAICompat, &m.TransportSupportsAnthropicCompat, &m.PreferredTransport, &m.FetchedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

type PoeDescriptionCacheEntry struct {
	ModelID       string
	DescriptionEN *string
	DescriptionJA *string
}

func (r *PoeModelRepo) ListLatestDescriptionCache(ctx context.Context) (map[string]PoeDescriptionCacheEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT ON (model_id) model_id, description_en, description_ja
		FROM poe_model_snapshots
		ORDER BY model_id, fetched_at DESC, id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]PoeDescriptionCacheEntry)
	for rows.Next() {
		var entry PoeDescriptionCacheEntry
		if err := rows.Scan(&entry.ModelID, &entry.DescriptionEN, &entry.DescriptionJA); err != nil {
			return nil, err
		}
		out[entry.ModelID] = entry
	}
	return out, rows.Err()
}

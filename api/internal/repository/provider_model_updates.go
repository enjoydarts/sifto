package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProviderModelUpdateRepo struct {
	db *pgxpool.Pool
}

func NewProviderModelUpdateRepo(db *pgxpool.Pool) *ProviderModelUpdateRepo {
	return &ProviderModelUpdateRepo{db: db}
}

func (r *ProviderModelUpdateRepo) GetSnapshot(ctx context.Context, provider string) (*model.ProviderModelSnapshot, error) {
	var s model.ProviderModelSnapshot
	var raw []byte
	var errText *string
	err := r.db.QueryRow(ctx, `
		SELECT provider, models, fetched_at, status, error
		FROM provider_model_snapshots
		WHERE provider = $1`, provider,
	).Scan(&s.Provider, &raw, &s.FetchedAt, &s.Status, &errText)
	if err != nil {
		return nil, mapDBError(err)
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s.Models); err != nil {
			return nil, err
		}
	}
	s.Error = errText
	return &s, nil
}

func (r *ProviderModelUpdateRepo) UpsertSnapshot(ctx context.Context, provider string, models []string, status string, errText *string) error {
	raw, err := json.Marshal(models)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO provider_model_snapshots (provider, models, fetched_at, status, error)
		VALUES ($1, $2::jsonb, NOW(), $3, $4)
		ON CONFLICT (provider) DO UPDATE SET
		  models = EXCLUDED.models,
		  fetched_at = EXCLUDED.fetched_at,
		  status = EXCLUDED.status,
		  error = EXCLUDED.error`,
		provider, string(raw), status, errText,
	)
	return err
}

func (r *ProviderModelUpdateRepo) InsertChangeEvents(ctx context.Context, events []model.ProviderModelChangeEvent) error {
	for _, ev := range events {
		raw, err := json.Marshal(ev.Metadata)
		if err != nil {
			return err
		}
		if _, err := r.db.Exec(ctx, `
			INSERT INTO provider_model_change_events (provider, change_type, model_id, detected_at, metadata)
			VALUES ($1, $2, $3, $4, $5::jsonb)`,
			ev.Provider, ev.ChangeType, ev.ModelID, ev.DetectedAt, string(raw),
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *ProviderModelUpdateRepo) ListRecent(ctx context.Context, since time.Time, limit int) ([]model.ProviderModelChangeEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, provider, change_type, model_id, detected_at, metadata
		FROM provider_model_change_events
		WHERE detected_at >= $1
		ORDER BY detected_at DESC, provider ASC, change_type ASC, model_id ASC
		LIMIT $2`, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []model.ProviderModelChangeEvent{}
	for rows.Next() {
		var ev model.ProviderModelChangeEvent
		var raw []byte
		if err := rows.Scan(&ev.ID, &ev.Provider, &ev.ChangeType, &ev.ModelID, &ev.DetectedAt, &raw); err != nil {
			return nil, err
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &ev.Metadata)
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (r *ProviderModelUpdateRepo) ListLatestProviderSummary(ctx context.Context, provider string) (*model.ProviderModelChangeSummary, error) {
	var detectedAt time.Time
	var trigger string
	err := r.db.QueryRow(ctx, `
		SELECT detected_at, COALESCE(metadata->>'trigger', '')
		FROM provider_model_change_events
		WHERE provider = $1
		ORDER BY detected_at DESC
		LIMIT 1`, provider,
	).Scan(&detectedAt, &trigger)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, provider, change_type, model_id, detected_at, metadata
		FROM provider_model_change_events
		WHERE provider = $1 AND detected_at = $2
		ORDER BY change_type ASC, model_id ASC`, provider, detectedAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summary := &model.ProviderModelChangeSummary{
		Provider:    provider,
		DetectedAt:  detectedAt,
		Trigger:     trigger,
		Added:       []model.ProviderModelChangeEvent{},
		Constrained: []model.ProviderModelChangeEvent{},
		Removed:     []model.ProviderModelChangeEvent{},
	}
	for rows.Next() {
		var ev model.ProviderModelChangeEvent
		var raw []byte
		if err := rows.Scan(&ev.ID, &ev.Provider, &ev.ChangeType, &ev.ModelID, &ev.DetectedAt, &raw); err != nil {
			return nil, err
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &ev.Metadata)
		}
		switch ev.ChangeType {
		case "added":
			summary.Added = append(summary.Added, ev)
		case "constrained":
			summary.Constrained = append(summary.Constrained, ev)
		case "removed":
			summary.Removed = append(summary.Removed, ev)
		}
	}
	return summary, rows.Err()
}

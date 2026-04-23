package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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
	models = normalizeProviderSnapshotModelIDs(models)
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

func normalizeProviderSnapshotModelIDs(models []string) []string {
	if len(models) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(models))
	out := make([]string, 0, len(models))
	for _, modelID := range models {
		modelID = strings.TrimSpace(modelID)
		if modelID == "" {
			continue
		}
		if _, ok := seen[modelID]; ok {
			continue
		}
		seen[modelID] = struct{}{}
		out = append(out, modelID)
	}
	sort.Strings(out)
	return out
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
		Provider:            provider,
		DetectedAt:          detectedAt,
		Trigger:             trigger,
		Added:               []model.ProviderModelChangeEvent{},
		Constrained:         []model.ProviderModelChangeEvent{},
		AvailabilityChanged: []model.ProviderModelChangeEvent{},
		GatedChanged:        []model.ProviderModelChangeEvent{},
		PricingChanged:      []model.ProviderModelChangeEvent{},
		ContextChanged:      []model.ProviderModelChangeEvent{},
		Removed:             []model.ProviderModelChangeEvent{},
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
		case "availability_changed":
			summary.AvailabilityChanged = append(summary.AvailabilityChanged, ev)
		case "gated_changed":
			summary.GatedChanged = append(summary.GatedChanged, ev)
		case "pricing_changed":
			summary.PricingChanged = append(summary.PricingChanged, ev)
		case "context_changed":
			summary.ContextChanged = append(summary.ContextChanged, ev)
		case "removed":
			summary.Removed = append(summary.Removed, ev)
		}
	}
	return summary, rows.Err()
}

func (r *ProviderModelUpdateRepo) ListSnapshotEntries(ctx context.Context, providers []string, query string, limit, offset int) ([]model.ProviderModelSnapshotEntry, int, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	excludedProviders := []string{"aivis", "openrouter", "poe", "featherless", "deepinfra"}
	args := []any{excludedProviders}
	conditions := []string{"NOT (provider = ANY($1))"}

	if len(providers) > 0 {
		normalized := make([]string, 0, len(providers))
		for _, provider := range providers {
			provider = strings.TrimSpace(strings.ToLower(provider))
			if provider == "" || provider == "aivis" {
				continue
			}
			normalized = append(normalized, provider)
		}
		if len(normalized) > 0 {
			args = append(args, normalized)
			conditions = append(conditions, fmt.Sprintf("provider = ANY($%d)", len(args)))
		}
	}

	if q := strings.TrimSpace(query); q != "" {
		args = append(args, "%"+q+"%")
		conditions = append(conditions, fmt.Sprintf("(provider ILIKE $%d OR model_id ILIKE $%d)", len(args), len(args)))
	}

	whereClause := strings.Join(conditions, " AND ")

	var total int
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM provider_model_snapshots pms
		LEFT JOIN LATERAL jsonb_array_elements_text(pms.models) AS model_entry(model_id) ON TRUE
		WHERE %s`, whereClause)
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	rowsQuery := fmt.Sprintf(`
		SELECT provider, COALESCE(model_entry.model_id, ''), fetched_at, status, error
		FROM provider_model_snapshots pms
		LEFT JOIN LATERAL jsonb_array_elements_text(pms.models) AS model_entry(model_id) ON TRUE
		WHERE %s
		ORDER BY fetched_at DESC, provider ASC, model_entry.model_id ASC
		LIMIT $%d OFFSET $%d`, whereClause, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, rowsQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]model.ProviderModelSnapshotEntry, 0, limit)
	for rows.Next() {
		var entry model.ProviderModelSnapshotEntry
		if err := rows.Scan(&entry.Provider, &entry.ModelID, &entry.FetchedAt, &entry.Status, &entry.Error); err != nil {
			return nil, 0, err
		}
		items = append(items, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *ProviderModelUpdateRepo) ListSnapshotProviders(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT provider
		FROM provider_model_snapshots
		WHERE provider NOT IN ('aivis', 'openrouter', 'poe', 'featherless', 'deepinfra')
		ORDER BY provider ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	providers := make([]string, 0, 16)
	for rows.Next() {
		var provider string
		if err := rows.Scan(&provider); err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return providers, nil
}

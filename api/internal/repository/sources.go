package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minoru-kitayama/sifto/api/internal/model"
)

type SourceRepo struct{ db *pgxpool.Pool }

func NewSourceRepo(db *pgxpool.Pool) *SourceRepo { return &SourceRepo{db} }

func (r *SourceRepo) CountByUser(ctx context.Context, userID string) (int, error) {
	var n int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*)::int FROM sources WHERE user_id = $1`, userID).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (r *SourceRepo) List(ctx context.Context, userID string) ([]model.Source, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, url, type, title, enabled, last_fetched_at, created_at, updated_at
		FROM sources WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []model.Source
	for rows.Next() {
		var s model.Source
		if err := rows.Scan(&s.ID, &s.UserID, &s.URL, &s.Type, &s.Title,
			&s.Enabled, &s.LastFetchedAt, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		sources = append(sources, s)
	}
	return sources, nil
}

func (r *SourceRepo) Create(ctx context.Context, userID, url, srcType string, title *string) (*model.Source, error) {
	var s model.Source
	err := r.db.QueryRow(ctx, `
		INSERT INTO sources (user_id, url, type, title)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, url, type, title, enabled, last_fetched_at, created_at, updated_at`,
		userID, url, srcType, title,
	).Scan(&s.ID, &s.UserID, &s.URL, &s.Type, &s.Title,
		&s.Enabled, &s.LastFetchedAt, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &s, nil
}

func (r *SourceRepo) Update(ctx context.Context, id, userID string, enabled *bool, updateTitle bool, title *string) (*model.Source, error) {
	var s model.Source
	err := r.db.QueryRow(ctx, `
		UPDATE sources
		SET enabled = COALESCE($1, enabled),
		    title = CASE WHEN $2 THEN $3 ELSE title END,
		    updated_at = NOW()
		WHERE id = $4 AND user_id = $5
		RETURNING id, user_id, url, type, title, enabled, last_fetched_at, created_at, updated_at`,
		enabled, updateTitle, title, id, userID,
	).Scan(&s.ID, &s.UserID, &s.URL, &s.Type, &s.Title,
		&s.Enabled, &s.LastFetchedAt, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &s, nil
}

func (r *SourceRepo) Delete(ctx context.Context, id, userID string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM sources WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SourceRepo) ListEnabled(ctx context.Context) ([]model.Source, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, url, type, title, enabled, last_fetched_at, created_at, updated_at
		FROM sources WHERE enabled = true AND type = 'rss'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []model.Source
	for rows.Next() {
		var s model.Source
		if err := rows.Scan(&s.ID, &s.UserID, &s.URL, &s.Type, &s.Title,
			&s.Enabled, &s.LastFetchedAt, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		sources = append(sources, s)
	}
	return sources, nil
}

func (r *SourceRepo) UpdateLastFetchedAt(ctx context.Context, id string, fetchedAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE sources
		SET last_fetched_at = $1, updated_at = NOW()
		WHERE id = $2`,
		fetchedAt, id)
	return err
}

func (r *SourceRepo) GetUserIDBySourceID(ctx context.Context, sourceID string) (string, error) {
	var userID string
	err := r.db.QueryRow(ctx, `SELECT user_id FROM sources WHERE id = $1`, sourceID).Scan(&userID)
	if err != nil {
		return "", mapDBError(err)
	}
	return userID, nil
}

func (r *SourceRepo) HealthByUser(ctx context.Context, userID string) ([]model.SourceHealth, error) {
	sources, err := r.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	snapshotBySourceID := map[string]model.SourceHealth{}
	rows, err := r.db.Query(ctx, `
		SELECT sh.source_id, sh.total_items, sh.failed_items, sh.summarized_items,
		       sh.failure_rate, sh.last_item_at, sh.last_fetched_at, sh.status
		FROM source_health_snapshots sh
		JOIN sources s ON s.id = sh.source_id
		WHERE s.user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var h model.SourceHealth
		if err := rows.Scan(
			&h.SourceID,
			&h.TotalItems,
			&h.FailedItems,
			&h.Summarized,
			&h.FailureRate,
			&h.LastItemAt,
			&h.LastFetchedAt,
			&h.Status,
		); err != nil {
			return nil, err
		}
		snapshotBySourceID[h.SourceID] = h
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	aggBySourceID := map[string]model.SourceHealth{}
	aggRows, err := r.db.Query(ctx, `
		SELECT
			s.id AS source_id,
			s.enabled AS enabled,
			s.last_fetched_at,
			COUNT(i.id)::int AS total_items,
			COUNT(*) FILTER (WHERE i.status = 'failed')::int AS failed_items,
			COUNT(*) FILTER (WHERE i.status = 'summarized')::int AS summarized_items,
			MAX(i.created_at) AS last_item_at
		FROM sources s
		LEFT JOIN items i ON i.source_id = s.id
		WHERE s.user_id = $1
		GROUP BY s.id, s.enabled, s.last_fetched_at`, userID)
	if err != nil {
		return nil, err
	}
	defer aggRows.Close()
	for aggRows.Next() {
		var (
			h       model.SourceHealth
			enabled bool
		)
		if err := aggRows.Scan(
			&h.SourceID,
			&enabled,
			&h.LastFetchedAt,
			&h.TotalItems,
			&h.FailedItems,
			&h.Summarized,
			&h.LastItemAt,
		); err != nil {
			return nil, err
		}
		h.Status = deriveSourceHealthStatus(enabled, h.TotalItems, h.FailedItems, h.FailureRate, h.LastFetchedAt)
		if h.TotalItems > 0 && h.FailureRate == 0 {
			h.FailureRate = float64(h.FailedItems) / float64(h.TotalItems)
		}
		aggBySourceID[h.SourceID] = h
	}
	if err := aggRows.Err(); err != nil {
		return nil, err
	}

	out := make([]model.SourceHealth, 0, len(sources))
	for _, s := range sources {
		if snap, ok := snapshotBySourceID[s.ID]; ok {
			out = append(out, snap)
			continue
		}
		h, ok := aggBySourceID[s.ID]
		if !ok {
			h = model.SourceHealth{
				SourceID:      s.ID,
				TotalItems:    0,
				FailedItems:   0,
				Summarized:    0,
				FailureRate:   0,
				LastFetchedAt: s.LastFetchedAt,
				Status:        deriveSourceHealthStatus(s.Enabled, 0, 0, 0, s.LastFetchedAt),
			}
		}
		out = append(out, h)
	}
	return out, nil
}

func (r *SourceRepo) RefreshHealthSnapshot(ctx context.Context, sourceID string, reason *string) error {
	var (
		h       model.SourceHealth
		enabled bool
	)
	err := r.db.QueryRow(ctx, `
		SELECT
			s.id AS source_id,
			s.enabled AS enabled,
			s.last_fetched_at,
			COUNT(i.id)::int AS total_items,
			COUNT(*) FILTER (WHERE i.status = 'failed')::int AS failed_items,
			COUNT(*) FILTER (WHERE i.status = 'summarized')::int AS summarized_items,
			MAX(i.created_at) AS last_item_at
		FROM sources s
		LEFT JOIN items i ON i.source_id = s.id
		WHERE s.id = $1
		GROUP BY s.id, s.enabled, s.last_fetched_at`, sourceID,
	).Scan(
		&h.SourceID,
		&enabled,
		&h.LastFetchedAt,
		&h.TotalItems,
		&h.FailedItems,
		&h.Summarized,
		&h.LastItemAt,
	)
	if err != nil {
		return mapDBError(err)
	}
	if h.TotalItems > 0 {
		h.FailureRate = float64(h.FailedItems) / float64(h.TotalItems)
	}
	h.Status = deriveSourceHealthStatus(enabled, h.TotalItems, h.FailedItems, h.FailureRate, h.LastFetchedAt)
	if reason != nil && *reason != "" {
		h.Status = "error"
	}
	if _, err := r.db.Exec(ctx, `
		INSERT INTO source_health_snapshots (
			source_id, total_items, failed_items, summarized_items, failure_rate,
			last_item_at, last_fetched_at, status, reason, checked_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, NOW(), NOW()
		)
		ON CONFLICT (source_id) DO UPDATE SET
			total_items = EXCLUDED.total_items,
			failed_items = EXCLUDED.failed_items,
			summarized_items = EXCLUDED.summarized_items,
			failure_rate = EXCLUDED.failure_rate,
			last_item_at = EXCLUDED.last_item_at,
			last_fetched_at = EXCLUDED.last_fetched_at,
			status = EXCLUDED.status,
			reason = EXCLUDED.reason,
			checked_at = NOW(),
			updated_at = NOW()`,
		h.SourceID, h.TotalItems, h.FailedItems, h.Summarized, h.FailureRate,
		h.LastItemAt, h.LastFetchedAt, h.Status, reason,
	); err != nil {
		return fmt.Errorf("upsert source health snapshot: %w", err)
	}
	return nil
}

func deriveSourceHealthStatus(enabled bool, totalItems, failedItems int, failureRate float64, lastFetchedAt *time.Time) string {
	switch {
	case !enabled:
		return "disabled"
	case totalItems == 0:
		return "new"
	case failedItems >= 3 && failureRate >= 0.5:
		return "error"
	case lastFetchedAt == nil || time.Since(*lastFetchedAt) > 72*time.Hour:
		return "stale"
	default:
		return "ok"
	}
}

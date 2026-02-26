package repository

import (
	"context"
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

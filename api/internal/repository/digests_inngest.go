package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minoru-kitayama/sifto/api/internal/model"
)

type DigestInngestRepo struct{ db *pgxpool.Pool }

func NewDigestInngestRepo(db *pgxpool.Pool) *DigestInngestRepo { return &DigestInngestRepo{db} }

func (r *DigestInngestRepo) Create(ctx context.Context, userID string, date time.Time, items []model.DigestItemDetail) (string, bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback(ctx)

	dateStr := date.Format("2006-01-02")
	var digestID string
	var sentAt *time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO digests (user_id, digest_date)
		VALUES ($1, $2)
		ON CONFLICT (user_id, digest_date) DO UPDATE SET digest_date = EXCLUDED.digest_date
		RETURNING id, sent_at`,
		userID, dateStr,
	).Scan(&digestID, &sentAt)
	if err != nil {
		return "", false, err
	}

	// Keep sent digests immutable to avoid changing already-delivered content.
	if sentAt != nil {
		if err := tx.Commit(ctx); err != nil {
			return "", true, err
		}
		return digestID, true, nil
	}

	// Clear existing items for idempotency
	if _, err := tx.Exec(ctx, `DELETE FROM digest_items WHERE digest_id = $1`, digestID); err != nil {
		return "", false, err
	}

	for _, item := range items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO digest_items (digest_id, item_id, rank) VALUES ($1, $2, $3)`,
			digestID, item.Item.ID, item.Rank); err != nil {
			return "", false, err
		}
	}

	return digestID, false, tx.Commit(ctx)
}

func (r *DigestInngestRepo) UpdateSentAt(ctx context.Context, digestID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE digests
		SET sent_at = NOW(),
		    send_status = 'sent',
		    send_error = NULL,
		    send_tried_at = NOW()
		WHERE id = $1`, digestID)
	return err
}

func (r *DigestInngestRepo) UpdateEmailCopy(ctx context.Context, digestID string, subject, body string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE digests
		SET email_subject = $1, email_body = $2
		WHERE id = $3`,
		subject, body, digestID)
	return err
}

func (r *DigestInngestRepo) ReplaceClusterDrafts(ctx context.Context, digestID string, drafts []model.DigestClusterDraft) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM digest_cluster_drafts WHERE digest_id = $1`, digestID); err != nil {
		return err
	}
	for _, d := range drafts {
		if _, err := tx.Exec(ctx, `
			INSERT INTO digest_cluster_drafts (
				digest_id, cluster_key, cluster_label, rank, item_count, topics, max_score, draft_summary
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			digestID, d.ClusterKey, d.ClusterLabel, d.Rank, d.ItemCount, d.Topics, d.MaxScore, d.DraftSummary,
		); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *DigestInngestRepo) ListClusterDrafts(ctx context.Context, digestID string) ([]model.DigestClusterDraft, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, digest_id, cluster_key, cluster_label, rank, item_count, topics, max_score, draft_summary, created_at, updated_at
		FROM digest_cluster_drafts
		WHERE digest_id = $1
		ORDER BY rank ASC, created_at ASC`, digestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.DigestClusterDraft{}
	for rows.Next() {
		var d model.DigestClusterDraft
		if err := rows.Scan(
			&d.ID, &d.DigestID, &d.ClusterKey, &d.ClusterLabel, &d.Rank, &d.ItemCount,
			&d.Topics, &d.MaxScore, &d.DraftSummary, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *DigestInngestRepo) UpdateSendStatus(ctx context.Context, digestID, status string, sendErr *string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE digests
		SET send_status = $1,
		    send_error = $2,
		    send_tried_at = NOW()
		WHERE id = $3`,
		status, sendErr, digestID)
	return err
}

func (r *DigestInngestRepo) GetForEmail(ctx context.Context, digestID string) (*model.DigestDetail, error) {
	repo := &DigestRepo{db: r.db}
	var userID string
	if err := r.db.QueryRow(ctx, `SELECT user_id FROM digests WHERE id = $1`, digestID).Scan(&userID); err != nil {
		return nil, err
	}
	return repo.GetDetail(ctx, digestID, userID)
}

package repository

import (
	"context"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DigestRepo struct{ db *pgxpool.Pool }

func NewDigestRepo(db *pgxpool.Pool) *DigestRepo { return &DigestRepo{db} }

func (r *DigestRepo) List(ctx context.Context, userID string) ([]model.Digest, error) {
	return r.ListLimit(ctx, userID, 30)
}

func (r *DigestRepo) ListLimit(ctx context.Context, userID string, limit int) ([]model.Digest, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, digest_date::text, email_subject, email_body,
		       digest_retry_count, cluster_draft_retry_count,
		       send_status, send_error, send_tried_at, sent_at, created_at
		FROM digests WHERE user_id = $1 ORDER BY digest_date DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var digests []model.Digest
	for rows.Next() {
		var d model.Digest
		if err := rows.Scan(&d.ID, &d.UserID, &d.DigestDate, &d.EmailSubject, &d.EmailBody,
			&d.DigestRetryCount, &d.ClusterDraftRetryCount,
			&d.SendStatus, &d.SendError, &d.SendTriedAt, &d.SentAt, &d.CreatedAt); err != nil {
			return nil, err
		}
		digests = append(digests, d)
	}
	return digests, nil
}

func (r *DigestRepo) GetDetail(ctx context.Context, id, userID string) (*model.DigestDetail, error) {
	d, err := r.loadDigestDetailBase(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	r.loadDigestLLMRefs(ctx, id, d)

	items, err := r.queryDigestItems(ctx, id)
	if err != nil {
		return nil, err
	}
	d.Items = items

	clusterDrafts, err := r.queryDigestClusterDrafts(ctx, id)
	if err != nil {
		return nil, err
	}
	d.ClusterDrafts = clusterDrafts
	return d, nil
}

func (r *DigestRepo) GetLatest(ctx context.Context, userID string) (*model.DigestDetail, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		SELECT id FROM digests WHERE user_id = $1 ORDER BY digest_date DESC LIMIT 1`, userID,
	).Scan(&id)
	if err != nil {
		return nil, mapDBError(err)
	}
	return r.GetDetail(ctx, id, userID)
}

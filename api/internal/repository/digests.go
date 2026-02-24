package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minoru-kitayama/sifto/api/internal/model"
)

type DigestRepo struct{ db *pgxpool.Pool }

func NewDigestRepo(db *pgxpool.Pool) *DigestRepo { return &DigestRepo{db} }

func (r *DigestRepo) List(ctx context.Context, userID string) ([]model.Digest, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, digest_date::text, email_subject, email_body,
		       send_status, send_error, send_tried_at, sent_at, created_at
		FROM digests WHERE user_id = $1 ORDER BY digest_date DESC LIMIT 30`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var digests []model.Digest
	for rows.Next() {
		var d model.Digest
		if err := rows.Scan(&d.ID, &d.UserID, &d.DigestDate, &d.EmailSubject, &d.EmailBody,
			&d.SendStatus, &d.SendError, &d.SendTriedAt, &d.SentAt, &d.CreatedAt); err != nil {
			return nil, err
		}
		digests = append(digests, d)
	}
	return digests, nil
}

func (r *DigestRepo) GetDetail(ctx context.Context, id, userID string) (*model.DigestDetail, error) {
	var d model.DigestDetail
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, digest_date::text, email_subject, email_body,
		       send_status, send_error, send_tried_at, sent_at, created_at
		FROM digests WHERE id = $1 AND user_id = $2`, id, userID,
	).Scan(&d.ID, &d.UserID, &d.DigestDate, &d.EmailSubject, &d.EmailBody,
		&d.SendStatus, &d.SendError, &d.SendTriedAt, &d.SentAt, &d.CreatedAt)
	if err != nil {
		return nil, mapDBError(err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT di.rank,
		       i.id, i.source_id, i.url, i.title, i.content_text, i.status,
		       i.published_at, i.fetched_at, i.created_at, i.updated_at,
		       s.id, s.item_id, s.summary, s.topics, s.score,
		       s.score_breakdown, s.score_reason, s.score_policy_version, s.summarized_at
		FROM digest_items di
		JOIN items i ON i.id = di.item_id
		JOIN item_summaries s ON s.item_id = i.id
		WHERE di.digest_id = $1
		ORDER BY di.rank`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var did model.DigestItemDetail
		if err := rows.Scan(
			&did.Rank,
			&did.Item.ID, &did.Item.SourceID, &did.Item.URL, &did.Item.Title,
			&did.Item.ContentText, &did.Item.Status, &did.Item.PublishedAt,
			&did.Item.FetchedAt, &did.Item.CreatedAt, &did.Item.UpdatedAt,
			&did.Summary.ID, &did.Summary.ItemID, &did.Summary.Summary,
			&did.Summary.Topics, &did.Summary.Score,
			scoreBreakdownScanner{dst: &did.Summary.ScoreBreakdown}, &did.Summary.ScoreReason,
			&did.Summary.ScorePolicyVersion, &did.Summary.SummarizedAt,
		); err != nil {
			return nil, err
		}
		d.Items = append(d.Items, did)
	}
	return &d, nil
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

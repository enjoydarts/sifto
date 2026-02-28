package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minoru-kitayama/sifto/api/internal/model"
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
			       i.id, i.source_id, i.url, i.title, i.thumbnail_url, i.content_text, i.status,
			       i.published_at, i.fetched_at, i.created_at, i.updated_at,
			       s.id, s.item_id, s.summary, s.topics, s.translated_title, s.score,
			       s.score_breakdown, s.score_reason, s.score_policy_version, s.summarized_at,
			       COALESCE(f.facts, '[]'::jsonb) AS facts
			FROM digest_items di
			JOIN items i ON i.id = di.item_id
			JOIN item_summaries s ON s.item_id = i.id
			LEFT JOIN item_facts f ON f.item_id = i.id
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
			&did.Item.ID, &did.Item.SourceID, &did.Item.URL, &did.Item.Title, &did.Item.ThumbnailURL,
			&did.Item.ContentText, &did.Item.Status, &did.Item.PublishedAt,
			&did.Item.FetchedAt, &did.Item.CreatedAt, &did.Item.UpdatedAt,
			&did.Summary.ID, &did.Summary.ItemID, &did.Summary.Summary,
			&did.Summary.Topics, &did.Summary.TranslatedTitle, &did.Summary.Score,
			scoreBreakdownScanner{dst: &did.Summary.ScoreBreakdown}, &did.Summary.ScoreReason,
			&did.Summary.ScorePolicyVersion, &did.Summary.SummarizedAt,
			jsonStringArrayScanner{dst: &did.Facts},
		); err != nil {
			return nil, err
		}
		d.Items = append(d.Items, did)
	}
	clusterDraftRows, err := r.db.Query(ctx, `
		SELECT id, digest_id, cluster_key, cluster_label, rank, item_count, topics, max_score, draft_summary, created_at, updated_at
		FROM digest_cluster_drafts
		WHERE digest_id = $1
		ORDER BY rank ASC, created_at ASC`, id)
	if err != nil {
		return nil, err
	}
	defer clusterDraftRows.Close()
	for clusterDraftRows.Next() {
		var cd model.DigestClusterDraft
		if err := clusterDraftRows.Scan(
			&cd.ID, &cd.DigestID, &cd.ClusterKey, &cd.ClusterLabel, &cd.Rank, &cd.ItemCount,
			&cd.Topics, &cd.MaxScore, &cd.DraftSummary, &cd.CreatedAt, &cd.UpdatedAt,
		); err != nil {
			return nil, err
		}
		d.ClusterDrafts = append(d.ClusterDrafts, cd)
	}
	if err := clusterDraftRows.Err(); err != nil {
		return nil, err
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

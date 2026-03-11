package repository

import (
	"context"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func (r *DigestRepo) loadDigestDetailBase(ctx context.Context, id, userID string) (*model.DigestDetail, error) {
	var d model.DigestDetail
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, digest_date::text, email_subject, email_body,
		       digest_retry_count, cluster_draft_retry_count,
		       send_status, send_error, send_tried_at, sent_at, created_at
		FROM digests
		WHERE id = $1 AND user_id = $2`, id, userID,
	).Scan(&d.ID, &d.UserID, &d.DigestDate, &d.EmailSubject, &d.EmailBody,
		&d.DigestRetryCount, &d.ClusterDraftRetryCount,
		&d.SendStatus, &d.SendError, &d.SendTriedAt, &d.SentAt, &d.CreatedAt)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &d, nil
}

func (r *DigestRepo) queryDigestItems(ctx context.Context, digestID string) ([]model.DigestItemDetail, error) {
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
		ORDER BY di.rank`, digestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.DigestItemDetail, 0)
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
		out = append(out, did)
	}
	return out, rows.Err()
}

func (r *DigestRepo) queryDigestClusterDrafts(ctx context.Context, digestID string) ([]model.DigestClusterDraft, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, digest_id, cluster_key, cluster_label, rank, item_count, topics, max_score, draft_summary, created_at, updated_at
		FROM digest_cluster_drafts
		WHERE digest_id = $1
		ORDER BY rank ASC, created_at ASC`, digestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.DigestClusterDraft, 0)
	for rows.Next() {
		var cd model.DigestClusterDraft
		if err := rows.Scan(
			&cd.ID, &cd.DigestID, &cd.ClusterKey, &cd.ClusterLabel, &cd.Rank, &cd.ItemCount,
			&cd.Topics, &cd.MaxScore, &cd.DraftSummary, &cd.CreatedAt, &cd.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, cd)
	}
	return out, rows.Err()
}

package repository

import (
	"context"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func (r *ItemRepo) loadItemDetailBase(ctx context.Context, id, userID string) (*model.ItemDetail, error) {
	var d model.ItemDetail
	var deleted bool
	err := r.db.QueryRow(ctx, `
		SELECT i.id, i.source_id, s.title, i.url, i.title, i.thumbnail_url, i.content_text, i.status,
		       i.deleted_at IS NOT NULL AS is_deleted,
		       sm.translated_title,
		       i.user_genre,
		       i.user_other_genre_label,
		       `+effectiveGenreExpr("i", "sm")+` AS genre,
		       `+effectiveOtherGenreLabelExpr("i", "sm")+` AS other_genre_label,
		       EXISTS (
		           SELECT 1 FROM item_reads ir
		           WHERE ir.item_id = i.id AND ir.user_id = $2
		       ) AS is_read, i.processing_error,
		       i.published_at, i.fetched_at, i.created_at, i.updated_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		LEFT JOIN item_summaries sm ON sm.item_id = i.id
		WHERE i.id = $1 AND s.user_id = $2`, id, userID,
	).Scan(&d.ID, &d.SourceID, &d.SourceTitle, &d.URL, &d.Title, &d.ThumbnailURL, &d.ContentText,
		&d.Status, &deleted, &d.TranslatedTitle, &d.UserGenre, &d.UserOtherGenreLabel, &d.Genre, &d.OtherGenreLabel, &d.IsRead, &d.ProcessingError, &d.PublishedAt, &d.FetchedAt, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, mapDBError(err)
	}
	d.Status = normalizeItemDetailStatus(d.Status, deleted)
	return &d, nil
}

func normalizeItemDetailStatus(status string, deleted bool) string {
	if deleted {
		return "deleted"
	}
	return status
}

func (r *ItemRepo) queryFactsDetail(ctx context.Context, itemID string) (*model.ItemFacts, error) {
	var facts model.ItemFacts
	err := r.db.QueryRow(ctx, `
		SELECT id, item_id, facts, extracted_at
		FROM item_facts
		WHERE item_id = $1`, itemID,
	).Scan(&facts.ID, &facts.ItemID, &facts.Facts, &facts.ExtractedAt)
	if err != nil {
		return nil, err
	}
	return &facts, nil
}

func (r *ItemRepo) querySummaryDetail(ctx context.Context, itemID string) (*model.ItemSummary, error) {
	var summary model.ItemSummary
	err := r.db.QueryRow(ctx, `
		SELECT id, item_id, summary, topics, genre, other_genre_label, translated_title, score, score_breakdown, score_reason, score_policy_version, summarized_at
		FROM item_summaries
		WHERE item_id = $1`, itemID,
	).Scan(&summary.ID, &summary.ItemID, &summary.Summary, &summary.Topics, &summary.Genre, &summary.OtherGenreLabel, &summary.TranslatedTitle, &summary.Score,
		scoreBreakdownScanner{dst: &summary.ScoreBreakdown}, &summary.ScoreReason, &summary.ScorePolicyVersion, &summary.SummarizedAt)
	if err != nil {
		return nil, err
	}
	return &summary, nil
}

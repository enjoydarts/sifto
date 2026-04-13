package repository

import (
	"context"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/jackc/pgx/v5"
)

type retryCandidate struct {
	item      model.Item
	isDeleted bool
}

func (r *ItemRepo) GetForRetry(ctx context.Context, id, userID string) (*model.Item, error) {
	candidate, err := r.loadRetryCandidate(ctx, r.db, id, userID, false)
	if err != nil {
		return nil, err
	}
	return &candidate.item, nil
}

func (r *ItemRepo) ResetForExtractRetry(ctx context.Context, id, userID string) (*model.Item, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	candidate, err := r.loadRetryCandidate(ctx, tx, id, userID, true)
	if err != nil {
		return nil, err
	}
	it := candidate.item

	if _, err := tx.Exec(ctx, `DELETE FROM item_embeddings WHERE item_id = $1`, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM summary_faithfulness_checks WHERE item_id = $1`, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM item_summaries WHERE item_id = $1`, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM item_facts_checks WHERE item_id = $1`, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM item_facts WHERE item_id = $1`, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE items
		SET status = 'new',
		    content_text = NULL,
		    fetched_at = NULL,
		    processing_error = NULL,
		    updated_at = NOW()
		WHERE id = $1`, id); err != nil {
		return nil, err
	}
	it.Status = "new"
	it.ContentText = nil
	it.Summary = nil

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &it, nil
}

func (r *ItemRepo) ResetForFactsRetry(ctx context.Context, id, userID string) (*model.Item, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	candidate, err := r.loadRetryCandidate(ctx, tx, id, userID, true)
	if err != nil {
		return nil, err
	}
	it := candidate.item
	if it.ContentText == nil || strings.TrimSpace(*it.ContentText) == "" {
		return nil, ErrConflict
	}

	if _, err := tx.Exec(ctx, `DELETE FROM item_embeddings WHERE item_id = $1`, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM summary_faithfulness_checks WHERE item_id = $1`, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM item_summaries WHERE item_id = $1`, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM item_facts_checks WHERE item_id = $1`, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM item_facts WHERE item_id = $1`, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE items
		SET status = 'fetched',
		    processing_error = NULL,
		    updated_at = NOW()
		WHERE id = $1`, id); err != nil {
		return nil, err
	}
	it.Status = "fetched"
	it.Summary = nil

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &it, nil
}

type retryCandidateQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func loadRetryCandidateQuery(forUpdate bool) string {
	query := `
		SELECT i.id, i.source_id, i.url, i.title, i.thumbnail_url, i.content_text, sm.summary, i.status,
		       i.deleted_at IS NOT NULL AS is_deleted,
		       FALSE AS is_read,
		       FALSE AS is_favorite,
		       0 AS feedback_rating,
		       i.published_at, i.fetched_at, i.created_at, i.updated_at
		FROM items i
		LEFT JOIN item_summaries sm ON sm.item_id = i.id
		JOIN sources s ON s.id = i.source_id
		WHERE i.id = $1 AND s.user_id = $2`
	if forUpdate {
		query += ` FOR UPDATE OF i`
	}
	return query
}

func (r *ItemRepo) loadRetryCandidate(ctx context.Context, q retryCandidateQuerier, id, userID string, forUpdate bool) (*retryCandidate, error) {
	query := loadRetryCandidateQuery(forUpdate)
	var candidate retryCandidate
	err := q.QueryRow(ctx, query, id, userID).Scan(
		&candidate.item.ID,
		&candidate.item.SourceID,
		&candidate.item.URL,
		&candidate.item.Title,
		&candidate.item.ThumbnailURL,
		&candidate.item.ContentText,
		&candidate.item.Summary,
		&candidate.item.Status,
		&candidate.isDeleted,
		&candidate.item.IsRead,
		&candidate.item.IsFavorite,
		&candidate.item.FeedbackRating,
		&candidate.item.PublishedAt,
		&candidate.item.FetchedAt,
		&candidate.item.CreatedAt,
		&candidate.item.UpdatedAt,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	if candidate.isDeleted {
		return nil, ErrConflict
	}
	return &candidate, nil
}

func (r *ItemRepo) ListFailedForRetry(ctx context.Context, userID string, sourceID *string) ([]model.Item, error) {
	query := `
		SELECT i.id, i.source_id, i.url, i.title, i.thumbnail_url, i.content_text, sm.summary, i.status,
		       FALSE AS is_read,
		       FALSE AS is_favorite,
		       0 AS feedback_rating,
		       i.published_at, i.fetched_at, i.created_at, i.updated_at
		FROM items i
		LEFT JOIN item_summaries sm ON sm.item_id = i.id
		JOIN sources s ON s.id = i.source_id
		WHERE s.user_id = $1
		  AND i.deleted_at IS NULL
		  AND (
		    i.status IN ('new', 'fetched', 'facts_extracted', 'failed')
		    OR (i.status = 'summarized' AND NULLIF(BTRIM(sm.summary), '') IS NULL)
		  )`
	args := []any{userID}
	if sourceID != nil {
		args = append(args, *sourceID)
		query += ` AND i.source_id = $2`
	}
	query += ` ORDER BY i.updated_at DESC LIMIT 500`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.Item
	for rows.Next() {
		var it model.Item
		if err := rows.Scan(&it.ID, &it.SourceID, &it.URL, &it.Title, &it.ThumbnailURL, &it.ContentText, &it.Summary,
			&it.Status, &it.IsRead, &it.IsFavorite, &it.FeedbackRating, &it.PublishedAt, &it.FetchedAt, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, nil
}

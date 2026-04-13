package repository

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ItemRepo struct{ db *pgxpool.Pool }

func NewItemRepo(db *pgxpool.Pool) *ItemRepo { return &ItemRepo{db} }

type ownedItemState string

const (
	ownedItemMissing ownedItemState = "missing"
	ownedItemDeleted ownedItemState = "deleted"
	ownedItemActive  ownedItemState = "active"
)

type ItemListParams struct {
	Status       *string
	SourceID     *string
	Topic        *string
	Query        *string
	UnreadOnly   bool
	ReadOnly     bool
	FavoriteOnly bool
	LaterOnly    bool
	Sort         string // newest | score | personal_score
	Page         int
	PageSize     int
}

type BulkMarkReadParams struct {
	Status        *string
	SourceID      *string
	Topic         *string
	UnreadOnly    bool
	ReadOnly      bool
	FavoriteOnly  bool
	LaterOnly     bool
	OlderThanDays *int
}

func itoa(n int) string { return strconv.Itoa(n) }

func appendItemStatusFilter(query string, args []any, status *string) (string, []any) {
	if status == nil || *status == "" {
		return query, args
	}
	if *status == "deleted" {
		return query + ` AND i.deleted_at IS NOT NULL`, args
	}
	if *status == "pending" {
		return query + ` AND i.deleted_at IS NULL AND i.status IN ('new', 'fetched', 'facts_extracted', 'failed')`, args
	}
	args = append(args, *status)
	return query + ` AND i.deleted_at IS NULL AND i.status = $` + itoa(len(args)), args
}

type itemRowScanner interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanItems(rows itemRowScanner) ([]model.Item, error) {
	var items []model.Item
	for rows.Next() {
		var it model.Item
		if err := rows.Scan(&it.ID, &it.SourceID, &it.SourceTitle, &it.URL, &it.Title, &it.ThumbnailURL, &it.ContentText,
			&it.Status, &it.ProcessingError, &it.FactsCheckResult, &it.FaithfulnessResult, &it.IsRead, &it.IsFavorite, &it.FeedbackRating, &it.SummaryScore, &it.PersonalScore, &it.PersonalScoreReason, &it.SummaryTopics, &it.TranslatedTitle, &it.PublishedAt, &it.FetchedAt, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func scanItemsWithBreakdown(rows itemRowScanner) ([]model.Item, error) {
	var items []model.Item
	for rows.Next() {
		var it model.Item
		if err := rows.Scan(&it.ID, &it.SourceID, &it.SourceTitle, &it.URL, &it.Title, &it.ThumbnailURL, &it.ContentText,
			&it.Status, &it.ProcessingError, &it.FactsCheckResult, &it.FaithfulnessResult, &it.IsRead, &it.IsFavorite, &it.FeedbackRating, &it.SummaryScore, &it.SummaryScoreBreakdown, &it.PersonalScore, &it.PersonalScoreReason, &it.SummaryTopics, &it.TranslatedTitle, &it.PublishedAt, &it.FetchedAt, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func firstTopicKey(topics []string) string {
	for _, t := range topics {
		if t != "" {
			return t
		}
	}
	return "__untagged__"
}

func (r *ItemRepo) List(ctx context.Context, userID string, status, sourceID *string, limit int) ([]model.Item, error) {
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}
	query := `
		SELECT i.id, i.source_id, s.title AS source_title, i.url, i.title, i.thumbnail_url, COALESCE(sm.summary, i.content_text) AS content_text, i.status, i.processing_error,
		       fc.final_result AS facts_check_result,
		       sfc.final_result AS faithfulness_result,
		       (ir.item_id IS NOT NULL) AS is_read,
		       COALESCE(fb.is_favorite, false) AS is_favorite,
		       COALESCE(fb.rating, 0) AS feedback_rating,
		       sm.score, COALESCE(sm.topics, '{}'::text[]), sm.translated_title,
		       i.published_at, i.fetched_at, i.created_at, i.updated_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
		LEFT JOIN item_feedbacks fb ON fb.item_id = i.id AND fb.user_id = $1
		LEFT JOIN item_summaries sm ON sm.item_id = i.id
		LEFT JOIN item_facts_checks fc ON fc.item_id = i.id
		LEFT JOIN summary_faithfulness_checks sfc ON sfc.item_id = i.id
		WHERE s.user_id = $1`
	args := []any{userID}

	if status != nil {
		query, args = appendItemStatusFilter(query, args, status)
	} else {
		query += ` AND i.deleted_at IS NULL`
	}
	if sourceID != nil {
		args = append(args, *sourceID)
		query += ` AND i.source_id = $` + itoa(len(args))
	}
	if status != nil && *status == "summarized" {
		query += ` ORDER BY sm.score DESC NULLS LAST, i.created_at DESC LIMIT ` + itoa(limit)
	} else {
		query += ` ORDER BY i.created_at DESC LIMIT ` + itoa(limit)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.Item
	for rows.Next() {
		var it model.Item
		if err := rows.Scan(&it.ID, &it.SourceID, &it.SourceTitle, &it.URL, &it.Title, &it.ThumbnailURL, &it.ContentText,
			&it.Status, &it.ProcessingError, &it.FactsCheckResult, &it.FaithfulnessResult, &it.IsRead, &it.IsFavorite, &it.FeedbackRating, &it.SummaryScore, &it.SummaryTopics, &it.TranslatedTitle, &it.PublishedAt, &it.FetchedAt, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, nil
}

func (r *ItemRepo) ListPage(ctx context.Context, userID string, p ItemListParams) (*model.ItemListResponse, error) {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.PageSize <= 0 {
		p.PageSize = 20
	}
	if p.PageSize > 200 {
		p.PageSize = 200
	}
	if p.Sort != "score" && p.Sort != "personal_score" {
		p.Sort = "newest"
	}

	baseWhere := ` FROM items i
		JOIN sources s ON s.id = i.source_id
		WHERE s.user_id = $1`
	args := []any{userID}
	if p.Status != nil {
		baseWhere, args = appendItemStatusFilter(baseWhere, args, p.Status)
	} else {
		baseWhere += ` AND i.deleted_at IS NULL`
	}
	if p.SourceID != nil {
		args = append(args, *p.SourceID)
		baseWhere += ` AND i.source_id = $` + itoa(len(args))
	}
	if p.Topic != nil && *p.Topic != "" {
		args = append(args, *p.Topic)
		baseWhere += ` AND EXISTS (
			SELECT 1
			FROM item_summaries smt
			WHERE smt.item_id = i.id
			  AND COALESCE(smt.topics, '{}'::text[]) @> ARRAY[$` + itoa(len(args)) + `::text]
		)`
	}
	if p.Query != nil && strings.TrimSpace(*p.Query) != "" {
		args = append(args, "%"+strings.TrimSpace(*p.Query)+"%")
		baseWhere += ` AND (
			COALESCE(i.title, '') ILIKE $` + itoa(len(args)) + `
			OR i.url ILIKE $` + itoa(len(args)) + `
			OR EXISTS (
				SELECT 1
				FROM item_summaries smq
				WHERE smq.item_id = i.id
				  AND COALESCE(smq.translated_title, '') ILIKE $` + itoa(len(args)) + `
			)
		)`
	}
	if p.UnreadOnly {
		baseWhere += ` AND NOT EXISTS (
			SELECT 1 FROM item_reads ir2
			WHERE ir2.item_id = i.id AND ir2.user_id = $1
		)`
	}
	if p.ReadOnly {
		baseWhere += ` AND EXISTS (
			SELECT 1 FROM item_reads ir2
			WHERE ir2.item_id = i.id AND ir2.user_id = $1
		)`
	}
	if p.FavoriteOnly {
		baseWhere += ` AND EXISTS (
			SELECT 1 FROM item_feedbacks fb2
			WHERE fb2.item_id = i.id AND fb2.user_id = $1 AND fb2.is_favorite = true
		)`
	}
	if p.LaterOnly {
		baseWhere += ` AND EXISTS (
			SELECT 1 FROM item_laters il2
			WHERE il2.item_id = i.id AND il2.user_id = $1
		)`
	}

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*)`+baseWhere, args...).Scan(&total); err != nil {
		return nil, err
	}

	offset := (p.Page - 1) * p.PageSize
	args = append(args, p.PageSize, offset)
	limitArg := `$` + itoa(len(args)-1)
	offsetArg := `$` + itoa(len(args))

	orderBy := ` ORDER BY i.created_at DESC`
	if p.Sort == "score" {
		orderBy = ` ORDER BY sm.score DESC NULLS LAST, i.created_at DESC`
	} else if p.Sort == "personal_score" {
		orderBy = ` ORDER BY sm.personal_score DESC NULLS LAST, sm.score DESC NULLS LAST, i.created_at DESC`
	}

	rows, err := r.db.Query(ctx, `
		SELECT i.id, i.source_id, s.title AS source_title, i.url, i.title, i.thumbnail_url, COALESCE(sm.summary, i.content_text) AS content_text, i.status, i.processing_error,
		       fc.final_result AS facts_check_result,
		       sfc.final_result AS faithfulness_result,
		       (ir.item_id IS NOT NULL) AS is_read,
		       COALESCE(fb.is_favorite, false) AS is_favorite,
		       COALESCE(fb.rating, 0) AS feedback_rating,
		       sm.score, sm.personal_score, sm.personal_score_reason, COALESCE(sm.topics, '{}'::text[]), sm.translated_title,
		       i.published_at, i.fetched_at, i.created_at, i.updated_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
		LEFT JOIN item_feedbacks fb ON fb.item_id = i.id AND fb.user_id = $1
		LEFT JOIN item_summaries sm ON sm.item_id = i.id
		LEFT JOIN item_facts_checks fc ON fc.item_id = i.id
		LEFT JOIN summary_faithfulness_checks sfc ON sfc.item_id = i.id
		WHERE s.user_id = $1`+
		func() string {
			q := ""
			nextIdx := 2
			if p.Status != nil {
				if *p.Status == "deleted" {
					q += ` AND i.deleted_at IS NOT NULL`
				} else if *p.Status == "pending" {
					q += ` AND i.deleted_at IS NULL AND i.status IN ('new', 'fetched', 'facts_extracted', 'failed')`
				} else {
					q += ` AND i.deleted_at IS NULL AND i.status = $` + itoa(nextIdx)
					nextIdx++
				}
			} else {
				q += ` AND i.deleted_at IS NULL`
			}
			if p.SourceID != nil {
				q += ` AND i.source_id = $` + itoa(nextIdx)
				nextIdx++
			}
			if p.Topic != nil && *p.Topic != "" {
				q += ` AND EXISTS (
					SELECT 1 FROM item_summaries smt
					WHERE smt.item_id = i.id
					  AND COALESCE(smt.topics, '{}'::text[]) @> ARRAY[$` + itoa(nextIdx) + `::text]
				)`
				nextIdx++
			}
			if p.Query != nil && strings.TrimSpace(*p.Query) != "" {
				q += ` AND (
					COALESCE(i.title, '') ILIKE $` + itoa(nextIdx) + `
					OR i.url ILIKE $` + itoa(nextIdx) + `
					OR COALESCE(sm.translated_title, '') ILIKE $` + itoa(nextIdx) + `
				)`
				nextIdx++
			}
			if p.UnreadOnly {
				q += ` AND ir.item_id IS NULL`
			}
			if p.ReadOnly {
				q += ` AND ir.item_id IS NOT NULL`
			}
			if p.FavoriteOnly {
				q += ` AND COALESCE(fb.is_favorite, false) = true`
			}
			if p.LaterOnly {
				q += ` AND EXISTS (
					SELECT 1 FROM item_laters il2
					WHERE il2.item_id = i.id AND il2.user_id = $1
				)`
			}
			return q
		}()+
		orderBy+` LIMIT `+limitArg+` OFFSET `+offsetArg,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items, err := scanItems(rows)
	if err != nil {
		return nil, err
	}
	return &model.ItemListResponse{
		Items:    items,
		Page:     p.Page,
		PageSize: p.PageSize,
		Total:    total,
		HasNext:  offset+len(items) < total,
		Sort:     p.Sort,
		Status:   p.Status,
		SourceID: p.SourceID,
	}, nil
}

func (r *ItemRepo) MarkReadBulk(ctx context.Context, userID string, p BulkMarkReadParams) (int, error) {
	where := ` FROM items i
		JOIN sources s ON s.id = i.source_id
		LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
		LEFT JOIN item_feedbacks fb ON fb.item_id = i.id AND fb.user_id = $1
		WHERE s.user_id = $1`
	args := []any{userID}
	if p.Status != nil {
		args = append(args, *p.Status)
		where += ` AND i.status = $` + itoa(len(args))
	}
	if p.SourceID != nil {
		args = append(args, *p.SourceID)
		where += ` AND i.source_id = $` + itoa(len(args))
	}
	if p.Topic != nil && *p.Topic != "" {
		args = append(args, *p.Topic)
		where += ` AND EXISTS (
			SELECT 1 FROM item_summaries smt
			WHERE smt.item_id = i.id
			  AND COALESCE(smt.topics, '{}'::text[]) @> ARRAY[$` + itoa(len(args)) + `::text]
		)`
	}
	if p.UnreadOnly {
		where += ` AND ir.item_id IS NULL`
	}
	if p.ReadOnly {
		where += ` AND ir.item_id IS NOT NULL`
	}
	if p.FavoriteOnly {
		where += ` AND COALESCE(fb.is_favorite, false) = true`
	}
	if p.LaterOnly {
		where += ` AND EXISTS (
			SELECT 1 FROM item_laters il2
			WHERE il2.item_id = i.id AND il2.user_id = $1
		)`
	}
	if p.OlderThanDays != nil && *p.OlderThanDays > 0 {
		args = append(args, *p.OlderThanDays)
		where += ` AND COALESCE(i.published_at, i.created_at) < (NOW() - ($` + itoa(len(args)) + `::int * INTERVAL '1 day'))`
	}

	var inserted int
	err := r.db.QueryRow(ctx, `
		WITH target_items AS (
			SELECT i.id
			`+where+`
		), inserted_rows AS (
			INSERT INTO item_reads (user_id, item_id, read_at)
			SELECT $1, t.id, NOW()
			FROM target_items t
			ON CONFLICT (user_id, item_id) DO NOTHING
			RETURNING 1
		)
		SELECT COUNT(*)::int FROM inserted_rows
	`, args...).Scan(&inserted)
	return inserted, err
}

func (r *ItemRepo) MarkReadBulkByIDs(ctx context.Context, userID string, itemIDs []string) (int, error) {
	if len(itemIDs) == 0 {
		return 0, nil
	}
	unique := make([]string, 0, len(itemIDs))
	seen := make(map[string]struct{}, len(itemIDs))
	for _, itemID := range itemIDs {
		itemID = strings.TrimSpace(itemID)
		if itemID == "" {
			continue
		}
		if _, ok := seen[itemID]; ok {
			continue
		}
		seen[itemID] = struct{}{}
		unique = append(unique, itemID)
	}
	if len(unique) == 0 {
		return 0, nil
	}

	var inserted int
	err := r.db.QueryRow(ctx, `
		WITH target_items AS (
			SELECT i.id
			FROM items i
			JOIN sources s ON s.id = i.source_id
			WHERE s.user_id = $1
			  AND i.deleted_at IS NULL
			  AND i.id = ANY($2::uuid[])
		), inserted_rows AS (
			INSERT INTO item_reads (user_id, item_id, read_at)
			SELECT $1, t.id, NOW()
			FROM target_items t
			ON CONFLICT (user_id, item_id) DO NOTHING
			RETURNING item_id
		), deleted_laters AS (
			DELETE FROM item_laters il
			USING target_items t
			WHERE il.user_id = $1
			  AND il.item_id = t.id
		)
		SELECT COUNT(*)::int FROM inserted_rows
	`, userID, unique).Scan(&inserted)
	return inserted, err
}

func (r *ItemRepo) GetDetail(ctx context.Context, id, userID string) (*model.ItemDetail, error) {
	d, err := r.loadItemDetailBase(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	_ = r.loadFactsDetail(ctx, id, d)
	_ = r.loadSummaryDetail(ctx, id, d)

	fb, err := r.GetFeedback(ctx, userID, id)
	if err == nil {
		d.Feedback = fb
	}
	if note, highlights, err := r.GetByItem(ctx, userID, id); err == nil {
		d.Note = note
		d.Highlights = highlights
	}
	if d.Status == "summarized" && (len(d.FactsExecutions) == 0 || len(d.SummaryExecutions) == 0) {
		log.Printf(
			"item detail executions missing item_id=%s facts_exec=%d summary_exec=%d has_facts=%t has_summary=%t",
			id,
			len(d.FactsExecutions),
			len(d.SummaryExecutions),
			d.Facts != nil && len(d.Facts.Facts) > 0,
			d.Summary != nil,
		)
	}

	return d, nil
}

func (r *ItemRepo) GetFeedback(ctx context.Context, userID, itemID string) (*model.ItemFeedback, error) {
	var fb model.ItemFeedback
	err := r.db.QueryRow(ctx, `
		SELECT user_id, item_id, rating, is_favorite, updated_at
		FROM item_feedbacks
		WHERE user_id = $1 AND item_id = $2`,
		userID, itemID,
	).Scan(&fb.UserID, &fb.ItemID, &fb.Rating, &fb.IsFavorite, &fb.UpdatedAt)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &fb, nil
}

func (r *ItemRepo) UpsertFeedback(ctx context.Context, userID, itemID string, rating int, isFavorite bool) (*model.ItemFeedback, error) {
	if err := r.ensureOwned(ctx, userID, itemID); err != nil {
		return nil, err
	}
	var fb model.ItemFeedback
	err := r.db.QueryRow(ctx, `
		INSERT INTO item_feedbacks (user_id, item_id, rating, is_favorite)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, item_id) DO UPDATE SET
		  rating = EXCLUDED.rating,
		  is_favorite = EXCLUDED.is_favorite,
		  updated_at = NOW()
		RETURNING user_id, item_id, rating, is_favorite, updated_at`,
		userID, itemID, rating, isFavorite,
	).Scan(&fb.UserID, &fb.ItemID, &fb.Rating, &fb.IsFavorite, &fb.UpdatedAt)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &fb, nil
}

func (r *ItemRepo) MarkRead(ctx context.Context, userID, itemID string) (bool, error) {
	if err := r.ensureOwned(ctx, userID, itemID); err != nil {
		return false, err
	}
	var inserted int
	err := r.db.QueryRow(ctx, `
		INSERT INTO item_reads (user_id, item_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, item_id) DO NOTHING
		RETURNING 1`,
		userID, itemID,
	).Scan(&inserted)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			_, _ = r.db.Exec(ctx, `DELETE FROM item_laters WHERE user_id = $1 AND item_id = $2`, userID, itemID)
			return false, nil
		}
		return false, err
	}
	_, _ = r.db.Exec(ctx, `DELETE FROM item_laters WHERE user_id = $1 AND item_id = $2`, userID, itemID)
	return true, nil
}

func (r *ItemRepo) MarkUnread(ctx context.Context, userID, itemID string) error {
	if err := r.ensureOwned(ctx, userID, itemID); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx, `DELETE FROM item_reads WHERE user_id = $1 AND item_id = $2`, userID, itemID)
	return err
}

func (r *ItemRepo) MarkLater(ctx context.Context, userID, itemID string) error {
	if err := r.ensureOwned(ctx, userID, itemID); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO item_laters (user_id, item_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, item_id) DO UPDATE
		SET updated_at = NOW()`,
		userID, itemID,
	)
	return err
}

func (r *ItemRepo) MarkLaterBulk(ctx context.Context, userID string, itemIDs []string) (int, error) {
	if len(itemIDs) == 0 {
		return 0, nil
	}
	unique := make([]string, 0, len(itemIDs))
	seen := make(map[string]struct{}, len(itemIDs))
	for _, itemID := range itemIDs {
		itemID = strings.TrimSpace(itemID)
		if itemID == "" {
			continue
		}
		if _, ok := seen[itemID]; ok {
			continue
		}
		seen[itemID] = struct{}{}
		unique = append(unique, itemID)
	}
	if len(unique) == 0 {
		return 0, nil
	}

	tag, err := r.db.Exec(ctx, `
		INSERT INTO item_laters (user_id, item_id)
		SELECT $1, i.id
		FROM items i
		JOIN sources s ON s.id = i.source_id
		WHERE s.user_id = $1
		  AND i.deleted_at IS NULL
		  AND i.id = ANY($2::uuid[])
		ON CONFLICT (user_id, item_id) DO UPDATE
		SET updated_at = NOW()`,
		userID, unique,
	)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func (r *ItemRepo) UnmarkLater(ctx context.Context, userID, itemID string) error {
	if err := r.ensureOwned(ctx, userID, itemID); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx, `DELETE FROM item_laters WHERE user_id = $1 AND item_id = $2`, userID, itemID)
	return err
}

func (r *ItemRepo) ensureOwned(ctx context.Context, userID, itemID string) error {
	state, err := r.ownedItemState(ctx, userID, itemID)
	if err != nil {
		return err
	}
	return errForOwnedItemState(state)
}

func (r *ItemRepo) ownedItemState(ctx context.Context, userID, itemID string) (ownedItemState, error) {
	var deleted bool
	err := r.db.QueryRow(ctx, `
		SELECT i.deleted_at IS NOT NULL
		FROM items i
		JOIN sources s ON s.id = i.source_id
		WHERE i.id = $1 AND s.user_id = $2`,
		itemID, userID,
	).Scan(&deleted)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ownedItemMissing, nil
		}
		return ownedItemMissing, err
	}
	if deleted {
		return ownedItemDeleted, nil
	}
	return ownedItemActive, nil
}

func errForOwnedItemState(state ownedItemState) error {
	switch state {
	case ownedItemActive:
		return nil
	case ownedItemDeleted:
		return ErrConflict
	default:
		return ErrNotFound
	}
}

func errForRestoreOwnedItemState(state ownedItemState) error {
	switch state {
	case ownedItemDeleted:
		return nil
	case ownedItemActive:
		return ErrConflict
	default:
		return ErrNotFound
	}
}

func (r *ItemRepo) UpsertFromFeed(ctx context.Context, sourceID, url string, title *string) (string, bool, error) {
	var id string
	var created bool
	err := r.db.QueryRow(ctx, `
		INSERT INTO items (source_id, url, title)
		VALUES ($1, $2, $3)
		ON CONFLICT (source_id, url) DO NOTHING
		RETURNING id, true`,
		sourceID, url, title,
	).Scan(&id, &created)
	if err != nil {
		err2 := r.db.QueryRow(ctx, `SELECT id FROM items WHERE source_id = $1 AND url = $2`, sourceID, url).Scan(&id)
		return id, false, err2
	}
	return id, true, nil
}

func (r *ItemRepo) Delete(ctx context.Context, itemID, userID string) error {
	if err := r.ensureOwned(ctx, userID, itemID); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx, `
		UPDATE items
		SET deleted_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1`, itemID)
	return err
}

func (r *ItemRepo) Restore(ctx context.Context, itemID, userID string) error {
	state, err := r.ownedItemState(ctx, userID, itemID)
	if err != nil {
		return err
	}
	if err := errForRestoreOwnedItemState(state); err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
		UPDATE items
		SET deleted_at = NULL,
		    updated_at = NOW()
		WHERE id = $1`, itemID)
	return err
}

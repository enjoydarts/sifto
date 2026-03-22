package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ItemSearchDocumentRepo struct{ db *pgxpool.Pool }

func NewItemSearchDocumentRepo(db *pgxpool.Pool) *ItemSearchDocumentRepo {
	return &ItemSearchDocumentRepo{db: db}
}

func (r *ItemSearchDocumentRepo) GetByItemID(ctx context.Context, itemID string) (*model.ItemSearchDocument, error) {
	docs, err := r.load(ctx, `AND i.id = $1`, itemID)
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return nil, pgx.ErrNoRows
	}
	return &docs[0], nil
}

func (r *ItemSearchDocumentRepo) ListSummarizedPage(ctx context.Context, offset, limit int) ([]model.ItemSearchDocument, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 5000 {
		limit = 5000
	}
	return r.load(ctx, `ORDER BY i.created_at DESC OFFSET $1 LIMIT $2`, offset, limit)
}

func (r *ItemSearchDocumentRepo) CountSummarized(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM items i
		JOIN item_summaries sm ON sm.item_id = i.id
		WHERE i.status = 'summarized'
	`).Scan(&count)
	return count, err
}

func (r *ItemSearchDocumentRepo) load(ctx context.Context, suffix string, args ...any) ([]model.ItemSearchDocument, error) {
	rows, err := r.db.Query(ctx, `
		SELECT i.id,
		       s.user_id::text,
		       i.source_id::text,
		       i.status,
		       i.deleted_at IS NOT NULL AS is_deleted,
		       EXISTS (
		           SELECT 1 FROM item_reads ir
		           WHERE ir.item_id = i.id AND ir.user_id = s.user_id
		       ) AS is_read,
		       EXISTS (
		           SELECT 1 FROM item_feedbacks fb
		           WHERE fb.item_id = i.id AND fb.user_id = s.user_id AND fb.is_favorite = true
		       ) AS is_favorite,
		       EXISTS (
		           SELECT 1 FROM item_laters il
		           WHERE il.item_id = i.id AND il.user_id = s.user_id
		       ) AS is_later,
		       COALESCE(i.title, '') AS title,
		       COALESCE(sm.translated_title, '') AS translated_title,
		       COALESCE(sm.summary, '') AS summary,
		       COALESCE(f.facts, '[]'::jsonb) AS facts,
		       COALESCE(i.content_text, '') AS content_text,
		       COALESCE(sm.topics, '{}'::text[]) AS topics,
		       i.published_at,
		       i.created_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		JOIN item_summaries sm ON sm.item_id = i.id
		LEFT JOIN item_facts f ON f.item_id = i.id
		WHERE i.status = 'summarized'
		`+suffix,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []model.ItemSearchDocument
	for rows.Next() {
		var doc model.ItemSearchDocument
		var facts []string
		if err := rows.Scan(
			&doc.ID,
			&doc.UserID,
			&doc.SourceID,
			&doc.Status,
			&doc.IsDeleted,
			&doc.IsRead,
			&doc.IsFavorite,
			&doc.IsLater,
			&doc.Title,
			&doc.TranslatedTitle,
			&doc.Summary,
			jsonStringArrayScanner{dst: &facts},
			&doc.ContentText,
			&doc.Topics,
			&doc.PublishedAt,
			&doc.CreatedAt,
		); err != nil {
			return nil, err
		}
		doc.FactsText = strings.TrimSpace(strings.Join(facts, "\n"))
		docs = append(docs, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(docs) == 0 && strings.Contains(suffix, "AND i.id") {
		return nil, pgx.ErrNoRows
	}
	return docs, nil
}

func IsMissingSearchDocument(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

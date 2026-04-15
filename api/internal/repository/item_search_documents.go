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

func itemSearchDocumentsQuery(suffix string) string {
	return `
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
		       COALESCE(n.content, '') AS note_text,
		       COALESCE(h.highlight_text, '') AS highlight_text,
		       COALESCE(i.content_text, '') AS content_text,
		       COALESCE(sm.topics, '{}'::text[]) AS topics,
		       ` + effectiveGenreExpr("i", "sm") + ` AS effective_genre,
		       i.published_at,
		       i.created_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		JOIN item_summaries sm ON sm.item_id = i.id
		LEFT JOIN item_facts f ON f.item_id = i.id
		LEFT JOIN item_notes n ON n.user_id = s.user_id::text AND n.item_id = i.id
		LEFT JOIN LATERAL (
			SELECT STRING_AGG(quote_text, E'\n' ORDER BY created_at DESC) AS highlight_text
			FROM (
				SELECT NULLIF(TRIM(ih.quote_text), '') AS quote_text, ih.created_at
				FROM item_highlights ih
				WHERE ih.user_id = s.user_id::text AND ih.item_id = i.id
			) filtered
			WHERE quote_text IS NOT NULL
		) h ON true
		WHERE i.status = 'summarized'
		` + suffix
}

func (r *ItemSearchDocumentRepo) load(ctx context.Context, suffix string, args ...any) ([]model.ItemSearchDocument, error) {
	rows, err := r.db.Query(ctx, itemSearchDocumentsQuery(suffix), args...)
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
			&doc.NoteText,
			&doc.HighlightText,
			&doc.ContentText,
			&doc.Topics,
			&doc.EffectiveGenre,
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

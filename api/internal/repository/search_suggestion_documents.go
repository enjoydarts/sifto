package repository

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SearchSuggestionDocumentRepo struct{ db *pgxpool.Pool }

func NewSearchSuggestionDocumentRepo(db *pgxpool.Pool) *SearchSuggestionDocumentRepo {
	return &SearchSuggestionDocumentRepo{db: db}
}

func SearchSuggestionArticleDocumentID(itemID string) string {
	return "article_" + strings.TrimSpace(itemID)
}

func SearchSuggestionSourceDocumentID(sourceID string) string {
	return "source_" + strings.TrimSpace(sourceID)
}

func SearchSuggestionTopicDocumentID(userID, topicKey string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(userID) + "|" + strings.TrimSpace(topicKey)))
	return "topic_" + hex.EncodeToString(sum[:])
}

func (r *SearchSuggestionDocumentRepo) GetArticleByItemID(ctx context.Context, itemID string) (*model.SearchSuggestionDocument, error) {
	rows, err := r.loadArticles(ctx, `AND i.id = $1`, `ORDER BY i.created_at DESC`, itemID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, pgx.ErrNoRows
	}
	return &rows[0], nil
}

func (r *SearchSuggestionDocumentRepo) ListArticlePage(ctx context.Context, offset, limit int) ([]model.SearchSuggestionDocument, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 200
	}
	if limit > 5000 {
		limit = 5000
	}
	return r.loadArticles(ctx, "", `ORDER BY i.created_at DESC OFFSET $1 LIMIT $2`, offset, limit)
}

func (r *SearchSuggestionDocumentRepo) GetSourceBySourceID(ctx context.Context, sourceID string) (*model.SearchSuggestionDocument, error) {
	rows, err := r.loadSources(ctx, `WHERE s.id = $1`, `ORDER BY s.created_at DESC`, sourceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, pgx.ErrNoRows
	}
	return &rows[0], nil
}

func (r *SearchSuggestionDocumentRepo) ListSourcePage(ctx context.Context, offset, limit int) ([]model.SearchSuggestionDocument, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 200
	}
	if limit > 5000 {
		limit = 5000
	}
	return r.loadSources(ctx, "", `ORDER BY s.created_at DESC OFFSET $1 LIMIT $2`, offset, limit)
}

func (r *SearchSuggestionDocumentRepo) ListTopicsByUser(ctx context.Context, userID string) ([]model.SearchSuggestionDocument, error) {
	return r.loadTopics(ctx, `AND s.user_id = $1`, `ORDER BY user_id, topic_key`, userID)
}

func (r *SearchSuggestionDocumentRepo) ListTopicPage(ctx context.Context, offset, limit int) ([]model.SearchSuggestionDocument, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 200
	}
	if limit > 5000 {
		limit = 5000
	}
	return r.loadTopics(ctx, "", `ORDER BY user_id, topic_key OFFSET $1 LIMIT $2`, offset, limit)
}

func (r *SearchSuggestionDocumentRepo) loadArticles(ctx context.Context, whereClause, orderClause string, args ...any) ([]model.SearchSuggestionDocument, error) {
	rows, err := r.db.Query(ctx, `
		SELECT i.id::text AS item_id,
		       s.user_id::text,
		       COALESCE(NULLIF(BTRIM(sm.translated_title), ''), NULLIF(BTRIM(i.title), ''), i.url) AS label,
		       LOWER(regexp_replace(COALESCE(NULLIF(BTRIM(sm.translated_title), ''), NULLIF(BTRIM(i.title), ''), i.url), '\s+', ' ', 'g')) AS normalized,
		       i.source_id::text AS source_id,
		       i.updated_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		JOIN item_summaries sm ON sm.item_id = i.id
		WHERE i.status = 'summarized'
		  AND i.deleted_at IS NULL
		`+whereClause+`
		`+orderClause,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	docs := make([]model.SearchSuggestionDocument, 0)
	for rows.Next() {
		var doc model.SearchSuggestionDocument
		doc.Kind = "article"
		doc.Score = 100
		if err := rows.Scan(
			&doc.ItemID,
			&doc.UserID,
			&doc.Label,
			&doc.Normalized,
			&doc.SourceID,
			&doc.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if doc.ItemID != nil {
			doc.ID = SearchSuggestionArticleDocumentID(*doc.ItemID)
		}
		doc.Label = strings.TrimSpace(doc.Label)
		doc.Normalized = strings.TrimSpace(doc.Normalized)
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

func (r *SearchSuggestionDocumentRepo) loadSources(ctx context.Context, whereClause, orderClause string, args ...any) ([]model.SearchSuggestionDocument, error) {
	rows, err := r.db.Query(ctx, `
		SELECT s.id::text AS source_id,
		       s.user_id::text,
		       COALESCE(NULLIF(BTRIM(s.title), ''), s.url) AS label,
		       LOWER(regexp_replace(COALESCE(NULLIF(BTRIM(s.title), ''), s.url), '\s+', ' ', 'g')) AS normalized,
		       (COUNT(i.id) FILTER (WHERE i.status = 'summarized' AND i.deleted_at IS NULL))::int AS article_count,
		       s.updated_at
		FROM sources s
		LEFT JOIN items i ON i.source_id = s.id
		`+whereClause+`
		GROUP BY s.id, s.user_id, s.title, s.url, s.updated_at
		`+orderClause,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	docs := make([]model.SearchSuggestionDocument, 0)
	for rows.Next() {
		var doc model.SearchSuggestionDocument
		doc.Kind = "source"
		doc.Score = 120
		if err := rows.Scan(
			&doc.SourceID,
			&doc.UserID,
			&doc.Label,
			&doc.Normalized,
			&doc.ArticleCount,
			&doc.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if doc.SourceID != nil {
			doc.ID = SearchSuggestionSourceDocumentID(*doc.SourceID)
		}
		doc.Label = strings.TrimSpace(doc.Label)
		doc.Normalized = strings.TrimSpace(doc.Normalized)
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

func (r *SearchSuggestionDocumentRepo) loadTopics(ctx context.Context, whereClause, orderClause string, args ...any) ([]model.SearchSuggestionDocument, error) {
	rows, err := r.db.Query(ctx, `
		WITH topic_rows AS (
			SELECT s.user_id::text AS user_id,
			       LOWER(regexp_replace(BTRIM(t.topic), '\s+', ' ', 'g')) AS topic_key,
			       MIN(BTRIM(t.topic)) AS label,
			       COUNT(DISTINCT i.id)::int AS article_count,
			       MAX(i.updated_at) AS updated_at
			FROM items i
			JOIN sources s ON s.id = i.source_id
			JOIN item_summaries sm ON sm.item_id = i.id
			CROSS JOIN LATERAL unnest(sm.topics) AS t(topic)
			WHERE i.status = 'summarized'
			  AND i.deleted_at IS NULL
			  AND NULLIF(BTRIM(t.topic), '') IS NOT NULL
			`+whereClause+`
			GROUP BY s.user_id, LOWER(regexp_replace(BTRIM(t.topic), '\s+', ' ', 'g'))
		)
		SELECT user_id,
		       label,
		       topic_key AS normalized,
		       label AS topic,
		       article_count,
		       updated_at
		FROM topic_rows
		`+orderClause,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	docs := make([]model.SearchSuggestionDocument, 0)
	for rows.Next() {
		var doc model.SearchSuggestionDocument
		doc.Kind = "topic"
		doc.Score = 110
		var topicKey string
		if err := rows.Scan(
			&doc.UserID,
			&doc.Label,
			&topicKey,
			&doc.Topic,
			&doc.ArticleCount,
			&doc.UpdatedAt,
		); err != nil {
			return nil, err
		}
		doc.Normalized = strings.TrimSpace(topicKey)
		doc.ID = SearchSuggestionTopicDocumentID(doc.UserID, doc.Normalized)
		doc.Label = strings.TrimSpace(doc.Label)
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

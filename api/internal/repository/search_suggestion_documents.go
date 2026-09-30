package repository

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/topiccatalog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SearchSuggestionDocumentRepo struct{ db *pgxpool.Pool }

// The shared catalog currently has 48 topics. Keep a hard upper bound if it
// grows, so refreshing suggestions cannot become an unbounded index job.
const maxSearchSuggestionTopicsPerUser = 100

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

func (r *SearchSuggestionDocumentRepo) ListTopicUserIDs(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT s.user_id::text
		FROM items i
		JOIN sources s ON s.id = i.source_id
		JOIN item_summaries sm ON sm.item_id = i.id
		WHERE i.status = 'summarized' AND i.deleted_at IS NULL
		ORDER BY 1
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	userIDs := []string{}
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs, rows.Err()
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
	topicMapJSON, genreMapJSON, err := topiccatalog.MappingsJSON()
	if err != nil {
		return nil, err
	}
	topicMapArg := itoa(len(args) + 1)
	genreMapArg := itoa(len(args) + 2)
	args = append(args, topicMapJSON, genreMapJSON)
	rows, err := r.db.Query(ctx, `
		WITH topic_map AS (
			SELECT key, value AS canonical FROM jsonb_each_text($`+topicMapArg+`::jsonb)
		), genre_map AS (
			SELECT key, value AS canonical FROM jsonb_each_text($`+genreMapArg+`::jsonb)
		), article_rows AS (
			SELECT i.id AS item_id, s.user_id::text AS user_id,
			       sm.topics, sm.genre, i.updated_at
			FROM items i
			JOIN sources s ON s.id = i.source_id
			JOIN item_summaries sm ON sm.item_id = i.id
			WHERE i.status = 'summarized'
			  AND i.deleted_at IS NULL
			`+whereClause+`
		), topic_items AS (
			SELECT a.item_id, a.user_id, m.canonical AS label, a.updated_at
			FROM article_rows a
			CROSS JOIN LATERAL unnest(a.topics) AS t(topic)
			JOIN topic_map m ON m.key = LOWER(regexp_replace(BTRIM(t.topic), '\s+', ' ', 'g'))
			UNION ALL
			SELECT a.item_id, a.user_id, g.canonical AS label, a.updated_at
			FROM article_rows a
			JOIN genre_map g ON g.key = a.genre
		), topic_rows AS (
			SELECT user_id, LOWER(label) AS topic_key, label,
			       COUNT(DISTINCT item_id)::int AS article_count,
			       MAX(updated_at) AS updated_at
			FROM topic_items
			GROUP BY user_id, label
		), ranked_topics AS (
			SELECT *, ROW_NUMBER() OVER (
				PARTITION BY user_id ORDER BY article_count DESC, topic_key
			) AS topic_rank
			FROM topic_rows
		)
		SELECT user_id,
		       label,
		       topic_key AS normalized,
		       label AS topic,
		       article_count,
		       updated_at
		FROM ranked_topics
		WHERE topic_rank <= `+itoa(maxSearchSuggestionTopicsPerUser)+`
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

package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minoru-kitayama/sifto/api/internal/model"
)

type ItemInngestRepo struct{ db *pgxpool.Pool }

func NewItemInngestRepo(db *pgxpool.Pool) *ItemInngestRepo { return &ItemInngestRepo{db} }

func (r *ItemInngestRepo) UpdateAfterExtract(ctx context.Context, id, contentText string, title *string, publishedAt *time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE items
		SET content_text = $1, title = COALESCE($2, title), published_at = $3,
		    status = 'fetched', fetched_at = NOW(), updated_at = NOW()
		WHERE id = $4`,
		contentText, title, publishedAt, id)
	return err
}

func (r *ItemInngestRepo) InsertFacts(ctx context.Context, itemID string, facts []string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO item_facts (item_id, facts)
		VALUES ($1, $2)
		ON CONFLICT (item_id) DO UPDATE SET facts = EXCLUDED.facts, extracted_at = NOW()`,
		itemID, facts)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
		UPDATE items SET status = 'facts_extracted', updated_at = NOW() WHERE id = $1`, itemID)
	return err
}

func (r *ItemInngestRepo) InsertSummary(ctx context.Context, itemID, summary string, topics []string, score float64) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO item_summaries (item_id, summary, topics, score)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (item_id) DO UPDATE SET
		    summary = EXCLUDED.summary, topics = EXCLUDED.topics,
		    score = EXCLUDED.score, summarized_at = NOW()`,
		itemID, summary, topics, score)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
		UPDATE items SET status = 'summarized', updated_at = NOW() WHERE id = $1`, itemID)
	return err
}

func (r *ItemInngestRepo) MarkFailed(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE items SET status = 'failed', updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (r *ItemInngestRepo) ListSummarizedForUser(ctx context.Context, userID string, since, until time.Time) ([]model.DigestItemDetail, error) {
	rows, err := r.db.Query(ctx, `
		SELECT i.id, i.source_id, i.url, i.title, i.content_text, i.status,
		       i.published_at, i.fetched_at, i.created_at, i.updated_at,
		       s.id, s.item_id, s.summary, s.topics, s.score, s.summarized_at
		FROM items i
		JOIN sources src ON src.id = i.source_id
		JOIN item_summaries s ON s.item_id = i.id
		WHERE src.user_id = $1
		  AND i.status = 'summarized'
		  AND s.summarized_at >= $2
		  AND s.summarized_at < $3
		ORDER BY s.score DESC NULLS LAST`,
		userID, since, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.DigestItemDetail
	rank := 1
	for rows.Next() {
		var d model.DigestItemDetail
		if err := rows.Scan(
			&d.Item.ID, &d.Item.SourceID, &d.Item.URL, &d.Item.Title,
			&d.Item.ContentText, &d.Item.Status, &d.Item.PublishedAt,
			&d.Item.FetchedAt, &d.Item.CreatedAt, &d.Item.UpdatedAt,
			&d.Summary.ID, &d.Summary.ItemID, &d.Summary.Summary,
			&d.Summary.Topics, &d.Summary.Score, &d.Summary.SummarizedAt,
		); err != nil {
			return nil, err
		}
		d.Rank = rank
		rank++
		items = append(items, d)
	}
	return items, nil
}

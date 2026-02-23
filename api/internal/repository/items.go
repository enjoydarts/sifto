package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minoru-kitayama/sifto/api/internal/model"
)

type ItemRepo struct{ db *pgxpool.Pool }

func NewItemRepo(db *pgxpool.Pool) *ItemRepo { return &ItemRepo{db} }

func (r *ItemRepo) List(ctx context.Context, userID string, status, sourceID *string) ([]model.Item, error) {
	query := `
		SELECT i.id, i.source_id, i.url, i.title, i.content_text, i.status,
		       i.published_at, i.fetched_at, i.created_at, i.updated_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		WHERE s.user_id = $1`
	args := []any{userID}

	if status != nil {
		args = append(args, *status)
		query += ` AND i.status = $` + itoa(len(args))
	}
	if sourceID != nil {
		args = append(args, *sourceID)
		query += ` AND i.source_id = $` + itoa(len(args))
	}
	query += ` ORDER BY i.created_at DESC LIMIT 100`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.Item
	for rows.Next() {
		var it model.Item
		if err := rows.Scan(&it.ID, &it.SourceID, &it.URL, &it.Title, &it.ContentText,
			&it.Status, &it.PublishedAt, &it.FetchedAt, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, nil
}

func (r *ItemRepo) GetDetail(ctx context.Context, id, userID string) (*model.ItemDetail, error) {
	var d model.ItemDetail
	err := r.db.QueryRow(ctx, `
		SELECT i.id, i.source_id, i.url, i.title, i.content_text, i.status,
		       i.published_at, i.fetched_at, i.created_at, i.updated_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		WHERE i.id = $1 AND s.user_id = $2`, id, userID,
	).Scan(&d.ID, &d.SourceID, &d.URL, &d.Title, &d.ContentText,
		&d.Status, &d.PublishedAt, &d.FetchedAt, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, err
	}

	// facts
	var f model.ItemFacts
	err = r.db.QueryRow(ctx, `
		SELECT id, item_id, facts, extracted_at FROM item_facts WHERE item_id = $1`, id,
	).Scan(&f.ID, &f.ItemID, &f.Facts, &f.ExtractedAt)
	if err == nil {
		d.Facts = &f
	}

	// summary
	var s model.ItemSummary
	err = r.db.QueryRow(ctx, `
		SELECT id, item_id, summary, topics, score, summarized_at FROM item_summaries WHERE item_id = $1`, id,
	).Scan(&s.ID, &s.ItemID, &s.Summary, &s.Topics, &s.Score, &s.SummarizedAt)
	if err == nil {
		d.Summary = &s
	}

	return &d, nil
}

func (r *ItemRepo) UpsertFromFeed(ctx context.Context, sourceID, url string, title *string) (string, bool, error) {
	var id string
	var created bool
	err := r.db.QueryRow(ctx, `
		INSERT INTO items (source_id, url, title)
		VALUES ($1, $2, $3)
		ON CONFLICT (url) DO NOTHING
		RETURNING id, true`,
		sourceID, url, title,
	).Scan(&id, &created)
	if err != nil {
		// conflict - already exists
		err2 := r.db.QueryRow(ctx, `SELECT id FROM items WHERE url = $1`, url).Scan(&id)
		return id, false, err2
	}
	return id, true, nil
}

func itoa(n int) string {
	return string(rune('0' + n))
}

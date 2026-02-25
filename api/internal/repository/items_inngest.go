package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minoru-kitayama/sifto/api/internal/model"
)

type ItemInngestRepo struct{ db *pgxpool.Pool }

func NewItemInngestRepo(db *pgxpool.Pool) *ItemInngestRepo { return &ItemInngestRepo{db} }

type ItemEmbeddingCandidate struct {
	ItemID   string
	SourceID string
	UserID   string
	Title    *string
	Summary  string
	Topics   []string
	Facts    []string
}

type ItemEmbeddingBackfillTarget struct {
	ItemID   string
	SourceID string
	UserID   string
	URL      string
}

func (r *ItemInngestRepo) UpdateAfterExtract(ctx context.Context, id, contentText string, title, thumbnailURL *string, publishedAt *time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE items
		SET content_text = $1, title = COALESCE($2, title), thumbnail_url = COALESCE($3, thumbnail_url), published_at = $4,
		    status = 'fetched', fetched_at = NOW(), updated_at = NOW()
		WHERE id = $5`,
		contentText, title, thumbnailURL, publishedAt, id)
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

func (r *ItemInngestRepo) InsertSummary(ctx context.Context, itemID, summary string, topics []string, score float64, scoreBreakdown map[string]any, scoreReason, scorePolicyVersion string) error {
	var scoreBreakdownJSON []byte
	if len(scoreBreakdown) > 0 {
		b, err := json.Marshal(scoreBreakdown)
		if err != nil {
			return err
		}
		scoreBreakdownJSON = b
	}
	var scoreReasonPtr *string
	if scoreReason != "" {
		scoreReasonPtr = &scoreReason
	}
	var scorePolicyVersionPtr *string
	if scorePolicyVersion != "" {
		scorePolicyVersionPtr = &scorePolicyVersion
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO item_summaries (item_id, summary, topics, score, score_breakdown, score_reason, score_policy_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (item_id) DO UPDATE SET
		    summary = EXCLUDED.summary, topics = EXCLUDED.topics,
		    score = EXCLUDED.score,
		    score_breakdown = EXCLUDED.score_breakdown,
		    score_reason = EXCLUDED.score_reason,
		    score_policy_version = EXCLUDED.score_policy_version,
		    summarized_at = NOW()`,
		itemID, summary, topics, score, scoreBreakdownJSON, scoreReasonPtr, scorePolicyVersionPtr)
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

func (r *ItemInngestRepo) UpsertEmbedding(ctx context.Context, itemID, model string, embedding []float64) error {
	if len(embedding) == 0 {
		return nil
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO item_embeddings (item_id, model, dimensions, embedding)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (item_id) DO UPDATE SET
		    model = EXCLUDED.model,
		    dimensions = EXCLUDED.dimensions,
		    embedding = EXCLUDED.embedding,
		    updated_at = NOW()`,
		itemID, model, len(embedding), embedding)
	return err
}

func (r *ItemInngestRepo) GetEmbeddingCandidate(ctx context.Context, itemID string) (*ItemEmbeddingCandidate, error) {
	var v ItemEmbeddingCandidate
	err := r.db.QueryRow(ctx, `
		SELECT i.id, i.source_id, src.user_id, i.title,
		       sm.summary, COALESCE(sm.topics, '{}'::text[]),
		       COALESCE(f.facts, '[]'::jsonb)
		FROM items i
		JOIN sources src ON src.id = i.source_id
		JOIN item_summaries sm ON sm.item_id = i.id
		LEFT JOIN item_facts f ON f.item_id = i.id
		WHERE i.id = $1
		  AND i.status = 'summarized'`, itemID).
		Scan(&v.ItemID, &v.SourceID, &v.UserID, &v.Title, &v.Summary, &v.Topics, jsonStringArrayScanner{dst: &v.Facts})
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *ItemInngestRepo) ListEmbeddingBackfillTargets(ctx context.Context, userID *string, limit int) ([]ItemEmbeddingBackfillTarget, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	query := `
		SELECT i.id, i.source_id, src.user_id, i.url
		FROM items i
		JOIN sources src ON src.id = i.source_id
		JOIN item_summaries sm ON sm.item_id = i.id
		LEFT JOIN item_embeddings ie ON ie.item_id = i.id
		WHERE i.status = 'summarized'
		  AND ie.item_id IS NULL`
	args := []any{}
	if userID != nil && *userID != "" {
		args = append(args, *userID)
		query += ` AND src.user_id = $1`
	}
	args = append(args, limit)
	query += ` ORDER BY sm.summarized_at DESC LIMIT $` + strconv.Itoa(len(args))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ItemEmbeddingBackfillTarget
	for rows.Next() {
		var v ItemEmbeddingBackfillTarget
		if err := rows.Scan(&v.ItemID, &v.SourceID, &v.UserID, &v.URL); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *ItemInngestRepo) ListSummarizedForUser(ctx context.Context, userID string, since, until time.Time) ([]model.DigestItemDetail, error) {
	rows, err := r.db.Query(ctx, `
			SELECT i.id, i.source_id, i.url, i.title, i.thumbnail_url, i.content_text, i.status,
			       COALESCE(fb.is_favorite, false) AS is_favorite,
			       COALESCE(fb.rating, 0) AS feedback_rating,
			       i.published_at, i.fetched_at, i.created_at, i.updated_at,
			       s.id, s.item_id, s.summary, s.topics, s.score,
			       s.score_breakdown, s.score_reason, s.score_policy_version, s.summarized_at,
			       COALESCE(f.facts, '[]'::jsonb) AS facts
			FROM items i
			JOIN sources src ON src.id = i.source_id
			JOIN item_summaries s ON s.item_id = i.id
			LEFT JOIN item_facts f ON f.item_id = i.id
			LEFT JOIN item_feedbacks fb ON fb.item_id = i.id AND fb.user_id = $1
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
	for rows.Next() {
		var d model.DigestItemDetail
		if err := rows.Scan(
			&d.Item.ID, &d.Item.SourceID, &d.Item.URL, &d.Item.Title, &d.Item.ThumbnailURL,
			&d.Item.ContentText, &d.Item.Status, &d.Item.IsFavorite, &d.Item.FeedbackRating, &d.Item.PublishedAt,
			&d.Item.FetchedAt, &d.Item.CreatedAt, &d.Item.UpdatedAt,
			&d.Summary.ID, &d.Summary.ItemID, &d.Summary.Summary,
			&d.Summary.Topics, &d.Summary.Score, scoreBreakdownScanner{dst: &d.Summary.ScoreBreakdown},
			&d.Summary.ScoreReason, &d.Summary.ScorePolicyVersion, &d.Summary.SummarizedAt,
			jsonStringArrayScanner{dst: &d.Facts},
		); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	profile, err := loadFeedbackPreferenceProfile(ctx, r.db, userID)
	if err != nil {
		return nil, err
	}
	itemIDs := make([]string, 0, len(items))
	for _, it := range items {
		itemIDs = append(itemIDs, it.Item.ID)
	}
	embeddingBiasByItemID, err := loadEmbeddingBiasByItemID(ctx, r.db, itemIDs, profile)
	if err != nil {
		return nil, err
	}
	sortDigestItemsByPreference(items, profile, embeddingBiasByItemID)
	for i := range items {
		items[i].Rank = i + 1
	}
	return items, nil
}

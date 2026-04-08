package repository

import (
	"context"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

type personalScoreRow struct {
	ItemID         string
	SourceID       string
	SummaryScore   *float64
	ScoreBreakdown *model.ItemSummaryScoreBreakdown
	Topics         []string
	PublishedAt    *time.Time
	FetchedAt      *time.Time
	CreatedAt      time.Time
}

func (r *ItemRepo) RefreshRecentPersonalScores(ctx context.Context, userID string, limit int) error {
	if limit <= 0 {
		limit = 1200
	}
	if limit > 4000 {
		limit = 4000
	}
	rows, err := r.db.Query(ctx, `
		SELECT i.id
		FROM items i
		JOIN sources s ON s.id = i.source_id
		JOIN item_summaries sm ON sm.item_id = i.id
		WHERE s.user_id = $1
		  AND i.deleted_at IS NULL
		  AND i.status = 'summarized'
		ORDER BY COALESCE(i.published_at, i.fetched_at, i.created_at) DESC, i.created_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return err
	}
	defer rows.Close()

	itemIDs := make([]string, 0, limit)
	for rows.Next() {
		var itemID string
		if err := rows.Scan(&itemID); err != nil {
			return err
		}
		itemIDs = append(itemIDs, itemID)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return r.PersistPersonalScores(ctx, userID, itemIDs)
}

func (r *ItemRepo) PersistPersonalScores(ctx context.Context, userID string, itemIDs []string) error {
	if len(itemIDs) == 0 {
		return nil
	}

	prefRepo := NewPreferenceProfileRepo(r.db)
	profile, err := prefRepo.GetProfile(ctx, userID)
	if err != nil && err != ErrNotFound {
		return err
	}
	if err == ErrNotFound {
		profile = nil
	}

	rows, err := r.db.Query(ctx, `
		SELECT i.id, i.source_id, sm.score, sm.score_breakdown, COALESCE(sm.topics, '{}'::text[]),
		       i.published_at, i.fetched_at, i.created_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		JOIN item_summaries sm ON sm.item_id = i.id
		WHERE s.user_id = $1
		  AND i.deleted_at IS NULL
		  AND i.id = ANY($2::uuid[])`,
		userID, itemIDs,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	inputs := make([]personalScoreRow, 0, len(itemIDs))
	for rows.Next() {
		var row personalScoreRow
		if err := rows.Scan(
			&row.ItemID,
			&row.SourceID,
			&row.SummaryScore,
			scoreBreakdownScanner{dst: &row.ScoreBreakdown},
			&row.Topics,
			&row.PublishedAt,
			&row.FetchedAt,
			&row.CreatedAt,
		); err != nil {
			return err
		}
		inputs = append(inputs, row)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(inputs) == 0 {
		return nil
	}

	embeddings, err := loadItemEmbeddingsByID(ctx, r.db, itemIDs)
	if err != nil {
		return err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	for _, row := range inputs {
		result := CalcPersonalScoreDetailed(PersonalScoreInput{
			SummaryScore:   row.SummaryScore,
			ScoreBreakdown: row.ScoreBreakdown,
			Topics:         row.Topics,
			Embedding:      embeddings[row.ItemID],
			SourceID:       row.SourceID,
			PublishedAt:    row.PublishedAt,
			FetchedAt:      row.FetchedAt,
			CreatedAt:      row.CreatedAt,
		}, profile)
		if _, err := tx.Exec(ctx, `
			UPDATE item_summaries
			SET personal_score = $2,
			    personal_score_reason = $3
			WHERE item_id = $1`,
			row.ItemID, result.Score, result.Reason,
		); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *ItemInngestRepo) PersistPersonalScores(ctx context.Context, userID string, itemIDs []string) error {
	viewRepo := NewItemRepo(r.db)
	return viewRepo.PersistPersonalScores(ctx, userID, itemIDs)
}

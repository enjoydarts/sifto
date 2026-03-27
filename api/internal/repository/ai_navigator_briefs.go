package repository

import (
	"context"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AINavigatorBriefRepo struct{ db *pgxpool.Pool }

func NewAINavigatorBriefRepo(db *pgxpool.Pool) *AINavigatorBriefRepo {
	return &AINavigatorBriefRepo{db: db}
}

func (r *AINavigatorBriefRepo) CreateBrief(ctx context.Context, brief *model.AINavigatorBrief) error {
	if brief.ID == "" {
		brief.ID = uuid.NewString()
	}
	return r.db.QueryRow(ctx, `
		INSERT INTO ai_navigator_briefs (
			id, user_id, slot, status, title, intro, summary, ending, persona, model,
			source_window_start, source_window_end, generated_at, notification_sent_at, error_message
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15
		)
		RETURNING created_at, updated_at
	`,
		brief.ID,
		brief.UserID,
		brief.Slot,
		brief.Status,
		brief.Title,
		brief.Intro,
		brief.Summary,
		brief.Ending,
		brief.Persona,
		brief.Model,
		brief.SourceWindowStart,
		brief.SourceWindowEnd,
		brief.GeneratedAt,
		brief.NotificationSentAt,
		brief.ErrorMessage,
	).Scan(&brief.CreatedAt, &brief.UpdatedAt)
}

func (r *AINavigatorBriefRepo) AddBriefItems(ctx context.Context, briefID string, items []model.AINavigatorBriefItem) error {
	for i := range items {
		if items[i].ID == "" {
			items[i].ID = uuid.NewString()
		}
		if err := r.db.QueryRow(ctx, `
			INSERT INTO ai_navigator_brief_items (
				id, brief_id, rank, item_id, title_snapshot, translated_title_snapshot, source_title_snapshot, comment
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING created_at
		`,
			items[i].ID,
			briefID,
			items[i].Rank,
			items[i].ItemID,
			items[i].TitleSnapshot,
			items[i].TranslatedTitleSnapshot,
			items[i].SourceTitleSnapshot,
			items[i].Comment,
		).Scan(&items[i].CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

func (r *AINavigatorBriefRepo) MarkBriefFailed(ctx context.Context, briefID, errorMessage string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ai_navigator_briefs
		SET status = $2,
		    error_message = $3,
		    updated_at = NOW()
		WHERE id = $1
	`, briefID, model.AINavigatorBriefStatusFailed, errorMessage)
	return err
}

func (r *AINavigatorBriefRepo) MarkBriefNotified(ctx context.Context, briefID string, sentAt any) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ai_navigator_briefs
		SET status = $2,
		    notification_sent_at = $3,
		    updated_at = NOW()
		WHERE id = $1
	`, briefID, model.AINavigatorBriefStatusNotified, sentAt)
	return err
}

func scanAINavigatorBrief(row interface{ Scan(dest ...any) error }) (*model.AINavigatorBrief, error) {
	var brief model.AINavigatorBrief
	err := row.Scan(
		&brief.ID,
		&brief.UserID,
		&brief.Slot,
		&brief.Status,
		&brief.Title,
		&brief.Intro,
		&brief.Summary,
		&brief.Ending,
		&brief.Persona,
		&brief.Model,
		&brief.SourceWindowStart,
		&brief.SourceWindowEnd,
		&brief.GeneratedAt,
		&brief.NotificationSentAt,
		&brief.ErrorMessage,
		&brief.CreatedAt,
		&brief.UpdatedAt,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &brief, nil
}

func (r *AINavigatorBriefRepo) ListBriefsByUser(ctx context.Context, userID, slot string, limit int) ([]model.AINavigatorBrief, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, slot, status, title, intro, summary, ending, persona, model,
		       source_window_start, source_window_end, generated_at, notification_sent_at, error_message,
		       created_at, updated_at
		FROM ai_navigator_briefs
		WHERE user_id = $1
		  AND ($2 = '' OR slot = $2)
		ORDER BY COALESCE(generated_at, created_at) DESC, created_at DESC
		LIMIT $3
	`, userID, slot, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.AINavigatorBrief, 0, limit)
	for rows.Next() {
		brief, err := scanAINavigatorBrief(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *brief)
	}
	return out, rows.Err()
}

func (r *AINavigatorBriefRepo) GetBriefDetail(ctx context.Context, userID, briefID string) (*model.AINavigatorBrief, error) {
	brief, err := scanAINavigatorBrief(r.db.QueryRow(ctx, `
		SELECT id, user_id, slot, status, title, intro, summary, ending, persona, model,
		       source_window_start, source_window_end, generated_at, notification_sent_at, error_message,
		       created_at, updated_at
		FROM ai_navigator_briefs
		WHERE user_id = $1 AND id = $2
	`, userID, briefID))
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, brief_id, rank, item_id, title_snapshot, translated_title_snapshot, source_title_snapshot, comment, created_at
		FROM ai_navigator_brief_items
		WHERE brief_id = $1
		ORDER BY rank ASC
	`, briefID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]model.AINavigatorBriefItem, 0, 10)
	for rows.Next() {
		var item model.AINavigatorBriefItem
		if err := rows.Scan(
			&item.ID,
			&item.BriefID,
			&item.Rank,
			&item.ItemID,
			&item.TitleSnapshot,
			&item.TranslatedTitleSnapshot,
			&item.SourceTitleSnapshot,
			&item.Comment,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	brief.Items = items
	return brief, nil
}

func (r *AINavigatorBriefRepo) LatestBriefByUserSlot(ctx context.Context, userID, slot string) (*model.AINavigatorBrief, error) {
	brief, err := scanAINavigatorBrief(r.db.QueryRow(ctx, `
		SELECT id, user_id, slot, status, title, intro, summary, ending, persona, model,
		       source_window_start, source_window_end, generated_at, notification_sent_at, error_message,
		       created_at, updated_at
		FROM ai_navigator_briefs
		WHERE user_id = $1 AND slot = $2
		ORDER BY COALESCE(generated_at, created_at) DESC, created_at DESC
		LIMIT 1
	`, userID, slot))
	if err != nil {
		if err == ErrNotFound || err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return brief, nil
}

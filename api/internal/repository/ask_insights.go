package repository

import (
	"context"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AskInsightRepo struct{ db *pgxpool.Pool }

func NewAskInsightRepo(db *pgxpool.Pool) *AskInsightRepo { return &AskInsightRepo{db: db} }

func (r *AskInsightRepo) Save(ctx context.Context, insight model.AskInsight, itemIDs []string) (model.AskInsight, error) {
	if insight.ID == "" {
		insight.ID = uuid.NewString()
	}
	if insight.GoalID != nil && strings.TrimSpace(*insight.GoalID) != "" {
		var exists bool
		if err := r.db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM reading_goals
				WHERE id = $1::uuid AND user_id::text = $2
			)`, *insight.GoalID, insight.UserID,
		).Scan(&exists); err != nil {
			return model.AskInsight{}, err
		}
		if !exists {
			return model.AskInsight{}, ErrNotFound
		}
	}
	err := r.db.QueryRow(ctx, `
		INSERT INTO ask_insights (id, user_id, title, body, query, goal_id, tags)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at`,
		insight.ID, insight.UserID, insight.Title, insight.Body, insight.Query, insight.GoalID, insight.Tags,
	).Scan(&insight.CreatedAt, &insight.UpdatedAt)
	if err != nil {
		return model.AskInsight{}, err
	}
	seen := map[string]struct{}{}
	position := 0
	for _, itemID := range itemIDs {
		itemID = strings.TrimSpace(itemID)
		if itemID == "" {
			continue
		}
		if _, ok := seen[itemID]; ok {
			continue
		}
		seen[itemID] = struct{}{}
		if _, err := r.db.Exec(ctx, `
			INSERT INTO ask_insight_items (insight_id, item_id, position)
			SELECT $1, i.id, $3
			FROM items i
			JOIN sources s ON s.id = i.source_id
			WHERE i.id = $2::uuid AND s.user_id::text = $4`,
			insight.ID, itemID, position, insight.UserID,
		); err != nil {
			return model.AskInsight{}, err
		}
		position++
	}
	return r.GetByID(ctx, insight.UserID, insight.ID)
}

func (r *AskInsightRepo) GetByID(ctx context.Context, userID, id string) (model.AskInsight, error) {
	var insight model.AskInsight
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, title, body, query, goal_id, tags, created_at, updated_at
		FROM ask_insights
		WHERE user_id = $1 AND id = $2`, userID, id,
	).Scan(&insight.ID, &insight.UserID, &insight.Title, &insight.Body, &insight.Query, &insight.GoalID, &insight.Tags, &insight.CreatedAt, &insight.UpdatedAt)
	if err != nil {
		return model.AskInsight{}, mapDBError(err)
	}
	items, err := r.listInsightItems(ctx, userID, []string{id})
	if err != nil {
		return model.AskInsight{}, err
	}
	insight.Items = items[id]
	return insight, nil
}

func (r *AskInsightRepo) ListRecent(ctx context.Context, userID string, limit int) ([]model.AskInsight, error) {
	if limit <= 0 {
		limit = 8
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, title, body, query, goal_id, tags, created_at, updated_at
		FROM ask_insights
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`, userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.AskInsight{}
	ids := []string{}
	for rows.Next() {
		var insight model.AskInsight
		if err := rows.Scan(&insight.ID, &insight.UserID, &insight.Title, &insight.Body, &insight.Query, &insight.GoalID, &insight.Tags, &insight.CreatedAt, &insight.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, insight)
		ids = append(ids, insight.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	itemsByInsight, err := r.listInsightItems(ctx, userID, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Items = itemsByInsight[out[i].ID]
	}
	return out, nil
}

func (r *AskInsightRepo) listInsightItems(ctx context.Context, userID string, insightIDs []string) (map[string][]model.AskInsightItemRef, error) {
	out := map[string][]model.AskInsightItemRef{}
	if len(insightIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT aii.insight_id, i.id, COALESCE(sm.translated_title, i.title, i.url) AS title, i.url, COALESCE(sm.topics, '{}'::text[])
		FROM ask_insight_items aii
		JOIN ask_insights ai ON ai.id = aii.insight_id
		JOIN items i ON i.id = aii.item_id
		JOIN sources s ON s.id = i.source_id AND s.user_id::text = ai.user_id
		LEFT JOIN item_summaries sm ON sm.item_id = i.id
		WHERE ai.user_id = $1
		  AND aii.insight_id = ANY($2::uuid[])
		ORDER BY aii.position ASC`, userID, insightIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var insightID string
		var item model.AskInsightItemRef
		if err := rows.Scan(&insightID, &item.ItemID, &item.Title, &item.URL, &item.Topics); err != nil {
			return nil, err
		}
		out[insightID] = append(out[insightID], item)
	}
	return out, rows.Err()
}

func (r *AskInsightRepo) Delete(ctx context.Context, userID, id string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM ask_insights WHERE user_id = $1 AND id = $2`, userID, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

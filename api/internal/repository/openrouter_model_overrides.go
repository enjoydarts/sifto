package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OpenRouterModelOverrideRepo struct{ db *pgxpool.Pool }

func NewOpenRouterModelOverrideRepo(db *pgxpool.Pool) *OpenRouterModelOverrideRepo {
	return &OpenRouterModelOverrideRepo{db: db}
}

type OpenRouterModelOverride struct {
	ID                    string    `json:"id"`
	UserID                string    `json:"user_id"`
	ModelID               string    `json:"model_id"`
	AllowStructuredOutput bool      `json:"allow_structured_output"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

func (r *OpenRouterModelOverrideRepo) ListByUser(ctx context.Context, userID string) (map[string]OpenRouterModelOverride, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, model_id, allow_structured_output, created_at, updated_at
		FROM user_openrouter_model_overrides
		WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]OpenRouterModelOverride{}
	for rows.Next() {
		var v OpenRouterModelOverride
		if err := rows.Scan(&v.ID, &v.UserID, &v.ModelID, &v.AllowStructuredOutput, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out[v.ModelID] = v
	}
	return out, rows.Err()
}

func (r *OpenRouterModelOverrideRepo) Upsert(ctx context.Context, userID, modelID string, allowStructuredOutput bool) (*OpenRouterModelOverride, error) {
	var v OpenRouterModelOverride
	err := r.db.QueryRow(ctx, `
		INSERT INTO user_openrouter_model_overrides (user_id, model_id, allow_structured_output)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, model_id) DO UPDATE
		SET allow_structured_output = EXCLUDED.allow_structured_output,
		    updated_at = NOW()
		RETURNING id, user_id, model_id, allow_structured_output, created_at, updated_at`,
		userID, modelID, allowStructuredOutput,
	).Scan(&v.ID, &v.UserID, &v.ModelID, &v.AllowStructuredOutput, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *OpenRouterModelOverrideRepo) Delete(ctx context.Context, userID, modelID string) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM user_openrouter_model_overrides
		WHERE user_id = $1 AND model_id = $2`, userID, modelID)
	return err
}

func (r *OpenRouterModelOverrideRepo) GetByUserAndModelID(ctx context.Context, userID, modelID string) (*OpenRouterModelOverride, error) {
	var v OpenRouterModelOverride
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, model_id, allow_structured_output, created_at, updated_at
		FROM user_openrouter_model_overrides
		WHERE user_id = $1 AND model_id = $2`,
		userID, modelID,
	).Scan(&v.ID, &v.UserID, &v.ModelID, &v.AllowStructuredOutput, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &v, nil
}

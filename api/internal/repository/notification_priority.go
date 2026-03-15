package repository

import (
	"context"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NotificationPriorityRepo struct{ db *pgxpool.Pool }

func NewNotificationPriorityRepo(db *pgxpool.Pool) *NotificationPriorityRepo {
	return &NotificationPriorityRepo{db: db}
}

func (r *NotificationPriorityRepo) EnsureDefaults(ctx context.Context, userID string) (*model.NotificationPriorityRule, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO notification_priority_rules (id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO NOTHING`, uuid.NewString(), userID)
	if err != nil {
		return nil, err
	}
	return r.GetByUserID(ctx, userID)
}

func (r *NotificationPriorityRepo) GetByUserID(ctx context.Context, userID string) (*model.NotificationPriorityRule, error) {
	var v model.NotificationPriorityRule
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, sensitivity, daily_cap, theme_weight, immediate_enabled, briefing_enabled, review_enabled, goal_match_enabled, created_at, updated_at
		FROM notification_priority_rules
		WHERE user_id = $1`, userID,
	).Scan(&v.ID, &v.UserID, &v.Sensitivity, &v.DailyCap, &v.ThemeWeight, &v.ImmediateEnabled, &v.BriefingEnabled, &v.ReviewEnabled, &v.GoalMatchEnabled, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &v, nil
}

func (r *NotificationPriorityRepo) Upsert(ctx context.Context, userID, sensitivity string, dailyCap int, themeWeight float64, immediateEnabled, briefingEnabled, reviewEnabled, goalMatchEnabled bool) (*model.NotificationPriorityRule, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO notification_priority_rules (id, user_id, sensitivity, daily_cap, theme_weight, immediate_enabled, briefing_enabled, review_enabled, goal_match_enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (user_id) DO UPDATE
		SET sensitivity = EXCLUDED.sensitivity,
		    daily_cap = EXCLUDED.daily_cap,
		    theme_weight = EXCLUDED.theme_weight,
		    immediate_enabled = EXCLUDED.immediate_enabled,
		    briefing_enabled = EXCLUDED.briefing_enabled,
		    review_enabled = EXCLUDED.review_enabled,
		    goal_match_enabled = EXCLUDED.goal_match_enabled,
		    updated_at = NOW()`,
		uuid.NewString(), userID, sensitivity, dailyCap, themeWeight, immediateEnabled, briefingEnabled, reviewEnabled, goalMatchEnabled)
	if err != nil {
		return nil, err
	}
	return r.GetByUserID(ctx, userID)
}

package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minoru-kitayama/sifto/api/internal/model"
)

type UserSettingsRepo struct{ db *pgxpool.Pool }

func NewUserSettingsRepo(db *pgxpool.Pool) *UserSettingsRepo { return &UserSettingsRepo{db: db} }

type BudgetAlertTarget struct {
	UserID                  string
	Email                   string
	Name                    *string
	MonthlyBudgetUSD        float64
	BudgetAlertThresholdPct int
}

func (r *UserSettingsRepo) GetByUserID(ctx context.Context, userID string) (*model.UserSettings, error) {
	var v model.UserSettings
	var anthropicKeyEnc *string
	var openAIKeyEnc *string
	err := r.db.QueryRow(ctx, `
		SELECT user_id,
		       anthropic_api_key_enc,
		       anthropic_api_key_last4,
		       openai_api_key_enc,
		       openai_api_key_last4,
		       monthly_budget_usd,
		       budget_alert_enabled,
		       budget_alert_threshold_pct,
		       reading_plan_window,
		       reading_plan_size,
		       reading_plan_diversify_topics,
		       reading_plan_exclude_read,
		       created_at,
		       updated_at
		FROM user_settings
		WHERE user_id = $1`,
		userID,
	).Scan(
		&v.UserID,
		&anthropicKeyEnc,
		&v.AnthropicAPIKeyLast4,
		&openAIKeyEnc,
		&v.OpenAIAPIKeyLast4,
		&v.MonthlyBudgetUSD,
		&v.BudgetAlertEnabled,
		&v.BudgetAlertThresholdPct,
		&v.ReadingPlanWindow,
		&v.ReadingPlanSize,
		&v.ReadingPlanDiversifyTopics,
		&v.ReadingPlanExcludeRead,
		&v.CreatedAt,
		&v.UpdatedAt,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	v.HasAnthropicAPIKey = anthropicKeyEnc != nil && *anthropicKeyEnc != ""
	v.HasOpenAIAPIKey = openAIKeyEnc != nil && *openAIKeyEnc != ""
	return &v, nil
}

func (r *UserSettingsRepo) EnsureDefaults(ctx context.Context, userID string) (*model.UserSettings, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_settings (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	return r.GetByUserID(ctx, userID)
}

func (r *UserSettingsRepo) GetAnthropicAPIKeyEncrypted(ctx context.Context, userID string) (*string, error) {
	var v *string
	err := r.db.QueryRow(ctx, `
		SELECT anthropic_api_key_enc
		FROM user_settings
		WHERE user_id = $1`,
		userID,
	).Scan(&v)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if v == nil || *v == "" {
		return nil, nil
	}
	return v, nil
}

func (r *UserSettingsRepo) UpsertBudgetConfig(ctx context.Context, userID string, monthlyBudgetUSD *float64, enabled bool, thresholdPct int) (*model.UserSettings, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_settings (
			user_id,
			monthly_budget_usd,
			budget_alert_enabled,
			budget_alert_threshold_pct
		) VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE
		SET monthly_budget_usd = EXCLUDED.monthly_budget_usd,
		    budget_alert_enabled = EXCLUDED.budget_alert_enabled,
		    budget_alert_threshold_pct = EXCLUDED.budget_alert_threshold_pct,
		    updated_at = NOW()`,
		userID, monthlyBudgetUSD, enabled, thresholdPct,
	)
	if err != nil {
		return nil, err
	}
	return r.GetByUserID(ctx, userID)
}

func (r *UserSettingsRepo) UpsertReadingPlanConfig(ctx context.Context, userID, window string, size int, diversifyTopics, excludeRead bool) (*model.UserSettings, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_settings (
			user_id,
			reading_plan_window,
			reading_plan_size,
			reading_plan_diversify_topics,
			reading_plan_exclude_read
		) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE
		SET reading_plan_window = EXCLUDED.reading_plan_window,
		    reading_plan_size = EXCLUDED.reading_plan_size,
		    reading_plan_diversify_topics = EXCLUDED.reading_plan_diversify_topics,
		    reading_plan_exclude_read = EXCLUDED.reading_plan_exclude_read,
		    updated_at = NOW()`,
		userID, window, size, diversifyTopics, excludeRead,
	)
	if err != nil {
		return nil, err
	}
	return r.GetByUserID(ctx, userID)
}

func (r *UserSettingsRepo) GetOpenAIAPIKeyEncrypted(ctx context.Context, userID string) (*string, error) {
	var v *string
	err := r.db.QueryRow(ctx, `
		SELECT openai_api_key_enc
		FROM user_settings
		WHERE user_id = $1`,
		userID,
	).Scan(&v)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if v == nil || *v == "" {
		return nil, nil
	}
	return v, nil
}

func (r *UserSettingsRepo) SetAnthropicAPIKey(ctx context.Context, userID, encryptedKey, last4 string) (*model.UserSettings, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_settings (user_id, anthropic_api_key_enc, anthropic_api_key_last4)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE
		SET anthropic_api_key_enc = EXCLUDED.anthropic_api_key_enc,
		    anthropic_api_key_last4 = EXCLUDED.anthropic_api_key_last4,
		    updated_at = NOW()`,
		userID, encryptedKey, last4,
	)
	if err != nil {
		return nil, err
	}
	return r.GetByUserID(ctx, userID)
}

func (r *UserSettingsRepo) SetOpenAIAPIKey(ctx context.Context, userID, encryptedKey, last4 string) (*model.UserSettings, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_settings (user_id, openai_api_key_enc, openai_api_key_last4)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE
		SET openai_api_key_enc = EXCLUDED.openai_api_key_enc,
		    openai_api_key_last4 = EXCLUDED.openai_api_key_last4,
		    updated_at = NOW()`,
		userID, encryptedKey, last4,
	)
	if err != nil {
		return nil, err
	}
	return r.GetByUserID(ctx, userID)
}

func (r *UserSettingsRepo) ClearAnthropicAPIKey(ctx context.Context, userID string) (*model.UserSettings, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_settings (user_id, anthropic_api_key_enc, anthropic_api_key_last4)
		VALUES ($1, NULL, NULL)
		ON CONFLICT (user_id) DO UPDATE
		SET anthropic_api_key_enc = NULL,
		    anthropic_api_key_last4 = NULL,
		    updated_at = NOW()`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	return r.GetByUserID(ctx, userID)
}

func (r *UserSettingsRepo) ClearOpenAIAPIKey(ctx context.Context, userID string) (*model.UserSettings, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_settings (user_id, openai_api_key_enc, openai_api_key_last4)
		VALUES ($1, NULL, NULL)
		ON CONFLICT (user_id) DO UPDATE
		SET openai_api_key_enc = NULL,
		    openai_api_key_last4 = NULL,
		    updated_at = NOW()`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	return r.GetByUserID(ctx, userID)
}

func (r *UserSettingsRepo) ListBudgetAlertTargets(ctx context.Context) ([]BudgetAlertTarget, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.email, u.name,
		       us.monthly_budget_usd,
		       us.budget_alert_threshold_pct
		FROM user_settings us
		JOIN users u ON u.id = us.user_id
		WHERE us.budget_alert_enabled = TRUE
		  AND us.monthly_budget_usd IS NOT NULL
		  AND us.monthly_budget_usd > 0
		ORDER BY u.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []BudgetAlertTarget
	for rows.Next() {
		var v BudgetAlertTarget
		if err := rows.Scan(&v.UserID, &v.Email, &v.Name, &v.MonthlyBudgetUSD, &v.BudgetAlertThresholdPct); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

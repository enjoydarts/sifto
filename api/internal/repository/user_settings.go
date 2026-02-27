package repository

import (
	"context"
	"time"

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
	var inoreaderAccessTokenEnc *string
	err := r.db.QueryRow(ctx, `
		SELECT user_id,
		       anthropic_api_key_enc,
		       anthropic_api_key_last4,
		       openai_api_key_enc,
		       openai_api_key_last4,
		       monthly_budget_usd,
		       budget_alert_enabled,
		       budget_alert_threshold_pct,
		       digest_email_enabled,
		       reading_plan_window,
		       reading_plan_size,
		       reading_plan_diversify_topics,
		       reading_plan_exclude_read,
		       anthropic_facts_model,
		       anthropic_summary_model,
		       anthropic_digest_cluster_model,
		       anthropic_digest_model,
		       anthropic_source_suggestion_model,
		       openai_embedding_model,
		       inoreader_access_token_enc,
		       inoreader_token_expires_at,
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
		&v.DigestEmailEnabled,
		&v.ReadingPlanWindow,
		&v.ReadingPlanSize,
		&v.ReadingPlanDiversifyTopics,
		&v.ReadingPlanExcludeRead,
		&v.AnthropicFactsModel,
		&v.AnthropicSummaryModel,
		&v.AnthropicDigestClusterModel,
		&v.AnthropicDigestModel,
		&v.AnthropicSourceSuggestModel,
		&v.OpenAIEmbeddingModel,
		&inoreaderAccessTokenEnc,
		&v.InoreaderTokenExpiresAt,
		&v.CreatedAt,
		&v.UpdatedAt,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	v.HasAnthropicAPIKey = anthropicKeyEnc != nil && *anthropicKeyEnc != ""
	v.HasOpenAIAPIKey = openAIKeyEnc != nil && *openAIKeyEnc != ""
	v.HasInoreaderOAuth = inoreaderAccessTokenEnc != nil && *inoreaderAccessTokenEnc != ""
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

func (r *UserSettingsRepo) UpsertBudgetConfig(ctx context.Context, userID string, monthlyBudgetUSD *float64, enabled bool, thresholdPct int, digestEmailEnabled bool) (*model.UserSettings, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_settings (
			user_id,
			monthly_budget_usd,
			budget_alert_enabled,
			budget_alert_threshold_pct,
			digest_email_enabled
		) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE
		SET monthly_budget_usd = EXCLUDED.monthly_budget_usd,
		    budget_alert_enabled = EXCLUDED.budget_alert_enabled,
		    budget_alert_threshold_pct = EXCLUDED.budget_alert_threshold_pct,
		    digest_email_enabled = EXCLUDED.digest_email_enabled,
		    updated_at = NOW()`,
		userID, monthlyBudgetUSD, enabled, thresholdPct, digestEmailEnabled,
	)
	if err != nil {
		return nil, err
	}
	return r.GetByUserID(ctx, userID)
}

func (r *UserSettingsRepo) IsDigestEmailEnabled(ctx context.Context, userID string) (bool, error) {
	var enabled bool
	err := r.db.QueryRow(ctx, `
		INSERT INTO user_settings (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO UPDATE SET user_id = EXCLUDED.user_id
		RETURNING digest_email_enabled`,
		userID,
	).Scan(&enabled)
	if err != nil {
		return false, err
	}
	return enabled, nil
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

func (r *UserSettingsRepo) UpsertLLMModelConfig(
	ctx context.Context,
	userID string,
	anthropicFactsModel, anthropicSummaryModel, anthropicDigestClusterModel, anthropicDigestModel, anthropicSourceSuggestionModel, openAIEmbeddingModel *string,
) (*model.UserSettings, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_settings (
			user_id,
				anthropic_facts_model,
				anthropic_summary_model,
				anthropic_digest_cluster_model,
				anthropic_digest_model,
				anthropic_source_suggestion_model,
				openai_embedding_model
			) VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (user_id) DO UPDATE
			SET anthropic_facts_model = EXCLUDED.anthropic_facts_model,
			    anthropic_summary_model = EXCLUDED.anthropic_summary_model,
			    anthropic_digest_cluster_model = EXCLUDED.anthropic_digest_cluster_model,
			    anthropic_digest_model = EXCLUDED.anthropic_digest_model,
			    anthropic_source_suggestion_model = EXCLUDED.anthropic_source_suggestion_model,
			    openai_embedding_model = EXCLUDED.openai_embedding_model,
			    updated_at = NOW()`,
		userID,
		anthropicFactsModel,
		anthropicSummaryModel,
		anthropicDigestClusterModel,
		anthropicDigestModel,
		anthropicSourceSuggestionModel,
		openAIEmbeddingModel,
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

func (r *UserSettingsRepo) GetInoreaderTokensEncrypted(ctx context.Context, userID string) (accessTokenEnc, refreshTokenEnc *string, expiresAt *time.Time, err error) {
	err = r.db.QueryRow(ctx, `
		SELECT inoreader_access_token_enc, inoreader_refresh_token_enc, inoreader_token_expires_at
		FROM user_settings
		WHERE user_id = $1`,
		userID,
	).Scan(&accessTokenEnc, &refreshTokenEnc, &expiresAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil, nil, nil
		}
		return nil, nil, nil, err
	}
	return accessTokenEnc, refreshTokenEnc, expiresAt, nil
}

func (r *UserSettingsRepo) SetInoreaderOAuthTokens(ctx context.Context, userID, accessTokenEnc string, refreshTokenEnc *string, expiresAt *time.Time) (*model.UserSettings, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_settings (user_id, inoreader_access_token_enc, inoreader_refresh_token_enc, inoreader_token_expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE
		SET inoreader_access_token_enc = EXCLUDED.inoreader_access_token_enc,
		    inoreader_refresh_token_enc = EXCLUDED.inoreader_refresh_token_enc,
		    inoreader_token_expires_at = EXCLUDED.inoreader_token_expires_at,
		    updated_at = NOW()`,
		userID, accessTokenEnc, refreshTokenEnc, expiresAt,
	)
	if err != nil {
		return nil, err
	}
	return r.GetByUserID(ctx, userID)
}

func (r *UserSettingsRepo) ClearInoreaderOAuthTokens(ctx context.Context, userID string) (*model.UserSettings, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_settings (user_id, inoreader_access_token_enc, inoreader_refresh_token_enc, inoreader_token_expires_at)
		VALUES ($1, NULL, NULL, NULL)
		ON CONFLICT (user_id) DO UPDATE
		SET inoreader_access_token_enc = NULL,
		    inoreader_refresh_token_enc = NULL,
		    inoreader_token_expires_at = NULL,
		    updated_at = NOW()`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	return r.GetByUserID(ctx, userID)
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

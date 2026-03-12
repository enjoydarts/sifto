package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
)

type SettingsService struct {
	repo         *repository.UserSettingsRepo
	obsidianRepo *repository.ObsidianExportRepo
	llmUsageRepo *repository.LLMUsageLogRepo
	cipher       *SecretCipher
	githubApp    *GitHubAppClient
}

type SettingsGetPayload struct {
	UserID                  string         `json:"user_id"`
	HasAnthropicAPIKey      bool           `json:"has_anthropic_api_key"`
	AnthropicAPIKeyLast4    *string        `json:"anthropic_api_key_last4,omitempty"`
	HasOpenAIAPIKey         bool           `json:"has_openai_api_key"`
	OpenAIAPIKeyLast4       *string        `json:"openai_api_key_last4,omitempty"`
	HasGoogleAPIKey         bool           `json:"has_google_api_key"`
	GoogleAPIKeyLast4       *string        `json:"google_api_key_last4,omitempty"`
	HasGroqAPIKey           bool           `json:"has_groq_api_key"`
	GroqAPIKeyLast4         *string        `json:"groq_api_key_last4,omitempty"`
	HasDeepSeekAPIKey       bool           `json:"has_deepseek_api_key"`
	DeepSeekAPIKeyLast4     *string        `json:"deepseek_api_key_last4,omitempty"`
	HasInoreaderOAuth       bool           `json:"has_inoreader_oauth"`
	InoreaderTokenExpiresAt *time.Time     `json:"inoreader_token_expires_at,omitempty"`
	MonthlyBudgetUSD        *float64       `json:"monthly_budget_usd,omitempty"`
	BudgetAlertEnabled      bool           `json:"budget_alert_enabled"`
	BudgetAlertThresholdPct int            `json:"budget_alert_threshold_pct"`
	DigestEmailEnabled      bool           `json:"digest_email_enabled"`
	ReadingPlan             map[string]any `json:"reading_plan"`
	LLMModels               map[string]any `json:"llm_models"`
	CurrentMonth            map[string]any `json:"current_month"`
	ObsidianExport          map[string]any `json:"obsidian_export"`
}

type UpdateLLMModelsInput struct {
	Facts             *string
	Summary           *string
	DigestCluster     *string
	Digest            *string
	Ask               *string
	SourceSuggestion  *string
	Embedding         *string
	FactsCheck        *string
	FaithfulnessCheck *string
}

var modelSettingPurposes = map[string]string{
	"facts":              "facts",
	"summary":            "summary",
	"digest_cluster":     "digest_cluster_draft",
	"digest":             "digest",
	"ask":                "ask",
	"source_suggestion":  "source_suggestion",
	"facts_check":        "facts",
	"faithfulness_check": "summary",
}

var modelSettingRequiredCapabilities = map[string][]string{
	"facts":              {"structured_output"},
	"summary":            {"structured_output"},
	"digest_cluster":     {"structured_output"},
	"digest":             {"structured_output"},
	"ask":                {"structured_output"},
	"source_suggestion":  {"structured_output"},
	"facts_check":        {"structured_output"},
	"faithfulness_check": {"structured_output"},
}

func NewSettingsService(repo *repository.UserSettingsRepo, obsidianRepo *repository.ObsidianExportRepo, llmUsageRepo *repository.LLMUsageLogRepo, cipher *SecretCipher, githubApp *GitHubAppClient) *SettingsService {
	return &SettingsService{repo: repo, obsidianRepo: obsidianRepo, llmUsageRepo: llmUsageRepo, cipher: cipher, githubApp: githubApp}
}

func obsidianExportPayload(settings *model.ObsidianExportSettings, githubApp *GitHubAppClient) map[string]any {
	out := map[string]any{
		"enabled":                settings.Enabled,
		"github_installation_id": settings.GitHubInstallationID,
		"github_repo_owner":      settings.GitHubRepoOwner,
		"github_repo_name":       settings.GitHubRepoName,
		"github_repo_branch":     settings.GitHubRepoBranch,
		"vault_root_path":        settings.VaultRootPath,
		"keyword_link_mode":      settings.KeywordLinkMode,
		"last_run_at":            settings.LastRunAt,
		"last_success_at":        settings.LastSuccessAt,
	}
	if githubApp != nil {
		out["github_app_enabled"] = githubApp.Enabled()
		out["github_app_install_url"] = githubApp.InstallURL()
	}
	return out
}

func LLMModelSettingsPayload(settings *model.UserSettings) map[string]any {
	return map[string]any{
		"facts":              settings.FactsModel,
		"summary":            settings.SummaryModel,
		"digest_cluster":     settings.DigestClusterModel,
		"digest":             settings.DigestModel,
		"ask":                settings.AskModel,
		"source_suggestion":  settings.SourceSuggestionModel,
		"embedding":          settings.EmbeddingModel,
		"facts_check":        settings.FactsCheckModel,
		"faithfulness_check": settings.FaithfulnessCheckModel,
	}
}

func readingPlanPayload(settings *model.UserSettings) map[string]any {
	return map[string]any{
		"window":           settings.ReadingPlanWindow,
		"size":             settings.ReadingPlanSize,
		"diversify_topics": settings.ReadingPlanDiversifyTopics,
		"exclude_read":     settings.ReadingPlanExcludeRead,
	}
}

func (s *SettingsService) Get(ctx context.Context, userID string) (*SettingsGetPayload, error) {
	settings, err := s.repo.EnsureDefaults(ctx, userID)
	if err != nil {
		return nil, err
	}
	obsidianSettings, err := s.obsidianRepo.EnsureDefaults(ctx, userID)
	if err != nil {
		return nil, err
	}
	nowJST := timeutil.NowJST()
	monthStart := time.Date(nowJST.Year(), nowJST.Month(), 1, 0, 0, 0, 0, timeutil.JST)
	nextMonth := monthStart.AddDate(0, 1, 0)
	usedCostUSD, err := s.llmUsageRepo.SumEstimatedCostByUserBetween(ctx, userID, monthStart, nextMonth)
	if err != nil {
		return nil, err
	}
	var remainingBudgetUSD *float64
	var remainingPct *float64
	if settings.MonthlyBudgetUSD != nil && *settings.MonthlyBudgetUSD > 0 {
		v := *settings.MonthlyBudgetUSD - usedCostUSD
		remainingBudgetUSD = &v
		p := (v / *settings.MonthlyBudgetUSD) * 100
		remainingPct = &p
	}
	return &SettingsGetPayload{
		UserID:                  settings.UserID,
		HasAnthropicAPIKey:      settings.HasAnthropicAPIKey,
		AnthropicAPIKeyLast4:    settings.AnthropicAPIKeyLast4,
		HasOpenAIAPIKey:         settings.HasOpenAIAPIKey,
		OpenAIAPIKeyLast4:       settings.OpenAIAPIKeyLast4,
		HasGoogleAPIKey:         settings.HasGoogleAPIKey,
		GoogleAPIKeyLast4:       settings.GoogleAPIKeyLast4,
		HasGroqAPIKey:           settings.HasGroqAPIKey,
		GroqAPIKeyLast4:         settings.GroqAPIKeyLast4,
		HasDeepSeekAPIKey:       settings.HasDeepSeekAPIKey,
		DeepSeekAPIKeyLast4:     settings.DeepSeekAPIKeyLast4,
		HasInoreaderOAuth:       settings.HasInoreaderOAuth,
		InoreaderTokenExpiresAt: settings.InoreaderTokenExpiresAt,
		MonthlyBudgetUSD:        settings.MonthlyBudgetUSD,
		BudgetAlertEnabled:      settings.BudgetAlertEnabled,
		BudgetAlertThresholdPct: settings.BudgetAlertThresholdPct,
		DigestEmailEnabled:      settings.DigestEmailEnabled,
		ReadingPlan:             readingPlanPayload(settings),
		LLMModels:               LLMModelSettingsPayload(settings),
		ObsidianExport:          obsidianExportPayload(obsidianSettings, s.githubApp),
		CurrentMonth: map[string]any{
			"month_jst":            monthStart.Format("2006-01"),
			"period_start_jst":     monthStart.Format(time.RFC3339),
			"period_end_jst":       nextMonth.Format(time.RFC3339),
			"estimated_cost_usd":   usedCostUSD,
			"remaining_budget_usd": remainingBudgetUSD,
			"remaining_budget_pct": remainingPct,
		},
	}, nil
}

type UpdateObsidianExportInput struct {
	Enabled          bool
	GitHubRepoOwner  *string
	GitHubRepoName   *string
	GitHubRepoBranch *string
	VaultRootPath    *string
	KeywordLinkMode  *string
}

func normalizeOptionalModel(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	return &s
}

func validateCatalogModelForPurpose(model *string, purpose string) error {
	if model == nil {
		return nil
	}
	if !CatalogModelSupportsPurpose(*model, purpose) {
		return fmt.Errorf("invalid model for %s", purpose)
	}
	return nil
}

func validateCatalogModelCapabilities(model *string, settingKey string) error {
	if model == nil {
		return nil
	}
	for _, capability := range modelSettingRequiredCapabilities[settingKey] {
		if !CatalogModelSupportsCapability(*model, capability) {
			return fmt.Errorf("model missing required capability for %s", settingKey)
		}
	}
	return nil
}

func (s *SettingsService) UpdateLLMModels(ctx context.Context, userID string, in UpdateLLMModelsInput) (*model.UserSettings, error) {
	normalized := map[string]*string{
		"facts":              normalizeOptionalModel(in.Facts),
		"summary":            normalizeOptionalModel(in.Summary),
		"digest_cluster":     normalizeOptionalModel(in.DigestCluster),
		"digest":             normalizeOptionalModel(in.Digest),
		"ask":                normalizeOptionalModel(in.Ask),
		"source_suggestion":  normalizeOptionalModel(in.SourceSuggestion),
		"embedding":          normalizeOptionalModel(in.Embedding),
		"facts_check":        normalizeOptionalModel(in.FactsCheck),
		"faithfulness_check": normalizeOptionalModel(in.FaithfulnessCheck),
	}
	for settingKey, purpose := range modelSettingPurposes {
		if err := validateCatalogModelForPurpose(normalized[settingKey], purpose); err != nil {
			return nil, err
		}
		if err := validateCatalogModelCapabilities(normalized[settingKey], settingKey); err != nil {
			return nil, err
		}
	}
	embeddingModel := normalized["embedding"]
	if embeddingModel != nil && !CatalogIsEmbeddingModel(*embeddingModel) {
		return nil, fmt.Errorf("invalid embedding model")
	}
	return s.repo.UpsertLLMModelConfig(
		ctx,
		userID,
		normalized["facts"],
		normalized["summary"],
		normalized["digest_cluster"],
		normalized["digest"],
		normalized["ask"],
		normalized["source_suggestion"],
		embeddingModel,
		normalized["facts_check"],
		normalized["faithfulness_check"],
	)
}

func (s *SettingsService) UpdateReadingPlan(ctx context.Context, userID, window string, size int, diversifyTopics, excludeRead bool) (*model.UserSettings, error) {
	return s.repo.UpsertReadingPlanConfig(ctx, userID, window, size, diversifyTopics, excludeRead)
}

func (s *SettingsService) UpdateBudget(ctx context.Context, userID string, monthlyBudgetUSD *float64, enabled bool, thresholdPct int, digestEmailEnabled bool) (*model.UserSettings, error) {
	var budget *float64
	if monthlyBudgetUSD != nil && *monthlyBudgetUSD > 0 {
		budget = monthlyBudgetUSD
	}
	return s.repo.UpsertBudgetConfig(ctx, userID, budget, enabled, thresholdPct, digestEmailEnabled)
}

func normalizeOptionalString(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	return &s
}

func (s *SettingsService) UpdateObsidianExport(ctx context.Context, userID string, in UpdateObsidianExportInput) (*model.ObsidianExportSettings, error) {
	repoOwner := normalizeOptionalString(in.GitHubRepoOwner)
	repoName := normalizeOptionalString(in.GitHubRepoName)
	repoBranch := normalizeOptionalString(in.GitHubRepoBranch)
	vaultRootPath := normalizeOptionalString(in.VaultRootPath)
	keywordLinkMode := normalizeOptionalString(in.KeywordLinkMode)
	if keywordLinkMode != nil && *keywordLinkMode != "topics_only" {
		return nil, fmt.Errorf("invalid keyword_link_mode")
	}
	return s.obsidianRepo.UpsertConfig(ctx, userID, in.Enabled, repoOwner, repoName, repoBranch, vaultRootPath, keywordLinkMode)
}

func (s *SettingsService) UpsertObsidianGitHubInstallation(ctx context.Context, userID string, installationID int64) (*model.ObsidianExportSettings, error) {
	var owner *string
	if s.githubApp != nil && s.githubApp.Enabled() {
		installation, err := s.githubApp.GetInstallation(ctx, installationID)
		if err != nil {
			return nil, err
		}
		if installation != nil && installation.Account != nil && strings.TrimSpace(installation.Account.Login) != "" {
			v := strings.TrimSpace(installation.Account.Login)
			owner = &v
		}
	}
	return s.obsidianRepo.UpsertInstallation(ctx, userID, installationID, owner)
}

func (s *SettingsService) SetAPIKey(ctx context.Context, userID, provider, apiKey string) (*model.UserSettings, error) {
	if s.cipher == nil || !s.cipher.Enabled() {
		return nil, fmt.Errorf("user secret encryption is not configured")
	}
	key := strings.TrimSpace(apiKey)
	enc, err := s.cipher.EncryptString(key)
	if err != nil {
		return nil, err
	}
	last4 := key
	if len(last4) > 4 {
		last4 = last4[len(last4)-4:]
	}
	switch provider {
	case "anthropic":
		return s.repo.SetAnthropicAPIKey(ctx, userID, enc, last4)
	case "openai":
		return s.repo.SetOpenAIAPIKey(ctx, userID, enc, last4)
	case "google":
		return s.repo.SetGoogleAPIKey(ctx, userID, enc, last4)
	case "groq":
		return s.repo.SetGroqAPIKey(ctx, userID, enc, last4)
	case "deepseek":
		return s.repo.SetDeepSeekAPIKey(ctx, userID, enc, last4)
	default:
		return nil, fmt.Errorf("unsupported provider")
	}
}

func (s *SettingsService) DeleteAPIKey(ctx context.Context, userID, provider string) (*model.UserSettings, error) {
	switch provider {
	case "anthropic":
		return s.repo.ClearAnthropicAPIKey(ctx, userID)
	case "openai":
		return s.repo.ClearOpenAIAPIKey(ctx, userID)
	case "google":
		return s.repo.ClearGoogleAPIKey(ctx, userID)
	case "groq":
		return s.repo.ClearGroqAPIKey(ctx, userID)
	case "deepseek":
		return s.repo.ClearDeepSeekAPIKey(ctx, userID)
	default:
		return nil, fmt.Errorf("unsupported provider")
	}
}

package service

import (
	"context"
	"strings"
	"testing"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

type ttsMarkupPreprocessUsageRepoStub struct {
	input repository.LLMUsageLogInput
}

func (s *ttsMarkupPreprocessUsageRepoStub) Insert(_ context.Context, in repository.LLMUsageLogInput) error {
	s.input = in
	return nil
}

func TestRecordTTSMarkupPreprocessLLMUsageUsesSeparatePromptMetadata(t *testing.T) {
	repo := &ttsMarkupPreprocessUsageRepoStub{}
	userID := "00000000-0000-4000-8000-000000000022"
	itemID := "00000000-0000-4000-8000-000000000099"
	usage := &LLMUsage{
		Provider:                 "openai",
		Model:                    "gpt-5.4-mini",
		RequestedModel:           "gpt-5.4-mini",
		ResolvedModel:            "gpt-5.4-mini",
		PricingModelFamily:       "openai",
		PricingSource:            "openai",
		InputTokens:              12,
		OutputTokens:             34,
		CacheCreationInputTokens: 1,
		CacheReadInputTokens:     2,
		EstimatedCostUSD:         0.123,
	}

	recordTTSMarkupPreprocessLLMUsage(context.Background(), repo, NoopJSONCache{}, usage, &userID, &itemID, fishSummaryPreprocessPromptKey, fishPreprocessPurpose)

	if got := repo.input.PromptKey; got != fishSummaryPreprocessPromptKey {
		t.Fatalf("PromptKey = %q, want %q", got, fishSummaryPreprocessPromptKey)
	}
	if got := repo.input.PromptSource; got != fishPreprocessPromptSource {
		t.Fatalf("PromptSource = %q, want %q", got, fishPreprocessPromptSource)
	}
	if got := repo.input.Purpose; got != fishPreprocessPurpose {
		t.Fatalf("Purpose = %q, want %q", got, fishPreprocessPurpose)
	}
	if repo.input.ItemID == nil || *repo.input.ItemID != itemID {
		t.Fatalf("ItemID = %v, want %q", repo.input.ItemID, itemID)
	}
	if repo.input.IdempotencyKey == nil || *repo.input.IdempotencyKey == "" {
		t.Fatal("IdempotencyKey is empty")
	}
}

type ttsMarkupPreprocessWorkerStub struct {
	gotAPIKey *string
	gotText   string
	gotModel  string
	gotPrompt string
	gotVars   map[string]string
	response  *TTSMarkupPreprocessResponse
}

func (s *ttsMarkupPreprocessWorkerStub) PreprocessTTSMarkupText(_ context.Context, text string, model string, promptKey string, variables map[string]string, apiKey *string) (*TTSMarkupPreprocessResponse, error) {
	s.gotAPIKey = apiKey
	s.gotText = text
	s.gotModel = model
	s.gotPrompt = promptKey
	s.gotVars = variables
	if s.response != nil {
		return s.response, nil
	}
	return &TTSMarkupPreprocessResponse{Text: "[自然に] テスト"}, nil
}

func TestTTSMarkupPreprocessUsesOnlySelectedProviderKey(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "fish-preprocess-test-key")
	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	lockSettingsServiceTestDB(t, db)

	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000023"
	sourceID := "00000000-0000-4000-8000-000000000121"
	itemID := "00000000-0000-4000-8000-000000000111"
	if _, err := db.Exec(ctx, `DELETE FROM llm_usage_logs WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset llm_usage_logs: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM items WHERE id = $1`, itemID); err != nil {
		t.Fatalf("reset items: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM sources WHERE id = $1`, sourceID); err != nil {
		t.Fatalf("reset sources: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM user_settings WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset user_settings: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("reset users: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO users (id, email, name) VALUES ($1, $2, $3)`, userID, "fish-preprocess@example.com", "Fish Preprocess"); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := repository.NewUserSettingsRepo(db)
	openRouterModel := strptr("openrouter::openai/gpt-oss-120b")
	if _, err := repo.UpsertLLMModelConfig(ctx, userID,
		nil, nil, 0, nil, nil, nil, 0, nil, nil, nil, nil, nil, nil, nil, nil,
		false, false, "", "", nil, nil, nil, nil, nil, nil, openRouterModel,
	); err != nil {
		t.Fatalf("UpsertLLMModelConfig() error = %v", err)
	}

	cipher := NewSecretCipher()
	openRouterEnc, err := cipher.EncryptString("openrouter-secret")
	if err != nil {
		t.Fatalf("EncryptString(openrouter): %v", err)
	}
	if _, err := repo.SetOpenRouterAPIKey(ctx, userID, openRouterEnc, "cret"); err != nil {
		t.Fatalf("SetOpenRouterAPIKey() error = %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE user_settings SET mistral_api_key_enc = $2, mistral_api_key_last4 = $3 WHERE user_id = $1`, userID, "not-base64", "fail"); err != nil {
		t.Fatalf("seed invalid mistral key: %v", err)
	}

	worker := &ttsMarkupPreprocessWorkerStub{}
	worker.response = &TTSMarkupPreprocessResponse{
		Text: "[自然に] テスト",
		LLM: &LLMUsage{
			Provider:           "openrouter",
			Model:              "openai/gpt-oss-20b",
			RequestedModel:     "openrouter::openai/gpt-oss-20b",
			ResolvedModel:      "openai/gpt-oss-20b",
			PricingModelFamily: "openrouter",
			PricingSource:      "openrouter",
			InputTokens:        21,
			OutputTokens:       8,
			EstimatedCostUSD:   0.001,
		},
	}
	service := NewTTSMarkupPreprocessService(repo, cipher, worker, repository.NewLLMUsageLogRepo(db), NoopJSONCache{})

	result, err := service.PreprocessSummaryAudioText(ctx, userID, "", "元テキスト")
	if err != nil {
		t.Fatalf("PreprocessSummaryAudioText() error = %v", err)
	}
	if result == nil || result.Text != "[自然に] テスト" {
		t.Fatalf("result = %#v, want preprocessed text", result)
	}
	if worker.gotAPIKey == nil || *worker.gotAPIKey != "openrouter-secret" {
		t.Fatalf("worker.gotAPIKey = %v, want openrouter-secret", worker.gotAPIKey)
	}
}

func TestTTSMarkupPreprocessRequiresConfiguredModel(t *testing.T) {
	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	lockSettingsServiceTestDB(t, db)

	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000023"
	if _, err := db.Exec(ctx, `DELETE FROM user_settings WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset user_settings: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("reset users: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO users (id, email, name) VALUES ($1, $2, $3)`, userID, "fish-preprocess-missing@example.com", "Fish Preprocess"); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	service := NewTTSMarkupPreprocessService(repository.NewUserSettingsRepo(db), NewSecretCipher(), &ttsMarkupPreprocessWorkerStub{}, repository.NewLLMUsageLogRepo(db), NoopJSONCache{})
	_, err = service.PreprocessSummaryAudioText(ctx, userID, "", "元テキスト")
	if err == nil || err != ErrTTSMarkupPreprocessModelNotConfigured {
		t.Fatalf("PreprocessSummaryAudioText() error = %v, want %v", err, ErrTTSMarkupPreprocessModelNotConfigured)
	}
}

func TestTTSMarkupPreprocessRejectsEmptyOutput(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "fish-preprocess-empty-output-test-key")
	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	lockSettingsServiceTestDB(t, db)

	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000024"
	if _, err := db.Exec(ctx, `DELETE FROM user_settings WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset user_settings: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("reset users: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO users (id, email, name) VALUES ($1, $2, $3)`, userID, "fish-preprocess-empty@example.com", "Fish Preprocess"); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := repository.NewUserSettingsRepo(db)
	openRouterModel := strptr("openrouter::openai/gpt-oss-120b")
	if _, err := repo.UpsertLLMModelConfig(ctx, userID,
		nil, nil, 0, nil, nil, nil, 0, nil, nil, nil, nil, nil, nil, nil, nil,
		false, false, "", "", nil, nil, nil, nil, nil, nil, openRouterModel,
	); err != nil {
		t.Fatalf("UpsertLLMModelConfig() error = %v", err)
	}
	cipher := NewSecretCipher()
	openRouterEnc, err := cipher.EncryptString("openrouter-secret")
	if err != nil {
		t.Fatalf("EncryptString(openrouter): %v", err)
	}
	if _, err := repo.SetOpenRouterAPIKey(ctx, userID, openRouterEnc, "cret"); err != nil {
		t.Fatalf("SetOpenRouterAPIKey() error = %v", err)
	}

	service := NewTTSMarkupPreprocessService(repo, cipher, &ttsMarkupPreprocessWorkerStub{
		response: &TTSMarkupPreprocessResponse{Text: "   "},
	}, repository.NewLLMUsageLogRepo(db), NoopJSONCache{})
	_, err = service.PreprocessSummaryAudioText(ctx, userID, "", "元テキスト")
	if err == nil || err != ErrTTSMarkupPreprocessEmptyOutput {
		t.Fatalf("PreprocessSummaryAudioText() error = %v, want %v", err, ErrTTSMarkupPreprocessEmptyOutput)
	}
}

func TestTTSMarkupPreprocessAudioBriefingSingleUsesPromptKeyAndPersonaVariables(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "fish-preprocess-briefing-single-test-key")
	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	lockSettingsServiceTestDB(t, db)

	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000025"
	sourceID := "00000000-0000-4000-8000-000000000123"
	itemID := "00000000-0000-4000-8000-000000000113"
	if _, err := db.Exec(ctx, `DELETE FROM llm_usage_logs WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset llm_usage_logs: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM items WHERE id = $1`, itemID); err != nil {
		t.Fatalf("reset items: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM sources WHERE id = $1`, sourceID); err != nil {
		t.Fatalf("reset sources: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM user_settings WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset user_settings: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("reset users: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO users (id, email, name) VALUES ($1, $2, $3)`, userID, "fish-preprocess-briefing-single@example.com", "Fish Preprocess"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO sources (id, user_id, url, type, title) VALUES ($1, $2, $3, 'manual', $4)`, sourceID, userID, "https://example.com/feed", "Example Source"); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO items (id, source_id, url, title, status, content_text) VALUES ($1, $2, $3, $4, 'fetched', $5)`, itemID, sourceID, "https://example.com/items/113", "Briefing Item", "body"); err != nil {
		t.Fatalf("insert item: %v", err)
	}

	repo := repository.NewUserSettingsRepo(db)
	modelName := strptr("openrouter::openai/gpt-oss-20b")
	if _, err := repo.UpsertLLMModelConfig(ctx, userID,
		nil, nil, 0, nil, nil, nil, 0, nil, nil, nil, nil, nil, nil, nil, nil,
		false, false, "", "", nil, nil, nil, nil, nil, nil, modelName,
	); err != nil {
		t.Fatalf("UpsertLLMModelConfig() error = %v", err)
	}
	cipher := NewSecretCipher()
	enc, err := cipher.EncryptString("openrouter-secret")
	if err != nil {
		t.Fatalf("EncryptString(openrouter): %v", err)
	}
	if _, err := repo.SetOpenRouterAPIKey(ctx, userID, enc, "cret"); err != nil {
		t.Fatalf("SetOpenRouterAPIKey() error = %v", err)
	}

	worker := &ttsMarkupPreprocessWorkerStub{}
	worker.response = &TTSMarkupPreprocessResponse{
		Text: "[自然に] テスト",
		LLM: &LLMUsage{
			Provider:           "openrouter",
			Model:              "openai/gpt-oss-20b",
			RequestedModel:     "openrouter::openai/gpt-oss-20b",
			ResolvedModel:      "openai/gpt-oss-20b",
			PricingModelFamily: "openrouter",
			PricingSource:      "openrouter",
			InputTokens:        21,
			OutputTokens:       8,
			EstimatedCostUSD:   0.001,
		},
	}
	service := NewTTSMarkupPreprocessService(repo, cipher, worker, repository.NewLLMUsageLogRepo(db), NoopJSONCache{})

	if _, err := service.PreprocessAudioBriefingSingleText(ctx, userID, itemID, "editor", "元テキスト"); err != nil {
		t.Fatalf("PreprocessAudioBriefingSingleText() error = %v", err)
	}
	if worker.gotPrompt != fishAudioBriefingSinglePreprocessPromptKey {
		t.Fatalf("promptKey = %q, want %q", worker.gotPrompt, fishAudioBriefingSinglePreprocessPromptKey)
	}
	if got := worker.gotVars["persona_name"]; got != "editor" {
		t.Fatalf("variables.persona_name = %q, want editor", got)
	}

	var (
		purpose   string
		promptKey string
		loggedID  *string
	)
	if err := db.QueryRow(ctx, `SELECT purpose, prompt_key, item_id FROM llm_usage_logs WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, userID).Scan(&purpose, &promptKey, &loggedID); err != nil {
		t.Fatalf("select llm_usage_logs: %v", err)
	}
	if purpose != fishPreprocessPurpose {
		t.Fatalf("purpose = %q, want %q", purpose, fishPreprocessPurpose)
	}
	if promptKey != fishAudioBriefingSinglePreprocessPromptKey {
		t.Fatalf("prompt_key = %q, want %q", promptKey, fishAudioBriefingSinglePreprocessPromptKey)
	}
	if loggedID == nil || strings.TrimSpace(*loggedID) != itemID {
		t.Fatalf("item_id = %v, want %s", loggedID, itemID)
	}
}

func TestTTSMarkupPreprocessAudioBriefingDuoUsesPersonaVariables(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "fish-preprocess-briefing-duo-test-key")
	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	lockSettingsServiceTestDB(t, db)

	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000023"
	sourceID := "00000000-0000-4000-8000-000000000122"
	itemID := "00000000-0000-4000-8000-000000000112"
	if _, err := db.Exec(ctx, `DELETE FROM llm_usage_logs WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset llm_usage_logs: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM items WHERE id = $1`, itemID); err != nil {
		t.Fatalf("reset items: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM sources WHERE id = $1`, sourceID); err != nil {
		t.Fatalf("reset sources: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM user_settings WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset user_settings: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("reset users: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO users (id, email, name) VALUES ($1, $2, $3)`, userID, "fish-preprocess-briefing-duo@example.com", "Fish Preprocess"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO sources (id, user_id, url, type, title) VALUES ($1, $2, $3, 'manual', $4)`, sourceID, userID, "https://example.com/feed", "Example Source"); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO items (id, source_id, url, title, status, content_text) VALUES ($1, $2, $3, $4, 'fetched', $5)`, itemID, sourceID, "https://example.com/items/112", "Briefing Item", "body"); err != nil {
		t.Fatalf("insert item: %v", err)
	}

	repo := repository.NewUserSettingsRepo(db)
	modelName := strptr("openrouter::openai/gpt-oss-20b")
	if _, err := repo.UpsertLLMModelConfig(ctx, userID,
		nil, nil, 0, nil, nil, nil, 0, nil, nil, nil, nil, nil, nil, nil, nil,
		false, false, "", "", nil, nil, nil, nil, nil, nil, modelName,
	); err != nil {
		t.Fatalf("UpsertLLMModelConfig() error = %v", err)
	}
	cipher := NewSecretCipher()
	enc, err := cipher.EncryptString("openrouter-secret")
	if err != nil {
		t.Fatalf("EncryptString(openrouter): %v", err)
	}
	if _, err := repo.SetOpenRouterAPIKey(ctx, userID, enc, "cret"); err != nil {
		t.Fatalf("SetOpenRouterAPIKey() error = %v", err)
	}

	worker := &ttsMarkupPreprocessWorkerStub{}
	worker.response = &TTSMarkupPreprocessResponse{
		Text: "<|speaker:0|>[自然に] 冒頭<|speaker:1|>[少し柔らかく] 補足",
		LLM: &LLMUsage{
			Provider:           "openrouter",
			Model:              "openai/gpt-oss-20b",
			RequestedModel:     "openrouter::openai/gpt-oss-20b",
			ResolvedModel:      "openai/gpt-oss-20b",
			PricingModelFamily: "openrouter",
			PricingSource:      "openrouter",
			InputTokens:        34,
			OutputTokens:       13,
			EstimatedCostUSD:   0.002,
		},
	}
	service := NewTTSMarkupPreprocessService(repo, cipher, worker, repository.NewLLMUsageLogRepo(db), NoopJSONCache{})

	if _, err := service.PreprocessAudioBriefingDuoText(ctx, userID, itemID, "native", "analyst", "<|speaker:0|>冒頭<|speaker:1|>補足"); err != nil {
		t.Fatalf("PreprocessAudioBriefingDuoText() error = %v", err)
	}
	if worker.gotPrompt != fishAudioBriefingDuoPreprocessPromptKey {
		t.Fatalf("promptKey = %q, want %q", worker.gotPrompt, fishAudioBriefingDuoPreprocessPromptKey)
	}
	if got := worker.gotVars["host_persona_name"]; got != "native" {
		t.Fatalf("variables.host_persona_name = %q, want native", got)
	}
	if got := worker.gotVars["partner_persona_name"]; got != "analyst" {
		t.Fatalf("variables.partner_persona_name = %q, want analyst", got)
	}

	var (
		purpose   string
		promptKey string
		loggedID  *string
	)
	if err := db.QueryRow(ctx, `SELECT purpose, prompt_key, item_id FROM llm_usage_logs WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, userID).Scan(&purpose, &promptKey, &loggedID); err != nil {
		t.Fatalf("select llm_usage_logs: %v", err)
	}
	if purpose != fishPreprocessPurpose {
		t.Fatalf("purpose = %q, want %q", purpose, fishPreprocessPurpose)
	}
	if promptKey != fishAudioBriefingDuoPreprocessPromptKey {
		t.Fatalf("prompt_key = %q, want %q", promptKey, fishAudioBriefingDuoPreprocessPromptKey)
	}
	if loggedID == nil || strings.TrimSpace(*loggedID) != itemID {
		t.Fatalf("item_id = %v, want %s", loggedID, itemID)
	}
}

func TestTTSMarkupPreprocessGeminiPromptUsesGeminiPurpose(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "tts-markup-preprocess-gemini-purpose-test-key")
	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	lockSettingsServiceTestDB(t, db)

	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000026"
	if _, err := db.Exec(ctx, `DELETE FROM llm_usage_logs WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset llm_usage_logs: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM user_settings WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset user_settings: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("reset users: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO users (id, email, name) VALUES ($1, $2, $3)`, userID, "tts-markup-preprocess-gemini@example.com", "TTS Markup Preprocess"); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := repository.NewUserSettingsRepo(db)
	modelName := strptr("openrouter::openai/gpt-oss-20b")
	if _, err := repo.UpsertLLMModelConfig(ctx, userID,
		nil, nil, 0, nil, nil, nil, 0, nil, nil, nil, nil, nil, nil, nil, nil,
		false, false, "", "", nil, nil, nil, nil, nil, nil, modelName,
	); err != nil {
		t.Fatalf("UpsertLLMModelConfig() error = %v", err)
	}
	cipher := NewSecretCipher()
	enc, err := cipher.EncryptString("openrouter-secret")
	if err != nil {
		t.Fatalf("EncryptString(openrouter): %v", err)
	}
	if _, err := repo.SetOpenRouterAPIKey(ctx, userID, enc, "cret"); err != nil {
		t.Fatalf("SetOpenRouterAPIKey() error = %v", err)
	}

	worker := &ttsMarkupPreprocessWorkerStub{
		response: &TTSMarkupPreprocessResponse{
			Text: "[short pause] テスト",
			LLM: &LLMUsage{
				Provider:           "openrouter",
				Model:              "openai/gpt-oss-20b",
				RequestedModel:     "openrouter::openai/gpt-oss-20b",
				ResolvedModel:      "openai/gpt-oss-20b",
				PricingModelFamily: "openrouter",
				PricingSource:      "openrouter",
				InputTokens:        10,
				OutputTokens:       4,
				EstimatedCostUSD:   0.001,
			},
		},
	}
	service := NewTTSMarkupPreprocessService(repo, cipher, worker, repository.NewLLMUsageLogRepo(db), NoopJSONCache{})

	if _, err := service.Preprocess(ctx, userID, "", geminiSummaryPreprocessPromptKey, "元テキスト", nil); err != nil {
		t.Fatalf("Preprocess() error = %v", err)
	}

	var (
		purpose   string
		promptKey string
	)
	if err := db.QueryRow(ctx, `SELECT purpose, prompt_key FROM llm_usage_logs WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, userID).Scan(&purpose, &promptKey); err != nil {
		t.Fatalf("select llm_usage_logs: %v", err)
	}
	if purpose != geminiTTSPreprocessPurpose {
		t.Fatalf("purpose = %q, want %q", purpose, geminiTTSPreprocessPurpose)
	}
	if promptKey != geminiSummaryPreprocessPromptKey {
		t.Fatalf("prompt_key = %q, want %q", promptKey, geminiSummaryPreprocessPromptKey)
	}
}

func TestTTSMarkupPreprocessElevenLabsPromptUsesElevenLabsPurpose(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "tts-markup-preprocess-elevenlabs-purpose-test-key")
	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	lockSettingsServiceTestDB(t, db)

	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000027"
	if _, err := db.Exec(ctx, `DELETE FROM llm_usage_logs WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset llm_usage_logs: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM user_settings WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset user_settings: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("reset users: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO users (id, email, name) VALUES ($1, $2, $3)`, userID, "tts-markup-preprocess-elevenlabs@example.com", "TTS Markup Preprocess"); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := repository.NewUserSettingsRepo(db)
	modelName := strptr("openrouter::openai/gpt-5.4-mini")
	if _, err := repo.UpsertLLMModelConfig(ctx, userID,
		nil, nil, 0, nil, nil, nil, 0, nil, nil, nil, nil, nil, nil, nil, nil,
		false, false, "", "", nil, nil, nil, nil, nil, nil, modelName,
	); err != nil {
		t.Fatalf("UpsertLLMModelConfig() error = %v", err)
	}
	cipher := NewSecretCipher()
	enc, err := cipher.EncryptString("openrouter-secret")
	if err != nil {
		t.Fatalf("EncryptString(openrouter): %v", err)
	}
	if _, err := repo.SetOpenRouterAPIKey(ctx, userID, enc, "cret"); err != nil {
		t.Fatalf("SetOpenRouterAPIKey() error = %v", err)
	}

	worker := &ttsMarkupPreprocessWorkerStub{
		response: &TTSMarkupPreprocessResponse{
			Text: "[自然に] テスト",
			LLM: &LLMUsage{
				Provider:           "openrouter",
				Model:              "openai/gpt-5.4-mini",
				RequestedModel:     "openrouter::openai/gpt-5.4-mini",
				ResolvedModel:      "openai/gpt-5.4-mini",
				PricingModelFamily: "openrouter",
				PricingSource:      "openrouter",
				InputTokens:        10,
				OutputTokens:       4,
				EstimatedCostUSD:   0.001,
			},
		},
	}
	service := NewTTSMarkupPreprocessService(repo, cipher, worker, repository.NewLLMUsageLogRepo(db), NoopJSONCache{})

	if _, err := service.PreprocessSummaryAudioTextForProvider(ctx, userID, "", "elevenlabs", "元テキスト"); err != nil {
		t.Fatalf("PreprocessSummaryAudioTextForProvider() error = %v", err)
	}

	var (
		purpose   string
		promptKey string
	)
	if err := db.QueryRow(ctx, `SELECT purpose, prompt_key FROM llm_usage_logs WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, userID).Scan(&purpose, &promptKey); err != nil {
		t.Fatalf("select llm_usage_logs: %v", err)
	}
	if purpose != elevenLabsTTSPreprocessPurpose {
		t.Fatalf("purpose = %q, want %q", purpose, elevenLabsTTSPreprocessPurpose)
	}
	if promptKey != elevenLabsSummaryPreprocessPromptKey {
		t.Fatalf("prompt_key = %q, want %q", promptKey, elevenLabsSummaryPreprocessPromptKey)
	}
}

func TestTTSMarkupPreprocessAzureSpeechPromptUsesAzurePurpose(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "tts-markup-preprocess-azure-purpose-test-key")
	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	lockSettingsServiceTestDB(t, db)

	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000030"
	if _, err := db.Exec(ctx, `DELETE FROM llm_usage_logs WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset llm_usage_logs: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM user_settings WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset user_settings: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("reset users: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO users (id, email, name) VALUES ($1, $2, $3)`, userID, "tts-markup-preprocess-azure@example.com", "TTS Markup Preprocess"); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := repository.NewUserSettingsRepo(db)
	modelName := strptr("gpt-5.4-mini")
	if _, err := repo.UpsertLLMModelConfig(ctx, userID,
		nil, nil, 0, nil, nil, nil, 0, nil, nil, nil, nil, nil, nil, nil, nil,
		false, false, "", "", nil, nil, nil, nil, nil, nil, modelName,
	); err != nil {
		t.Fatalf("UpsertLLMModelConfig() error = %v", err)
	}
	cipher := NewSecretCipher()
	enc, err := cipher.EncryptString("openai-secret")
	if err != nil {
		t.Fatalf("EncryptString(openai): %v", err)
	}
	if _, err := repo.SetOpenAIAPIKey(ctx, userID, enc, "cret"); err != nil {
		t.Fatalf("SetOpenAIAPIKey() error = %v", err)
	}

	worker := &ttsMarkupPreprocessWorkerStub{
		response: &TTSMarkupPreprocessResponse{
			Text: `<speak version="1.0" xml:lang="ja-JP"></speak>`,
			LLM: &LLMUsage{
				Provider:           "openai",
				Model:              "gpt-5.4-mini",
				RequestedModel:     "gpt-5.4-mini",
				ResolvedModel:      "gpt-5.4-mini",
				PricingModelFamily: "openai",
				PricingSource:      "openai",
				InputTokens:        10,
				OutputTokens:       4,
				EstimatedCostUSD:   0.001,
			},
		},
	}
	service := NewTTSMarkupPreprocessService(repo, cipher, worker, repository.NewLLMUsageLogRepo(db), NoopJSONCache{})

	if _, err := service.PreprocessSummaryAudioTextForProviderWithVariables(ctx, userID, "", "azure_speech", "元テキスト", map[string]string{
		"voice_name":   "ja-JP-AoiNeural",
		"voice_locale": "ja-JP",
	}); err != nil {
		t.Fatalf("PreprocessSummaryAudioTextForProviderWithVariables() error = %v", err)
	}

	var (
		purpose   string
		promptKey string
	)
	if err := db.QueryRow(ctx, `SELECT purpose, prompt_key FROM llm_usage_logs WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, userID).Scan(&purpose, &promptKey); err != nil {
		t.Fatalf("select llm_usage_logs: %v", err)
	}
	if purpose != azureSpeechTTSPreprocessPurpose {
		t.Fatalf("purpose = %q, want %q", purpose, azureSpeechTTSPreprocessPurpose)
	}
	if promptKey != azureSpeechSummaryPreprocessPromptKey {
		t.Fatalf("prompt_key = %q, want %q", promptKey, azureSpeechSummaryPreprocessPromptKey)
	}
}

func TestTTSMarkupPreprocessElevenLabsAudioBriefingSingleUsesPromptKeyAndPersonaVariables(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "tts-markup-preprocess-elevenlabs-single-test-key")
	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	lockSettingsServiceTestDB(t, db)

	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000028"
	if _, err := db.Exec(ctx, `DELETE FROM llm_usage_logs WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset llm_usage_logs: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM user_settings WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset user_settings: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("reset users: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO users (id, email, name) VALUES ($1, $2, $3)`, userID, "tts-markup-preprocess-elevenlabs-single@example.com", "TTS Markup Preprocess"); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := repository.NewUserSettingsRepo(db)
	modelName := strptr("openrouter::openai/gpt-5.4-mini")
	if _, err := repo.UpsertLLMModelConfig(ctx, userID,
		nil, nil, 0, nil, nil, nil, 0, nil, nil, nil, nil, nil, nil, nil, nil,
		false, false, "", "", nil, nil, nil, nil, nil, nil, modelName,
	); err != nil {
		t.Fatalf("UpsertLLMModelConfig() error = %v", err)
	}
	cipher := NewSecretCipher()
	enc, err := cipher.EncryptString("openrouter-secret")
	if err != nil {
		t.Fatalf("EncryptString(openrouter): %v", err)
	}
	if _, err := repo.SetOpenRouterAPIKey(ctx, userID, enc, "cret"); err != nil {
		t.Fatalf("SetOpenRouterAPIKey() error = %v", err)
	}

	worker := &ttsMarkupPreprocessWorkerStub{}
	worker.response = &TTSMarkupPreprocessResponse{
		Text: "[自然に] テスト",
		LLM: &LLMUsage{
			Provider:           "openrouter",
			Model:              "openai/gpt-5.4-mini",
			RequestedModel:     "openrouter::openai/gpt-5.4-mini",
			ResolvedModel:      "openai/gpt-5.4-mini",
			PricingModelFamily: "openrouter",
			PricingSource:      "openrouter",
			InputTokens:        21,
			OutputTokens:       8,
			EstimatedCostUSD:   0.001,
		},
	}
	service := NewTTSMarkupPreprocessService(repo, cipher, worker, repository.NewLLMUsageLogRepo(db), NoopJSONCache{})

	if _, err := service.PreprocessAudioBriefingSingleTextForProvider(ctx, userID, "", "elevenlabs", "editor", "元テキスト"); err != nil {
		t.Fatalf("PreprocessAudioBriefingSingleTextForProvider() error = %v", err)
	}
	if worker.gotPrompt != elevenLabsAudioBriefingSinglePreprocessPromptKey {
		t.Fatalf("promptKey = %q, want %q", worker.gotPrompt, elevenLabsAudioBriefingSinglePreprocessPromptKey)
	}
	if got := worker.gotVars["persona_name"]; got != "editor" {
		t.Fatalf("variables.persona_name = %q, want editor", got)
	}

	var (
		purpose   string
		promptKey string
	)
	if err := db.QueryRow(ctx, `SELECT purpose, prompt_key FROM llm_usage_logs WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, userID).Scan(&purpose, &promptKey); err != nil {
		t.Fatalf("select llm_usage_logs: %v", err)
	}
	if purpose != elevenLabsTTSPreprocessPurpose {
		t.Fatalf("purpose = %q, want %q", purpose, elevenLabsTTSPreprocessPurpose)
	}
	if promptKey != elevenLabsAudioBriefingSinglePreprocessPromptKey {
		t.Fatalf("prompt_key = %q, want %q", promptKey, elevenLabsAudioBriefingSinglePreprocessPromptKey)
	}
}

func TestTTSMarkupPreprocessElevenLabsAudioBriefingDuoUsesPersonaVariables(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "tts-markup-preprocess-elevenlabs-duo-test-key")
	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	lockSettingsServiceTestDB(t, db)

	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000029"
	if _, err := db.Exec(ctx, `DELETE FROM llm_usage_logs WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset llm_usage_logs: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM user_settings WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset user_settings: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("reset users: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO users (id, email, name) VALUES ($1, $2, $3)`, userID, "tts-markup-preprocess-elevenlabs-duo@example.com", "TTS Markup Preprocess"); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := repository.NewUserSettingsRepo(db)
	modelName := strptr("openrouter::openai/gpt-5.4-mini")
	if _, err := repo.UpsertLLMModelConfig(ctx, userID,
		nil, nil, 0, nil, nil, nil, 0, nil, nil, nil, nil, nil, nil, nil, nil,
		false, false, "", "", nil, nil, nil, nil, nil, nil, modelName,
	); err != nil {
		t.Fatalf("UpsertLLMModelConfig() error = %v", err)
	}
	cipher := NewSecretCipher()
	enc, err := cipher.EncryptString("openrouter-secret")
	if err != nil {
		t.Fatalf("EncryptString(openrouter): %v", err)
	}
	if _, err := repo.SetOpenRouterAPIKey(ctx, userID, enc, "cret"); err != nil {
		t.Fatalf("SetOpenRouterAPIKey() error = %v", err)
	}

	worker := &ttsMarkupPreprocessWorkerStub{}
	worker.response = &TTSMarkupPreprocessResponse{
		Text: "<|speaker:0|>[自然に] 冒頭<|speaker:1|>[少し柔らかく] 補足",
		LLM: &LLMUsage{
			Provider:           "openrouter",
			Model:              "openai/gpt-5.4-mini",
			RequestedModel:     "openrouter::openai/gpt-5.4-mini",
			ResolvedModel:      "openai/gpt-5.4-mini",
			PricingModelFamily: "openrouter",
			PricingSource:      "openrouter",
			InputTokens:        34,
			OutputTokens:       13,
			EstimatedCostUSD:   0.002,
		},
	}
	service := NewTTSMarkupPreprocessService(repo, cipher, worker, repository.NewLLMUsageLogRepo(db), NoopJSONCache{})

	if _, err := service.PreprocessAudioBriefingDuoTextForProvider(ctx, userID, "", "elevenlabs", "native", "analyst", "<|speaker:0|>冒頭<|speaker:1|>補足"); err != nil {
		t.Fatalf("PreprocessAudioBriefingDuoTextForProvider() error = %v", err)
	}
	if worker.gotPrompt != elevenLabsAudioBriefingDuoPreprocessPromptKey {
		t.Fatalf("promptKey = %q, want %q", worker.gotPrompt, elevenLabsAudioBriefingDuoPreprocessPromptKey)
	}
	if got := worker.gotVars["host_persona_name"]; got != "native" {
		t.Fatalf("variables.host_persona_name = %q, want native", got)
	}
	if got := worker.gotVars["partner_persona_name"]; got != "analyst" {
		t.Fatalf("variables.partner_persona_name = %q, want analyst", got)
	}

	var (
		purpose   string
		promptKey string
	)
	if err := db.QueryRow(ctx, `SELECT purpose, prompt_key FROM llm_usage_logs WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, userID).Scan(&purpose, &promptKey); err != nil {
		t.Fatalf("select llm_usage_logs: %v", err)
	}
	if purpose != elevenLabsTTSPreprocessPurpose {
		t.Fatalf("purpose = %q, want %q", purpose, elevenLabsTTSPreprocessPurpose)
	}
	if promptKey != elevenLabsAudioBriefingDuoPreprocessPromptKey {
		t.Fatalf("prompt_key = %q, want %q", promptKey, elevenLabsAudioBriefingDuoPreprocessPromptKey)
	}
}

func TestTTSMarkupPreprocessPromptKeyFamiliesByProvider(t *testing.T) {
	tests := []struct {
		name        string
		provider    string
		wantSummary string
		wantSingle  string
		wantDuo     string
	}{
		{
			name:        "fish",
			provider:    "fish",
			wantSummary: fishSummaryPreprocessPromptKey,
			wantSingle:  fishAudioBriefingSinglePreprocessPromptKey,
			wantDuo:     fishAudioBriefingDuoPreprocessPromptKey,
		},
		{
			name:        "gemini_tts",
			provider:    "gemini_tts",
			wantSummary: geminiSummaryPreprocessPromptKey,
			wantSingle:  geminiAudioBriefingSinglePreprocessPromptKey,
			wantDuo:     geminiAudioBriefingDuoPreprocessPromptKey,
		},
		{
			name:        "elevenlabs",
			provider:    "elevenlabs",
			wantSummary: elevenLabsSummaryPreprocessPromptKey,
			wantSingle:  elevenLabsAudioBriefingSinglePreprocessPromptKey,
			wantDuo:     elevenLabsAudioBriefingDuoPreprocessPromptKey,
		},
		{
			name:        "xai",
			provider:    "xai",
			wantSummary: xaiSummaryPreprocessPromptKey,
			wantSingle:  xaiAudioBriefingSinglePreprocessPromptKey,
			wantDuo:     xaiAudioBriefingDuoPreprocessPromptKey,
		},
		{
			name:        "azure_speech",
			provider:    "azure_speech",
			wantSummary: azureSpeechSummaryPreprocessPromptKey,
			wantSingle:  azureSpeechAudioBriefingSinglePreprocessPromptKey,
			wantDuo:     azureSpeechAudioBriefingDuoPreprocessPromptKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := summaryPreprocessPromptKeyForProvider(tt.provider); got != tt.wantSummary {
				t.Fatalf("summaryPreprocessPromptKeyForProvider(%q) = %q, want %q", tt.provider, got, tt.wantSummary)
			}
			if got := audioBriefingSinglePreprocessPromptKeyForProvider(tt.provider); got != tt.wantSingle {
				t.Fatalf("audioBriefingSinglePreprocessPromptKeyForProvider(%q) = %q, want %q", tt.provider, got, tt.wantSingle)
			}
			if got := audioBriefingDuoPreprocessPromptKeyForProvider(tt.provider); got != tt.wantDuo {
				t.Fatalf("audioBriefingDuoPreprocessPromptKeyForProvider(%q) = %q, want %q", tt.provider, got, tt.wantDuo)
			}
		})
	}
}

func TestTTSMarkupPreprocessPromptKeyFamilyUnknownProviderReturnsEmptyPromptKey(t *testing.T) {
	if got := summaryPreprocessPromptKeyForProvider("custom"); got != "" {
		t.Fatalf("summaryPreprocessPromptKeyForProvider(custom) = %q, want empty", got)
	}
	if got := audioBriefingSinglePreprocessPromptKeyForProvider("custom"); got != "" {
		t.Fatalf("audioBriefingSinglePreprocessPromptKeyForProvider(custom) = %q, want empty", got)
	}
	if got := audioBriefingDuoPreprocessPromptKeyForProvider("custom"); got != "" {
		t.Fatalf("audioBriefingDuoPreprocessPromptKeyForProvider(custom) = %q, want empty", got)
	}
}

func TestTTSMarkupPreprocessPurposeRoutingByPromptFamily(t *testing.T) {
	tests := []struct {
		name      string
		promptKey string
		want      string
	}{
		{name: "fish summary", promptKey: fishSummaryPreprocessPromptKey, want: fishPreprocessPurpose},
		{name: "fish briefing single", promptKey: fishAudioBriefingSinglePreprocessPromptKey, want: fishPreprocessPurpose},
		{name: "fish briefing duo", promptKey: fishAudioBriefingDuoPreprocessPromptKey, want: fishPreprocessPurpose},
		{name: "gemini summary", promptKey: geminiSummaryPreprocessPromptKey, want: geminiTTSPreprocessPurpose},
		{name: "elevenlabs summary", promptKey: elevenLabsSummaryPreprocessPromptKey, want: elevenLabsTTSPreprocessPurpose},
		{name: "xai summary", promptKey: xaiSummaryPreprocessPromptKey, want: xaiTTSPreprocessPurpose},
		{name: "azure summary", promptKey: azureSpeechSummaryPreprocessPromptKey, want: azureSpeechTTSPreprocessPurpose},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := preprocessPurposeForPromptKey(tt.promptKey)
			if !ok {
				t.Fatalf("preprocessPurposeForPromptKey(%q) reported unknown prompt key", tt.promptKey)
			}
			if got != tt.want {
				t.Fatalf("preprocessPurposeForPromptKey(%q) = %q, want %q", tt.promptKey, got, tt.want)
			}
		})
	}
}

func TestTTSMarkupPreprocessPurposeRoutingRejectsUnknownPromptFamily(t *testing.T) {
	if got, ok := preprocessPurposeForPromptKey("custom.summary_preprocess"); ok || got != "" {
		t.Fatalf("preprocessPurposeForPromptKey(custom.summary_preprocess) = (%q, %v), want empty,false", got, ok)
	}
}

package service

import (
	"context"
	"testing"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

type fishPreprocessUsageRepoStub struct {
	input repository.LLMUsageLogInput
}

func (s *fishPreprocessUsageRepoStub) Insert(_ context.Context, in repository.LLMUsageLogInput) error {
	s.input = in
	return nil
}

func TestRecordFishPreprocessLLMUsageUsesSeparatePromptMetadata(t *testing.T) {
	repo := &fishPreprocessUsageRepoStub{}
	userID := "00000000-0000-4000-8000-000000000021"
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

	recordFishPreprocessLLMUsage(context.Background(), repo, NoopJSONCache{}, usage, &userID, &itemID, fishSummaryPreprocessPromptKey)

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

type fishPreprocessWorkerStub struct {
	gotAPIKey *string
	response  *FishSpeechPreprocessResponse
}

func (s *fishPreprocessWorkerStub) PreprocessFishSpeechText(_ context.Context, _ string, _ string, _ string, apiKey *string) (*FishSpeechPreprocessResponse, error) {
	s.gotAPIKey = apiKey
	if s.response != nil {
		return s.response, nil
	}
	return &FishSpeechPreprocessResponse{Text: "[自然に] テスト"}, nil
}

func TestFishSpeechPreprocessUsesOnlySelectedProviderKey(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "fish-preprocess-test-key")
	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	lockSettingsServiceTestDB(t, db)

	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000021"
	if _, err := db.Exec(ctx, `DELETE FROM llm_usage_logs WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset llm_usage_logs: %v", err)
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

	worker := &fishPreprocessWorkerStub{}
	service := NewFishSpeechPreprocessService(repo, cipher, worker, repository.NewLLMUsageLogRepo(db), NoopJSONCache{})

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

func TestFishSpeechPreprocessRequiresConfiguredModel(t *testing.T) {
	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	lockSettingsServiceTestDB(t, db)

	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000021"
	if _, err := db.Exec(ctx, `DELETE FROM user_settings WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset user_settings: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("reset users: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO users (id, email, name) VALUES ($1, $2, $3)`, userID, "fish-preprocess-missing@example.com", "Fish Preprocess"); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	service := NewFishSpeechPreprocessService(repository.NewUserSettingsRepo(db), NewSecretCipher(), &fishPreprocessWorkerStub{}, repository.NewLLMUsageLogRepo(db), NoopJSONCache{})
	_, err = service.PreprocessSummaryAudioText(ctx, userID, "", "元テキスト")
	if err == nil || err != ErrFishPreprocessModelNotConfigured {
		t.Fatalf("PreprocessSummaryAudioText() error = %v, want %v", err, ErrFishPreprocessModelNotConfigured)
	}
}

func TestFishSpeechPreprocessRejectsEmptyOutput(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "fish-preprocess-empty-output-test-key")
	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	lockSettingsServiceTestDB(t, db)

	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000021"
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

	service := NewFishSpeechPreprocessService(repo, cipher, &fishPreprocessWorkerStub{
		response: &FishSpeechPreprocessResponse{Text: "   "},
	}, repository.NewLLMUsageLogRepo(db), NoopJSONCache{})
	_, err = service.PreprocessSummaryAudioText(ctx, userID, "", "元テキスト")
	if err == nil || err != ErrFishPreprocessEmptyOutput {
		t.Fatalf("PreprocessSummaryAudioText() error = %v, want %v", err, ErrFishPreprocessEmptyOutput)
	}
}

package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
)

const (
	fishSummaryPreprocessPromptKey             = "fish.summary_preprocess"
	fishAudioBriefingSinglePreprocessPromptKey = "fish.audio_briefing_single_preprocess"
	fishAudioBriefingDuoPreprocessPromptKey    = "fish.audio_briefing_duo_preprocess"
	fishPreprocessPurpose                      = "fish_preprocess"
	fishPreprocessPromptSource                 = "shared_template"
)

var (
	ErrFishPreprocessModelNotConfigured = fmt.Errorf("fish preprocess model is not configured")
	ErrFishPreprocessEmptyOutput        = fmt.Errorf("fish preprocess returned empty text")
)

type FishSpeechPreprocessResult struct {
	Text string
	LLM  *LLMUsage
}

type fishSpeechPreprocessWorker interface {
	PreprocessFishSpeechText(
		ctx context.Context,
		text string,
		model string,
		promptKey string,
		variables map[string]string,
		apiKey *string,
	) (*FishSpeechPreprocessResponse, error)
}

type FishSpeechPreprocessService struct {
	userSettings *repository.UserSettingsRepo
	cipher       *SecretCipher
	worker       fishSpeechPreprocessWorker
	llmUsage     *repository.LLMUsageLogRepo
	cache        JSONCache
}

func NewFishSpeechPreprocessService(
	userSettings *repository.UserSettingsRepo,
	cipher *SecretCipher,
	worker fishSpeechPreprocessWorker,
	llmUsage *repository.LLMUsageLogRepo,
	cache JSONCache,
) *FishSpeechPreprocessService {
	return &FishSpeechPreprocessService{
		userSettings: userSettings,
		cipher:       cipher,
		worker:       worker,
		llmUsage:     llmUsage,
		cache:        cache,
	}
}

func (s *FishSpeechPreprocessService) PreprocessSummaryAudioText(ctx context.Context, userID, itemID, text string) (*FishSpeechPreprocessResult, error) {
	return s.Preprocess(ctx, userID, itemID, fishSummaryPreprocessPromptKey, text, nil)
}

func (s *FishSpeechPreprocessService) PreprocessAudioBriefingSingleText(ctx context.Context, userID, itemID, persona, text string) (*FishSpeechPreprocessResult, error) {
	return s.Preprocess(ctx, userID, itemID, fishAudioBriefingSinglePreprocessPromptKey, text, map[string]string{
		"persona_name": strings.TrimSpace(persona),
	})
}

func (s *FishSpeechPreprocessService) PreprocessAudioBriefingDuoText(ctx context.Context, userID, itemID, hostPersona, partnerPersona, text string) (*FishSpeechPreprocessResult, error) {
	return s.Preprocess(ctx, userID, itemID, fishAudioBriefingDuoPreprocessPromptKey, text, map[string]string{
		"host_persona_name":    strings.TrimSpace(hostPersona),
		"partner_persona_name": strings.TrimSpace(partnerPersona),
	})
}

func (s *FishSpeechPreprocessService) Preprocess(ctx context.Context, userID, itemID, promptKey, text string, variables map[string]string) (*FishSpeechPreprocessResult, error) {
	trimmedText := strings.TrimSpace(text)
	if trimmedText == "" {
		return &FishSpeechPreprocessResult{Text: text}, nil
	}
	if s == nil || s.userSettings == nil || s.worker == nil {
		return &FishSpeechPreprocessResult{Text: text}, nil
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return &FishSpeechPreprocessResult{Text: text}, nil
	}
	settings, err := s.userSettings.EnsureDefaults(ctx, userID)
	if err != nil {
		return nil, err
	}
	modelName := strings.TrimSpace(derefString(settings.FishPreprocessModel))
	if modelName == "" {
		return nil, ErrFishPreprocessModelNotConfigured
	}
	provider := LLMProviderForModel(&modelName)
	if !hasFishPreprocessProviderKey(settings, provider) {
		return nil, fmt.Errorf("%s api key is not configured", provider)
	}
	promptKey = strings.TrimSpace(promptKey)
	if promptKey == "" {
		promptKey = fishSummaryPreprocessPromptKey
	}

	apiKey, err := s.loadProviderKey(ctx, userID, provider)
	if err != nil {
		return nil, err
	}
	traceCtx := WithWorkerTraceMetadata(ctx, fishPreprocessPurpose, &userID, nil, &itemID, nil)
	resp, err := s.worker.PreprocessFishSpeechText(
		traceCtx,
		text,
		modelName,
		promptKey,
		variables,
		apiKey,
	)
	if err != nil {
		return nil, err
	}
	recordFishPreprocessLLMUsage(ctx, s.llmUsage, s.cache, resp.LLM, &userID, stringPtrOrNil(itemID), strings.TrimSpace(promptKey))
	if strings.TrimSpace(resp.Text) == "" {
		return nil, ErrFishPreprocessEmptyOutput
	}
	return &FishSpeechPreprocessResult{Text: resp.Text, LLM: resp.LLM}, nil
}

func (s *FishSpeechPreprocessService) loadProviderKey(ctx context.Context, userID, provider string) (*string, error) {
	if s == nil || s.userSettings == nil {
		return nil, fmt.Errorf("fish preprocess settings repo is not configured")
	}
	switch strings.TrimSpace(provider) {
	case "google":
		return loadAndDecryptAudioBriefingUserSecret(ctx, s.userSettings.GetGoogleAPIKeyEncrypted, s.cipher, userID, "google api key is not configured")
	case "groq":
		return loadAndDecryptAudioBriefingUserSecret(ctx, s.userSettings.GetGroqAPIKeyEncrypted, s.cipher, userID, "groq api key is not configured")
	case "deepseek":
		return loadAndDecryptAudioBriefingUserSecret(ctx, s.userSettings.GetDeepSeekAPIKeyEncrypted, s.cipher, userID, "deepseek api key is not configured")
	case "alibaba":
		return loadAndDecryptAudioBriefingUserSecret(ctx, s.userSettings.GetAlibabaAPIKeyEncrypted, s.cipher, userID, "alibaba api key is not configured")
	case "mistral":
		return loadAndDecryptAudioBriefingUserSecret(ctx, s.userSettings.GetMistralAPIKeyEncrypted, s.cipher, userID, "mistral api key is not configured")
	case "moonshot":
		return loadAndDecryptAudioBriefingUserSecret(ctx, s.userSettings.GetMoonshotAPIKeyEncrypted, s.cipher, userID, "moonshot api key is not configured")
	case "xai":
		return loadAndDecryptAudioBriefingUserSecret(ctx, s.userSettings.GetXAIAPIKeyEncrypted, s.cipher, userID, "xai api key is not configured")
	case "zai":
		return loadAndDecryptAudioBriefingUserSecret(ctx, s.userSettings.GetZAIAPIKeyEncrypted, s.cipher, userID, "zai api key is not configured")
	case "fireworks":
		return loadAndDecryptAudioBriefingUserSecret(ctx, s.userSettings.GetFireworksAPIKeyEncrypted, s.cipher, userID, "fireworks api key is not configured")
	case "openai":
		return loadAndDecryptAudioBriefingUserSecret(ctx, s.userSettings.GetOpenAIAPIKeyEncrypted, s.cipher, userID, "openai api key is not configured")
	case "openrouter":
		return loadAndDecryptAudioBriefingUserSecret(ctx, s.userSettings.GetOpenRouterAPIKeyEncrypted, s.cipher, userID, "openrouter api key is not configured")
	case "poe":
		return loadAndDecryptAudioBriefingUserSecret(ctx, s.userSettings.GetPoeAPIKeyEncrypted, s.cipher, userID, "poe api key is not configured")
	case "siliconflow":
		return loadAndDecryptAudioBriefingUserSecret(ctx, s.userSettings.GetSiliconFlowAPIKeyEncrypted, s.cipher, userID, "siliconflow api key is not configured")
	default:
		return loadAndDecryptAudioBriefingUserSecret(ctx, s.userSettings.GetAnthropicAPIKeyEncrypted, s.cipher, userID, "anthropic api key is not configured")
	}
}

func hasFishPreprocessProviderKey(settings *model.UserSettings, provider string) bool {
	if settings == nil {
		return false
	}
	switch strings.TrimSpace(provider) {
	case "google":
		return settings.HasGoogleAPIKey
	case "groq":
		return settings.HasGroqAPIKey
	case "deepseek":
		return settings.HasDeepSeekAPIKey
	case "alibaba":
		return settings.HasAlibabaAPIKey
	case "mistral":
		return settings.HasMistralAPIKey
	case "moonshot":
		return settings.HasMoonshotAPIKey
	case "xai":
		return settings.HasXAIAPIKey
	case "zai":
		return settings.HasZAIAPIKey
	case "fireworks":
		return settings.HasFireworksAPIKey
	case "openai":
		return settings.HasOpenAIAPIKey
	case "openrouter":
		return settings.HasOpenRouterAPIKey
	case "poe":
		return settings.HasPoeAPIKey
	case "siliconflow":
		return settings.HasSiliconFlowAPIKey
	default:
		return settings.HasAnthropicAPIKey
	}
}

type fishPreprocessLLMUsageRepo interface {
	Insert(ctx context.Context, in repository.LLMUsageLogInput) error
}

func recordFishPreprocessLLMUsage(ctx context.Context, repo fishPreprocessLLMUsageRepo, cache JSONCache, usage *LLMUsage, userID, itemID *string, promptKey string) {
	usage = NormalizeCatalogPricedUsage(fishPreprocessPurpose, usage)
	if repo == nil || usage == nil || userID == nil || *userID == "" {
		return
	}
	promptKey = strings.TrimSpace(promptKey)
	if promptKey == "" {
		promptKey = fishSummaryPreprocessPromptKey
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%s|%d|%d|%d|%d", fishPreprocessPurpose, promptKey, usage.Provider, usage.Model, *userID, usage.InputTokens, usage.OutputTokens, usage.CacheCreationInputTokens, usage.CacheReadInputTokens)))
	key := hex.EncodeToString(sum[:])
	pricingSource := usage.PricingSource
	if pricingSource == "" {
		pricingSource = "unknown"
	}
	if err := repo.Insert(ctx, repository.LLMUsageLogInput{
		IdempotencyKey:           &key,
		UserID:                   userID,
		ItemID:                   itemID,
		PromptKey:                promptKey,
		PromptSource:             fishPreprocessPromptSource,
		Provider:                 usage.Provider,
		Model:                    usage.Model,
		RequestedModel:           usage.RequestedModel,
		ResolvedModel:            usage.ResolvedModel,
		PricingModelFamily:       usage.PricingModelFamily,
		PricingSource:            pricingSource,
		OpenRouterCostUSD:        usage.OpenRouterCostUSD,
		OpenRouterGenerationID:   strings.TrimSpace(usage.OpenRouterGenerationID),
		Purpose:                  fishPreprocessPurpose,
		InputTokens:              usage.InputTokens,
		OutputTokens:             usage.OutputTokens,
		CacheCreationInputTokens: usage.CacheCreationInputTokens,
		CacheReadInputTokens:     usage.CacheReadInputTokens,
		EstimatedCostUSD:         usage.EstimatedCostUSD,
	}); err == nil {
		_ = BumpUserLLMUsageCacheVersion(ctx, cache, *userID)
	} else {
		log.Printf("llm usage insert failed purpose=%s user_id=%s provider=%s model=%s err=%v", fishPreprocessPurpose, *userID, usage.Provider, usage.Model, err)
	}
}

func stringPtrOrNil(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}

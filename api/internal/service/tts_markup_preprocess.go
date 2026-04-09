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
	fishSummaryPreprocessPromptKey                   = "fish.summary_preprocess"
	fishAudioBriefingSinglePreprocessPromptKey       = "fish.audio_briefing_single_preprocess"
	fishAudioBriefingDuoPreprocessPromptKey          = "fish.audio_briefing_duo_preprocess"
	geminiSummaryPreprocessPromptKey                 = "gemini.summary_preprocess"
	geminiAudioBriefingSinglePreprocessPromptKey     = "gemini.audio_briefing_single_preprocess"
	geminiAudioBriefingDuoPreprocessPromptKey        = "gemini.audio_briefing_duo_preprocess"
	elevenLabsSummaryPreprocessPromptKey             = "elevenlabs.summary_preprocess"
	elevenLabsAudioBriefingSinglePreprocessPromptKey = "elevenlabs.audio_briefing_single_preprocess"
	elevenLabsAudioBriefingDuoPreprocessPromptKey    = "elevenlabs.audio_briefing_duo_preprocess"
	fishPreprocessPurpose                            = "fish_preprocess"
	geminiTTSPreprocessPurpose                       = "gemini_tts_preprocess"
	elevenLabsTTSPreprocessPurpose                   = "elevenlabs_tts_preprocess"
	fishPreprocessPromptSource                       = "shared_template"
)

var (
	ErrTTSMarkupPreprocessModelNotConfigured = fmt.Errorf("tts markup preprocess model is not configured")
	ErrTTSMarkupPreprocessEmptyOutput        = fmt.Errorf("tts markup preprocess returned empty text")
)

type TTSMarkupPreprocessResult struct {
	Text string
	LLM  *LLMUsage
}

type ttsMarkupPreprocessWorker interface {
	PreprocessTTSMarkupText(
		ctx context.Context,
		text string,
		model string,
		promptKey string,
		variables map[string]string,
		apiKey *string,
	) (*TTSMarkupPreprocessResponse, error)
}

type TTSMarkupPreprocessService struct {
	userSettings *repository.UserSettingsRepo
	cipher       *SecretCipher
	worker       ttsMarkupPreprocessWorker
	llmUsage     *repository.LLMUsageLogRepo
	cache        JSONCache
}

func NewTTSMarkupPreprocessService(
	userSettings *repository.UserSettingsRepo,
	cipher *SecretCipher,
	worker ttsMarkupPreprocessWorker,
	llmUsage *repository.LLMUsageLogRepo,
	cache JSONCache,
) *TTSMarkupPreprocessService {
	return &TTSMarkupPreprocessService{
		userSettings: userSettings,
		cipher:       cipher,
		worker:       worker,
		llmUsage:     llmUsage,
		cache:        cache,
	}
}

func (s *TTSMarkupPreprocessService) PreprocessSummaryAudioText(ctx context.Context, userID, itemID, text string) (*TTSMarkupPreprocessResult, error) {
	return s.Preprocess(ctx, userID, itemID, fishSummaryPreprocessPromptKey, text, nil)
}

func (s *TTSMarkupPreprocessService) PreprocessSummaryAudioTextForProvider(ctx context.Context, userID, itemID, provider, text string) (*TTSMarkupPreprocessResult, error) {
	return s.Preprocess(ctx, userID, itemID, summaryPreprocessPromptKeyForProvider(provider), text, nil)
}

func (s *TTSMarkupPreprocessService) PreprocessAudioBriefingSingleText(ctx context.Context, userID, itemID, persona, text string) (*TTSMarkupPreprocessResult, error) {
	return s.Preprocess(ctx, userID, itemID, fishAudioBriefingSinglePreprocessPromptKey, text, map[string]string{
		"persona_name": strings.TrimSpace(persona),
	})
}

func (s *TTSMarkupPreprocessService) PreprocessAudioBriefingSingleTextForProvider(ctx context.Context, userID, itemID, provider, persona, text string) (*TTSMarkupPreprocessResult, error) {
	return s.Preprocess(ctx, userID, itemID, audioBriefingSinglePreprocessPromptKeyForProvider(provider), text, map[string]string{
		"persona_name": strings.TrimSpace(persona),
	})
}

func (s *TTSMarkupPreprocessService) PreprocessAudioBriefingDuoText(ctx context.Context, userID, itemID, hostPersona, partnerPersona, text string) (*TTSMarkupPreprocessResult, error) {
	return s.Preprocess(ctx, userID, itemID, fishAudioBriefingDuoPreprocessPromptKey, text, map[string]string{
		"host_persona_name":    strings.TrimSpace(hostPersona),
		"partner_persona_name": strings.TrimSpace(partnerPersona),
	})
}

func (s *TTSMarkupPreprocessService) PreprocessAudioBriefingDuoTextForProvider(ctx context.Context, userID, itemID, provider, hostPersona, partnerPersona, text string) (*TTSMarkupPreprocessResult, error) {
	return s.Preprocess(ctx, userID, itemID, audioBriefingDuoPreprocessPromptKeyForProvider(provider), text, map[string]string{
		"host_persona_name":    strings.TrimSpace(hostPersona),
		"partner_persona_name": strings.TrimSpace(partnerPersona),
	})
}

func (s *TTSMarkupPreprocessService) Preprocess(ctx context.Context, userID, itemID, promptKey, text string, variables map[string]string) (*TTSMarkupPreprocessResult, error) {
	trimmedText := strings.TrimSpace(text)
	if trimmedText == "" {
		return &TTSMarkupPreprocessResult{Text: text}, nil
	}
	if s == nil || s.userSettings == nil || s.worker == nil {
		return &TTSMarkupPreprocessResult{Text: text}, nil
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return &TTSMarkupPreprocessResult{Text: text}, nil
	}
	settings, err := s.userSettings.EnsureDefaults(ctx, userID)
	if err != nil {
		return nil, err
	}
	modelName := strings.TrimSpace(derefString(settings.TTSMarkupPreprocessModel))
	if modelName == "" {
		return nil, ErrTTSMarkupPreprocessModelNotConfigured
	}
	provider := LLMProviderForModel(&modelName)
	if !hasFishPreprocessProviderKey(settings, provider) {
		return nil, fmt.Errorf("%s api key is not configured", provider)
	}
	promptKey = strings.TrimSpace(promptKey)
	if promptKey == "" {
		promptKey = fishSummaryPreprocessPromptKey
	}
	purpose := preprocessPurposeForPromptKey(promptKey)

	apiKey, err := s.loadProviderKey(ctx, userID, provider)
	if err != nil {
		return nil, err
	}
	traceCtx := WithWorkerTraceMetadata(ctx, purpose, &userID, nil, &itemID, nil)
	resp, err := s.worker.PreprocessTTSMarkupText(
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
	recordTTSMarkupPreprocessLLMUsage(ctx, s.llmUsage, s.cache, resp.LLM, &userID, stringPtrOrNil(itemID), strings.TrimSpace(promptKey), purpose)
	if strings.TrimSpace(resp.Text) == "" {
		return nil, ErrTTSMarkupPreprocessEmptyOutput
	}
	return &TTSMarkupPreprocessResult{Text: resp.Text, LLM: resp.LLM}, nil
}

func (s *TTSMarkupPreprocessService) loadProviderKey(ctx context.Context, userID, provider string) (*string, error) {
	if s == nil || s.userSettings == nil {
		return nil, fmt.Errorf("tts markup preprocess settings repo is not configured")
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
	case "together":
		return loadAndDecryptAudioBriefingUserSecret(ctx, s.userSettings.GetTogetherAPIKeyEncrypted, s.cipher, userID, "together api key is not configured")
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

func summaryPreprocessPromptKeyForProvider(provider string) string {
	switch strings.TrimSpace(strings.ToLower(provider)) {
	case "gemini_tts":
		return geminiSummaryPreprocessPromptKey
	case "elevenlabs":
		return elevenLabsSummaryPreprocessPromptKey
	}
	return fishSummaryPreprocessPromptKey
}

func audioBriefingSinglePreprocessPromptKeyForProvider(provider string) string {
	switch strings.TrimSpace(strings.ToLower(provider)) {
	case "gemini_tts":
		return geminiAudioBriefingSinglePreprocessPromptKey
	case "elevenlabs":
		return elevenLabsAudioBriefingSinglePreprocessPromptKey
	}
	return fishAudioBriefingSinglePreprocessPromptKey
}

func audioBriefingDuoPreprocessPromptKeyForProvider(provider string) string {
	switch strings.TrimSpace(strings.ToLower(provider)) {
	case "gemini_tts":
		return geminiAudioBriefingDuoPreprocessPromptKey
	case "elevenlabs":
		return elevenLabsAudioBriefingDuoPreprocessPromptKey
	}
	return fishAudioBriefingDuoPreprocessPromptKey
}

func preprocessPurposeForPromptKey(promptKey string) string {
	switch {
	case strings.HasPrefix(strings.TrimSpace(promptKey), "gemini."):
		return geminiTTSPreprocessPurpose
	case strings.HasPrefix(strings.TrimSpace(promptKey), "elevenlabs."):
		return elevenLabsTTSPreprocessPurpose
	}
	return fishPreprocessPurpose
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
	case "together":
		return settings.HasTogetherAPIKey
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

type ttsMarkupPreprocessLLMUsageRepo interface {
	Insert(ctx context.Context, in repository.LLMUsageLogInput) error
}

func recordTTSMarkupPreprocessLLMUsage(ctx context.Context, repo ttsMarkupPreprocessLLMUsageRepo, cache JSONCache, usage *LLMUsage, userID, itemID *string, promptKey string, purpose string) {
	usage = NormalizeCatalogPricedUsage(purpose, usage)
	if repo == nil || usage == nil || userID == nil || *userID == "" {
		return
	}
	promptKey = strings.TrimSpace(promptKey)
	if promptKey == "" {
		promptKey = fishSummaryPreprocessPromptKey
	}
	purpose = strings.TrimSpace(purpose)
	if purpose == "" {
		purpose = fishPreprocessPurpose
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%s|%d|%d|%d|%d", purpose, promptKey, usage.Provider, usage.Model, *userID, usage.InputTokens, usage.OutputTokens, usage.CacheCreationInputTokens, usage.CacheReadInputTokens)))
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
		Purpose:                  purpose,
		InputTokens:              usage.InputTokens,
		OutputTokens:             usage.OutputTokens,
		CacheCreationInputTokens: usage.CacheCreationInputTokens,
		CacheReadInputTokens:     usage.CacheReadInputTokens,
		EstimatedCostUSD:         usage.EstimatedCostUSD,
	}); err == nil {
		_ = BumpUserLLMUsageCacheVersion(ctx, cache, *userID)
	} else {
		log.Printf("llm usage insert failed purpose=%s user_id=%s provider=%s model=%s err=%v", purpose, *userID, usage.Provider, usage.Model, err)
	}
}

func stringPtrOrNil(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}

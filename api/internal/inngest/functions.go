package inngest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
	"github.com/inngest/inngest/pkg/enums"
	"github.com/inngest/inngestgo"
	"github.com/inngest/inngestgo/step"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcdole/gofeed"
)

var llmUsageCache service.JSONCache

type llmExecutionTriggerContextKey struct{}

type llmExecutionTrigger struct {
	TriggerID     string
	TriggerReason string
}

func withLLMExecutionTrigger(ctx context.Context, triggerID, triggerReason string) context.Context {
	if strings.TrimSpace(triggerID) == "" {
		return ctx
	}
	return context.WithValue(ctx, llmExecutionTriggerContextKey{}, llmExecutionTrigger{
		TriggerID:     strings.TrimSpace(triggerID),
		TriggerReason: strings.TrimSpace(triggerReason),
	})
}

func llmExecutionTriggerFromContext(ctx context.Context) *llmExecutionTrigger {
	v, ok := ctx.Value(llmExecutionTriggerContextKey{}).(llmExecutionTrigger)
	if !ok || strings.TrimSpace(v.TriggerID) == "" {
		return nil
	}
	return &v
}

func recordLLMUsage(ctx context.Context, repo *repository.LLMUsageLogRepo, purpose string, usage *service.LLMUsage, userID, sourceID, itemID, digestID *string) {
	usage = service.NormalizeCatalogPricedUsage(purpose, usage)
	if repo == nil || usage == nil {
		return
	}
	storedModel := llmUsageStoredModel(usage)
	if usage.Provider == "" || storedModel == "" {
		return
	}
	idempotencyKey := llmUsageIdempotencyKey(purpose, usage, userID, sourceID, itemID, digestID)
	pricingSource := usage.PricingSource
	if pricingSource == "" {
		pricingSource = "unknown"
	}
	if err := repo.Insert(ctx, repository.LLMUsageLogInput{
		IdempotencyKey:           &idempotencyKey,
		UserID:                   userID,
		SourceID:                 sourceID,
		ItemID:                   itemID,
		DigestID:                 digestID,
		Provider:                 usage.Provider,
		Model:                    storedModel,
		RequestedModel:           strings.TrimSpace(usage.RequestedModel),
		ResolvedModel:            strings.TrimSpace(usage.ResolvedModel),
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
	}); err != nil {
		log.Printf("record llm usage purpose=%s: %v", purpose, err)
		return
	}
	if userID != nil {
		_ = service.BumpUserLLMUsageCacheVersion(ctx, llmUsageCache, *userID)
	}
}

func llmUsageStoredModel(usage *service.LLMUsage) string {
	if usage == nil {
		return ""
	}
	if strings.TrimSpace(usage.Provider) == "openrouter" && strings.TrimSpace(usage.ResolvedModel) != "" {
		return strings.TrimSpace(usage.ResolvedModel)
	}
	return strings.TrimSpace(usage.Model)
}

func recordLLMExecutionSuccess(ctx context.Context, repo *repository.LLMExecutionEventRepo, purpose string, usage *service.LLMUsage, attemptIndex int, userID, sourceID, itemID, digestID *string) {
	if repo == nil || usage == nil {
		return
	}
	if usage.Provider == "" || usage.Model == "" {
		return
	}
	var idempotencyKey *string
	var triggerID *string
	var triggerReason *string
	if trigger := llmExecutionTriggerFromContext(ctx); trigger != nil {
		key := llmExecutionEventIdempotencyKey(repository.LLMExecutionEventInput{
			UserID:        userID,
			SourceID:      sourceID,
			ItemID:        itemID,
			DigestID:      digestID,
			TriggerID:     &trigger.TriggerID,
			TriggerReason: triggerReason,
			Provider:      usage.Provider,
			Model:         usage.Model,
			Purpose:       purpose,
			Status:        "success",
			AttemptIndex:  attemptIndex,
		})
		idempotencyKey = &key
		triggerID = &trigger.TriggerID
		if strings.TrimSpace(trigger.TriggerReason) != "" {
			v := strings.TrimSpace(trigger.TriggerReason)
			triggerReason = &v
		}
	}
	if err := repo.Insert(ctx, repository.LLMExecutionEventInput{
		IdempotencyKey: idempotencyKey,
		UserID:         userID,
		SourceID:       sourceID,
		ItemID:         itemID,
		DigestID:       digestID,
		TriggerID:      triggerID,
		TriggerReason:  triggerReason,
		Provider:       usage.Provider,
		Model:          usage.Model,
		Purpose:        purpose,
		Status:         "success",
		AttemptIndex:   attemptIndex,
	}); err != nil {
		log.Printf("record llm execution success purpose=%s: %v", purpose, err)
		return
	}
	if userID != nil {
		_ = service.BumpUserLLMUsageCacheVersion(ctx, llmUsageCache, *userID)
	}
}

func recordLLMExecutionFailure(ctx context.Context, repo *repository.LLMExecutionEventRepo, purpose string, model *string, attemptIndex int, userID, sourceID, itemID, digestID *string, err error) {
	if repo == nil || model == nil || strings.TrimSpace(*model) == "" || err == nil {
		return
	}
	modelVal := strings.TrimSpace(*model)
	provider := service.LLMProviderForModel(&modelVal)
	errorKind, emptyResponse := classifyLLMExecutionError(err)
	message := err.Error()
	if len(message) > 500 {
		message = message[:500]
	}
	var idempotencyKey *string
	var triggerID *string
	var triggerReason *string
	if trigger := llmExecutionTriggerFromContext(ctx); trigger != nil {
		key := llmExecutionEventIdempotencyKey(repository.LLMExecutionEventInput{
			UserID:        userID,
			SourceID:      sourceID,
			ItemID:        itemID,
			DigestID:      digestID,
			TriggerID:     &trigger.TriggerID,
			TriggerReason: triggerReason,
			Provider:      provider,
			Model:         modelVal,
			Purpose:       purpose,
			Status:        "failure",
			AttemptIndex:  attemptIndex,
			EmptyResponse: emptyResponse,
			ErrorKind:     &errorKind,
			ErrorMessage:  &message,
		})
		idempotencyKey = &key
		triggerID = &trigger.TriggerID
		if strings.TrimSpace(trigger.TriggerReason) != "" {
			v := strings.TrimSpace(trigger.TriggerReason)
			triggerReason = &v
		}
	}
	if err := repo.Insert(ctx, repository.LLMExecutionEventInput{
		IdempotencyKey: idempotencyKey,
		UserID:         userID,
		SourceID:       sourceID,
		ItemID:         itemID,
		DigestID:       digestID,
		TriggerID:      triggerID,
		TriggerReason:  triggerReason,
		Provider:       provider,
		Model:          modelVal,
		Purpose:        purpose,
		Status:         "failure",
		AttemptIndex:   attemptIndex,
		EmptyResponse:  emptyResponse,
		ErrorKind:      &errorKind,
		ErrorMessage:   &message,
	}); err != nil {
		log.Printf("record llm execution failure purpose=%s: %v", purpose, err)
		return
	}
	if userID != nil {
		_ = service.BumpUserLLMUsageCacheVersion(ctx, llmUsageCache, toVal(userID))
	}
}

func recordLLMExecutionFailuresFromUsage(ctx context.Context, repo *repository.LLMExecutionEventRepo, purpose string, usage *service.LLMUsage, attemptIndex int, userID, sourceID, itemID, digestID *string) {
	if repo == nil || usage == nil || len(usage.ExecutionFailures) == 0 {
		return
	}
	for _, failure := range usage.ExecutionFailures {
		model := strings.TrimSpace(failure.Model)
		if model == "" {
			continue
		}
		reason := strings.TrimSpace(failure.Reason)
		if reason == "" {
			reason = "worker internal fallback"
		}
		recordLLMExecutionFailure(ctx, repo, purpose, &model, attemptIndex, userID, sourceID, itemID, digestID, fmt.Errorf("%s", reason))
	}
}

func llmExecutionEventIdempotencyKey(in repository.LLMExecutionEventInput) string {
	raw := fmt.Sprintf(
		"trigger=%s|reason=%s|purpose=%s|provider=%s|model=%s|status=%s|attempt=%d|u=%s|s=%s|i=%s|d=%s|empty=%t|ek=%s|em=%s",
		toVal(in.TriggerID),
		toVal(in.TriggerReason),
		in.Purpose,
		in.Provider,
		in.Model,
		in.Status,
		in.AttemptIndex,
		toVal(in.UserID),
		toVal(in.SourceID),
		toVal(in.ItemID),
		toVal(in.DigestID),
		in.EmptyResponse,
		toVal(in.ErrorKind),
		toVal(in.ErrorMessage),
	)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func toVal(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func classifyLLMExecutionError(err error) (string, bool) {
	s := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(s, "response_snippet=(empty)"),
		strings.Contains(s, "returned nil response"),
		strings.Contains(s, "empty summary"),
		strings.Contains(s, "empty facts"),
		strings.Contains(s, "empty response"):
		return "empty_response", true
	case strings.Contains(s, "short_comment missing"),
		strings.Contains(s, "parse failed"),
		strings.Contains(s, "json"):
		return "parse_error", false
	case strings.Contains(s, "looks truncated"),
		strings.Contains(s, "incomplete after"),
		strings.Contains(s, "compose digest incomplete"):
		return "incomplete_output", false
	case strings.Contains(s, "timeout"),
		strings.Contains(s, "deadline exceeded"):
		return "timeout", false
	default:
		return "worker_error", false
	}
}

func llmUsageIdempotencyKey(purpose string, usage *service.LLMUsage, userID, sourceID, itemID, digestID *string) string {
	model := llmUsageStoredModel(usage)
	raw := fmt.Sprintf(
		"purpose=%s|provider=%s|model=%s|u=%s|s=%s|i=%s|d=%s|in=%d|out=%d|cw=%d|cr=%d",
		purpose,
		usage.Provider,
		model,
		toVal(userID),
		toVal(sourceID),
		toVal(itemID),
		toVal(digestID),
		usage.InputTokens,
		usage.OutputTokens,
		usage.CacheCreationInputTokens,
		usage.CacheReadInputTokens,
	)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func digestTextLooksComplete(text string, minLen int) bool {
	s := strings.TrimSpace(text)
	if len([]rune(s)) < minLen {
		return false
	}
	if strings.Count(s, "```")%2 != 0 {
		return false
	}
	last := []rune(s)[len([]rune(s))-1]
	switch last {
	case '。', '！', '？', '.', '!', '?', '」', '』':
		return true
	default:
		return false
	}
}

func digestClusterDraftValidationReason(text string) string {
	s := strings.TrimSpace(text)
	if len([]rune(s)) < 40 {
		return "too_short"
	}
	if strings.Count(s, "```")%2 != 0 {
		return "unclosed_code_fence"
	}
	lines := strings.Split(s, "\n")
	bullets := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		bullets = append(bullets, line)
	}
	if len(bullets) < 2 {
		return "too_few_lines"
	}
	last := bullets[len(bullets)-1]
	if strings.HasPrefix(last, "-") || strings.HasPrefix(last, "・") || strings.HasPrefix(last, "•") {
		trimmed := strings.TrimSpace(strings.TrimLeft(last, "-・• "))
		if len([]rune(trimmed)) < 8 {
			return "last_bullet_too_short"
		}
		if strings.HasSuffix(trimmed, "、") ||
			strings.HasSuffix(trimmed, ",") ||
			strings.HasSuffix(trimmed, "：") ||
			strings.HasSuffix(trimmed, ":") ||
			strings.HasSuffix(trimmed, "は") ||
			strings.HasSuffix(trimmed, "が") ||
			strings.HasSuffix(trimmed, "を") ||
			strings.HasSuffix(trimmed, "に") ||
			strings.HasSuffix(trimmed, "で") ||
			strings.HasSuffix(trimmed, "と") ||
			strings.HasSuffix(trimmed, "の") ||
			strings.HasSuffix(trimmed, "も") ||
			strings.HasSuffix(trimmed, "より") ||
			strings.HasSuffix(trimmed, "から") {
			return "last_bullet_ends_with_particle"
		}
		return ""
	}
	if !digestTextLooksComplete(s, 80) {
		return "text_looks_incomplete"
	}
	return ""
}

func validateDigestClusterDraftCompletion(text string) error {
	if digestClusterDraftValidationReason(text) != "" {
		return fmt.Errorf("cluster draft looks truncated")
	}
	return nil
}

func validateDigestCompletion(subject, body string) error {
	if strings.TrimSpace(subject) == "" {
		return fmt.Errorf("digest subject is empty")
	}
	if !digestTextLooksComplete(body, 220) {
		return fmt.Errorf("digest body looks truncated")
	}
	return nil
}

func appPageURL(path string) string {
	base := strings.TrimSpace(os.Getenv("NEXTAUTH_URL"))
	if base == "" {
		return ""
	}
	base = strings.TrimRight(base, "/")
	path = "/" + strings.TrimLeft(strings.TrimSpace(path), "/")
	return base + path
}

func loadUserAnthropicAPIKey(ctx context.Context, settingsRepo *repository.UserSettingsRepo, cipher *service.SecretCipher, userID *string) (*string, error) {
	if settingsRepo == nil || userID == nil || *userID == "" {
		return nil, fmt.Errorf("user anthropic api key is required")
	}
	enc, err := settingsRepo.GetAnthropicAPIKeyEncrypted(ctx, *userID)
	if err != nil || enc == nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("user anthropic api key is required")
	}
	if cipher == nil || !cipher.Enabled() {
		return nil, fmt.Errorf("user secret encryption is not configured")
	}
	plain, err := cipher.DecryptString(*enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt user anthropic key: %w", err)
	}
	return &plain, nil
}

func loadUserOpenAIAPIKey(ctx context.Context, settingsRepo *repository.UserSettingsRepo, cipher *service.SecretCipher, userID *string) (*string, error) {
	if settingsRepo == nil || userID == nil || *userID == "" {
		return nil, fmt.Errorf("user openai api key is required")
	}
	enc, err := settingsRepo.GetOpenAIAPIKeyEncrypted(ctx, *userID)
	if err != nil || enc == nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("user openai api key is required")
	}
	if cipher == nil || !cipher.Enabled() {
		return nil, fmt.Errorf("user secret encryption is not configured")
	}
	plain, err := cipher.DecryptString(*enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt user openai key: %w", err)
	}
	return &plain, nil
}

func loadUserGoogleAPIKey(ctx context.Context, settingsRepo *repository.UserSettingsRepo, cipher *service.SecretCipher, userID *string) (*string, error) {
	if settingsRepo == nil || userID == nil || *userID == "" {
		return nil, fmt.Errorf("user google api key is required")
	}
	enc, err := settingsRepo.GetGoogleAPIKeyEncrypted(ctx, *userID)
	if err != nil || enc == nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("user google api key is required")
	}
	if cipher == nil || !cipher.Enabled() {
		return nil, fmt.Errorf("user secret encryption is not configured")
	}
	plain, err := cipher.DecryptString(*enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt user google key: %w", err)
	}
	return &plain, nil
}

func loadUserGroqAPIKey(ctx context.Context, settingsRepo *repository.UserSettingsRepo, cipher *service.SecretCipher, userID *string) (*string, error) {
	if settingsRepo == nil || userID == nil || *userID == "" {
		return nil, fmt.Errorf("user groq api key is required")
	}
	enc, err := settingsRepo.GetGroqAPIKeyEncrypted(ctx, *userID)
	if err != nil || enc == nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("user groq api key is required")
	}
	if cipher == nil || !cipher.Enabled() {
		return nil, fmt.Errorf("user secret encryption is not configured")
	}
	plain, err := cipher.DecryptString(*enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt user groq key: %w", err)
	}
	return &plain, nil
}

func loadUserDeepSeekAPIKey(ctx context.Context, settingsRepo *repository.UserSettingsRepo, cipher *service.SecretCipher, userID *string) (*string, error) {
	if settingsRepo == nil || userID == nil || *userID == "" {
		return nil, fmt.Errorf("user deepseek api key is required")
	}
	enc, err := settingsRepo.GetDeepSeekAPIKeyEncrypted(ctx, *userID)
	if err != nil || enc == nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("user deepseek api key is required")
	}
	if cipher == nil || !cipher.Enabled() {
		return nil, fmt.Errorf("user secret encryption is not configured")
	}
	plain, err := cipher.DecryptString(*enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt user deepseek key: %w", err)
	}
	return &plain, nil
}

func loadUserAlibabaAPIKey(ctx context.Context, settingsRepo *repository.UserSettingsRepo, cipher *service.SecretCipher, userID *string) (*string, error) {
	if settingsRepo == nil || userID == nil || *userID == "" {
		return nil, fmt.Errorf("user alibaba api key is required")
	}
	enc, err := settingsRepo.GetAlibabaAPIKeyEncrypted(ctx, *userID)
	if err != nil || enc == nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("user alibaba api key is required")
	}
	if cipher == nil || !cipher.Enabled() {
		return nil, fmt.Errorf("user secret encryption is not configured")
	}
	plain, err := cipher.DecryptString(*enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt user alibaba key: %w", err)
	}
	return &plain, nil
}

func loadUserMistralAPIKey(ctx context.Context, settingsRepo *repository.UserSettingsRepo, cipher *service.SecretCipher, userID *string) (*string, error) {
	if settingsRepo == nil || userID == nil || *userID == "" {
		return nil, fmt.Errorf("user mistral api key is required")
	}
	enc, err := settingsRepo.GetMistralAPIKeyEncrypted(ctx, *userID)
	if err != nil || enc == nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("user mistral api key is required")
	}
	if cipher == nil || !cipher.Enabled() {
		return nil, fmt.Errorf("user secret encryption is not configured")
	}
	plain, err := cipher.DecryptString(*enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt user mistral key: %w", err)
	}
	return &plain, nil
}

func loadUserMoonshotAPIKey(ctx context.Context, settingsRepo *repository.UserSettingsRepo, cipher *service.SecretCipher, userID *string) (*string, error) {
	if settingsRepo == nil || userID == nil || *userID == "" {
		return nil, fmt.Errorf("user moonshot api key is required")
	}
	enc, err := settingsRepo.GetMoonshotAPIKeyEncrypted(ctx, *userID)
	if err != nil || enc == nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("user moonshot api key is required")
	}
	if cipher == nil || !cipher.Enabled() {
		return nil, fmt.Errorf("user secret encryption is not configured")
	}
	plain, err := cipher.DecryptString(*enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt user moonshot key: %w", err)
	}
	return &plain, nil
}

func loadUserXAIAPIKey(ctx context.Context, settingsRepo *repository.UserSettingsRepo, cipher *service.SecretCipher, userID *string) (*string, error) {
	if settingsRepo == nil || userID == nil || *userID == "" {
		return nil, fmt.Errorf("user xai api key is required")
	}
	enc, err := settingsRepo.GetXAIAPIKeyEncrypted(ctx, *userID)
	if err != nil || enc == nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("user xai api key is required")
	}
	if cipher == nil || !cipher.Enabled() {
		return nil, fmt.Errorf("user secret encryption is not configured")
	}
	plain, err := cipher.DecryptString(*enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt user xai key: %w", err)
	}
	return &plain, nil
}

func loadUserZAIAPIKey(ctx context.Context, settingsRepo *repository.UserSettingsRepo, cipher *service.SecretCipher, userID *string) (*string, error) {
	if settingsRepo == nil || userID == nil || *userID == "" {
		return nil, fmt.Errorf("user zai api key is required")
	}
	enc, err := settingsRepo.GetZAIAPIKeyEncrypted(ctx, *userID)
	if err != nil || enc == nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("user zai api key is required")
	}
	if cipher == nil || !cipher.Enabled() {
		return nil, fmt.Errorf("user secret encryption is not configured")
	}
	plain, err := cipher.DecryptString(*enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt user zai key: %w", err)
	}
	return &plain, nil
}

func loadUserFireworksAPIKey(ctx context.Context, settingsRepo *repository.UserSettingsRepo, cipher *service.SecretCipher, userID *string) (*string, error) {
	if settingsRepo == nil || userID == nil || *userID == "" {
		return nil, fmt.Errorf("user fireworks api key is required")
	}
	enc, err := settingsRepo.GetFireworksAPIKeyEncrypted(ctx, *userID)
	if err != nil || enc == nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("user fireworks api key is required")
	}
	if cipher == nil || !cipher.Enabled() {
		return nil, fmt.Errorf("user secret encryption is not configured")
	}
	plain, err := cipher.DecryptString(*enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt user fireworks key: %w", err)
	}
	return &plain, nil
}

func loadUserOpenRouterAPIKey(ctx context.Context, settingsRepo *repository.UserSettingsRepo, cipher *service.SecretCipher, userID *string) (*string, error) {
	if settingsRepo == nil || userID == nil || *userID == "" {
		return nil, fmt.Errorf("user openrouter api key is required")
	}
	enc, err := settingsRepo.GetOpenRouterAPIKeyEncrypted(ctx, *userID)
	if err != nil || enc == nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("user openrouter api key is required")
	}
	if cipher == nil || !cipher.Enabled() {
		return nil, fmt.Errorf("user secret encryption is not configured")
	}
	plain, err := cipher.DecryptString(*enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt user openrouter key: %w", err)
	}
	return &plain, nil
}

func loadUserPoeAPIKey(ctx context.Context, settingsRepo *repository.UserSettingsRepo, cipher *service.SecretCipher, userID *string) (*string, error) {
	if settingsRepo == nil || userID == nil || *userID == "" {
		return nil, fmt.Errorf("user poe api key is required")
	}
	enc, err := settingsRepo.GetPoeAPIKeyEncrypted(ctx, *userID)
	if err != nil || enc == nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("user poe api key is required")
	}
	if cipher == nil || !cipher.Enabled() {
		return nil, fmt.Errorf("user secret encryption is not configured")
	}
	plain, err := cipher.DecryptString(*enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt user poe key: %w", err)
	}
	return &plain, nil
}

func loadUserSiliconFlowAPIKey(ctx context.Context, settingsRepo *repository.UserSettingsRepo, cipher *service.SecretCipher, userID *string) (*string, error) {
	if settingsRepo == nil || userID == nil || *userID == "" {
		return nil, fmt.Errorf("user siliconflow api key is required")
	}
	enc, err := settingsRepo.GetSiliconFlowAPIKeyEncrypted(ctx, *userID)
	if err != nil || enc == nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("user siliconflow api key is required")
	}
	if cipher == nil || !cipher.Enabled() {
		return nil, fmt.Errorf("user secret encryption is not configured")
	}
	plain, err := cipher.DecryptString(*enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt user siliconflow key: %w", err)
	}
	return &plain, nil
}

func ptrStringOrNil(v *string) *string {
	if v == nil || *v == "" {
		return nil
	}
	s := *v
	return &s
}

func loadLLMKeysForModel(ctx context.Context, settingsRepo *repository.UserSettingsRepo, cipher *service.SecretCipher, userID *string, model *string, purpose string) (*string, *string, *string, *string, *string, *string, *string, *string, *string, *string, *string, error) {
	provider := service.LLMProviderForModel(model)
	resolvedModel := model
	if resolvedModel == nil || strings.TrimSpace(*resolvedModel) == "" {
		switch {
		case userID != nil && *userID != "" && settingsRepo != nil:
			for _, candidateProvider := range service.CostEfficientLLMProviders("") {
				switch candidateProvider {
				case "groq":
					if key, err := loadUserGroqAPIKey(ctx, settingsRepo, cipher, userID); err == nil && key != nil && strings.TrimSpace(*key) != "" {
						fallback := service.DefaultLLMModelForPurpose(candidateProvider, purpose)
						return nil, nil, key, nil, nil, nil, nil, nil, nil, nil, &fallback, nil
					}
				case "google":
					if key, err := loadUserGoogleAPIKey(ctx, settingsRepo, cipher, userID); err == nil && key != nil && strings.TrimSpace(*key) != "" {
						fallback := service.DefaultLLMModelForPurpose(candidateProvider, purpose)
						return nil, key, nil, nil, nil, nil, nil, nil, nil, nil, &fallback, nil
					}
				case "deepseek":
					if key, err := loadUserDeepSeekAPIKey(ctx, settingsRepo, cipher, userID); err == nil && key != nil && strings.TrimSpace(*key) != "" {
						fallback := service.DefaultLLMModelForPurpose(candidateProvider, purpose)
						return nil, nil, nil, key, nil, nil, nil, nil, nil, nil, &fallback, nil
					}
				case "alibaba":
					if key, err := loadUserAlibabaAPIKey(ctx, settingsRepo, cipher, userID); err == nil && key != nil && strings.TrimSpace(*key) != "" {
						fallback := service.DefaultLLMModelForPurpose(candidateProvider, purpose)
						return nil, nil, nil, nil, key, nil, nil, nil, nil, nil, &fallback, nil
					}
				case "mistral":
					if key, err := loadUserMistralAPIKey(ctx, settingsRepo, cipher, userID); err == nil && key != nil && strings.TrimSpace(*key) != "" {
						fallback := service.DefaultLLMModelForPurpose(candidateProvider, purpose)
						return nil, nil, nil, nil, nil, key, nil, nil, nil, nil, &fallback, nil
					}
				case "moonshot":
					if key, err := loadUserMoonshotAPIKey(ctx, settingsRepo, cipher, userID); err == nil && key != nil && strings.TrimSpace(*key) != "" {
						fallback := service.DefaultLLMModelForPurpose(candidateProvider, purpose)
						return nil, nil, nil, nil, nil, nil, nil, nil, nil, key, &fallback, nil
					}
				case "xai":
					if key, err := loadUserXAIAPIKey(ctx, settingsRepo, cipher, userID); err == nil && key != nil && strings.TrimSpace(*key) != "" {
						fallback := service.DefaultLLMModelForPurpose(candidateProvider, purpose)
						return nil, nil, nil, nil, nil, nil, key, nil, nil, nil, &fallback, nil
					}
				case "zai":
					if key, err := loadUserZAIAPIKey(ctx, settingsRepo, cipher, userID); err == nil && key != nil && strings.TrimSpace(*key) != "" {
						fallback := service.DefaultLLMModelForPurpose(candidateProvider, purpose)
						return nil, nil, nil, nil, nil, nil, nil, key, nil, nil, &fallback, nil
					}
				case "fireworks":
					if key, err := loadUserFireworksAPIKey(ctx, settingsRepo, cipher, userID); err == nil && key != nil && strings.TrimSpace(*key) != "" {
						fallback := service.DefaultLLMModelForPurpose(candidateProvider, purpose)
						return nil, nil, nil, nil, nil, nil, nil, nil, key, nil, &fallback, nil
					}
				case "openai":
					if key, err := loadUserOpenAIAPIKey(ctx, settingsRepo, cipher, userID); err == nil && key != nil && strings.TrimSpace(*key) != "" {
						fallback := service.DefaultLLMModelForPurpose(candidateProvider, purpose)
						return nil, nil, nil, nil, nil, nil, nil, nil, nil, key, &fallback, nil
					}
				case "openrouter":
					if key, err := loadUserOpenRouterAPIKey(ctx, settingsRepo, cipher, userID); err == nil && key != nil && strings.TrimSpace(*key) != "" {
						fallback := service.DefaultLLMModelForPurpose(candidateProvider, purpose)
						return nil, nil, nil, nil, nil, nil, nil, nil, nil, key, &fallback, nil
					}
				case "poe":
					if key, err := loadUserPoeAPIKey(ctx, settingsRepo, cipher, userID); err == nil && key != nil && strings.TrimSpace(*key) != "" {
						fallback := service.DefaultLLMModelForPurpose(candidateProvider, purpose)
						return nil, nil, nil, nil, nil, nil, nil, nil, nil, key, &fallback, nil
					}
				case "siliconflow":
					if key, err := loadUserSiliconFlowAPIKey(ctx, settingsRepo, cipher, userID); err == nil && key != nil && strings.TrimSpace(*key) != "" {
						fallback := service.DefaultLLMModelForPurpose(candidateProvider, purpose)
						return nil, nil, nil, nil, nil, nil, nil, nil, nil, key, &fallback, nil
					}
				case "anthropic":
					if key, err := loadUserAnthropicAPIKey(ctx, settingsRepo, cipher, userID); err == nil && key != nil && strings.TrimSpace(*key) != "" {
						fallback := service.DefaultLLMModelForPurpose(candidateProvider, purpose)
						return key, nil, nil, nil, nil, nil, nil, nil, nil, nil, &fallback, nil
					}
				}
			}
		}
	}
	switch provider {
	case "google":
		key, err := loadUserGoogleAPIKey(ctx, settingsRepo, cipher, userID)
		return nil, key, nil, nil, nil, nil, nil, nil, nil, nil, model, err
	case "groq":
		key, err := loadUserGroqAPIKey(ctx, settingsRepo, cipher, userID)
		return nil, nil, key, nil, nil, nil, nil, nil, nil, nil, model, err
	case "deepseek":
		key, err := loadUserDeepSeekAPIKey(ctx, settingsRepo, cipher, userID)
		return nil, nil, nil, key, nil, nil, nil, nil, nil, nil, model, err
	case "alibaba":
		key, err := loadUserAlibabaAPIKey(ctx, settingsRepo, cipher, userID)
		return nil, nil, nil, nil, key, nil, nil, nil, nil, nil, model, err
	case "mistral":
		key, err := loadUserMistralAPIKey(ctx, settingsRepo, cipher, userID)
		return nil, nil, nil, nil, nil, key, nil, nil, nil, nil, model, err
	case "moonshot":
		key, err := loadUserMoonshotAPIKey(ctx, settingsRepo, cipher, userID)
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, key, model, err
	case "xai":
		key, err := loadUserXAIAPIKey(ctx, settingsRepo, cipher, userID)
		return nil, nil, nil, nil, nil, nil, key, nil, nil, nil, model, err
	case "zai":
		key, err := loadUserZAIAPIKey(ctx, settingsRepo, cipher, userID)
		return nil, nil, nil, nil, nil, nil, nil, key, nil, nil, model, err
	case "fireworks":
		key, err := loadUserFireworksAPIKey(ctx, settingsRepo, cipher, userID)
		return nil, nil, nil, nil, nil, nil, nil, nil, key, nil, model, err
	case "openai":
		key, err := loadUserOpenAIAPIKey(ctx, settingsRepo, cipher, userID)
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, key, model, err
	case "openrouter":
		key, err := loadUserOpenRouterAPIKey(ctx, settingsRepo, cipher, userID)
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, key, model, err
	case "poe":
		key, err := loadUserPoeAPIKey(ctx, settingsRepo, cipher, userID)
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, key, model, err
	case "siliconflow":
		key, err := loadUserSiliconFlowAPIKey(ctx, settingsRepo, cipher, userID)
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, key, model, err
	default:
		key, err := loadUserAnthropicAPIKey(ctx, settingsRepo, cipher, userID)
		return key, nil, nil, nil, nil, nil, nil, nil, nil, nil, model, err
	}
}

func digestTopicKey(topics []string) string {
	for _, t := range topics {
		t = strings.TrimSpace(t)
		if t != "" {
			return t
		}
	}
	return "__untagged__"
}

// buildDigestClusterDrafts compacts digest inputs into topic buckets and stores
// representative snippets per bucket. This becomes the intermediate artifact for final compose.
func buildDigestClusterDrafts(details []model.DigestItemDetail, embClusters []model.ReadingPlanCluster) []model.DigestClusterDraft {
	if len(details) == 0 {
		return nil
	}
	byID := make(map[string]model.DigestItemDetail, len(details))
	for _, d := range details {
		byID[d.Item.ID] = d
	}
	seen := map[string]struct{}{}
	out := make([]model.DigestClusterDraft, 0, len(details))

	appendDraft := func(idx int, key, label string, group []model.DigestItemDetail) {
		if len(group) == 0 {
			return
		}
		maxScore := 0.0
		hasScore := false
		lines := make([]string, 0, minInt(4, len(group)))
		for i, it := range group {
			if it.Summary.Score != nil {
				if !hasScore || *it.Summary.Score > maxScore {
					maxScore = *it.Summary.Score
					hasScore = true
				}
			}
			if i >= 4 {
				continue
			}
			title := strings.TrimSpace(coalescePtrStr(it.Item.Title, it.Item.URL))
			summary := strings.TrimSpace(it.Summary.Summary)
			factLine := ""
			if len(it.Facts) > 0 {
				facts := make([]string, 0, minInt(2, len(it.Facts)))
				for _, f := range it.Facts {
					f = strings.TrimSpace(f)
					if f == "" {
						continue
					}
					facts = append(facts, f)
					if len(facts) >= 2 {
						break
					}
				}
				if len(facts) > 0 {
					factLine = strings.Join(facts, " / ")
				}
			}
			switch {
			case summary != "" && factLine != "":
				lines = append(lines, "- "+title+": "+summary+" | facts: "+factLine)
			case summary != "":
				lines = append(lines, "- "+title+": "+summary)
			case factLine != "":
				lines = append(lines, "- "+title+": "+factLine)
			default:
				lines = append(lines, "- "+title)
			}
		}
		draftSummary := strings.Join(lines, "\n")
		if len(group) > 4 {
			draftSummary += fmt.Sprintf("\n- ...and %d more related items", len(group)-4)
		}
		var scorePtr *float64
		if hasScore {
			v := maxScore
			scorePtr = &v
		}
		out = append(out, model.DigestClusterDraft{
			ClusterKey:   key,
			ClusterLabel: label,
			Rank:         idx,
			ItemCount:    len(group),
			Topics:       group[0].Summary.Topics,
			MaxScore:     scorePtr,
			DraftSummary: draftSummary,
		})
	}

	rank := 1
	for _, c := range embClusters {
		group := make([]model.DigestItemDetail, 0, len(c.Items))
		for _, m := range c.Items {
			d, ok := byID[m.ID]
			if !ok {
				continue
			}
			if _, dup := seen[d.Item.ID]; dup {
				continue
			}
			seen[d.Item.ID] = struct{}{}
			group = append(group, d)
		}
		if len(group) == 0 {
			continue
		}
		label := c.Label
		if strings.TrimSpace(label) == "" {
			label = digestTopicKey(group[0].Summary.Topics)
		}
		appendDraft(rank, c.ID, label, group)
		rank++
	}

	// Add remaining singletons so the first-stage processing still covers all items.
	for _, d := range details {
		if _, ok := seen[d.Item.ID]; ok {
			continue
		}
		seen[d.Item.ID] = struct{}{}
		key := d.Item.ID
		label := digestTopicKey(d.Summary.Topics)
		appendDraft(rank, key, label, []model.DigestItemDetail{d})
		rank++
	}
	return out
}

func draftSourceLines(draftSummary string) []string {
	lines := strings.Split(draftSummary, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		out = append(out, l)
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func buildBroadDigestDraftFromChunk(chunk []model.DigestClusterDraft, key, label string) model.DigestClusterDraft {
	itemCount := 0
	var maxScore *float64
	lines := make([]string, 0, len(chunk))
	topicsSet := map[string]struct{}{}
	for _, d := range chunk {
		itemCount += d.ItemCount
		if d.MaxScore != nil && (maxScore == nil || *d.MaxScore > *maxScore) {
			v := *d.MaxScore
			maxScore = &v
		}
		for _, t := range d.Topics {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			topicsSet[t] = struct{}{}
		}
		line := strings.TrimSpace(d.DraftSummary)
		if line == "" {
			continue
		}
		first := strings.Split(line, "\n")[0]
		lines = append(lines, fmt.Sprintf("- [%s] %s", d.ClusterLabel, first))
	}
	topics := make([]string, 0, len(topicsSet))
	for t := range topicsSet {
		topics = append(topics, t)
	}
	sort.Strings(topics)
	return model.DigestClusterDraft{
		ClusterKey:   key,
		ClusterLabel: label,
		ItemCount:    itemCount,
		Topics:       topics,
		MaxScore:     maxScore,
		DraftSummary: strings.Join(lines, "\n"),
	}
}

func compressDigestClusterDrafts(drafts []model.DigestClusterDraft, target int) []model.DigestClusterDraft {
	if target <= 0 {
		target = 20
	}
	if len(drafts) <= target {
		return drafts
	}

	// Keep larger/more informative clusters first; merge tail singletons/small clusters.
	keep := make([]model.DigestClusterDraft, 0, len(drafts))
	tail := make([]model.DigestClusterDraft, 0, len(drafts))
	for i, d := range drafts {
		if i < 10 || d.ItemCount >= 3 {
			keep = append(keep, d)
			continue
		}
		tail = append(tail, d)
	}
	broadCount := 0
	if len(tail) >= 4 {
		broadCount = 1
	}
	if len(tail) >= 10 {
		broadCount = 2
	}
	if len(keep) >= target {
		cut := target - broadCount
		if cut < 1 {
			cut = target
			broadCount = 0
		}
		keep = keep[:cut]
		if broadCount > 0 {
			if broadCount == 1 {
				keep = append(keep, buildBroadDigestDraftFromChunk(tail, "broad-1", "幅広い話題（横断）"))
			} else {
				mid := len(tail) / 2
				if mid < 1 {
					mid = 1
				}
				keep = append(keep, buildBroadDigestDraftFromChunk(tail[:mid], "broad-1", "幅広い話題（横断）A"))
				keep = append(keep, buildBroadDigestDraftFromChunk(tail[mid:], "broad-2", "幅広い話題（横断）B"))
			}
		}
		for i := range keep {
			keep[i].Rank = i + 1
		}
		return keep
	}

	remainingSlots := target - len(keep)
	if remainingSlots <= 0 || len(tail) == 0 {
		for i := range keep {
			keep[i].Rank = i + 1
		}
		return keep
	}

	// Merge tail clusters into grouped "other" buckets to preserve coverage.
	chunkSize := int(math.Ceil(float64(len(tail)) / float64(remainingSlots)))
	if chunkSize < 2 {
		chunkSize = 2
	}
	for i := 0; i < len(tail) && len(keep) < target; i += chunkSize {
		end := i + chunkSize
		if end > len(tail) {
			end = len(tail)
		}
		chunk := tail[i:end]
		if len(chunk) == 1 {
			keep = append(keep, chunk[0])
			continue
		}
		keep = append(keep, buildBroadDigestDraftFromChunk(chunk, fmt.Sprintf("merged-tail-%d", len(keep)+1), "その他の話題"))
	}

	for i := range keep {
		keep[i].Rank = i + 1
	}
	return keep
}

func buildComposeItemsFromClusterDrafts(drafts []model.DigestClusterDraft, maxItems int) []service.ComposeDigestItem {
	_ = maxItems // keep signature compatible; compose now uses all cluster drafts by default.
	out := make([]service.ComposeDigestItem, 0, len(drafts))
	for i, d := range drafts {
		title := d.ClusterLabel
		if d.ItemCount > 1 {
			title = fmt.Sprintf("%s (%d items)", d.ClusterLabel, d.ItemCount)
		}
		summary := d.DraftSummary
		// Keep coverage across all cluster drafts, while reducing detail for lower-ranked clusters.
		// This avoids "top clusters only" behavior without sending every draft at full verbosity.
		if i >= 12 {
			lines := strings.Split(strings.TrimSpace(d.DraftSummary), "\n")
			if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
				summary = lines[0]
			}
			if len(lines) > 1 {
				summary += fmt.Sprintf("\n- ...%d more lines omitted in compose input", len(lines)-1)
			}
		}
		titlePtr := title
		out = append(out, service.ComposeDigestItem{
			Rank:    i + 1,
			Title:   &titlePtr,
			URL:     "",
			Summary: summary,
			Topics:  d.Topics,
			Score:   d.MaxScore,
		})
	}
	return out
}

func coalescePtrStr(a *string, b string) string {
	if a != nil && strings.TrimSpace(*a) != "" {
		return *a
	}
	return b
}

// Event payloads

type ItemCreatedData struct {
	ItemID   string `json:"item_id"`
	SourceID string `json:"source_id"`
	URL      string `json:"url"`
}

type DigestCreatedData struct {
	DigestID string `json:"digest_id"`
	UserID   string `json:"user_id"`
	To       string `json:"to"`
}

type DigestCopyComposedData struct {
	DigestID string `json:"digest_id"`
	UserID   string `json:"user_id"`
	To       string `json:"to"`
}

// NewHandler registers all Inngest functions and returns the HTTP handler.
func NewHandler(db *pgxpool.Pool, worker *service.WorkerClient, resend *service.ResendClient, oneSignal *service.OneSignalClient, obsidianExport *service.ObsidianExportService, cache service.JSONCache, search *service.MeilisearchService) http.Handler {
	secretCipher := service.NewSecretCipher()
	openAI := service.NewOpenAIClient()
	llmUsageCache = cache
	_ = search
	client, err := service.NewInngestClient("sifto-api")
	if err != nil {
		log.Fatalf("inngest client: %v", err)
	}

	register := func(f inngestgo.ServableFunction, err error) {
		if err != nil {
			log.Fatalf("register function: %v", err)
		}
	}

	register(fetchRSSFn(client, db))
	register(processItemFn(client, db, worker, openAI, oneSignal, secretCipher, cache))
	register(itemSearchUpsertFn(client, db, search))
	register(itemSearchDeleteFn(client, search))
	register(searchSuggestionArticleUpsertFn(client, db, search))
	register(searchSuggestionArticleDeleteFn(client, search))
	register(searchSuggestionSourceUpsertFn(client, db, search))
	register(searchSuggestionSourceDeleteFn(client, search))
	register(searchSuggestionTopicsRefreshFn(client, db, search))
	register(itemSearchBackfillRunFn(client, db))
	register(itemSearchBackfillFn(client, db, search))
	register(embedItemFn(client, db, openAI, secretCipher))
	register(generateBriefingSnapshotsFn(client, db, oneSignal))
	register(notifyReviewQueueFn(client, db, oneSignal))
	register(exportObsidianFavoritesFn(client, db, obsidianExport))
	register(trackProviderModelUpdatesFn(client, db, oneSignal))
	register(syncOpenRouterModelsFn(client, db, resend, oneSignal))
	register(syncPoeUsageHistoryFn(client, db, secretCipher))
	register(generateAudioBriefingsFn(client, db, worker, cache))
	register(runAudioBriefingPipelineFn(client, db, worker, cache))
	register(failStaleAudioBriefingVoicingFn(client, db))
	register(moveAudioBriefingsToIAFn(client, db, worker))
	register(generateDigestFn(client, db))
	register(composeDigestCopyFn(client, db, worker, secretCipher))
	register(sendDigestFn(client, db, worker, resend, oneSignal, secretCipher))
	register(checkBudgetAlertsFn(client, db, resend, oneSignal))
	register(computePreferenceProfilesFn(client, db))
	register(computeTopicPulseDailyFn(client, db))
	register(generateAINavigatorBriefsFn(client, db, worker, oneSignal))
	register(runAINavigatorBriefPipelineFn(client, db, worker, oneSignal, llmUsageCache))

	return client.Serve()
}

func generateAudioBriefingsFn(client inngestgo.Client, db *pgxpool.Pool, worker *service.WorkerClient, cache service.JSONCache) (inngestgo.ServableFunction, error) {
	audioBriefingRepo := repository.NewAudioBriefingRepo(db)
	userSettingsRepo := repository.NewUserSettingsRepo(db)
	llmUsageRepo := repository.NewLLMUsageLogRepo(db)
	secretCipher := service.NewSecretCipher()
	audioConcatRunner := service.NewAudioConcatRunnerFromEnv()
	audioBriefingVoiceRunner := service.NewAudioBriefingVoiceRunner(audioBriefingRepo, userSettingsRepo, secretCipher, worker)
	audioBriefingConcatStarter := service.NewAudioBriefingConcatStarter(audioBriefingRepo, audioConcatRunner)
	orchestrator := service.NewAudioBriefingOrchestrator(audioBriefingRepo, userSettingsRepo, llmUsageRepo, secretCipher, worker, cache, audioBriefingVoiceRunner, audioBriefingConcatStarter)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "generate-audio-briefings", Name: "Generate Audio Briefings"},
		inngestgo.CronTrigger("0 * * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			settings, err := audioBriefingRepo.ListEnabledSettings(ctx)
			if err != nil {
				return nil, err
			}

			now := timeutil.NowJST()
			processed := 0
			started := 0
			skipped := 0
			failed := 0

			for _, row := range settings {
				processed++
				job, err := orchestrator.GenerateScheduled(ctx, row.UserID, now)
				if err != nil {
					failed++
					log.Printf("generate audio briefing user=%s: %v", row.UserID, err)
					continue
				}
				if job == nil {
					skipped++
					continue
				}
				switch {
				case audioBriefingShouldDispatch(job):
					if _, err := client.Send(ctx, service.NewAudioBriefingRunEvent(row.UserID, job.ID, "scheduled")); err != nil {
						failed++
						log.Printf("enqueue audio briefing run user=%s job=%s: %v", row.UserID, job.ID, err)
						continue
					}
					started++
				case strings.TrimSpace(job.Status) == "skipped":
					skipped++
				default:
					skipped++
				}
			}

			return map[string]any{
				"processed": processed,
				"started":   started,
				"skipped":   skipped,
				"failed":    failed,
			}, nil
		},
	)
}

func generateAINavigatorBriefsFn(client inngestgo.Client, db *pgxpool.Pool, worker *service.WorkerClient, oneSignal *service.OneSignalClient) (inngestgo.ServableFunction, error) {
	briefRepo := repository.NewAINavigatorBriefRepo(db)
	itemRepo := repository.NewItemRepo(db)
	settingsRepo := repository.NewUserSettingsRepo(db)
	userRepo := repository.NewUserRepo(db)
	pushLogRepo := repository.NewPushNotificationLogRepo(db)
	llmUsageRepo := repository.NewLLMUsageLogRepo(db)
	secretCipher := service.NewSecretCipher()
	briefService := service.NewAINavigatorBriefService(briefRepo, itemRepo, settingsRepo, userRepo, pushLogRepo, llmUsageRepo, worker, secretCipher, oneSignal, nil, llmUsageCache, nil)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "generate-ai-navigator-briefs", Name: "Generate AI Navigator Briefs"},
		inngestgo.CronTrigger("0 * * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			now := timeutil.NowJST()
			var slot string
			switch now.Hour() {
			case 8:
				slot = model.AINavigatorBriefSlotMorning
			case 12:
				slot = model.AINavigatorBriefSlotNoon
			case 18:
				slot = model.AINavigatorBriefSlotEvening
			default:
				return map[string]any{
					"status": "skipped",
					"reason": "outside_scheduled_slots",
					"hour":   now.Hour(),
				}, nil
			}

			windowStart, _, err := service.ResolveAINavigatorBriefSlotWindow(now, slot)
			if err != nil {
				return nil, err
			}
			userIDs, err := settingsRepo.ListUserIDsWithAINavigatorBriefEnabled(ctx)
			if err != nil {
				return nil, err
			}

			processed := 0
			enqueued := 0
			skipped := 0
			failed := 0

			for _, userID := range userIDs {
				processed++
				latest, err := briefRepo.LatestBriefByUserSlot(ctx, userID, slot)
				switch {
				case err == nil && latest != nil:
					if !latest.CreatedAt.Before(windowStart) {
						skipped++
						continue
					}
				case err != nil && err != repository.ErrNotFound:
					failed++
					log.Printf("ai navigator brief latest user=%s slot=%s: %v", userID, slot, err)
					continue
				}

				brief, err := briefService.EnqueueBriefForSlot(ctx, userID, slot)
				if err != nil {
					failed++
					log.Printf("ai navigator brief enqueue user=%s slot=%s: %v", userID, slot, err)
					continue
				}
				if brief == nil {
					skipped++
					continue
				}
				if _, err := client.Send(ctx, service.NewAINavigatorBriefRunEvent(userID, brief.ID, "scheduled")); err != nil {
					failed++
					_ = briefRepo.MarkBriefFailedAt(ctx, brief.ID, "failed to enqueue generation", timeutil.NowJST())
					log.Printf("ai navigator brief send run event user=%s brief=%s: %v", userID, brief.ID, err)
					continue
				}
				enqueued++
			}

			return map[string]any{
				"slot":      slot,
				"processed": processed,
				"enqueued":  enqueued,
				"skipped":   skipped,
				"failed":    failed,
			}, nil
		},
	)
}

type aiNavigatorBriefRunEventData struct {
	UserID  string `json:"user_id"`
	BriefID string `json:"brief_id"`
	Trigger string `json:"trigger"`
}

func runAINavigatorBriefPipelineFn(client inngestgo.Client, db *pgxpool.Pool, worker *service.WorkerClient, oneSignal *service.OneSignalClient, cache service.JSONCache) (inngestgo.ServableFunction, error) {
	briefRepo := repository.NewAINavigatorBriefRepo(db)
	itemRepo := repository.NewItemRepo(db)
	settingsRepo := repository.NewUserSettingsRepo(db)
	userRepo := repository.NewUserRepo(db)
	pushLogRepo := repository.NewPushNotificationLogRepo(db)
	llmUsageRepo := repository.NewLLMUsageLogRepo(db)
	secretCipher := service.NewSecretCipher()
	briefService := service.NewAINavigatorBriefService(briefRepo, itemRepo, settingsRepo, userRepo, pushLogRepo, llmUsageRepo, worker, secretCipher, oneSignal, nil, cache, nil)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{
			ID:   "run-ai-navigator-brief-pipeline",
			Name: "Run AI Navigator Brief Pipeline",
			Concurrency: []inngestgo.ConfigStepConcurrency{
				{
					Limit: 1,
					Key:   inngestgo.StrPtr("event.data.brief_id"),
					Scope: enums.ConcurrencyScopeFn,
				},
			},
		},
		inngestgo.EventTrigger("ai-navigator-brief/run", nil),
		func(ctx context.Context, input inngestgo.Input[aiNavigatorBriefRunEventData]) (any, error) {
			data := input.Event.Data
			brief, err := briefService.RunQueuedBrief(ctx, data.UserID, data.BriefID)
			if err != nil {
				return nil, err
			}
			if brief == nil {
				return map[string]any{"status": "missing"}, nil
			}
			if brief.Status == model.AINavigatorBriefStatusGenerated {
				if err := briefService.NotifyBrief(ctx, brief); err != nil {
					return nil, err
				}
			}
			return map[string]any{
				"brief_id": data.BriefID,
				"status":   brief.Status,
				"trigger":  data.Trigger,
			}, nil
		},
	)
}

type audioBriefingRunEventData struct {
	UserID  string `json:"user_id"`
	JobID   string `json:"job_id"`
	Trigger string `json:"trigger"`
}

func runAudioBriefingPipelineFn(client inngestgo.Client, db *pgxpool.Pool, worker *service.WorkerClient, cache service.JSONCache) (inngestgo.ServableFunction, error) {
	audioBriefingRepo := repository.NewAudioBriefingRepo(db)
	userSettingsRepo := repository.NewUserSettingsRepo(db)
	llmUsageRepo := repository.NewLLMUsageLogRepo(db)
	secretCipher := service.NewSecretCipher()
	audioConcatRunner := service.NewAudioConcatRunnerFromEnv()
	audioBriefingVoiceRunner := service.NewAudioBriefingVoiceRunner(audioBriefingRepo, userSettingsRepo, secretCipher, worker)
	audioBriefingConcatStarter := service.NewAudioBriefingConcatStarter(audioBriefingRepo, audioConcatRunner)
	orchestrator := service.NewAudioBriefingOrchestrator(audioBriefingRepo, userSettingsRepo, llmUsageRepo, secretCipher, worker, cache, audioBriefingVoiceRunner, audioBriefingConcatStarter)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{
			ID:   "run-audio-briefing-pipeline",
			Name: "Run Audio Briefing Pipeline",
			Concurrency: []inngestgo.ConfigStepConcurrency{
				{
					Limit: 1,
					Key:   inngestgo.StrPtr("event.data.job_id"),
					Scope: enums.ConcurrencyScopeFn,
				},
			},
		},
		inngestgo.EventTrigger("audio-briefing/run", nil),
		func(ctx context.Context, input inngestgo.Input[audioBriefingRunEventData]) (any, error) {
			data := input.Event.Data
			if strings.TrimSpace(data.UserID) == "" || strings.TrimSpace(data.JobID) == "" {
				return nil, fmt.Errorf("audio briefing run requires user_id and job_id")
			}
			job, shouldRequeue, err := orchestrator.RunPipelineStep(ctx, strings.TrimSpace(data.UserID), strings.TrimSpace(data.JobID))
			if err != nil {
				return nil, err
			}
			if job == nil {
				return map[string]any{"status": "missing"}, nil
			}
			if shouldRequeue && strings.TrimSpace(job.Status) == "voicing" {
				if _, err := client.Send(ctx, service.NewAudioBriefingRunEvent(job.UserID, job.ID, "continue-voicing")); err != nil {
					return nil, err
				}
			}
			return map[string]any{
				"job_id":         job.ID,
				"user_id":        job.UserID,
				"status":         job.Status,
				"trigger":        strings.TrimSpace(data.Trigger),
				"should_requeue": shouldRequeue,
			}, nil
		},
	)
}

func moveAudioBriefingsToIAFn(client inngestgo.Client, db *pgxpool.Pool, worker *service.WorkerClient) (inngestgo.ServableFunction, error) {
	audioBriefingRepo := repository.NewAudioBriefingRepo(db)
	archiveSvc := service.NewAudioBriefingArchiveService(audioBriefingRepo, worker)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "move-audio-briefings-to-ia", Name: "Move Audio Briefings To IA"},
		inngestgo.CronTrigger("17 3 * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			result, err := archiveSvc.MovePublishedToIA(ctx)
			if err != nil {
				return nil, err
			}
			if result == nil {
				result = &service.AudioBriefingArchiveResult{}
			}
			return map[string]any{
				"processed": result.Processed,
				"moved":     result.Moved,
				"failed":    result.Failed,
			}, nil
		},
	)
}

func failStaleAudioBriefingVoicingFn(client inngestgo.Client, db *pgxpool.Pool) (inngestgo.ServableFunction, error) {
	audioBriefingRepo := repository.NewAudioBriefingRepo(db)
	eventPublisher, err := service.NewEventPublisher()
	if err != nil {
		return nil, err
	}
	staleVoicingSvc := service.NewAudioBriefingStaleVoicingService(audioBriefingRepo).WithPublisher(eventPublisher)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "fail-stale-audio-briefing-voicing", Name: "Fail Stale Audio Briefing Voicing"},
		inngestgo.CronTrigger("*/5 * * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			result, err := staleVoicingSvc.FailStaleJobs(ctx)
			if err != nil {
				return nil, err
			}
			if result == nil {
				result = &service.AudioBriefingStaleVoicingResult{}
			}
			return map[string]any{
				"processed": result.Processed,
				"failed":    result.Failed,
			}, nil
		},
	)
}

func audioBriefingShouldDispatch(job *model.AudioBriefingJob) bool {
	if job == nil {
		return false
	}
	switch strings.TrimSpace(job.Status) {
	case "pending", "scripted", "voiced", "failed":
		return true
	default:
		return false
	}
}

func envFloat64OrDefault(key string, fallback float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}

func envIntOrDefault(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// cron/generate-briefing-snapshots — 30分ごとに当日ブリーフィングのスナップショットを更新
func generateBriefingSnapshotsFn(client inngestgo.Client, db *pgxpool.Pool, oneSignal *service.OneSignalClient) (inngestgo.ServableFunction, error) {
	userRepo := repository.NewUserRepo(db)
	itemRepo := repository.NewItemRepo(db)
	streakRepo := repository.NewReadingStreakRepo(db)
	snapshotRepo := repository.NewBriefingSnapshotRepo(db)
	pushLogRepo := repository.NewPushNotificationLogRepo(db)
	notificationRepo := repository.NewNotificationPriorityRepo(db)
	reviewRepo := repository.NewReviewQueueRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "generate-briefing-snapshots", Name: "Generate Briefing Snapshots"},
		inngestgo.CronTrigger("*/30 * * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			users, err := userRepo.ListAll(ctx)
			if err != nil {
				return nil, fmt.Errorf("list users: %w", err)
			}
			today := timeutil.StartOfDayJST(timeutil.NowJST())
			dateStr := today.Format("2006-01-02")
			updated := 0
			failed := 0
			for _, u := range users {
				payload, err := service.BuildBriefingToday(ctx, itemRepo, streakRepo, u.ID, today, 18)
				if err != nil {
					failed++
					log.Printf("generate-briefing-snapshots build user=%s: %v", u.ID, err)
					continue
				}
				payload.Status = "ready"
				if err := snapshotRepo.Upsert(ctx, u.ID, dateStr, "ready", payload); err != nil {
					failed++
					log.Printf("generate-briefing-snapshots upsert user=%s: %v", u.ID, err)
					continue
				}
				if oneSignal != nil && oneSignal.Enabled() && (len(payload.HighlightItems) > 0 || len(payload.Clusters) > 0) {
					rule, _ := notificationRepo.EnsureDefaults(ctx, u.ID)
					if rule != nil && !rule.BriefingEnabled {
						continue
					}
					alreadyNotified, err := pushLogRepo.CountByUserKindDay(ctx, u.ID, "briefing_ready", today)
					if err != nil {
						log.Printf("generate-briefing-snapshots push count user=%s: %v", u.ID, err)
					} else if alreadyNotified == 0 {
						dueReviews, _ := reviewRepo.CountDue(ctx, u.ID, timeutil.NowJST())
						title := "Sifto: 今日のブリーフィングを更新しました"
						message := fmt.Sprintf("注目%d件・クラスタ%d件・再訪%d件を確認できます。", len(payload.HighlightItems), len(payload.Clusters), dueReviews)
						pushRes, pErr := oneSignal.SendToExternalID(
							ctx,
							u.Email,
							title,
							message,
							appPageURL("/"),
							map[string]any{
								"type":         "briefing_ready",
								"briefing_url": appPageURL("/"),
								"date":         dateStr,
								"highlights":   len(payload.HighlightItems),
								"clusters":     len(payload.Clusters),
								"reviews":      dueReviews,
							},
						)
						if pErr != nil {
							log.Printf("generate-briefing-snapshots push send user=%s: %v", u.ID, pErr)
						} else {
							var oneSignalID *string
							recipients := 0
							if pushRes != nil {
								if strings.TrimSpace(pushRes.ID) != "" {
									id := strings.TrimSpace(pushRes.ID)
									oneSignalID = &id
								}
								recipients = pushRes.Recipients
							}
							if err := pushLogRepo.Insert(ctx, repository.PushNotificationLogInput{
								UserID:                  u.ID,
								Kind:                    "briefing_ready",
								ItemID:                  nil,
								DayJST:                  today,
								Title:                   title,
								Message:                 message,
								OneSignalNotificationID: oneSignalID,
								Recipients:              recipients,
							}); err != nil {
								log.Printf("generate-briefing-snapshots push log user=%s: %v", u.ID, err)
							}
						}
					}
				}
				updated++
			}
			return map[string]any{
				"date":    dateStr,
				"users":   len(users),
				"updated": updated,
				"failed":  failed,
			}, nil
		},
	)
}

func notifyReviewQueueFn(client inngestgo.Client, db *pgxpool.Pool, oneSignal *service.OneSignalClient) (inngestgo.ServableFunction, error) {
	userRepo := repository.NewUserRepo(db)
	reviewRepo := repository.NewReviewQueueRepo(db)
	pushLogRepo := repository.NewPushNotificationLogRepo(db)
	notificationRepo := repository.NewNotificationPriorityRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "notify-review-queue", Name: "Notify Review Queue"},
		inngestgo.CronTrigger("0 * * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			if oneSignal == nil || !oneSignal.Enabled() {
				return map[string]any{"enabled": false}, nil
			}
			users, err := userRepo.ListAll(ctx)
			if err != nil {
				return nil, err
			}
			now := timeutil.NowJST()
			day := timeutil.StartOfDayJST(now)
			sent := 0
			for _, u := range users {
				rule, _ := notificationRepo.EnsureDefaults(ctx, u.ID)
				if rule != nil && !rule.ReviewEnabled {
					continue
				}
				count, err := reviewRepo.CountDue(ctx, u.ID, now)
				if err != nil || count == 0 {
					continue
				}
				already, err := pushLogRepo.CountByUserKindDay(ctx, u.ID, "review_due", day)
				if err != nil || already > 0 {
					continue
				}
				title := "Sifto: 再訪キューがたまっています"
				message := fmt.Sprintf("今日見返したい記事が%d件あります。5分で確認できます。", count)
				pushRes, err := oneSignal.SendToExternalID(ctx, u.Email, title, message, appPageURL("/"), map[string]any{
					"type":       "review_due",
					"target_url": appPageURL("/"),
					"count":      count,
				})
				if err != nil {
					log.Printf("notify-review-queue push user=%s: %v", u.ID, err)
					continue
				}
				var oneSignalID *string
				recipients := 0
				if pushRes != nil {
					if strings.TrimSpace(pushRes.ID) != "" {
						id := strings.TrimSpace(pushRes.ID)
						oneSignalID = &id
					}
					recipients = pushRes.Recipients
				}
				if err := pushLogRepo.Insert(ctx, repository.PushNotificationLogInput{
					UserID:                  u.ID,
					Kind:                    "review_due",
					ItemID:                  nil,
					DayJST:                  day,
					Title:                   title,
					Message:                 message,
					OneSignalNotificationID: oneSignalID,
					Recipients:              recipients,
				}); err != nil {
					log.Printf("notify-review-queue push log user=%s: %v", u.ID, err)
				}
				sent++
			}
			return map[string]any{"users": len(users), "sent": sent}, nil
		},
	)
}

func exportObsidianFavoritesFn(client inngestgo.Client, db *pgxpool.Pool, obsidianExport *service.ObsidianExportService) (inngestgo.ServableFunction, error) {
	obsidianRepo := repository.NewObsidianExportRepo(db)
	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "export-obsidian-favorites", Name: "Export Obsidian Favorites"},
		inngestgo.CronTrigger("0 * * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			if obsidianExport == nil {
				return map[string]any{"enabled": false}, nil
			}
			configs, err := obsidianRepo.ListEnabled(ctx)
			if err != nil {
				return nil, fmt.Errorf("list enabled obsidian exports: %w", err)
			}
			updated := 0
			skipped := 0
			failed := 0
			for _, cfg := range configs {
				res, runErr := obsidianExport.RunUser(ctx, cfg, 100)
				if runErr != nil {
					failed++
					log.Printf("export-obsidian-favorites user=%s: %v", cfg.UserID, runErr)
					_ = obsidianRepo.MarkRun(ctx, cfg.UserID, false)
					continue
				}
				updated += res.Updated
				skipped += res.Skipped
				failed += res.Failed
			}
			return map[string]any{
				"users":   len(configs),
				"updated": updated,
				"skipped": skipped,
				"failed":  failed,
			}, nil
		},
	)
}

func trackProviderModelUpdatesFn(client inngestgo.Client, db *pgxpool.Pool, oneSignal *service.OneSignalClient) (inngestgo.ServableFunction, error) {
	updateRepo := repository.NewProviderModelUpdateRepo(db)
	syncSvc := service.NewProviderModelSnapshotSyncService(
		repository.NewUserRepo(db),
		repository.NewUserSettingsRepo(db),
		updateRepo,
		repository.NewPushNotificationLogRepo(db),
		oneSignal,
		service.NewSecretCipher(),
	)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "track-provider-model-updates", Name: "Track Provider Model Updates"},
		inngestgo.CronTrigger("0 */6 * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			result, err := syncSvc.SyncCommonProviders(ctx, "cron")
			if err != nil {
				return nil, err
			}
			return map[string]any{"providers": result.Providers, "changes": result.Changes}, nil
		},
	)
}

func syncOpenRouterModelsFn(client inngestgo.Client, db *pgxpool.Pool, resend *service.ResendClient, oneSignal *service.OneSignalClient) (inngestgo.ServableFunction, error) {
	userRepo := repository.NewUserRepo(db)
	modelRepo := repository.NewOpenRouterModelRepo(db)
	updateRepo := repository.NewProviderModelUpdateRepo(db)
	openrouterSvc := service.NewOpenRouterCatalogService()
	openAI := service.NewOpenAIClient()
	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "sync-openrouter-models", Name: "Sync OpenRouter Models"},
		inngestgo.CronTrigger("0 3 * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			syncRunID, err := modelRepo.StartSyncRun(ctx, "cron")
			if err != nil {
				return nil, err
			}
			fetchedAt := time.Now().UTC()
			models, fetchErr := openrouterSvc.FetchTextGenerationModels(ctx)
			if fetchErr != nil {
				msg := fetchErr.Error()
				_ = modelRepo.FinishSyncRun(ctx, syncRunID, 0, 0, &msg)
				return nil, fetchErr
			}
			models = service.EnrichOpenRouterDescriptionsJA(ctx, modelRepo, openAI, models)
			if err := modelRepo.InsertSnapshots(ctx, syncRunID, fetchedAt, models); err != nil {
				msg := err.Error()
				_ = modelRepo.FinishSyncRun(ctx, syncRunID, len(models), 0, &msg)
				return nil, err
			}
			total := 0
			for _, item := range models {
				if item.DescriptionEN != nil && strings.TrimSpace(*item.DescriptionEN) != "" {
					total++
				}
			}
			if err := modelRepo.UpdateTranslationProgress(ctx, syncRunID, total, total); err != nil {
				return nil, err
			}
			if err := modelRepo.FinishSyncRun(ctx, syncRunID, len(models), len(models), nil); err != nil {
				return nil, err
			}
			service.SetDynamicChatModels(service.OpenRouterSnapshotsToCatalogModels(models))

			prevModels, err := modelRepo.ListPreviousSuccessfulSnapshots(ctx, syncRunID)
			if err != nil {
				return nil, err
			}
			addedModelIDs, constrainedModelIDs, removedModelIDs := diffOpenRouterModelAvailability(prevModels, models)
			if len(addedModelIDs) == 0 && len(constrainedModelIDs) == 0 && len(removedModelIDs) == 0 {
				return map[string]any{"fetched": len(models), "added_models": 0, "constrained_models": 0, "removed_models": 0}, nil
			}

			nowJST := timeutil.NowJST()
			changeEvents := make([]model.ProviderModelChangeEvent, 0, len(addedModelIDs)+len(constrainedModelIDs)+len(removedModelIDs))
			for _, modelID := range addedModelIDs {
				changeEvents = append(changeEvents, model.ProviderModelChangeEvent{
					Provider:   "openrouter",
					ChangeType: "added",
					ModelID:    modelID,
					DetectedAt: nowJST,
					Metadata:   map[string]any{"source": "openrouter_sync", "trigger": "cron"},
				})
			}
			for _, modelID := range constrainedModelIDs {
				changeEvents = append(changeEvents, model.ProviderModelChangeEvent{
					Provider:   "openrouter",
					ChangeType: "constrained",
					ModelID:    modelID,
					DetectedAt: nowJST,
					Metadata:   map[string]any{"source": "openrouter_sync", "trigger": "cron"},
				})
			}
			for _, modelID := range removedModelIDs {
				changeEvents = append(changeEvents, model.ProviderModelChangeEvent{
					Provider:   "openrouter",
					ChangeType: "removed",
					ModelID:    modelID,
					DetectedAt: nowJST,
					Metadata:   map[string]any{"source": "openrouter_sync", "trigger": "cron"},
				})
			}
			if len(changeEvents) > 0 {
				if err := updateRepo.InsertChangeEvents(ctx, changeEvents); err != nil {
					return nil, err
				}
			}

			users, err := userRepo.ListAll(ctx)
			if err != nil {
				return nil, err
			}
			title := buildOpenRouterModelAlertTitle(addedModelIDs, constrainedModelIDs, removedModelIDs)
			message := buildOpenRouterModelMessage(addedModelIDs, constrainedModelIDs, removedModelIDs)
			targetURL := appPageURL("/openrouter-models")
			day := timeutil.StartOfDayJST(nowJST)
			pushLogRepo := repository.NewPushNotificationLogRepo(db)
			for _, u := range users {
				alreadyNotified, err := pushLogRepo.CountByUserKindDay(ctx, u.ID, "openrouter_model_update", day)
				if err != nil || alreadyNotified > 0 {
					continue
				}
				var oneSignalID *string
				recipients := 0
				notified := false
				if oneSignal != nil && oneSignal.Enabled() {
					pushRes, pErr := oneSignal.SendToExternalID(
						ctx,
						u.Email,
						title,
						message,
						targetURL,
						map[string]any{
							"type":              "openrouter_model_update",
							"url":               targetURL,
							"added_count":       len(addedModelIDs),
							"constrained_count": len(constrainedModelIDs),
							"removed_count":     len(removedModelIDs),
						},
					)
					if pErr != nil {
						log.Printf("sync-openrouter-models push user=%s: %v", u.ID, pErr)
					} else {
						notified = true
						if pushRes != nil {
							if strings.TrimSpace(pushRes.ID) != "" {
								id := strings.TrimSpace(pushRes.ID)
								oneSignalID = &id
							}
							recipients = pushRes.Recipients
						}
					}
				}
				if resend != nil && resend.Enabled() && strings.TrimSpace(u.Email) != "" {
					if err := resend.SendOpenRouterModelAlert(ctx, u.Email, service.OpenRouterModelAlertEmail{
						Added:       limitStrings(addedModelIDs, 12),
						Constrained: limitStrings(constrainedModelIDs, 12),
						Removed:     limitStrings(removedModelIDs, 12),
						TargetURL:   targetURL,
					}); err != nil {
						log.Printf("sync-openrouter-models email user=%s: %v", u.ID, err)
					} else {
						notified = true
					}
				}
				if !notified {
					continue
				}
				if err := pushLogRepo.Insert(ctx, repository.PushNotificationLogInput{
					UserID:                  u.ID,
					Kind:                    "openrouter_model_update",
					ItemID:                  nil,
					DayJST:                  day,
					Title:                   title,
					Message:                 message,
					OneSignalNotificationID: oneSignalID,
					Recipients:              recipients,
				}); err != nil {
					log.Printf("sync-openrouter-models notify log user=%s: %v", u.ID, err)
				}
			}
			return map[string]any{
				"fetched":            len(models),
				"added_models":       len(addedModelIDs),
				"constrained_models": len(constrainedModelIDs),
				"removed_models":     len(removedModelIDs),
			}, nil
		},
	)
}

func syncPoeUsageHistoryFn(client inngestgo.Client, db *pgxpool.Pool, secretCipher *service.SecretCipher) (inngestgo.ServableFunction, error) {
	settingsRepo := repository.NewUserSettingsRepo(db)
	poeUsageRepo := repository.NewPoeUsageRepo(db)
	poeUsageSvc := service.NewPoeUsageService(poeUsageRepo)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "sync-poe-usage-history", Name: "Sync Poe Usage History"},
		inngestgo.CronTrigger("0 */6 * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			userIDs, err := settingsRepo.ListUserIDsWithPoeAPIKey(ctx)
			if err != nil {
				return nil, fmt.Errorf("list users with poe api key: %w", err)
			}
			synced := 0
			failed := 0
			skipped := 0
			for _, userID := range userIDs {
				id := userID
				apiKey, err := loadUserPoeAPIKey(ctx, settingsRepo, secretCipher, &id)
				if err != nil {
					log.Printf("sync-poe-usage-history load key user=%s: %v", userID, err)
					failed++
					continue
				}
				if apiKey == nil || strings.TrimSpace(*apiKey) == "" {
					skipped++
					continue
				}
				if _, err := poeUsageSvc.SyncHistory(ctx, userID, *apiKey, "cron"); err != nil {
					log.Printf("sync-poe-usage-history sync user=%s: %v", userID, err)
					failed++
					continue
				}
				synced++
			}
			return map[string]any{
				"users":   len(userIDs),
				"synced":  synced,
				"failed":  failed,
				"skipped": skipped,
			}, nil
		},
	)
}

func buildOpenRouterModelAlertTitle(added, constrained, removed []string) string {
	total := len(added) + len(constrained) + len(removed)
	return fmt.Sprintf("Sifto: OpenRouter モデル更新 %d 件", total)
}

func buildOpenRouterModelMessage(added, constrained, removed []string) string {
	parts := make([]string, 0, 3)
	if len(added) > 0 {
		parts = append(parts, fmt.Sprintf("追加 %d件", len(added)))
	}
	if len(constrained) > 0 {
		parts = append(parts, fmt.Sprintf("制約あり %d件", len(constrained)))
	}
	if len(removed) > 0 {
		parts = append(parts, fmt.Sprintf("削除 %d件", len(removed)))
	}
	if len(parts) == 0 {
		return "OpenRouter のモデル更新を検知しました。"
	}
	return strings.Join(parts, " / ")
}

func limitStrings(in []string, limit int) []string {
	if len(in) <= limit {
		return append([]string{}, in...)
	}
	return append([]string{}, in[:limit]...)
}

func diffOpenRouterModelAvailability(previous, current []repository.OpenRouterModelSnapshot) (added, constrained, removed []string) {
	prevMap := make(map[string]service.OpenRouterModelAvailability, len(previous))
	for _, item := range previous {
		state, _ := service.OpenRouterSnapshotAvailability(item)
		prevMap[item.ModelID] = state
	}
	currMap := make(map[string]service.OpenRouterModelAvailability, len(current))
	for _, item := range current {
		state, _ := service.OpenRouterSnapshotAvailability(item)
		currMap[item.ModelID] = state
		if _, existed := prevMap[item.ModelID]; !existed {
			added = append(added, item.ModelID)
			continue
		}
		if prevMap[item.ModelID] == service.OpenRouterModelAvailable && state == service.OpenRouterModelConstrained {
			constrained = append(constrained, item.ModelID)
		}
	}
	for _, item := range previous {
		if _, exists := currMap[item.ModelID]; !exists {
			removed = append(removed, item.ModelID)
		}
	}
	sort.Strings(added)
	sort.Strings(constrained)
	sort.Strings(removed)
	return added, constrained, removed
}

// ① cron/fetch-rss — 10分ごとにRSSを取得し新規アイテムを登録
func fetchRSSFn(client inngestgo.Client, db *pgxpool.Pool) (inngestgo.ServableFunction, error) {
	sourceRepo := repository.NewSourceRepo(db)
	itemRepo := repository.NewItemRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "fetch-rss", Name: "Fetch RSS Feeds"},
		inngestgo.CronTrigger("*/10 * * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			sources, err := sourceRepo.ListEnabled(ctx)
			if err != nil {
				return nil, fmt.Errorf("list sources: %w", err)
			}

			fp := gofeed.NewParser()
			newCount := 0

			for _, src := range sources {
				feed, err := fp.ParseURLWithContext(src.URL, ctx)
				if err != nil {
					log.Printf("fetch rss %s: %v", src.URL, err)
					_ = sourceRepo.UpdateLastFetchedAt(ctx, src.ID, timeutil.NowJST())
					reason := fmt.Sprintf("fetch error: %v", err)
					_ = sourceRepo.RefreshHealthSnapshot(ctx, src.ID, &reason)
					continue
				}

				for _, entry := range feed.Items {
					if entry.Link == "" {
						continue
					}
					var title *string
					if entry.Title != "" {
						title = &entry.Title
					}
					itemID, created, err := itemRepo.UpsertFromFeed(ctx, src.ID, entry.Link, title)
					if err != nil {
						log.Printf("upsert item %s: %v", entry.Link, err)
						continue
					}
					if !created {
						continue
					}
					newCount++
					reason := "fetch_rss"
					titleVal := title
					if _, err := client.Send(ctx, service.NewItemCreatedEvent(itemID, src.ID, entry.Link, titleVal, reason)); err != nil {
						log.Printf("send item/created: %v", err)
					}
				}
				_ = sourceRepo.UpdateLastFetchedAt(ctx, src.ID, timeutil.NowJST())
				_ = sourceRepo.RefreshHealthSnapshot(ctx, src.ID, nil)
			}
			return map[string]int{"new_items": newCount}, nil
		},
	)
}

// ② event/process-item — 本文抽出 → 事実抽出 → 要約（各stepでリトライ可能）
func processItemFn(client inngestgo.Client, db *pgxpool.Pool, worker *service.WorkerClient, openAI *service.OpenAIClient, oneSignal *service.OneSignalClient, secretCipher *service.SecretCipher, cache service.JSONCache) (inngestgo.ServableFunction, error) {
	deps := processItemDeps{
		itemRepo:           repository.NewItemInngestRepo(db),
		itemViewRepo:       repository.NewItemRepo(db),
		llmUsageRepo:       repository.NewLLMUsageLogRepo(db),
		llmExecutionRepo:   repository.NewLLMExecutionEventRepo(db),
		sourceRepo:         repository.NewSourceRepo(db),
		userSettingsRepo:   repository.NewUserSettingsRepo(db),
		userRepo:           repository.NewUserRepo(db),
		pushLogRepo:        repository.NewPushNotificationLogRepo(db),
		notificationRepo:   repository.NewNotificationPriorityRepo(db),
		readingGoalRepo:    repository.NewReadingGoalRepo(db),
		worker:             worker,
		openAI:             openAI,
		oneSignal:          oneSignal,
		publisher:          mustEventPublisher(),
		secretCipher:       secretCipher,
		cache:              cache,
		pickScoreThreshold: envFloat64OrDefault("ONESIGNAL_PICK_SCORE_THRESHOLD", 0.90),
		pickMaxPerDay:      envIntOrDefault("ONESIGNAL_PICK_MAX_PER_DAY", 2),
	}

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{
			ID:   "process-item",
			Name: "Process Item",
			Concurrency: []inngestgo.ConfigStepConcurrency{
				{
					Limit: 5,
				},
			},
			Throttle: &inngestgo.ConfigThrottle{
				Limit:  30,
				Period: time.Minute,
				Burst:  6,
			},
		},
		inngestgo.EventTrigger("item/created", nil),
		func(ctx context.Context, input inngestgo.Input[processItemEventData]) (any, error) {
			data := input.Event.Data
			ctx = withLLMExecutionTrigger(ctx, data.TriggerID, data.Reason)
			itemID := data.ItemID
			url := data.URL
			var userIDPtr *string
			if data.SourceID != "" {
				if uid, err := deps.sourceRepo.GetUserIDBySourceID(ctx, data.SourceID); err == nil {
					userIDPtr = &uid
				} else {
					log.Printf("process-item source owner lookup failed source_id=%s err=%v", data.SourceID, err)
				}
			}
			var userModelSettings *model.UserSettings
			if userIDPtr != nil && *userIDPtr != "" {
				userModelSettings, _ = deps.userSettingsRepo.GetByUserID(ctx, *userIDPtr)
			}
			log.Printf("process-item start item_id=%s url=%s trigger_id=%s reason=%s", itemID, url, strings.TrimSpace(data.TriggerID), strings.TrimSpace(data.Reason))

			// Step 1: 本文抽出
			var extracted *service.ExtractBodyResponse
			var err error
			for attempt := 0; attempt < 3; attempt++ {
				stepLabel := "extract-body"
				if attempt > 0 {
					stepLabel = fmt.Sprintf("extract-body-%d", attempt+1)
				}
				extracted, err = step.Run(ctx, stepLabel, func(ctx context.Context) (*service.ExtractBodyResponse, error) {
					log.Printf("process-item extract-body start item_id=%s attempt=%d", itemID, attempt+1)
					return deps.worker.ExtractBody(ctx, url)
				})
				if err == nil {
					break
				}
				log.Printf("process-item extract-body failed item_id=%s attempt=%d err=%v", itemID, attempt+1, err)
				if !shouldRetryExtractBody(attempt, err) {
					return nil, markProcessItemDeleted(ctx, deps.itemRepo, deps.cache, itemID, "extract body retried and deleted", err)
				}
			}
			log.Printf("process-item extract-body done item_id=%s content_len=%d", itemID, len(extracted.Content))
			if reason := invalidExtractReason(extracted.Title, extracted.Content); reason != "" {
				log.Printf("process-item invalid-extract deleted item_id=%s reason=%s", itemID, reason)
				return nil, markProcessItemDeleted(ctx, deps.itemRepo, deps.cache, itemID, reason, fmt.Errorf("content rejected after extract"))
			}

			if err := updateItemAfterExtract(ctx, deps.itemRepo, itemID, extracted); err != nil {
				log.Printf("process-item update-after-extract failed item_id=%s err=%v", itemID, err)
				return nil, fmt.Errorf("update after extract: %w", err)
			}
			bumpProcessItemDetailCacheVersion(ctx, deps.cache, itemID)
			log.Printf("process-item update-after-extract done item_id=%s", itemID)
			titleForLLM := resolveProcessItemTitleForLLM(extracted.Title, data.Title)
			factsStage, err := extractAndPersistFacts(ctx, deps, data, itemID, userIDPtr, userModelSettings, titleForLLM, extracted.Content)
			if err != nil {
				return nil, err
			}
			summaryStage, err := summarizeAndPersistItem(ctx, deps, data, itemID, userIDPtr, userModelSettings, titleForLLM, extracted.Content, factsStage.Facts.Facts)
			if err != nil {
				return nil, err
			}
			sendPickNotificationIfNeeded(ctx, deps, itemID, url, userIDPtr, titleForLLM, summaryStage.Summary)
			createEmbeddingIfPossible(ctx, deps, data, itemID, userIDPtr, userModelSettings, titleForLLM, summaryStage.Summary, factsStage.Facts.Facts)
			log.Printf("process-item complete item_id=%s", itemID)

			return map[string]string{"item_id": itemID, "status": "summarized"}, nil
		},
	)
}

func mustEventPublisher() *service.EventPublisher {
	publisher, err := service.NewEventPublisher()
	if err != nil {
		log.Fatalf("event publisher: %v", err)
	}
	return publisher
}

func embedItemFn(client inngestgo.Client, db *pgxpool.Pool, openAI *service.OpenAIClient, secretCipher *service.SecretCipher) (inngestgo.ServableFunction, error) {
	itemRepo := repository.NewItemInngestRepo(db)
	llmUsageRepo := repository.NewLLMUsageLogRepo(db)
	llmExecutionRepo := repository.NewLLMExecutionEventRepo(db)
	userSettingsRepo := repository.NewUserSettingsRepo(db)

	type EventData struct {
		ItemID   string `json:"item_id"`
		SourceID string `json:"source_id"`
	}

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "embed-item", Name: "Create Item Embedding"},
		inngestgo.EventTrigger("item/embed", nil),
		func(ctx context.Context, input inngestgo.Input[EventData]) (any, error) {
			data := input.Event.Data
			if data.ItemID == "" {
				return nil, fmt.Errorf("item_id is required")
			}

			candidate, err := itemRepo.GetEmbeddingCandidate(ctx, data.ItemID)
			if err != nil {
				return nil, fmt.Errorf("get embedding candidate: %w", err)
			}
			userID := candidate.UserID
			userOpenAIKey, err := loadUserOpenAIAPIKey(ctx, userSettingsRepo, secretCipher, &userID)
			if err != nil {
				return nil, err
			}
			userModelSettings, _ := userSettingsRepo.GetByUserID(ctx, userID)

			inputText := buildItemEmbeddingInput(candidate.Title, candidate.Summary, candidate.Topics, candidate.Facts)
			embModel := service.OpenAIEmbeddingModel()
			if userModelSettings != nil && userModelSettings.EmbeddingModel != nil && service.IsSupportedOpenAIEmbeddingModel(*userModelSettings.EmbeddingModel) {
				embModel = *userModelSettings.EmbeddingModel
			}
			embResp, err := step.Run(ctx, "create-embedding", func(ctx context.Context) (*service.CreateEmbeddingResponse, error) {
				return openAI.CreateEmbedding(ctx, *userOpenAIKey, embModel, inputText)
			})
			if err != nil {
				recordLLMExecutionFailure(ctx, llmExecutionRepo, "embedding", &embModel, 0, &candidate.UserID, &candidate.SourceID, &candidate.ItemID, nil, err)
				return nil, err
			}
			if err := itemRepo.UpsertEmbedding(ctx, candidate.ItemID, embModel, embResp.Embedding); err != nil {
				return nil, fmt.Errorf("upsert embedding: %w", err)
			}

			recordLLMUsage(ctx, llmUsageRepo, "embedding", embResp.LLM, &candidate.UserID, &candidate.SourceID, &candidate.ItemID, nil)
			recordLLMExecutionSuccess(ctx, llmExecutionRepo, "embedding", embResp.LLM, 0, &candidate.UserID, &candidate.SourceID, &candidate.ItemID, nil)
			return map[string]any{
				"item_id":    candidate.ItemID,
				"source_id":  candidate.SourceID,
				"dimensions": len(embResp.Embedding),
				"status":     "embedded",
				"model":      embModel,
			}, nil
		},
	)
}

func buildItemEmbeddingInput(title *string, summary string, topics, facts []string) string {
	out := ""
	if title != nil && *title != "" {
		out += "title: " + *title + "\n"
	}
	if summary != "" {
		out += "summary: " + summary + "\n"
	}
	if len(topics) > 0 {
		out += "topics: " + fmt.Sprintf("%v", topics) + "\n"
	}
	if len(facts) > 0 {
		out += "facts:\n"
		limit := len(facts)
		if limit > 12 {
			limit = 12
		}
		for i := 0; i < limit; i++ {
			out += "- " + facts[i] + "\n"
		}
	}
	return out
}

// ③ cron/generate-digest — 毎朝6:00 JST (UTC 21:00) にDigest生成
func generateDigestFn(client inngestgo.Client, db *pgxpool.Pool) (inngestgo.ServableFunction, error) {
	userRepo := repository.NewUserRepo(db)
	itemRepo := repository.NewItemInngestRepo(db)
	digestRepo := repository.NewDigestInngestRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "generate-digest", Name: "Generate Daily Digest"},
		inngestgo.CronTrigger("0 21 * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			users, err := userRepo.ListAll(ctx)
			if err != nil {
				return nil, fmt.Errorf("list users: %w", err)
			}

			today := timeutil.StartOfDayJST(timeutil.NowJST())
			since := today.AddDate(0, 0, -1)

			created := 0
			skippedSent := 0
			for _, u := range users {
				items, err := itemRepo.ListSummarizedForUser(ctx, u.ID, since, today)
				if err != nil || len(items) == 0 {
					continue
				}

				digestID, alreadySent, err := digestRepo.Create(ctx, u.ID, today, items)
				if err != nil {
					log.Printf("create digest for %s: %v", u.Email, err)
					continue
				}
				if alreadySent {
					skippedSent++
					continue
				}

				if _, err := client.Send(ctx, inngestgo.Event{
					Name: "digest/created",
					Data: map[string]any{
						"digest_id": digestID,
						"user_id":   u.ID,
						"to":        u.Email,
					},
				}); err != nil {
					log.Printf("send digest/created: %v", err)
				}
				created++
			}
			return map[string]int{
				"digests_created":      created,
				"digests_skipped_sent": skippedSent,
			}, nil
		},
	)
}

// ④ event/compose-digest-copy — メール本文生成（重い処理を分離）
func composeDigestCopyFn(client inngestgo.Client, db *pgxpool.Pool, worker *service.WorkerClient, secretCipher *service.SecretCipher) (inngestgo.ServableFunction, error) {
	digestRepo := repository.NewDigestInngestRepo(db)
	itemRepo := repository.NewItemRepo(db)
	llmUsageRepo := repository.NewLLMUsageLogRepo(db)
	llmExecutionRepo := repository.NewLLMExecutionEventRepo(db)
	userSettingsRepo := repository.NewUserSettingsRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "compose-digest-copy", Name: "Compose Digest Email Copy"},
		inngestgo.EventTrigger("digest/created", nil),
		func(ctx context.Context, input inngestgo.Input[DigestCreatedData]) (any, error) {
			data := input.Event.Data
			log.Printf("compose-digest-copy start digest_id=%s", data.DigestID)
			markStatus := func(status string, sendErr error) {
				var msg *string
				if sendErr != nil {
					s := sendErr.Error()
					if len(s) > 2000 {
						s = s[:2000]
					}
					msg = &s
				}
				if err := digestRepo.UpdateSendStatus(ctx, data.DigestID, status, msg); err != nil {
					log.Printf("compose-digest-copy update-status failed digest_id=%s status=%s err=%v", data.DigestID, status, err)
				}
			}
			userModelSettings, _ := userSettingsRepo.GetByUserID(ctx, data.UserID)

			// Read-only DB fetch does not need step state, and keeping large nested structs
			// out of step results avoids serialization/replay issues.
			digest, err := digestRepo.GetForEmail(ctx, data.DigestID)
			if err != nil {
				markStatus("fetch_failed", err)
				return nil, fmt.Errorf("fetch digest: %w", err)
			}
			log.Printf("compose-digest-copy fetched digest_id=%s items=%d", data.DigestID, len(digest.Items))

			if len(digest.Items) == 0 {
				log.Printf("compose-digest-copy skip-no-items digest_id=%s", data.DigestID)
				markStatus("skipped_no_items", nil)
				return map[string]string{"status": "skipped", "reason": "no items"}, nil
			}
			markStatus("processing", nil)

			if digest.EmailSubject != nil && digest.EmailBody != nil {
				log.Printf("compose-digest-copy reuse-copy digest_id=%s", data.DigestID)
			} else {
				_, err := step.Run(ctx, "compose-digest-copy", func(ctx context.Context) (string, error) {
					if err := composeDigestEmailCopy(ctx, digestRepo, itemRepo, userSettingsRepo, llmUsageRepo, llmExecutionRepo, processItemDeps{worker: worker, secretCipher: secretCipher}, data, digest, userModelSettings); err != nil {
						return "", err
					}
					return "stored", nil
				})
				if err != nil {
					markStatus("compose_failed", err)
					return nil, fmt.Errorf("compose digest copy: %w", err)
				}
			}

			if _, err := client.Send(ctx, inngestgo.Event{
				Name: "digest/copy-composed",
				Data: map[string]any{
					"digest_id": data.DigestID,
					"user_id":   data.UserID,
					"to":        data.To,
				},
			}); err != nil {
				markStatus("enqueue_send_failed", err)
				return nil, fmt.Errorf("send digest/copy-composed: %w", err)
			}
			log.Printf("compose-digest-copy complete digest_id=%s", data.DigestID)
			return map[string]string{"status": "composed", "digest_id": data.DigestID}, nil
		},
	)
}

// ⑤ event/send-digest — メール送信（compose完了後）
func sendDigestFn(client inngestgo.Client, db *pgxpool.Pool, worker *service.WorkerClient, resend *service.ResendClient, oneSignal *service.OneSignalClient, secretCipher *service.SecretCipher) (inngestgo.ServableFunction, error) {
	_ = worker
	_ = secretCipher
	digestRepo := repository.NewDigestInngestRepo(db)
	userSettingsRepo := repository.NewUserSettingsRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "send-digest", Name: "Send Digest Email"},
		inngestgo.EventTrigger("digest/copy-composed", nil),
		func(ctx context.Context, input inngestgo.Input[DigestCopyComposedData]) (any, error) {
			data := input.Event.Data
			log.Printf("send-digest start digest_id=%s to=%s", data.DigestID, data.To)
			markStatus := func(status string, sendErr error) {
				var msg *string
				if sendErr != nil {
					s := sendErr.Error()
					if len(s) > 2000 {
						s = s[:2000]
					}
					msg = &s
				}
				if err := digestRepo.UpdateSendStatus(ctx, data.DigestID, status, msg); err != nil {
					log.Printf("send-digest update-status failed digest_id=%s status=%s err=%v", data.DigestID, status, err)
				}
			}

			digest, err := digestRepo.GetForEmail(ctx, data.DigestID)
			if err != nil {
				markStatus("fetch_failed", err)
				return nil, fmt.Errorf("fetch digest: %w", err)
			}
			if digest.EmailSubject == nil || digest.EmailBody == nil {
				err := fmt.Errorf("digest email copy is missing")
				markStatus("compose_failed", err)
				return nil, err
			}
			if !resend.Enabled() {
				markStatus("skipped_resend_disabled", nil)
				return map[string]string{"status": "skipped", "reason": "resend_disabled"}, nil
			}
			digestEmailEnabled, err := userSettingsRepo.IsDigestEmailEnabled(ctx, data.UserID)
			if err != nil {
				markStatus("user_settings_failed", err)
				return nil, fmt.Errorf("load user digest email setting: %w", err)
			}
			if !digestEmailEnabled {
				markStatus("skipped_user_disabled", nil)
				return map[string]string{"status": "skipped", "reason": "user_disabled"}, nil
			}
			markStatus("processing", nil)

			_, err = step.Run(ctx, "send-email", func(ctx context.Context) (string, error) {
				if err := resend.SendDigest(ctx, data.To, digest, &service.DigestEmailCopy{
					Subject: *digest.EmailSubject,
					Body:    *digest.EmailBody,
				}); err != nil {
					return "", err
				}
				return "sent", nil
			})
			if err != nil {
				markStatus("send_email_failed", err)
				return nil, fmt.Errorf("send email: %w", err)
			}
			if err := digestRepo.UpdateSentAt(ctx, data.DigestID); err != nil {
				log.Printf("update sent_at: %v", err)
			}
			if oneSignal != nil && oneSignal.Enabled() {
				_, pErr := oneSignal.SendToExternalID(
					ctx,
					data.To,
					"Sifto: ダイジェストを配信しました",
					fmt.Sprintf("%s のダイジェストを配信しました。", digest.DigestDate),
					appPageURL("/digests/"+data.DigestID),
					map[string]any{
						"type":       "digest_sent",
						"digest_id":  data.DigestID,
						"digest_url": appPageURL("/digests/" + data.DigestID),
					},
				)
				if pErr != nil {
					log.Printf("send-digest push failed digest_id=%s to=%s: %v", data.DigestID, data.To, pErr)
				}
			}
			log.Printf("send-digest complete digest_id=%s", data.DigestID)
			return map[string]string{"status": "sent", "to": data.To}, nil
		},
	)
}

func checkBudgetAlertsFn(client inngestgo.Client, db *pgxpool.Pool, resend *service.ResendClient, oneSignal *service.OneSignalClient) (inngestgo.ServableFunction, error) {
	settingsRepo := repository.NewUserSettingsRepo(db)
	alertLogRepo := repository.NewBudgetAlertLogRepo(db)
	forecastAlertLogRepo := repository.NewBudgetForecastAlertLogRepo(db)
	llmUsageRepo := repository.NewLLMUsageLogRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "check-budget-alerts", Name: "Check Monthly Budget Alerts"},
		inngestgo.CronTrigger("0 0 * * *"), // 09:00 JST
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			if (resend == nil || !resend.Enabled()) && (oneSignal == nil || !oneSignal.Enabled()) {
				return map[string]any{"status": "skipped", "reason": "no_budget_alert_channel"}, nil
			}

			targets, err := settingsRepo.ListBudgetAlertTargets(ctx)
			if err != nil {
				return nil, fmt.Errorf("list budget alert targets: %w", err)
			}

			nowJST := timeutil.NowJST()
			monthStartJST := time.Date(nowJST.Year(), nowJST.Month(), 1, 0, 0, 0, 0, timeutil.JST)
			nextMonthJST := monthStartJST.AddDate(0, 1, 0)
			daysInMonth := nextMonthJST.AddDate(0, 0, -1).Day()
			elapsedDays := nowJST.Day()
			checked := 0
			sent := 0
			skipped := 0

			for _, tgt := range targets {
				checked++
				usedCostUSD, err := llmUsageRepo.SumEstimatedCostByUserBetween(ctx, tgt.UserID, monthStartJST, nextMonthJST)
				if err != nil {
					log.Printf("check-budget-alerts sum cost user_id=%s: %v", tgt.UserID, err)
					continue
				}
				if tgt.MonthlyBudgetUSD <= 0 {
					skipped++
					continue
				}
				remainingRatio := (tgt.MonthlyBudgetUSD - usedCostUSD) / tgt.MonthlyBudgetUSD
				thresholdRatio := float64(tgt.BudgetAlertThresholdPct) / 100.0
				remainingUSD := tgt.MonthlyBudgetUSD - usedCostUSD
				monthAvgDailyPace := 0.0
				if elapsedDays > 0 {
					monthAvgDailyPace = usedCostUSD / float64(elapsedDays)
				}
				forecastCostUSD := monthAvgDailyPace * float64(daysInMonth)
				forecastDeltaUSD := forecastCostUSD - tgt.MonthlyBudgetUSD

				sentThisTarget := false

				if remainingRatio < thresholdRatio {
					alreadySent, err := alertLogRepo.Exists(ctx, tgt.UserID, monthStartJST, tgt.BudgetAlertThresholdPct)
					if err != nil {
						log.Printf("check-budget-alerts exists user_id=%s: %v", tgt.UserID, err)
					} else if !alreadySent {
						emailSent := false
						pushSent := false
						if resend != nil && resend.Enabled() {
							if err := resend.SendBudgetAlert(ctx, tgt.Email, service.BudgetAlertEmail{
								MonthJST:           monthStartJST.Format("2006-01"),
								MonthlyBudgetUSD:   tgt.MonthlyBudgetUSD,
								UsedCostUSD:        usedCostUSD,
								RemainingBudgetUSD: remainingUSD,
								RemainingPct:       remainingRatio * 100,
								ThresholdPct:       tgt.BudgetAlertThresholdPct,
							}); err != nil {
								log.Printf("check-budget-alerts send user_id=%s email=%s: %v", tgt.UserID, tgt.Email, err)
							} else {
								emailSent = true
							}
						}
						if oneSignal != nil && oneSignal.Enabled() {
							if _, pErr := oneSignal.SendToExternalID(
								ctx,
								tgt.Email,
								"Sifto: 月次LLM予算アラート",
								fmt.Sprintf("残り予算がしきい値(%d%%)を下回りました。", tgt.BudgetAlertThresholdPct),
								appPageURL("/llm-usage"),
								map[string]any{
									"type":          "budget_alert",
									"month_jst":     monthStartJST.Format("2006-01"),
									"threshold_pct": tgt.BudgetAlertThresholdPct,
									"target_url":    appPageURL("/llm-usage"),
								},
							); pErr != nil {
								log.Printf("check-budget-alerts push user_id=%s email=%s: %v", tgt.UserID, tgt.Email, pErr)
							} else {
								pushSent = true
							}
						}
						if emailSent || pushSent {
							if err := alertLogRepo.Insert(ctx, tgt.UserID, monthStartJST, tgt.BudgetAlertThresholdPct, tgt.MonthlyBudgetUSD, usedCostUSD, remainingRatio); err != nil {
								log.Printf("check-budget-alerts log user_id=%s: %v", tgt.UserID, err)
							}
							sentThisTarget = true
						}
					}
				}

				shouldForecastAlert := usedCostUSD > tgt.MonthlyBudgetUSD || (elapsedDays >= 3 && forecastDeltaUSD > 0)
				if shouldForecastAlert {
					forecastSent, err := forecastAlertLogRepo.Exists(ctx, tgt.UserID, monthStartJST)
					if err != nil {
						log.Printf("check-budget-alerts forecast exists user_id=%s: %v", tgt.UserID, err)
					} else if !forecastSent {
						emailSent := false
						pushSent := false
						if resend != nil && resend.Enabled() {
							if err := resend.SendBudgetForecastAlert(ctx, tgt.Email, service.BudgetForecastAlertEmail{
								MonthJST:         monthStartJST.Format("2006-01"),
								MonthlyBudgetUSD: tgt.MonthlyBudgetUSD,
								UsedCostUSD:      usedCostUSD,
								ForecastCostUSD:  forecastCostUSD,
								ForecastDeltaUSD: forecastDeltaUSD,
							}); err != nil {
								log.Printf("check-budget-alerts forecast email user_id=%s email=%s: %v", tgt.UserID, tgt.Email, err)
							} else {
								emailSent = true
							}
						}
						if oneSignal != nil && oneSignal.Enabled() {
							message := fmt.Sprintf("月末着地予測が予算を $%.4f 上回っています。", forecastDeltaUSD)
							if usedCostUSD > tgt.MonthlyBudgetUSD {
								message = "今月のLLM予算をすでに超過しています。"
							}
							if _, pErr := oneSignal.SendToExternalID(
								ctx,
								tgt.Email,
								"Sifto: 月次LLM予算の着地予測アラート",
								message,
								appPageURL("/llm-usage"),
								map[string]any{
									"type":               "budget_forecast_alert",
									"month_jst":          monthStartJST.Format("2006-01"),
									"forecast_cost_usd":  forecastCostUSD,
									"forecast_delta_usd": forecastDeltaUSD,
									"target_url":         appPageURL("/llm-usage"),
								},
							); pErr != nil {
								log.Printf("check-budget-alerts forecast push user_id=%s email=%s: %v", tgt.UserID, tgt.Email, pErr)
							} else {
								pushSent = true
							}
						}
						if emailSent || pushSent {
							if err := forecastAlertLogRepo.Insert(ctx, tgt.UserID, monthStartJST, tgt.MonthlyBudgetUSD, usedCostUSD, forecastCostUSD, forecastDeltaUSD); err != nil {
								log.Printf("check-budget-alerts forecast log user_id=%s: %v", tgt.UserID, err)
							}
							sentThisTarget = true
						}
					}
				}

				if sentThisTarget {
					sent++
				} else {
					skipped++
				}
			}

			return map[string]any{
				"checked":   checked,
				"sent":      sent,
				"skipped":   skipped,
				"month_jst": monthStartJST.Format("2006-01"),
			}, nil
		},
	)
}

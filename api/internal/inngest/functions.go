package inngest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
	"github.com/inngest/inngestgo"
	"github.com/inngest/inngestgo/step"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcdole/gofeed"
)

var llmUsageCache service.JSONCache

func recordLLMUsage(ctx context.Context, repo *repository.LLMUsageLogRepo, purpose string, usage *service.LLMUsage, userID, sourceID, itemID, digestID *string) {
	if repo == nil || usage == nil {
		return
	}
	if usage.Provider == "" || usage.Model == "" {
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
		Model:                    usage.Model,
		PricingModelFamily:       usage.PricingModelFamily,
		PricingSource:            pricingSource,
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

func recordLLMExecutionSuccess(ctx context.Context, repo *repository.LLMExecutionEventRepo, purpose string, usage *service.LLMUsage, attemptIndex int, userID, sourceID, itemID, digestID *string) {
	if repo == nil || usage == nil {
		return
	}
	if usage.Provider == "" || usage.Model == "" {
		return
	}
	if err := repo.Insert(ctx, repository.LLMExecutionEventInput{
		UserID:       userID,
		SourceID:     sourceID,
		ItemID:       itemID,
		DigestID:     digestID,
		Provider:     usage.Provider,
		Model:        usage.Model,
		Purpose:      purpose,
		Status:       "success",
		AttemptIndex: attemptIndex,
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
	if err := repo.Insert(ctx, repository.LLMExecutionEventInput{
		UserID:        userID,
		SourceID:      sourceID,
		ItemID:        itemID,
		DigestID:      digestID,
		Provider:      provider,
		Model:         modelVal,
		Purpose:       purpose,
		Status:        "failure",
		AttemptIndex:  attemptIndex,
		EmptyResponse: emptyResponse,
		ErrorKind:     &errorKind,
		ErrorMessage:  &message,
	}); err != nil {
		log.Printf("record llm execution failure purpose=%s: %v", purpose, err)
		return
	}
	if userID != nil {
		_ = service.BumpUserLLMUsageCacheVersion(ctx, llmUsageCache, toVal(userID))
	}
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
	raw := fmt.Sprintf(
		"purpose=%s|provider=%s|model=%s|u=%s|s=%s|i=%s|d=%s|in=%d|out=%d|cw=%d|cr=%d",
		purpose,
		usage.Provider,
		usage.Model,
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

func validateDigestClusterDraftCompletion(text string) error {
	s := strings.TrimSpace(text)
	if len([]rune(s)) < 40 {
		return fmt.Errorf("cluster draft looks truncated")
	}
	if strings.Count(s, "```")%2 != 0 {
		return fmt.Errorf("cluster draft looks truncated")
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
		return fmt.Errorf("cluster draft looks truncated")
	}
	last := bullets[len(bullets)-1]
	if strings.HasPrefix(last, "-") || strings.HasPrefix(last, "・") || strings.HasPrefix(last, "•") {
		trimmed := strings.TrimSpace(strings.TrimLeft(last, "-・• "))
		if len([]rune(trimmed)) < 8 {
			return fmt.Errorf("cluster draft looks truncated")
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
			return fmt.Errorf("cluster draft looks truncated")
		}
		return nil
	}
	if !digestTextLooksComplete(s, 80) {
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

func ptrStringOrNil(v *string) *string {
	if v == nil || *v == "" {
		return nil
	}
	s := *v
	return &s
}

func loadLLMKeysForModel(ctx context.Context, settingsRepo *repository.UserSettingsRepo, cipher *service.SecretCipher, userID *string, model *string, purpose string) (*string, *string, *string, *string, *string, *string, *string, *string, *string, error) {
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
						return nil, nil, key, nil, nil, nil, nil, nil, &fallback, nil
					}
				case "google":
					if key, err := loadUserGoogleAPIKey(ctx, settingsRepo, cipher, userID); err == nil && key != nil && strings.TrimSpace(*key) != "" {
						fallback := service.DefaultLLMModelForPurpose(candidateProvider, purpose)
						return nil, key, nil, nil, nil, nil, nil, nil, &fallback, nil
					}
				case "deepseek":
					if key, err := loadUserDeepSeekAPIKey(ctx, settingsRepo, cipher, userID); err == nil && key != nil && strings.TrimSpace(*key) != "" {
						fallback := service.DefaultLLMModelForPurpose(candidateProvider, purpose)
						return nil, nil, nil, key, nil, nil, nil, nil, &fallback, nil
					}
				case "alibaba":
					if key, err := loadUserAlibabaAPIKey(ctx, settingsRepo, cipher, userID); err == nil && key != nil && strings.TrimSpace(*key) != "" {
						fallback := service.DefaultLLMModelForPurpose(candidateProvider, purpose)
						return nil, nil, nil, nil, key, nil, nil, nil, &fallback, nil
					}
				case "mistral":
					if key, err := loadUserMistralAPIKey(ctx, settingsRepo, cipher, userID); err == nil && key != nil && strings.TrimSpace(*key) != "" {
						fallback := service.DefaultLLMModelForPurpose(candidateProvider, purpose)
						return nil, nil, nil, nil, nil, key, nil, nil, &fallback, nil
					}
				case "xai":
					if key, err := loadUserXAIAPIKey(ctx, settingsRepo, cipher, userID); err == nil && key != nil && strings.TrimSpace(*key) != "" {
						fallback := service.DefaultLLMModelForPurpose(candidateProvider, purpose)
						return nil, nil, nil, nil, nil, nil, key, nil, &fallback, nil
					}
				case "openai":
					if key, err := loadUserOpenAIAPIKey(ctx, settingsRepo, cipher, userID); err == nil && key != nil && strings.TrimSpace(*key) != "" {
						fallback := service.DefaultLLMModelForPurpose(candidateProvider, purpose)
						return nil, nil, nil, nil, nil, nil, nil, key, &fallback, nil
					}
				case "anthropic":
					if key, err := loadUserAnthropicAPIKey(ctx, settingsRepo, cipher, userID); err == nil && key != nil && strings.TrimSpace(*key) != "" {
						fallback := service.DefaultLLMModelForPurpose(candidateProvider, purpose)
						return key, nil, nil, nil, nil, nil, nil, nil, &fallback, nil
					}
				}
			}
		}
	}
	switch provider {
	case "google":
		key, err := loadUserGoogleAPIKey(ctx, settingsRepo, cipher, userID)
		return nil, key, nil, nil, nil, nil, nil, nil, model, err
	case "groq":
		key, err := loadUserGroqAPIKey(ctx, settingsRepo, cipher, userID)
		return nil, nil, key, nil, nil, nil, nil, nil, model, err
	case "deepseek":
		key, err := loadUserDeepSeekAPIKey(ctx, settingsRepo, cipher, userID)
		return nil, nil, nil, key, nil, nil, nil, nil, model, err
	case "alibaba":
		key, err := loadUserAlibabaAPIKey(ctx, settingsRepo, cipher, userID)
		return nil, nil, nil, nil, key, nil, nil, nil, model, err
	case "mistral":
		key, err := loadUserMistralAPIKey(ctx, settingsRepo, cipher, userID)
		return nil, nil, nil, nil, nil, key, nil, nil, model, err
	case "xai":
		key, err := loadUserXAIAPIKey(ctx, settingsRepo, cipher, userID)
		return nil, nil, nil, nil, nil, nil, key, nil, model, err
	case "openai":
		key, err := loadUserOpenAIAPIKey(ctx, settingsRepo, cipher, userID)
		return nil, nil, nil, nil, nil, nil, nil, key, model, err
	default:
		key, err := loadUserAnthropicAPIKey(ctx, settingsRepo, cipher, userID)
		return key, nil, nil, nil, nil, nil, nil, nil, model, err
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
func NewHandler(db *pgxpool.Pool, worker *service.WorkerClient, resend *service.ResendClient, oneSignal *service.OneSignalClient, obsidianExport *service.ObsidianExportService, cache service.JSONCache) http.Handler {
	secretCipher := service.NewSecretCipher()
	openAI := service.NewOpenAIClient()
	llmUsageCache = cache
	client, err := inngestgo.NewClient(inngestgo.ClientOpts{
		AppID: "sifto-api",
	})
	if err != nil {
		log.Fatalf("inngest client: %v", err)
	}

	register := func(f inngestgo.ServableFunction, err error) {
		if err != nil {
			log.Fatalf("register function: %v", err)
		}
	}

	register(fetchRSSFn(client, db))
	register(processItemFn(client, db, worker, openAI, oneSignal, secretCipher))
	register(embedItemFn(client, db, openAI, secretCipher))
	register(generateBriefingSnapshotsFn(client, db, oneSignal))
	register(exportObsidianFavoritesFn(client, db, obsidianExport))
	register(trackProviderModelUpdatesFn(client, db, oneSignal))
	register(generateDigestFn(client, db))
	register(composeDigestCopyFn(client, db, worker, secretCipher))
	register(sendDigestFn(client, db, worker, resend, oneSignal, secretCipher))
	register(checkBudgetAlertsFn(client, db, resend, oneSignal))
	register(computePreferenceProfilesFn(client, db))
	register(computeTopicPulseDailyFn(client, db))

	return client.Serve()
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
					alreadyNotified, err := pushLogRepo.CountByUserKindDay(ctx, u.ID, "briefing_ready", today)
					if err != nil {
						log.Printf("generate-briefing-snapshots push count user=%s: %v", u.ID, err)
					} else if alreadyNotified == 0 {
						title := "Sifto: 今日のブリーフィングを更新しました"
						message := fmt.Sprintf("注目%d件・クラスタ%d件を確認できます。", len(payload.HighlightItems), len(payload.Clusters))
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
	userRepo := repository.NewUserRepo(db)
	updateRepo := repository.NewProviderModelUpdateRepo(db)
	discovery := service.NewProviderModelDiscoveryService()
	pushLogRepo := repository.NewPushNotificationLogRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "track-provider-model-updates", Name: "Track Provider Model Updates"},
		inngestgo.CronTrigger("0 */6 * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			results, err := discovery.DiscoverAll(ctx)
			if err != nil {
				return nil, err
			}
			now := timeutil.NowJST()
			events := make([]model.ProviderModelChangeEvent, 0)
			for _, res := range results {
				prev, err := updateRepo.GetSnapshot(ctx, res.Provider)
				if err != nil && !errors.Is(err, repository.ErrNotFound) {
					return nil, err
				}
				if prev != nil {
					prevSet := make(map[string]struct{}, len(prev.Models))
					for _, modelID := range prev.Models {
						prevSet[modelID] = struct{}{}
					}
					nextSet := make(map[string]struct{}, len(res.Models))
					for _, modelID := range res.Models {
						nextSet[modelID] = struct{}{}
						if _, ok := prevSet[modelID]; !ok {
							events = append(events, model.ProviderModelChangeEvent{
								Provider:   res.Provider,
								ChangeType: "added",
								ModelID:    modelID,
								DetectedAt: now,
								Metadata:   map[string]any{"source": "provider_api"},
							})
						}
					}
					for _, modelID := range prev.Models {
						if _, ok := nextSet[modelID]; !ok {
							events = append(events, model.ProviderModelChangeEvent{
								Provider:   res.Provider,
								ChangeType: "removed",
								ModelID:    modelID,
								DetectedAt: now,
								Metadata:   map[string]any{"source": "provider_api"},
							})
						}
					}
				}
				if err := updateRepo.UpsertSnapshot(ctx, res.Provider, res.Models, "ok", nil); err != nil {
					return nil, err
				}
			}
			if len(events) == 0 {
				return map[string]any{"providers": len(results), "changes": 0}, nil
			}
			if err := updateRepo.InsertChangeEvents(ctx, events); err != nil {
				return nil, err
			}

			if oneSignal != nil && oneSignal.Enabled() {
				users, err := userRepo.ListAll(ctx)
				if err != nil {
					return nil, err
				}
				added := 0
				removed := 0
				providers := make(map[string]struct{})
				for _, ev := range events {
					providers[ev.Provider] = struct{}{}
					if ev.ChangeType == "added" {
						added++
					} else if ev.ChangeType == "removed" {
						removed++
					}
				}
				title := "Sifto: LLMモデル更新を検知しました"
				message := fmt.Sprintf("追加%d件 / 削除%d件。%dプロバイダーで変更があります。", added, removed, len(providers))
				day := timeutil.StartOfDayJST(now)
				for _, u := range users {
					alreadyNotified, err := pushLogRepo.CountByUserKindDay(ctx, u.ID, "provider_model_update", day)
					if err != nil || alreadyNotified > 0 {
						continue
					}
					pushRes, pErr := oneSignal.SendToExternalID(
						ctx,
						u.Email,
						title,
						message,
						appPageURL("/settings"),
						map[string]any{
							"type":    "provider_model_update",
							"url":     appPageURL("/settings"),
							"added":   added,
							"removed": removed,
						},
					)
					if pErr != nil {
						log.Printf("track-provider-model-updates push user=%s: %v", u.ID, pErr)
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
						Kind:                    "provider_model_update",
						ItemID:                  nil,
						DayJST:                  day,
						Title:                   title,
						Message:                 message,
						OneSignalNotificationID: oneSignalID,
						Recipients:              recipients,
					}); err != nil {
						log.Printf("track-provider-model-updates push log user=%s: %v", u.ID, err)
					}
				}
			}
			return map[string]any{"providers": len(results), "changes": len(events)}, nil
		},
	)
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
					payload := map[string]any{
						"item_id":   itemID,
						"source_id": src.ID,
						"url":       entry.Link,
					}
					if title != nil && strings.TrimSpace(*title) != "" {
						payload["title"] = strings.TrimSpace(*title)
					}
					if _, err := client.Send(ctx, inngestgo.Event{
						Name: "item/created",
						Data: payload,
					}); err != nil {
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
func processItemFn(client inngestgo.Client, db *pgxpool.Pool, worker *service.WorkerClient, openAI *service.OpenAIClient, oneSignal *service.OneSignalClient, secretCipher *service.SecretCipher) (inngestgo.ServableFunction, error) {
	deps := processItemDeps{
		itemRepo:           repository.NewItemInngestRepo(db),
		llmUsageRepo:       repository.NewLLMUsageLogRepo(db),
		llmExecutionRepo:   repository.NewLLMExecutionEventRepo(db),
		sourceRepo:         repository.NewSourceRepo(db),
		userSettingsRepo:   repository.NewUserSettingsRepo(db),
		userRepo:           repository.NewUserRepo(db),
		pushLogRepo:        repository.NewPushNotificationLogRepo(db),
		notificationRepo:   repository.NewNotificationPriorityRepo(db),
		worker:             worker,
		openAI:             openAI,
		oneSignal:          oneSignal,
		secretCipher:       secretCipher,
		pickScoreThreshold: envFloat64OrDefault("ONESIGNAL_PICK_SCORE_THRESHOLD", 0.90),
		pickMaxPerDay:      envIntOrDefault("ONESIGNAL_PICK_MAX_PER_DAY", 2),
	}

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "process-item", Name: "Process Item"},
		inngestgo.EventTrigger("item/created", nil),
		func(ctx context.Context, input inngestgo.Input[processItemEventData]) (any, error) {
			data := input.Event.Data
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
			log.Printf("process-item start item_id=%s url=%s", itemID, url)

			// Step 1: 本文抽出
			extracted, err := step.Run(ctx, "extract-body", func(ctx context.Context) (*service.ExtractBodyResponse, error) {
				log.Printf("process-item extract-body start item_id=%s", itemID)
				return deps.worker.ExtractBody(ctx, url)
			})
			if err != nil {
				log.Printf("process-item extract-body failed item_id=%s err=%v", itemID, err)
				return nil, markProcessItemFailed(ctx, deps.itemRepo, itemID, "extract body", err)
			}
			log.Printf("process-item extract-body done item_id=%s content_len=%d", itemID, len(extracted.Content))

			if err := updateItemAfterExtract(ctx, deps.itemRepo, itemID, extracted); err != nil {
				log.Printf("process-item update-after-extract failed item_id=%s err=%v", itemID, err)
				return nil, fmt.Errorf("update after extract: %w", err)
			}
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

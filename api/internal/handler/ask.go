package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
)

type AskHandler struct {
	itemRepo     *repository.ItemRepo
	settingsRepo *repository.UserSettingsRepo
	llmUsageRepo *repository.LLMUsageLogRepo
	cipher       *service.SecretCipher
	worker       *service.WorkerClient
	openAI       *service.OpenAIClient
	cache        service.JSONCache
}

func NewAskHandler(
	itemRepo *repository.ItemRepo,
	settingsRepo *repository.UserSettingsRepo,
	llmUsageRepo *repository.LLMUsageLogRepo,
	cipher *service.SecretCipher,
	worker *service.WorkerClient,
	openAI *service.OpenAIClient,
	cache service.JSONCache,
) *AskHandler {
	return &AskHandler{
		itemRepo:     itemRepo,
		settingsRepo: settingsRepo,
		llmUsageRepo: llmUsageRepo,
		cipher:       cipher,
		worker:       worker,
		openAI:       openAI,
		cache:        cache,
	}
}

const askCacheTTL = 2 * time.Minute

func (h *AskHandler) Ask(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		Query      string   `json:"query"`
		Days       int      `json:"days"`
		UnreadOnly bool     `json:"unread_only"`
		Limit      int      `json:"limit"`
		SourceIDs  []string `json:"source_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	query := strings.TrimSpace(body.Query)
	if query == "" {
		http.Error(w, "query is required", http.StatusBadRequest)
		return
	}
	if body.Days <= 0 {
		body.Days = 30
	}
	if body.Limit <= 0 {
		body.Limit = 8
	}
	if body.Limit > 12 {
		body.Limit = 12
	}

	settings, err := h.settingsRepo.EnsureDefaults(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	embeddingModel := service.OpenAIEmbeddingModel()
	if settings.EmbeddingModel != nil && service.IsSupportedOpenAIEmbeddingModel(*settings.EmbeddingModel) {
		embeddingModel = *settings.EmbeddingModel
	}
	modelName := chooseAskModel(settings, settings.HasAnthropicAPIKey, settings.HasGoogleAPIKey, settings.HasGroqAPIKey, settings.HasDeepSeekAPIKey, settings.HasOpenAIAPIKey)
	if modelName == nil {
		http.Error(w, "anthropic or google or groq or deepseek or openai api key is required", http.StatusBadRequest)
		return
	}
	cacheKey := cacheKeyAsk(userID, query, *modelName, embeddingModel, body.Days, body.UnreadOnly, body.Limit, body.SourceIDs)
	cacheBust := r.URL.Query().Get("cache_bust") == "1"
	if h.cache != nil && !cacheBust {
		var cached model.AskResponse
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			askCacheCounter.hits.Add(1)
			incrCacheMetric(r.Context(), h.cache, userID, "ask.hit")
			writeJSON(w, cached)
			return
		} else if err != nil {
			askCacheCounter.errors.Add(1)
			incrCacheMetric(r.Context(), h.cache, userID, "ask.error")
		}
		askCacheCounter.misses.Add(1)
		incrCacheMetric(r.Context(), h.cache, userID, "ask.miss")
	} else if cacheBust && h.cache != nil {
		askCacheCounter.bypass.Add(1)
		incrCacheMetric(r.Context(), h.cache, userID, "ask.bypass")
	}
	openAIKey, err := loadAndDecryptUserSecret(r.Context(), h.settingsRepo.GetOpenAIAPIKeyEncrypted, h.cipher, userID, "user openai api key is required")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	embResp, err := h.openAI.CreateEmbedding(r.Context(), *openAIKey, embeddingModel, query)
	if err != nil {
		http.Error(w, fmt.Sprintf("create query embedding: %v", err), http.StatusBadGateway)
		return
	}
	recordAskLLMUsage(r.Context(), h.llmUsageRepo, "ask", embResp.LLM, &userID)

	candidates, err := h.itemRepo.AskCandidatesByEmbedding(r.Context(), userID, embResp.Embedding, body.Days, body.UnreadOnly, body.SourceIDs, body.Limit)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if len(candidates) == 0 {
		writeJSON(w, model.AskResponse{
			Query:        query,
			Answer:       "該当する記事はまだ見つかりませんでした。",
			Citations:    []model.AskCitation{},
			RelatedItems: []model.AskCandidate{},
		})
		return
	}

	anthropicKey, _ := loadAndDecryptUserSecret(r.Context(), h.settingsRepo.GetAnthropicAPIKeyEncrypted, h.cipher, userID, "")
	googleKey, _ := loadAndDecryptUserSecret(r.Context(), h.settingsRepo.GetGoogleAPIKeyEncrypted, h.cipher, userID, "")
	groqKey, _ := loadAndDecryptUserSecret(r.Context(), h.settingsRepo.GetGroqAPIKeyEncrypted, h.cipher, userID, "")
	deepseekKey, _ := loadAndDecryptUserSecret(r.Context(), h.settingsRepo.GetDeepSeekAPIKeyEncrypted, h.cipher, userID, "")
	openAIChatKey, _ := loadAndDecryptUserSecret(r.Context(), h.settingsRepo.GetOpenAIAPIKeyEncrypted, h.cipher, userID, "")
	modelName = chooseAskModel(settings, anthropicKey != nil, googleKey != nil, groqKey != nil, deepseekKey != nil, openAIChatKey != nil)
	if modelName == nil {
		http.Error(w, "anthropic or google or groq or deepseek or openai api key is required", http.StatusBadRequest)
		return
	}

	workerCandidates := make([]service.AskCandidate, 0, len(candidates))
	for _, c := range candidates {
		var publishedAt *string
		if c.PublishedAt != nil {
			v := c.PublishedAt.Format("2006-01-02T15:04:05Z07:00")
			publishedAt = &v
		}
		workerCandidates = append(workerCandidates, service.AskCandidate{
			ItemID:          c.ID,
			Title:           c.Title,
			TranslatedTitle: c.TranslatedTitle,
			URL:             c.URL,
			Summary:         c.Summary,
			Facts:           c.Facts,
			Topics:          c.SummaryTopics,
			PublishedAt:     publishedAt,
			Similarity:      c.Similarity,
		})
	}
	askResp, err := h.worker.AskWithModel(r.Context(), query, workerCandidates, anthropicKey, googleKey, groqKey, deepseekKey, openAIChatKey, modelName)
	if err != nil {
		http.Error(w, fmt.Sprintf("ask worker: %v", err), http.StatusBadGateway)
		return
	}
	recordAskLLMUsage(r.Context(), h.llmUsageRepo, "ask", askResp.LLM, &userID)

	citationMap := make(map[string]model.AskCandidate, len(candidates))
	for _, c := range candidates {
		citationMap[c.ID] = c
	}
	citations := make([]model.AskCitation, 0, len(askResp.Citations))
	seen := map[string]struct{}{}
	for _, c := range askResp.Citations {
		item, ok := citationMap[c.ItemID]
		if !ok {
			continue
		}
		if _, dup := seen[c.ItemID]; dup {
			continue
		}
		seen[c.ItemID] = struct{}{}
		citations = append(citations, model.AskCitation{
			ItemID:      item.ID,
			Title:       askCitationTitle(item),
			URL:         item.URL,
			Reason:      strings.TrimSpace(c.Reason),
			PublishedAt: askCitationPublishedAt(item),
			Topics:      item.SummaryTopics,
		})
	}
	if len(citations) == 0 {
		for _, item := range candidates[:minAskInt(3, len(candidates))] {
			citations = append(citations, model.AskCitation{
				ItemID:      item.ID,
				Title:       askCitationTitle(item),
				URL:         item.URL,
				Reason:      "類似度と内容一致で選定",
				PublishedAt: askCitationPublishedAt(item),
				Topics:      item.SummaryTopics,
			})
		}
	}
	if len(citations) < minAskInt(3, len(candidates)) {
		for _, item := range candidates {
			if _, dup := seen[item.ID]; dup {
				continue
			}
			seen[item.ID] = struct{}{}
			citations = append(citations, model.AskCitation{
				ItemID:      item.ID,
				Title:       askCitationTitle(item),
				URL:         item.URL,
				Reason:      "関連候補として補完",
				PublishedAt: askCitationPublishedAt(item),
				Topics:      item.SummaryTopics,
			})
			if len(citations) >= minAskInt(5, len(candidates)) {
				break
			}
		}
	}
	citationIndexByItemID := make(map[string]int, len(citations))
	for i, citation := range citations {
		citationIndexByItemID[citation.ItemID] = i + 1
	}
	answer := formatAskCitationMarkers(strings.TrimSpace(askResp.Answer), citationIndexByItemID)
	bullets := make([]string, 0, len(askResp.Bullets))
	for _, bullet := range askResp.Bullets {
		formatted := formatAskCitationMarkers(strings.TrimSpace(bullet), citationIndexByItemID)
		if formatted != "" {
			bullets = append(bullets, formatted)
		}
	}
	resp := model.AskResponse{
		Query:        query,
		Answer:       answer,
		Bullets:      bullets,
		Citations:    citations,
		RelatedItems: candidates,
	}
	if h.cache != nil {
		if err := h.cache.SetJSON(r.Context(), cacheKey, resp, askCacheTTL); err != nil {
			askCacheCounter.errors.Add(1)
			incrCacheMetric(r.Context(), h.cache, userID, "ask.error")
		}
	}
	writeJSON(w, resp)
}

func askCitationTitle(item model.AskCandidate) string {
	if item.TranslatedTitle != nil && strings.TrimSpace(*item.TranslatedTitle) != "" {
		return strings.TrimSpace(*item.TranslatedTitle)
	}
	if item.Title != nil && strings.TrimSpace(*item.Title) != "" {
		return strings.TrimSpace(*item.Title)
	}
	return item.URL
}

func askCitationPublishedAt(item model.AskCandidate) *string {
	if item.PublishedAt == nil {
		return nil
	}
	v := item.PublishedAt.Format("2006-01-02T15:04:05Z07:00")
	return &v
}

func chooseAskModel(settings *model.UserSettings, hasAnthropic, hasGoogle, hasGroq, hasDeepSeek, hasOpenAI bool) *string {
	if settings != nil && settings.AskModel != nil && strings.TrimSpace(*settings.AskModel) != "" {
		v := strings.TrimSpace(*settings.AskModel)
		switch service.LLMProviderForModel(&v) {
		case "google":
			if hasGoogle {
				return &v
			}
		case "groq":
			if hasGroq {
				return &v
			}
		case "deepseek":
			if hasDeepSeek {
				return &v
			}
		case "openai":
			if hasOpenAI {
				return &v
			}
		default:
			if hasAnthropic {
				return &v
			}
		}
	}
	if settings != nil && settings.DigestModel != nil && strings.TrimSpace(*settings.DigestModel) != "" {
		v := strings.TrimSpace(*settings.DigestModel)
		switch service.LLMProviderForModel(&v) {
		case "google":
			if hasGoogle {
				return &v
			}
		case "groq":
			if hasGroq {
				return &v
			}
		case "deepseek":
			if hasDeepSeek {
				return &v
			}
		case "openai":
			if hasOpenAI {
				return &v
			}
		default:
			if hasAnthropic {
				return &v
			}
		}
	}
	if settings != nil && settings.SummaryModel != nil && strings.TrimSpace(*settings.SummaryModel) != "" {
		v := strings.TrimSpace(*settings.SummaryModel)
		switch service.LLMProviderForModel(&v) {
		case "google":
			if hasGoogle {
				return &v
			}
		case "groq":
			if hasGroq {
				return &v
			}
		case "deepseek":
			if hasDeepSeek {
				return &v
			}
		case "openai":
			if hasOpenAI {
				return &v
			}
		default:
			if hasAnthropic {
				return &v
			}
		}
	}
	for _, provider := range service.CostEfficientLLMProviders("") {
		switch provider {
		case "groq":
			if hasGroq {
				v := service.DefaultLLMModelForPurpose(provider, "ask")
				return &v
			}
		case "google":
			if hasGoogle {
				v := service.DefaultLLMModelForPurpose(provider, "ask")
				return &v
			}
		case "deepseek":
			if hasDeepSeek {
				v := service.DefaultLLMModelForPurpose(provider, "ask")
				return &v
			}
		case "openai":
			if hasOpenAI {
				v := service.DefaultLLMModelForPurpose(provider, "ask")
				return &v
			}
		case "anthropic":
			if hasAnthropic {
				v := service.DefaultLLMModelForPurpose(provider, "ask")
				return &v
			}
		}
	}
	return nil
}

func loadAndDecryptUserSecret(
	ctx context.Context,
	load func(context.Context, string) (*string, error),
	cipher *service.SecretCipher,
	userID string,
	notFoundMessage string,
) (*string, error) {
	if load == nil {
		return nil, fmt.Errorf("secret loader is not configured")
	}
	enc, err := load(ctx, userID)
	if err != nil {
		return nil, err
	}
	if enc == nil || strings.TrimSpace(*enc) == "" {
		if notFoundMessage == "" {
			return nil, nil
		}
		return nil, errors.New(notFoundMessage)
	}
	if cipher == nil || !cipher.Enabled() {
		return nil, fmt.Errorf("user secret encryption is not configured")
	}
	plain, err := cipher.DecryptString(*enc)
	if err != nil {
		return nil, err
	}
	return &plain, nil
}

func recordAskLLMUsage(ctx context.Context, repo *repository.LLMUsageLogRepo, purpose string, usage *service.LLMUsage, userID *string) {
	if repo == nil || usage == nil || userID == nil || *userID == "" {
		return
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%d|%d|%d|%d", purpose, usage.Provider, usage.Model, *userID, usage.InputTokens, usage.OutputTokens, usage.CacheCreationInputTokens, usage.CacheReadInputTokens)))
	key := hex.EncodeToString(sum[:])
	pricingSource := usage.PricingSource
	if pricingSource == "" {
		pricingSource = "unknown"
	}
	_ = repo.Insert(ctx, repository.LLMUsageLogInput{
		IdempotencyKey:           &key,
		UserID:                   userID,
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
	})
}

func minAskInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var askCitationMarkerPattern = regexp.MustCompile(`\[\[([a-zA-Z0-9-]+)\]\]`)

func formatAskCitationMarkers(text string, citationIndexByItemID map[string]int) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	if len(citationIndexByItemID) == 0 {
		return askCitationMarkerPattern.ReplaceAllString(text, "")
	}
	used := map[int]struct{}{}
	out := askCitationMarkerPattern.ReplaceAllStringFunc(text, func(match string) string {
		groups := askCitationMarkerPattern.FindStringSubmatch(match)
		if len(groups) != 2 {
			return ""
		}
		n, ok := citationIndexByItemID[strings.TrimSpace(groups[1])]
		if !ok {
			return ""
		}
		if _, dup := used[n]; dup {
			return ""
		}
		used[n] = struct{}{}
		return fmt.Sprintf("[%d]", n)
	})
	out = strings.Join(strings.Fields(out), " ")
	out = strings.ReplaceAll(out, " 。", "。")
	out = strings.ReplaceAll(out, " 、", "、")
	return strings.TrimSpace(out)
}

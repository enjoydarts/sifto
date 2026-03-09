package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/minoru-kitayama/sifto/api/internal/middleware"
	"github.com/minoru-kitayama/sifto/api/internal/model"
	"github.com/minoru-kitayama/sifto/api/internal/repository"
	"github.com/minoru-kitayama/sifto/api/internal/service"
)

type AskHandler struct {
	itemRepo     *repository.ItemRepo
	settingsRepo *repository.UserSettingsRepo
	llmUsageRepo *repository.LLMUsageLogRepo
	cipher       *service.SecretCipher
	worker       *service.WorkerClient
	openAI       *service.OpenAIClient
}

func NewAskHandler(
	itemRepo *repository.ItemRepo,
	settingsRepo *repository.UserSettingsRepo,
	llmUsageRepo *repository.LLMUsageLogRepo,
	cipher *service.SecretCipher,
	worker *service.WorkerClient,
	openAI *service.OpenAIClient,
) *AskHandler {
	return &AskHandler{
		itemRepo:     itemRepo,
		settingsRepo: settingsRepo,
		llmUsageRepo: llmUsageRepo,
		cipher:       cipher,
		worker:       worker,
		openAI:       openAI,
	}
}

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
	openAIKey, err := loadAndDecryptUserSecret(r.Context(), h.settingsRepo.GetOpenAIAPIKeyEncrypted, h.cipher, userID, "user openai api key is required")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	embeddingModel := service.OpenAIEmbeddingModel()
	if settings.OpenAIEmbeddingModel != nil && service.IsSupportedOpenAIEmbeddingModel(*settings.OpenAIEmbeddingModel) {
		embeddingModel = *settings.OpenAIEmbeddingModel
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
	modelName := chooseAskModel(settings, anthropicKey != nil, googleKey != nil)
	if modelName == nil {
		http.Error(w, "anthropic or google api key is required", http.StatusBadRequest)
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
	askResp, err := h.worker.AskWithModel(r.Context(), query, workerCandidates, anthropicKey, googleKey, modelName)
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
	writeJSON(w, model.AskResponse{
		Query:        query,
		Answer:       strings.TrimSpace(askResp.Answer),
		Bullets:      askResp.Bullets,
		Citations:    citations,
		RelatedItems: candidates,
	})
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

func chooseAskModel(settings *model.UserSettings, hasAnthropic, hasGoogle bool) *string {
	if settings != nil && settings.AnthropicAskModel != nil && strings.TrimSpace(*settings.AnthropicAskModel) != "" {
		v := strings.TrimSpace(*settings.AnthropicAskModel)
		if strings.HasPrefix(strings.ToLower(v), "gemini-") {
			if hasGoogle {
				return &v
			}
		} else if hasAnthropic {
			return &v
		}
	}
	if settings != nil && settings.AnthropicDigestModel != nil && strings.TrimSpace(*settings.AnthropicDigestModel) != "" {
		v := strings.TrimSpace(*settings.AnthropicDigestModel)
		if strings.HasPrefix(strings.ToLower(v), "gemini-") {
			if hasGoogle {
				return &v
			}
		} else if hasAnthropic {
			return &v
		}
	}
	if settings != nil && settings.AnthropicSummaryModel != nil && strings.TrimSpace(*settings.AnthropicSummaryModel) != "" {
		v := strings.TrimSpace(*settings.AnthropicSummaryModel)
		if strings.HasPrefix(strings.ToLower(v), "gemini-") {
			if hasGoogle {
				return &v
			}
		} else if hasAnthropic {
			return &v
		}
	}
	if hasAnthropic {
		v := "claude-sonnet-4-6"
		return &v
	}
	if hasGoogle {
		v := "gemini-2.5-flash"
		return &v
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

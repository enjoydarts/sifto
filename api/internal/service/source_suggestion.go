package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/mmcdole/gofeed"
)

type FeedCandidate struct {
	URL   string  `json:"url"`
	Title *string `json:"title"`
}

var (
	reFeedLink1 = regexp.MustCompile(`(?i)<link[^>]+type="application/(rss|atom)\+xml"[^>]+href="([^"]+)"`)
	reFeedLink2 = regexp.MustCompile(`(?i)<link[^>]+href="([^"]+)"[^>]+type="application/(rss|atom)\+xml"`)
	reTitleAttr = regexp.MustCompile(`(?i)\btitle="([^"]+)"`)
)

func DiscoverRSSFeeds(ctx context.Context, rawURL string) ([]FeedCandidate, error) {
	fp := gofeed.NewParser()
	if feed, err := fp.ParseURLWithContext(rawURL, ctx); err == nil {
		var t *string
		if feed.Title != "" {
			t = &feed.Title
		}
		return []FeedCandidate{{URL: rawURL, Title: t}}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Sifto/1.0")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	base, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var candidates []FeedCandidate

	addCandidate := func(href string, tag []byte) {
		ref, e := url.Parse(href)
		if e != nil {
			return
		}
		absURL := base.ResolveReference(ref).String()
		if seen[absURL] {
			return
		}
		seen[absURL] = true
		var title *string
		if m := reTitleAttr.FindSubmatch(tag); m != nil {
			t := string(m[1])
			title = &t
		}
		candidates = append(candidates, FeedCandidate{URL: absURL, Title: title})
	}

	for _, m := range reFeedLink1.FindAllSubmatch(body, -1) {
		addCandidate(string(m[2]), m[0])
	}
	for _, m := range reFeedLink2.FindAllSubmatch(body, -1) {
		addCandidate(string(m[1]), m[0])
	}

	if len(candidates) == 0 {
		return nil, errors.New("指定されたURLからRSSフィードが見つかりませんでした")
	}
	return candidates, nil
}

type SourceSuggestionResponse struct {
	URL           string   `json:"url"`
	Title         *string  `json:"title"`
	Reasons       []string `json:"reasons"`
	MatchedTopics []string `json:"matched_topics,omitempty"`
	AIReason      *string  `json:"ai_reason,omitempty"`
	AIConfidence  *float64 `json:"ai_confidence,omitempty"`
	SeedSourceIDs []string `json:"seed_source_ids"`
}

type sourceSuggestionAgg struct {
	URL           string
	Title         *string
	Reasons       map[string]bool
	MatchedTopics map[string]bool
	SeedSourceIDs map[string]bool
	Score         int
}

type probeSeed struct {
	SourceID string
	ProbeURL string
	Reason   string
}

type suggestionProbe struct {
	url    string
	reason string
}

type SourceSuggestionService struct {
	repo         *repository.SourceRepo
	itemRepo     *repository.ItemRepo
	settingsRepo *repository.UserSettingsRepo
	llmUsageRepo *repository.LLMUsageLogRepo
	worker       *WorkerClient
	cache        JSONCache
	keyProvider  *UserKeyProvider
}

func NewSourceSuggestionService(
	repo *repository.SourceRepo,
	itemRepo *repository.ItemRepo,
	settingsRepo *repository.UserSettingsRepo,
	llmUsageRepo *repository.LLMUsageLogRepo,
	worker *WorkerClient,
	cache JSONCache,
	keyProvider *UserKeyProvider,
) *SourceSuggestionService {
	return &SourceSuggestionService{
		repo:         repo,
		itemRepo:     itemRepo,
		settingsRepo: settingsRepo,
		llmUsageRepo: llmUsageRepo,
		worker:       worker,
		cache:        cache,
		keyProvider:  keyProvider,
	}
}

func (s *SourceSuggestionService) BuildSourceRecommendations(ctx context.Context, userID string, limit int) ([]SourceSuggestionResponse, map[string]any, error) {
	sources, err := s.repo.List(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if len(sources) == 0 {
		return []SourceSuggestionResponse{}, nil, nil
	}
	allKeys := s.keyProvider.GetAllKeys(ctx, userID)
	anthropicAPIKey := allKeys["anthropic"]
	googleAPIKey := allKeys["google"]
	groqAPIKey := allKeys["groq"]
	fireworksAPIKey := allKeys["fireworks"]
	deepseekAPIKey := allKeys["deepseek"]
	alibabaAPIKey := allKeys["alibaba"]
	mistralAPIKey := allKeys["mistral"]
	togetherAPIKey := allKeys["together"]
	moonshotAPIKey := allKeys["moonshot"]
	minimaxAPIKey := allKeys["minimax"]
	xiaomiMiMoTokenPlanAPIKey := allKeys["xiaomi_mimo_token_plan"]
	xaiAPIKey := allKeys["xai"]
	zaiAPIKey := allKeys["zai"]
	openRouterAPIKey := allKeys["openrouter"]
	poeAPIKey := allKeys["poe"]
	siliconFlowAPIKey := allKeys["siliconflow"]
	featherlessAPIKey := allKeys["featherless"]
	openAIAPIKey := allKeys["openai"]
	anthropicSourceSuggestionModel := s.getUserSourceSuggestionModel(ctx, userID)
	resolved := selectSourceSuggestionLLM(
		anthropicAPIKey,
		googleAPIKey,
		groqAPIKey,
		fireworksAPIKey,
		deepseekAPIKey,
		alibabaAPIKey,
		mistralAPIKey,
		togetherAPIKey,
		moonshotAPIKey,
		minimaxAPIKey,
		xiaomiMiMoTokenPlanAPIKey,
		xaiAPIKey,
		zaiAPIKey,
		openRouterAPIKey,
		poeAPIKey,
		siliconFlowAPIKey,
		featherlessAPIKey,
		openAIAPIKey,
		anthropicSourceSuggestionModel,
	)
	var preferredTopics []string
	if s.itemRepo != nil {
		if topics, err := s.itemRepo.PositiveFeedbackTopics(ctx, userID, 8); err == nil {
			preferredTopics = topics
		}
	}
	positiveExamples, negativeExamples := s.buildSourceSuggestionFewShotExamples(ctx, userID)

	registered := map[string]bool{}
	startAt := time.Now()
	const suggestionMaxLatency = 60 * time.Second
	isOverBudget := func() bool { return time.Since(startAt) >= suggestionMaxLatency }
	remainingSuggestionBudget := func() time.Duration {
		if d := suggestionMaxLatency - time.Since(startAt); d > 0 {
			return d
		}
		return 0
	}
	var probes []probeSeed
	seenProbe := map[string]bool{}
	for _, src := range sources {
		registered[normalizeFeedURL(src.URL)] = true
		for _, p := range suggestionProbeURLs(src.URL) {
			if seenProbe[p.url] {
				continue
			}
			seenProbe[p.url] = true
			probes = append(probes, probeSeed{
				SourceID: src.ID,
				ProbeURL: p.url,
				Reason:   p.reason,
			})
		}
	}
	if len(probes) > 12 {
		probes = probes[:12]
	}

	cands := map[string]*sourceSuggestionAgg{}
	aiReady := (resolved.AnthropicAPIKey != nil || resolved.GoogleAPIKey != nil || resolved.GroqAPIKey != nil || resolved.FireworksAPIKey != nil || resolved.DeepseekAPIKey != nil || resolved.AlibabaAPIKey != nil || resolved.MistralAPIKey != nil || resolved.TogetherAPIKey != nil || resolved.MoonshotAPIKey != nil || resolved.MiniMaxAPIKey != nil || resolved.XiaomiMiMoTokenPlanAPIKey != nil || resolved.XAIAPIKey != nil || resolved.ZAIAPIKey != nil || resolved.OpenAIAPIKey != nil || resolved.OpenRouterAPIKey != nil || resolved.PoeAPIKey != nil || resolved.SiliconFlowAPIKey != nil || resolved.FeatherlessAPIKey != nil) && s.worker != nil
	var seedLLMMeta map[string]any
	timedOutInAiStep := false
	if aiReady {
		seedLLMMeta, timedOutInAiStep = s.expandSourceSuggestionsWithLLMSeeds(
			ctx,
			userID,
			sources,
			preferredTopics,
			positiveExamples,
			negativeExamples,
			registered,
			cands,
			resolved.AnthropicAPIKey,
			resolved.GoogleAPIKey,
			resolved.GroqAPIKey,
			resolved.FireworksAPIKey,
			resolved.DeepseekAPIKey,
			resolved.AlibabaAPIKey,
			resolved.MistralAPIKey,
			resolved.TogetherAPIKey,
			resolved.MoonshotAPIKey,
			resolved.MiniMaxAPIKey,
			resolved.OpenRouterAPIKey,
			resolved.PoeAPIKey,
			resolved.SiliconFlowAPIKey,
			resolved.FeatherlessAPIKey,
			resolved.XAIAPIKey,
			resolved.ZAIAPIKey,
			resolved.OpenAIAPIKey,
			resolved.SelectedModel,
			remainingSuggestionBudget,
		)
	}
	if !aiReady {
		populateSourceSuggestionsFromProbes(ctx, probes, preferredTopics, registered, cands, remainingSuggestionBudget, DiscoverRSSFeeds)
	}

	out := make([]SourceSuggestionResponse, 0, len(cands))
	type sortable struct {
		row   SourceSuggestionResponse
		score int
	}
	rows := make([]sortable, 0, len(cands))
	for _, a := range cands {
		reasons := mapKeys(a.Reasons)
		matchedTopics := mapKeys(a.MatchedTopics)
		seedIDs := mapKeys(a.SeedSourceIDs)
		if len(matchedTopics) > 0 {
			reasons = append([]string{"高評価トピックに近い候補"}, reasons...)
		}
		rows = append(rows, sortable{
			score: a.Score,
			row: SourceSuggestionResponse{
				URL:           a.URL,
				Title:         a.Title,
				Reasons:       reasons,
				MatchedTopics: matchedTopics,
				SeedSourceIDs: seedIDs,
			},
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].score != rows[j].score {
			return rows[i].score > rows[j].score
		}
		if rows[i].row.Title != nil && rows[j].row.Title != nil && *rows[i].row.Title != *rows[j].row.Title {
			return *rows[i].row.Title < *rows[j].row.Title
		}
		return rows[i].row.URL < rows[j].row.URL
	})
	poolLimit := limit * 6
	if poolLimit < 24 {
		poolLimit = 24
	}
	if poolLimit > 120 {
		poolLimit = 120
	}
	if len(rows) > poolLimit {
		rows = rows[:poolLimit]
	}
	for _, r := range rows {
		out = append(out, r.row)
	}
	llmMeta := s.rankSourceSuggestionsWithLLM(
		ctx,
		userID,
		sources,
		preferredTopics,
		positiveExamples,
		negativeExamples,
		out,
		resolved.AnthropicAPIKey,
		resolved.GoogleAPIKey,
		resolved.GroqAPIKey,
		resolved.FireworksAPIKey,
		resolved.DeepseekAPIKey,
		resolved.AlibabaAPIKey,
		resolved.MistralAPIKey,
		resolved.TogetherAPIKey,
		resolved.MoonshotAPIKey,
		resolved.MiniMaxAPIKey,
		resolved.OpenRouterAPIKey,
		resolved.PoeAPIKey,
		resolved.SiliconFlowAPIKey,
		resolved.FeatherlessAPIKey,
		resolved.XAIAPIKey,
		resolved.ZAIAPIKey,
		resolved.OpenAIAPIKey,
		resolved.SelectedModel,
		remainingSuggestionBudget,
	)
	if llmMeta == nil && seedLLMMeta != nil {
		llmMeta = seedLLMMeta
	}
	if isOverBudget() {
		llmMeta = mergeLLMWarning(llmMeta, "source suggestion timed out; partial results returned", "timeout")
	}
	if timedOutInAiStep {
		llmMeta = mergeLLMWarning(llmMeta, "source suggestion timed out during AI seed generation", "seed_generation")
	}
	if resolved.SelectedModel != nil && strings.TrimSpace(*resolved.SelectedModel) != "" {
		if llmMeta == nil {
			llmMeta = map[string]any{}
		}
		if _, ok := llmMeta["requested_model"]; !ok {
			llmMeta["requested_model"] = strings.TrimSpace(*resolved.SelectedModel)
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, llmMeta, nil
}

func (s *SourceSuggestionService) rankSourceSuggestionsWithLLM(
	ctx context.Context,
	userID string,
	sources []model.Source,
	preferredTopics []string,
	positiveExamples []RankFeedSuggestionsExample,
	negativeExamples []RankFeedSuggestionsExample,
	suggestions []SourceSuggestionResponse,
	anthropicAPIKey *string,
	googleAPIKey *string,
	groqAPIKey *string,
	fireworksAPIKey *string,
	deepseekAPIKey *string,
	alibabaAPIKey *string,
	mistralAPIKey *string,
	togetherAPIKey *string,
	moonshotAPIKey *string,
	minimaxAPIKey *string,
	openRouterAPIKey *string,
	poeAPIKey *string,
	siliconFlowAPIKey *string,
	featherlessAPIKey *string,
	xaiAPIKey *string,
	zaiAPIKey *string,
	openAIAPIKey *string,
	model *string,
	remainingSuggestionBudget func() time.Duration,
) map[string]any {
	if s.worker == nil || len(suggestions) == 0 {
		return nil
	}
	if remainingSuggestionBudget == nil {
		remainingSuggestionBudget = func() time.Duration { return 0 }
	}
	if remainingSuggestionBudget() <= 0 {
		return map[string]any{
			"warning": "source suggestion rank skipped due timeout budget",
			"stage":   "rank",
		}
	}
	hasAnthropic := anthropicAPIKey != nil && strings.TrimSpace(*anthropicAPIKey) != ""
	hasGoogle := googleAPIKey != nil && strings.TrimSpace(*googleAPIKey) != ""
	hasGroq := groqAPIKey != nil && strings.TrimSpace(*groqAPIKey) != ""
	hasFireworks := fireworksAPIKey != nil && strings.TrimSpace(*fireworksAPIKey) != ""
	hasDeepSeek := deepseekAPIKey != nil && strings.TrimSpace(*deepseekAPIKey) != ""
	hasAlibaba := alibabaAPIKey != nil && strings.TrimSpace(*alibabaAPIKey) != ""
	hasMistral := mistralAPIKey != nil && strings.TrimSpace(*mistralAPIKey) != ""
	hasTogether := togetherAPIKey != nil && strings.TrimSpace(*togetherAPIKey) != ""
	hasMoonshot := moonshotAPIKey != nil && strings.TrimSpace(*moonshotAPIKey) != ""
	hasOpenRouter := openRouterAPIKey != nil && strings.TrimSpace(*openRouterAPIKey) != ""
	hasPoe := poeAPIKey != nil && strings.TrimSpace(*poeAPIKey) != ""
	hasSiliconFlow := siliconFlowAPIKey != nil && strings.TrimSpace(*siliconFlowAPIKey) != ""
	hasFeatherless := featherlessAPIKey != nil && strings.TrimSpace(*featherlessAPIKey) != ""
	hasXAI := xaiAPIKey != nil && strings.TrimSpace(*xaiAPIKey) != ""
	hasZAI := zaiAPIKey != nil && strings.TrimSpace(*zaiAPIKey) != ""
	hasOpenAI := openAIAPIKey != nil && strings.TrimSpace(*openAIAPIKey) != ""
	if !hasAnthropic && !hasGoogle && !hasGroq && !hasFireworks && !hasDeepSeek && !hasAlibaba && !hasMistral && !hasTogether && !hasMoonshot && !hasOpenRouter && !hasPoe && !hasSiliconFlow && !hasFeatherless && !hasXAI && !hasZAI && !hasOpenAI {
		return nil
	}
	existing := make([]RankFeedSuggestionsExistingSource, 0, len(sources))
	for _, src := range sources {
		existing = append(existing, RankFeedSuggestionsExistingSource{
			URL:   src.URL,
			Title: src.Title,
		})
	}
	cands := make([]RankFeedSuggestionsCandidate, 0, len(suggestions))
	byID := map[string]*SourceSuggestionResponse{}
	for i := range suggestions {
		sug := suggestions[i]
		id := fmt.Sprintf("c%03d", i+1)
		cands = append(cands, RankFeedSuggestionsCandidate{
			ID:            id,
			URL:           sug.URL,
			Title:         sug.Title,
			Reasons:       sug.Reasons,
			MatchedTopics: sug.MatchedTopics,
		})
		byID[id] = &suggestions[i]
	}
	rankBudget := capDuration(20*time.Second, remainingSuggestionBudget())
	if rankBudget <= 0 {
		return map[string]any{
			"warning": "source suggestion rank skipped due timeout budget",
			"stage":   "rank",
		}
	}
	rankCtx, cancel := context.WithTimeout(ctx, rankBudget)
	resp, err := s.worker.RankFeedSuggestionsWithModel(
		rankCtx,
		existing,
		preferredTopics,
		cands,
		positiveExamples,
		negativeExamples,
		anthropicAPIKey,
		googleAPIKey,
		groqAPIKey,
		deepseekAPIKey,
		alibabaAPIKey,
		mistralAPIKey,
		togetherAPIKey,
		moonshotAPIKey,
		minimaxAPIKey,
		openRouterAPIKey,
		poeAPIKey,
		siliconFlowAPIKey,
		featherlessAPIKey,
		xaiAPIKey,
		zaiAPIKey,
		fireworksAPIKey,
		openAIAPIKey,
		model,
	)
	cancel()
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return map[string]any{
				"warning": "source suggestion ranking timed out",
				"stage":   "rank",
			}
		}
		return map[string]any{
			"error": err.Error(),
			"stage": "rank",
		}
	}
	if resp == nil {
		return map[string]any{
			"error": "empty response from rank-feed-suggestions",
			"stage": "rank",
		}
	}
	resp.LLM = NormalizeCatalogPricedUsage("source_suggestion", resp.LLM)
	s.recordSourceSuggestionLLMUsage(ctx, userID, resp.LLM)
	if len(resp.Items) == 0 {
		if resp.LLM != nil {
			return map[string]any{
				"provider":             resp.LLM.Provider,
				"model":                resp.LLM.Model,
				"requested_model":      resp.LLM.RequestedModel,
				"resolved_model":       resp.LLM.ResolvedModel,
				"estimated_cost_usd":   resp.LLM.EstimatedCostUSD,
				"input_tokens":         resp.LLM.InputTokens,
				"output_tokens":        resp.LLM.OutputTokens,
				"pricing_source":       resp.LLM.PricingSource,
				"pricing_model_family": resp.LLM.PricingModelFamily,
				"warning":              "rank returned no items",
				"stage":                "rank",
			}
		}
		return map[string]any{
			"warning": "rank returned no items and no llm meta",
			"stage":   "rank",
		}
	}
	byURL := map[string]*SourceSuggestionResponse{}
	for i := range suggestions {
		byURL[normalizeFeedURL(suggestions[i].URL)] = &suggestions[i]
	}
	rank := make(map[string]int, len(resp.Items))
	for i, it := range resp.Items {
		if it.ID != nil {
			if sug, ok := byID[strings.TrimSpace(*it.ID)]; ok {
				reason := strings.TrimSpace(it.Reason)
				if reason != "" {
					sug.AIReason = &reason
					sug.Reasons = []string{reason}
				}
				conf := it.Confidence
				sug.AIConfidence = &conf
				rank[normalizeFeedURL(sug.URL)] = i
				continue
			}
		}
		k := normalizeFeedURL(it.URL)
		if k == "" {
			continue
		}
		rank[k] = i
		if sug, ok := byURL[k]; ok {
			reason := strings.TrimSpace(it.Reason)
			if reason != "" {
				sug.AIReason = &reason
				sug.Reasons = []string{reason}
			}
			conf := it.Confidence
			sug.AIConfidence = &conf
		}
	}
	sort.SliceStable(suggestions, func(i, j int) bool {
		ki := normalizeFeedURL(suggestions[i].URL)
		kj := normalizeFeedURL(suggestions[j].URL)
		ri, iok := rank[ki]
		rj, jok := rank[kj]
		if iok && jok && ri != rj {
			return ri < rj
		}
		if iok != jok {
			return iok
		}
		return suggestions[i].URL < suggestions[j].URL
	})
	if resp.LLM == nil {
		return map[string]any{
			"warning": "rank succeeded but llm meta is empty",
			"stage":   "rank",
		}
	}
	return map[string]any{
		"provider":             resp.LLM.Provider,
		"model":                resp.LLM.Model,
		"requested_model":      resp.LLM.RequestedModel,
		"resolved_model":       resp.LLM.ResolvedModel,
		"estimated_cost_usd":   resp.LLM.EstimatedCostUSD,
		"input_tokens":         resp.LLM.InputTokens,
		"output_tokens":        resp.LLM.OutputTokens,
		"pricing_source":       resp.LLM.PricingSource,
		"pricing_model_family": resp.LLM.PricingModelFamily,
		"stage":                "rank",
		"items_count":          len(resp.Items),
	}
}

func (s *SourceSuggestionService) getUserSourceSuggestionModel(ctx context.Context, userID string) *string {
	if s.settingsRepo == nil {
		return nil
	}
	settings, err := s.settingsRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil
	}
	if settings.SourceSuggestionModel == nil || strings.TrimSpace(*settings.SourceSuggestionModel) == "" {
		return nil
	}
	v := strings.TrimSpace(*settings.SourceSuggestionModel)
	return &v
}

type resolvedProviderKeys struct {
	AnthropicAPIKey           *string
	GoogleAPIKey              *string
	GroqAPIKey                *string
	FireworksAPIKey           *string
	DeepseekAPIKey            *string
	AlibabaAPIKey             *string
	MistralAPIKey             *string
	TogetherAPIKey            *string
	MoonshotAPIKey            *string
	MiniMaxAPIKey             *string
	XiaomiMiMoTokenPlanAPIKey *string
	XAIAPIKey                 *string
	ZAIAPIKey                 *string
	OpenRouterAPIKey          *string
	PoeAPIKey                 *string
	SiliconFlowAPIKey         *string
	FeatherlessAPIKey         *string
	OpenAIAPIKey              *string
	SelectedModel             *string
}

func selectSourceSuggestionLLM(anthropicAPIKey, googleAPIKey, groqAPIKey, fireworksAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, togetherAPIKey, moonshotAPIKey, miniMaxAPIKey, xiaomiMiMoTokenPlanAPIKey, xaiAPIKey, zaiAPIKey, openRouterAPIKey, poeAPIKey, siliconFlowAPIKey, featherlessAPIKey, openAIAPIKey, model *string) resolvedProviderKeys {
	hasAnthropic := anthropicAPIKey != nil && strings.TrimSpace(*anthropicAPIKey) != ""
	hasGoogle := googleAPIKey != nil && strings.TrimSpace(*googleAPIKey) != ""
	hasGroq := groqAPIKey != nil && strings.TrimSpace(*groqAPIKey) != ""
	hasFireworks := fireworksAPIKey != nil && strings.TrimSpace(*fireworksAPIKey) != ""
	hasDeepSeek := deepseekAPIKey != nil && strings.TrimSpace(*deepseekAPIKey) != ""
	hasAlibaba := alibabaAPIKey != nil && strings.TrimSpace(*alibabaAPIKey) != ""
	hasMistral := mistralAPIKey != nil && strings.TrimSpace(*mistralAPIKey) != ""
	hasTogether := togetherAPIKey != nil && strings.TrimSpace(*togetherAPIKey) != ""
	hasMoonshot := moonshotAPIKey != nil && strings.TrimSpace(*moonshotAPIKey) != ""
	hasMiniMax := miniMaxAPIKey != nil && strings.TrimSpace(*miniMaxAPIKey) != ""
	hasXiaomiMiMoTokenPlan := xiaomiMiMoTokenPlanAPIKey != nil && strings.TrimSpace(*xiaomiMiMoTokenPlanAPIKey) != ""
	hasXAI := xaiAPIKey != nil && strings.TrimSpace(*xaiAPIKey) != ""
	hasZAI := zaiAPIKey != nil && strings.TrimSpace(*zaiAPIKey) != ""
	hasOpenRouter := openRouterAPIKey != nil && strings.TrimSpace(*openRouterAPIKey) != ""
	hasPoe := poeAPIKey != nil && strings.TrimSpace(*poeAPIKey) != ""
	hasSiliconFlow := siliconFlowAPIKey != nil && strings.TrimSpace(*siliconFlowAPIKey) != ""
	hasFeatherless := featherlessAPIKey != nil && strings.TrimSpace(*featherlessAPIKey) != ""
	hasOpenAI := openAIAPIKey != nil && strings.TrimSpace(*openAIAPIKey) != ""
	purpose := "source_suggestion"

	selectByProvider := func(provider string, explicitModel *string) resolvedProviderKeys {
		resolved := explicitModel
		if resolved == nil || strings.TrimSpace(*resolved) == "" {
			v := DefaultLLMModelForPurpose(provider, purpose)
			resolved = &v
		}
		switch provider {
		case "google":
			if hasGoogle {
				return resolvedProviderKeys{GoogleAPIKey: googleAPIKey, SelectedModel: resolved}
			}
		case "groq":
			if hasGroq {
				return resolvedProviderKeys{GroqAPIKey: groqAPIKey, SelectedModel: resolved}
			}
		case "fireworks":
			if hasFireworks {
				return resolvedProviderKeys{FireworksAPIKey: fireworksAPIKey, SelectedModel: resolved}
			}
		case "deepseek":
			if hasDeepSeek {
				return resolvedProviderKeys{DeepseekAPIKey: deepseekAPIKey, SelectedModel: resolved}
			}
		case "alibaba":
			if hasAlibaba {
				return resolvedProviderKeys{AlibabaAPIKey: alibabaAPIKey, SelectedModel: resolved}
			}
		case "mistral":
			if hasMistral {
				return resolvedProviderKeys{MistralAPIKey: mistralAPIKey, SelectedModel: resolved}
			}
		case "together":
			if hasTogether {
				return resolvedProviderKeys{TogetherAPIKey: togetherAPIKey, SelectedModel: resolved}
			}
		case "moonshot":
			if hasMoonshot {
				return resolvedProviderKeys{MoonshotAPIKey: moonshotAPIKey, SelectedModel: resolved}
			}
		case "minimax":
			if hasMiniMax {
				return resolvedProviderKeys{MiniMaxAPIKey: miniMaxAPIKey, SelectedModel: resolved}
			}
		case "xiaomi_mimo_token_plan":
			if hasXiaomiMiMoTokenPlan {
				return resolvedProviderKeys{
					XiaomiMiMoTokenPlanAPIKey: xiaomiMiMoTokenPlanAPIKey,
					OpenAIAPIKey:              xiaomiMiMoTokenPlanAPIKey,
					SelectedModel:             resolved,
				}
			}
		case "xai":
			if hasXAI {
				return resolvedProviderKeys{XAIAPIKey: xaiAPIKey, SelectedModel: resolved}
			}
		case "zai":
			if hasZAI {
				return resolvedProviderKeys{ZAIAPIKey: zaiAPIKey, SelectedModel: resolved}
			}
		case "openrouter":
			if hasOpenRouter {
				return resolvedProviderKeys{OpenRouterAPIKey: openRouterAPIKey, SelectedModel: resolved}
			}
		case "poe":
			if hasPoe {
				return resolvedProviderKeys{PoeAPIKey: poeAPIKey, SelectedModel: resolved}
			}
		case "siliconflow":
			if hasSiliconFlow {
				return resolvedProviderKeys{SiliconFlowAPIKey: siliconFlowAPIKey, SelectedModel: resolved}
			}
		case "featherless":
			if hasFeatherless {
				return resolvedProviderKeys{
					FeatherlessAPIKey: featherlessAPIKey,
					OpenAIAPIKey:      featherlessAPIKey,
					SelectedModel:     resolved,
				}
			}
		case "openai":
			if hasOpenAI {
				return resolvedProviderKeys{OpenAIAPIKey: openAIAPIKey, SelectedModel: resolved}
			}
		case "anthropic":
			if hasAnthropic {
				return resolvedProviderKeys{AnthropicAPIKey: anthropicAPIKey, SelectedModel: resolved}
			}
		}
		return resolvedProviderKeys{}
	}

	if model != nil && strings.TrimSpace(*model) != "" {
		preferredProvider := LLMProviderForModel(model)
		if out := selectByProvider(preferredProvider, model); out.SelectedModel != nil {
			return out
		}
		for _, provider := range CostEfficientLLMProviders(preferredProvider) {
			if out := selectByProvider(provider, nil); out.SelectedModel != nil {
				return out
			}
		}
		return resolvedProviderKeys{SelectedModel: model}
	}

	for _, provider := range CostEfficientLLMProviders("") {
		if out := selectByProvider(provider, nil); out.SelectedModel != nil {
			return out
		}
	}
	return resolvedProviderKeys{}
}

func (s *SourceSuggestionService) buildSourceSuggestionFewShotExamples(
	ctx context.Context,
	userID string,
) ([]RankFeedSuggestionsExample, []RankFeedSuggestionsExample) {
	positiveRows, err := s.repo.RecommendedByUser(ctx, userID, 5)
	if err != nil {
		positiveRows = nil
	}
	negativeRows, err := s.repo.LowAffinityByUser(ctx, userID, 3)
	if err != nil {
		negativeRows = nil
	}
	positive := make([]RankFeedSuggestionsExample, 0, len(positiveRows))
	for _, row := range positiveRows {
		reason := fmt.Sprintf("読了%d / Fav%d / 直近親和%.2f", row.ReadCount30d, row.FavoriteCount30d, row.AffinityScore)
		positive = append(positive, RankFeedSuggestionsExample{
			URL:    row.URL,
			Title:  row.Title,
			Reason: reason,
		})
	}
	negative := make([]RankFeedSuggestionsExample, 0, len(negativeRows))
	for _, row := range negativeRows {
		reason := fmt.Sprintf("読了%d / Fav%d / 直近親和%.2f", row.ReadCount30d, row.FavoriteCount30d, row.AffinityScore)
		negative = append(negative, RankFeedSuggestionsExample{
			URL:    row.URL,
			Title:  row.Title,
			Reason: reason,
		})
	}
	return positive, negative
}

func (s *SourceSuggestionService) expandSourceSuggestionsWithLLMSeeds(
	ctx context.Context,
	userID string,
	sources []model.Source,
	preferredTopics []string,
	positiveExamples []RankFeedSuggestionsExample,
	negativeExamples []RankFeedSuggestionsExample,
	registered map[string]bool,
	cands map[string]*sourceSuggestionAgg,
	anthropicAPIKey *string,
	googleAPIKey *string,
	groqAPIKey *string,
	fireworksAPIKey *string,
	deepseekAPIKey *string,
	alibabaAPIKey *string,
	mistralAPIKey *string,
	togetherAPIKey *string,
	moonshotAPIKey *string,
	minimaxAPIKey *string,
	openRouterAPIKey *string,
	poeAPIKey *string,
	siliconFlowAPIKey *string,
	featherlessAPIKey *string,
	xaiAPIKey *string,
	zaiAPIKey *string,
	openAIAPIKey *string,
	model *string,
	remainingSuggestionBudget func() time.Duration,
) (map[string]any, bool) {
	if remainingSuggestionBudget == nil {
		remainingSuggestionBudget = func() time.Duration { return 0 }
	}
	if remainingSuggestionBudget() <= 0 {
		return map[string]any{
			"warning": "source suggestion seed skipped due timeout budget",
			"stage":   "seed_generation",
		}, true
	}
	existing := make([]RankFeedSuggestionsExistingSource, 0, len(sources))
	for _, src := range sources {
		existing = append(existing, RankFeedSuggestionsExistingSource{URL: src.URL, Title: src.Title})
	}
	seedBudget := capDuration(25*time.Second, remainingSuggestionBudget())
	if seedBudget <= 0 {
		return map[string]any{
			"warning": "source suggestion seed skipped due timeout budget",
			"stage":   "seed_generation",
		}, true
	}
	ctxSeed, cancelSeed := context.WithTimeout(ctx, seedBudget)
	defer cancelSeed()
	resp, err := s.worker.SuggestFeedSeedSitesWithModel(
		ctxSeed,
		existing,
		preferredTopics,
		positiveExamples,
		negativeExamples,
		anthropicAPIKey,
		googleAPIKey,
		groqAPIKey,
		deepseekAPIKey,
		alibabaAPIKey,
		mistralAPIKey,
		togetherAPIKey,
		moonshotAPIKey,
		minimaxAPIKey,
		openRouterAPIKey,
		poeAPIKey,
		siliconFlowAPIKey,
		featherlessAPIKey,
		xaiAPIKey,
		zaiAPIKey,
		fireworksAPIKey,
		openAIAPIKey,
		model,
	)
	if err != nil || resp == nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return map[string]any{
				"warning": "source suggestion seed generation timed out",
				"stage":   "seed_generation",
			}, true
		}
		if err != nil {
			return map[string]any{
				"error": err.Error(),
				"stage": "seed_generation",
			}, false
		}
		return map[string]any{
			"error": "empty response from suggest-feed-seed-sites",
			"stage": "seed_generation",
		}, false
	}
	if remainingSuggestionBudget() <= 0 {
		return map[string]any{
			"warning": "source suggestion seed generation timed out",
			"stage":   "seed_generation",
		}, true
	}
	resp.LLM = NormalizeCatalogPricedUsage("source_suggestion", resp.LLM)
	s.recordSourceSuggestionLLMUsage(ctx, userID, resp.LLM)
	meta := llmUsageMetaMap(resp.LLM, "seed_generation")
	if meta == nil {
		meta = map[string]any{"stage": "seed_generation"}
	}
	meta["items_count"] = len(resp.Items)
	seedItems := resp.Items
	if len(seedItems) > 10 {
		seedItems = seedItems[:10]
	}
	beforeCount := len(cands)
	for _, seed := range seedItems {
		seedURL := coerceHTTPURL(strings.TrimSpace(seed.URL))
		if seedURL == "" {
			continue
		}
		addedFromSeed := false
		probeURLs := aiSeedFeedProbeURLs(seedURL)
		if len(probeURLs) == 0 {
			probeURLs = []string{seedURL}
		}
		if len(probeURLs) > 4 {
			probeURLs = probeURLs[:4]
		}
		for _, probe := range probeURLs {
			if remainingSuggestionBudget() <= 0 {
				break
			}
			probeTimeout := capDuration(1500*time.Millisecond, remainingSuggestionBudget())
			if probeTimeout <= 0 {
				break
			}
			ctxOne, cancel := context.WithTimeout(ctx, probeTimeout)
			feeds, err := DiscoverRSSFeeds(ctxOne, probe)
			cancel()
			if err != nil {
				continue
			}
			for _, f := range feeds {
				key := normalizeFeedURL(f.URL)
				if key == "" || registered[key] {
					continue
				}
				a := cands[key]
				if a == nil {
					a = &sourceSuggestionAgg{
						URL:           f.URL,
						Title:         f.Title,
						Reasons:       map[string]bool{},
						MatchedTopics: map[string]bool{},
						SeedSourceIDs: map[string]bool{},
					}
					cands[key] = a
				}
				if a.Title == nil && f.Title != nil {
					a.Title = f.Title
				}
				reason := "AI提案サイトから発見"
				if strings.TrimSpace(seed.Reason) != "" {
					reason = "AI候補: " + strings.TrimSpace(seed.Reason)
				}
				if !a.Reasons[reason] {
					a.Reasons[reason] = true
					a.Score += 6
				}
				for _, topic := range preferredTopics {
					if sourceSuggestionTopicMatch(f, topic) && !a.MatchedTopics[topic] {
						a.MatchedTopics[topic] = true
						a.Score += 3
					}
				}
				addedFromSeed = true
			}
		}
		if remainingSuggestionBudget() <= 0 && !addedFromSeed {
			meta = mergeLLMWarning(meta, "source suggestion timed out during AI seed probing", "seed_generation")
		}
		if !addedFromSeed {
			key := normalizeFeedURL(seedURL)
			if key == "" || registered[key] {
				continue
			}
			a := cands[key]
			if a == nil {
				a = &sourceSuggestionAgg{
					URL:           seedURL,
					Title:         seed.Title,
					Reasons:       map[string]bool{},
					MatchedTopics: map[string]bool{},
					SeedSourceIDs: map[string]bool{},
				}
				cands[key] = a
			}
			if a.Title == nil && seed.Title != nil && strings.TrimSpace(*seed.Title) != "" {
				a.Title = seed.Title
			}
			reason := "AI提案サイト（登録時にFeed検出）"
			if strings.TrimSpace(seed.Reason) != "" {
				reason = "AI候補: " + strings.TrimSpace(seed.Reason)
			}
			if !a.Reasons[reason] {
				a.Reasons[reason] = true
				a.Score += 4
			}
		}
	}
	if len(seedItems) == 0 {
		meta = mergeLLMWarning(meta, "seed returned no items", "seed_generation")
	} else if len(cands) == beforeCount {
		meta = mergeLLMWarning(meta, "seed produced no usable candidates", "seed_generation")
	}
	return meta, false
}

func (s *SourceSuggestionService) recordSourceSuggestionLLMUsage(ctx context.Context, userID string, llm *LLMUsage) {
	llm = NormalizeCatalogPricedUsage("source_suggestion", llm)
	if s.llmUsageRepo == nil || llm == nil {
		return
	}
	if llm.Provider == "" || llm.Model == "" {
		return
	}
	uid := userID
	if err := s.llmUsageRepo.Insert(ctx, repository.LLMUsageLogInput{
		UserID:                   &uid,
		Provider:                 llm.Provider,
		Model:                    llm.Model,
		RequestedModel:           llm.RequestedModel,
		ResolvedModel:            llm.ResolvedModel,
		PricingModelFamily:       llm.PricingModelFamily,
		PricingSource:            llm.PricingSource,
		OpenRouterCostUSD:        llm.OpenRouterCostUSD,
		OpenRouterGenerationID:   strings.TrimSpace(llm.OpenRouterGenerationID),
		Purpose:                  "source_suggestion",
		InputTokens:              llm.InputTokens,
		OutputTokens:             llm.OutputTokens,
		CacheCreationInputTokens: llm.CacheCreationInputTokens,
		CacheReadInputTokens:     llm.CacheReadInputTokens,
		EstimatedCostUSD:         llm.EstimatedCostUSD,
	}); err != nil {
		return
	}
	_ = BumpUserLLMUsageCacheVersion(ctx, s.cache, userID)
}

func populateSourceSuggestionsFromProbes(
	ctx context.Context,
	probes []probeSeed,
	preferredTopics []string,
	registered map[string]bool,
	cands map[string]*sourceSuggestionAgg,
	remainingSuggestionBudget func() time.Duration,
	discover func(context.Context, string) ([]FeedCandidate, error),
) {
	if remainingSuggestionBudget == nil {
		remainingSuggestionBudget = func() time.Duration { return 0 }
	}
	if discover == nil {
		return
	}
	for _, p := range probes {
		probeTimeout := capDuration(1200*time.Millisecond, remainingSuggestionBudget())
		if probeTimeout <= 0 {
			break
		}
		probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
		feeds, err := discover(probeCtx, p.ProbeURL)
		cancel()
		if err != nil {
			continue
		}
		for _, f := range feeds {
			key := normalizeFeedURL(f.URL)
			if key == "" || registered[key] {
				continue
			}
			a := cands[key]
			if a == nil {
				a = &sourceSuggestionAgg{
					URL:           f.URL,
					Title:         f.Title,
					Reasons:       map[string]bool{},
					MatchedTopics: map[string]bool{},
					SeedSourceIDs: map[string]bool{},
				}
				cands[key] = a
			}
			if a.Title == nil && f.Title != nil {
				a.Title = f.Title
			}
			if !a.Reasons[p.Reason] {
				a.Reasons[p.Reason] = true
				a.Score++
			}
			if !a.SeedSourceIDs[p.SourceID] {
				a.SeedSourceIDs[p.SourceID] = true
				a.Score += 2
			}
			for _, topic := range preferredTopics {
				if topic == "" {
					continue
				}
				if sourceSuggestionTopicMatch(f, topic) && !a.MatchedTopics[topic] {
					a.MatchedTopics[topic] = true
					a.Score += 3
				}
			}
		}
	}
}

func suggestionProbeURLs(raw string) []suggestionProbe {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil
	}
	seen := map[string]bool{}
	var out []suggestionProbe
	add := func(v, reason string) {
		if v == "" || seen[v] {
			return
		}
		seen[v] = true
		out = append(out, suggestionProbe{url: v, reason: reason})
	}
	root := &url.URL{Scheme: u.Scheme, Host: u.Host, Path: "/"}
	add(root.String(), "同一サイトのトップページから発見")

	cleanPath := path.Clean(u.Path)
	if cleanPath != "." && cleanPath != "/" {
		parent := path.Dir(cleanPath)
		if parent != "." && parent != "/" {
			parentURL := &url.URL{Scheme: u.Scheme, Host: u.Host, Path: parent + "/"}
			add(parentURL.String(), "登録ソースの親URLから発見")
		}
	}
	return out
}

func normalizeFeedURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return ""
	}
	u.Fragment = ""
	if (u.Scheme == "http" && strings.HasSuffix(u.Host, ":80")) || (u.Scheme == "https" && strings.HasSuffix(u.Host, ":443")) {
		u.Host = strings.Split(u.Host, ":")[0]
	}
	u.Host = strings.ToLower(u.Host)
	return u.String()
}

func mapKeys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sourceSuggestionTopicMatch(f FeedCandidate, topic string) bool {
	t := strings.ToLower(strings.TrimSpace(topic))
	if t == "" {
		return false
	}
	if f.Title != nil && strings.Contains(strings.ToLower(*f.Title), t) {
		return true
	}
	return strings.Contains(strings.ToLower(f.URL), t)
}

func aiSeedFeedProbeURLs(raw string) []string {
	u, err := url.Parse(coerceHTTPURL(strings.TrimSpace(raw)))
	if err != nil || u.Host == "" {
		return nil
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	base := &url.URL{Scheme: u.Scheme, Host: u.Host}
	seen := map[string]bool{}
	var out []string
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			return
		}
		seen[v] = true
		out = append(out, v)
	}
	add(u.String())
	for _, p := range []string{"/feed", "/rss", "/atom.xml", "/feed.xml", "/rss.xml", "/index.xml"} {
		c := *base
		c.Path = p
		add(c.String())
	}
	return out
}

func coerceHTTPURL(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return ""
	}
	u, err := url.Parse(v)
	if err == nil && u.Host != "" {
		if u.Scheme == "" {
			u.Scheme = "https"
		}
		return u.String()
	}
	if !strings.Contains(v, "://") && strings.Contains(v, ".") && !strings.ContainsAny(v, " \t\r\n") {
		return "https://" + v
	}
	return ""
}

func capDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func mergeLLMWarning(llmMeta map[string]any, warning string, stage string) map[string]any {
	if llmMeta == nil {
		llmMeta = map[string]any{}
	}
	existing, ok := llmMeta["warning"].(string)
	if ok && existing != "" {
		if !strings.Contains(existing, warning) {
			llmMeta["warning"] = fmt.Sprintf("%s; %s", existing, warning)
		}
	} else {
		llmMeta["warning"] = warning
	}
	if stage != "" {
		llmMeta["stage"] = stage
	}
	return llmMeta
}

func llmUsageMetaMap(llm *LLMUsage, stage string) map[string]any {
	llm = NormalizeCatalogPricedUsage("source_suggestion", llm)
	if llm == nil {
		if stage == "" {
			return nil
		}
		return map[string]any{"stage": stage}
	}
	meta := map[string]any{
		"provider":             llm.Provider,
		"model":                llm.Model,
		"requested_model":      llm.RequestedModel,
		"resolved_model":       llm.ResolvedModel,
		"estimated_cost_usd":   llm.EstimatedCostUSD,
		"input_tokens":         llm.InputTokens,
		"output_tokens":        llm.OutputTokens,
		"pricing_source":       llm.PricingSource,
		"pricing_model_family": llm.PricingModelFamily,
	}
	if stage != "" {
		meta["stage"] = stage
	}
	return meta
}

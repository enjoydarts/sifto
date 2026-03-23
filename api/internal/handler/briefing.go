package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
)

var briefingSnapshotMaxAge = loadBriefingSnapshotMaxAge()

const briefingNavigatorCacheTTL = 30 * time.Minute

type BriefingHandler struct {
	itemRepo     *repository.ItemRepo
	snapshotRepo *repository.BriefingSnapshotRepo
	streakRepo   *repository.ReadingStreakRepo
	settingsRepo *repository.UserSettingsRepo
	llmUsageRepo *repository.LLMUsageLogRepo
	cipher       *service.SecretCipher
	worker       *service.WorkerClient
	cache        service.JSONCache
}

type briefingNavigatorIntroContext = service.BriefingNavigatorIntroContext

func NewBriefingHandler(
	itemRepo *repository.ItemRepo,
	snapshotRepo *repository.BriefingSnapshotRepo,
	streakRepo *repository.ReadingStreakRepo,
	settingsRepo *repository.UserSettingsRepo,
	llmUsageRepo *repository.LLMUsageLogRepo,
	cipher *service.SecretCipher,
	worker *service.WorkerClient,
	cache service.JSONCache,
) *BriefingHandler {
	return &BriefingHandler{
		itemRepo:     itemRepo,
		snapshotRepo: snapshotRepo,
		streakRepo:   streakRepo,
		settingsRepo: settingsRepo,
		llmUsageRepo: llmUsageRepo,
		cipher:       cipher,
		worker:       worker,
		cache:        cache,
	}
}

func (h *BriefingHandler) Today(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	size := parseIntOrDefault(r.URL.Query().Get("size"), 12)
	if size < 1 {
		size = 12
	}
	if size > 30 {
		size = 30
	}
	now := timeutil.NowJST()
	cacheKey := cacheKeyBriefingToday(userID, size)
	cacheBust := r.URL.Query().Get("cache_bust") == "1"
	if h.cache != nil && !cacheBust {
		var cached model.BriefingTodayResponse
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			cached.Navigator = nil
			incrCacheMetric(r.Context(), h.cache, userID, "briefing.hit")
			writeJSON(w, cached)
			return
		} else if err != nil {
			incrCacheMetric(r.Context(), h.cache, userID, "briefing.error")
			log.Printf("briefing cache get failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
		incrCacheMetric(r.Context(), h.cache, userID, "briefing.miss")
	} else if cacheBust && h.cache != nil {
		incrCacheMetric(r.Context(), h.cache, userID, "briefing.bypass")
	}
	today := timeutil.StartOfDayJST(now)
	dateStr := today.Format("2006-01-02")
	var fallbackSnapshot *model.BriefingTodayResponse

	if h.snapshotRepo != nil {
		s, err := h.snapshotRepo.GetByUserAndDate(r.Context(), userID, dateStr)
		if err == nil {
			var payload model.BriefingTodayResponse
			if len(s.PayloadJSON) > 0 && json.Unmarshal(s.PayloadJSON, &payload) == nil {
				if payload.Date == "" {
					payload.Date = dateStr
				}
				if payload.Greeting == "" {
					payload.Greeting = service.GreetingByHour(timeutil.NowJST())
				}
				payload.Status = s.Status
				payload.GeneratedAt = s.GeneratedAt
				payload.Navigator = nil
				if !cacheBust && isSnapshotFresh(s.GeneratedAt, now) {
					writeJSON(w, payload)
					return
				}
				payload.Status = "stale"
				fallbackSnapshot = &payload
			}
		}
	}

	payload, err := service.BuildBriefingToday(r.Context(), h.itemRepo, h.streakRepo, userID, today, size)
	if err != nil {
		if fallbackSnapshot != nil {
			if h.cache != nil {
				if cacheErr := h.cache.SetJSON(r.Context(), cacheKey, fallbackSnapshot, 15*time.Second); cacheErr != nil {
					log.Printf("briefing cache set stale failed user_id=%s key=%s err=%v", userID, cacheKey, cacheErr)
				}
			}
			writeJSON(w, fallbackSnapshot)
			return
		}
		writeRepoError(w, err)
		return
	}
	payload.Status = "ready"
	generatedAt := now
	payload.GeneratedAt = &generatedAt
	payload.Navigator = nil
	if h.snapshotRepo != nil {
		if err := h.snapshotRepo.Upsert(r.Context(), userID, dateStr, "ready", payload); err != nil {
			log.Printf("briefing snapshot upsert user=%s date=%s: %v", userID, dateStr, err)
		}
	}
	if h.cache != nil {
		if err := h.cache.SetJSON(r.Context(), cacheKey, payload, 20*time.Second); err != nil {
			incrCacheMetric(r.Context(), h.cache, userID, "briefing.error")
			log.Printf("briefing cache set failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
	}
	writeJSON(w, payload)
}

func (h *BriefingHandler) Navigator(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	cacheBust := r.URL.Query().Get("cache_bust") == "1"
	preview := r.URL.Query().Get("navigator_preview") == "1"

	if h.settingsRepo == nil {
		writeJSON(w, model.BriefingNavigatorEnvelope{})
		return
	}
	settings, err := h.settingsRepo.EnsureDefaults(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if settings == nil || !settings.NavigatorEnabled {
		writeJSON(w, model.BriefingNavigatorEnvelope{})
		return
	}
	persona := normalizeBriefingNavigatorPersona(settings.NavigatorPersona)
	modelName := resolveBriefingNavigatorModel(settings)
	resolvedModel := ""
	if modelName != nil {
		resolvedModel = strings.TrimSpace(*modelName)
	}
	cacheKey := cacheKeyBriefingNavigator(userID, persona, resolvedModel, preview)
	if h.cache != nil && !cacheBust {
		var cached model.BriefingNavigatorEnvelope
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			writeJSON(w, cached)
			return
		}
	}

	generatedAt := timeutil.NowJST()
	var navigator *model.BriefingNavigator
	if preview {
		navigator = h.buildNavigatorPreview(r.Context(), userID, generatedAt, nil)
	} else {
		navigator = h.buildNavigator(r.Context(), userID, generatedAt)
	}
	resp := model.BriefingNavigatorEnvelope{Navigator: navigator}
	if h.cache != nil {
		if err := h.cache.SetJSON(r.Context(), cacheKey, resp, briefingNavigatorCacheTTL); err != nil {
			log.Printf("briefing navigator cache set failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
	}
	writeJSON(w, resp)
}

func isSnapshotFresh(generatedAt *time.Time, now time.Time) bool {
	if generatedAt == nil {
		return false
	}
	if now.Before(*generatedAt) {
		return true
	}
	return now.Sub(*generatedAt) <= briefingSnapshotMaxAge
}

func loadBriefingSnapshotMaxAge() time.Duration {
	const defaultMaxAge = 10 * time.Minute
	v := strings.TrimSpace(os.Getenv("BRIEFING_SNAPSHOT_MAX_AGE_SEC"))
	if v == "" {
		return defaultMaxAge
	}
	sec, err := strconv.Atoi(v)
	if err != nil || sec <= 0 {
		return defaultMaxAge
	}
	return time.Duration(sec) * time.Second
}

func (h *BriefingHandler) buildNavigator(ctx context.Context, userID string, generatedAt time.Time) *model.BriefingNavigator {
	if h.itemRepo == nil || h.settingsRepo == nil || h.worker == nil || h.cipher == nil {
		return nil
	}
	settings, err := h.settingsRepo.EnsureDefaults(ctx, userID)
	if err != nil {
		log.Printf("briefing navigator settings user=%s: %v", userID, err)
		return nil
	}
	if settings == nil || !settings.NavigatorEnabled {
		return nil
	}
	modelName := resolveBriefingNavigatorModel(settings)
	if modelName == nil {
		return nil
	}
	candidates, err := h.itemRepo.BriefingNavigatorCandidates24h(ctx, userID, 12)
	if err != nil {
		log.Printf("briefing navigator candidates user=%s: %v", userID, err)
		return nil
	}
	if len(candidates) == 0 {
		return nil
	}
	anthropicKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetAnthropicAPIKeyEncrypted, h.cipher, userID, "")
	googleKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetGoogleAPIKeyEncrypted, h.cipher, userID, "")
	groqKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetGroqAPIKeyEncrypted, h.cipher, userID, "")
	fireworksKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetFireworksAPIKeyEncrypted, h.cipher, userID, "")
	deepseekKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetDeepSeekAPIKeyEncrypted, h.cipher, userID, "")
	alibabaKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetAlibabaAPIKeyEncrypted, h.cipher, userID, "")
	mistralKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetMistralAPIKeyEncrypted, h.cipher, userID, "")
	xaiKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetXAIAPIKeyEncrypted, h.cipher, userID, "")
	zaiKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetZAIAPIKeyEncrypted, h.cipher, userID, "")
	openRouterKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetOpenRouterAPIKeyEncrypted, h.cipher, userID, "")
	poeKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetPoeAPIKeyEncrypted, h.cipher, userID, "")
	openAIKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetOpenAIAPIKeyEncrypted, h.cipher, userID, "")
	switch service.LLMProviderForModel(modelName) {
	case "openrouter":
		openAIKey = openRouterKey
	case "poe":
		openAIKey = poeKey
	}

	persona := normalizeBriefingNavigatorPersona(settings.NavigatorPersona)
	introContext := buildBriefingNavigatorIntroContext(generatedAt)
	workerCandidates := make([]service.BriefingNavigatorCandidate, 0, len(candidates))
	candidateByID := make(map[string]model.BriefingNavigatorCandidate, len(candidates))
	for _, candidate := range candidates {
		candidateByID[candidate.ItemID] = candidate
		var publishedAt *string
		if candidate.PublishedAt != nil {
			v := candidate.PublishedAt.Format(time.RFC3339)
			publishedAt = &v
		}
		workerCandidates = append(workerCandidates, service.BriefingNavigatorCandidate{
			ItemID:          candidate.ItemID,
			Title:           candidate.Title,
			TranslatedTitle: candidate.TranslatedTitle,
			SourceTitle:     candidate.SourceTitle,
			Summary:         candidate.Summary,
			Topics:          candidate.Topics,
			PublishedAt:     publishedAt,
			Score:           candidate.Score,
		})
	}

	workerCtx := service.WithWorkerTraceMetadata(ctx, "briefing_navigator", &userID, nil, nil, nil)
	resp, err := h.worker.GenerateBriefingNavigatorWithModel(
		workerCtx,
		persona,
		workerCandidates,
		introContext,
		anthropicKey,
		googleKey,
		groqKey,
		deepseekKey,
		alibabaKey,
		mistralKey,
		xaiKey,
		zaiKey,
		fireworksKey,
		openAIKey,
		modelName,
	)
	if err != nil {
		log.Printf("briefing navigator worker user=%s model=%s: %v", userID, strings.TrimSpace(*modelName), err)
		return nil
	}
	if resp.LLM == nil {
		log.Printf("briefing navigator llm missing user=%s model=%s", userID, strings.TrimSpace(*modelName))
	}
	recordAskLLMUsage(ctx, h.llmUsageRepo, h.cache, "briefing_navigator", resp.LLM, &userID)
	meta := briefingNavigatorPersonaMeta(persona)
	picks := make([]model.BriefingNavigatorPick, 0, len(resp.Picks))
	for idx, pick := range resp.Picks {
		if strings.TrimSpace(pick.ItemID) == "" || strings.TrimSpace(pick.Comment) == "" {
			continue
		}
		candidate, ok := candidateByID[strings.TrimSpace(pick.ItemID)]
		if !ok {
			continue
		}
		picks = append(picks, model.BriefingNavigatorPick{
			ItemID:      strings.TrimSpace(pick.ItemID),
			Rank:        idx + 1,
			Title:       briefingNavigatorCandidateTitle(candidate),
			SourceTitle: candidate.SourceTitle,
			Comment:     strings.TrimSpace(pick.Comment),
			ReasonTags:  pick.ReasonTags,
		})
	}
	if len(picks) == 0 {
		return nil
	}
	return &model.BriefingNavigator{
		Enabled:        true,
		Persona:        persona,
		CharacterName:  meta.CharacterName,
		CharacterTitle: meta.CharacterTitle,
		AvatarStyle:    meta.AvatarStyle,
		SpeechStyle:    meta.SpeechStyle,
		Intro:          strings.TrimSpace(resp.Intro),
		GeneratedAt:    &generatedAt,
		Picks:          picks,
	}
}

func buildBriefingNavigatorIntroContext(now time.Time) briefingNavigatorIntroContext {
	now = now.In(timeutil.JST)
	return briefingNavigatorIntroContext{
		NowJST:     now.Format(time.RFC3339),
		DateJST:    now.Format("2006-01-02"),
		WeekdayJST: now.Weekday().String(),
		TimeOfDay:  briefingNavigatorTimeOfDay(now.Hour()),
		SeasonHint: briefingNavigatorSeasonHint(now),
	}
}

func (h *BriefingHandler) buildNavigatorPreview(ctx context.Context, userID string, generatedAt time.Time, items []model.Item) *model.BriefingNavigator {
	if h.settingsRepo == nil {
		return nil
	}
	settings, err := h.settingsRepo.EnsureDefaults(ctx, userID)
	if err != nil || settings == nil || !settings.NavigatorEnabled {
		return nil
	}
	persona := normalizeBriefingNavigatorPersona(settings.NavigatorPersona)
	meta := briefingNavigatorPersonaMeta(persona)
	picks := make([]model.BriefingNavigatorPick, 0, 3)
	for _, item := range items {
		if item.ID == "" {
			continue
		}
		title := ""
		if item.TranslatedTitle != nil {
			title = strings.TrimSpace(*item.TranslatedTitle)
		}
		if title == "" && item.Title != nil {
			title = strings.TrimSpace(*item.Title)
		}
		if title == "" {
			continue
		}
		var sourceTitle *string
		if item.SourceTitle != nil && strings.TrimSpace(*item.SourceTitle) != "" {
			v := strings.TrimSpace(*item.SourceTitle)
			sourceTitle = &v
		}
		picks = append(picks, model.BriefingNavigatorPick{
			ItemID:      item.ID,
			Rank:        len(picks) + 1,
			Title:       title,
			SourceTitle: sourceTitle,
			Comment:     briefingNavigatorPreviewComment(persona, len(picks)+1),
		})
		if len(picks) >= 3 {
			break
		}
	}
	if len(picks) == 0 {
		picks = []model.BriefingNavigatorPick{
			{
				ItemID:  "preview-1",
				Rank:    1,
				Title:   "AIナビゲーターの見た目確認用プレビュー 1",
				Comment: briefingNavigatorPreviewComment(persona, 1),
			},
			{
				ItemID:  "preview-2",
				Rank:    2,
				Title:   "AIナビゲーターの見た目確認用プレビュー 2",
				Comment: briefingNavigatorPreviewComment(persona, 2),
			},
			{
				ItemID:  "preview-3",
				Rank:    3,
				Title:   "AIナビゲーターの見た目確認用プレビュー 3",
				Comment: briefingNavigatorPreviewComment(persona, 3),
			},
		}
	}
	if len(picks) == 0 {
		return nil
	}
	return &model.BriefingNavigator{
		Enabled:        true,
		Persona:        persona,
		CharacterName:  meta.CharacterName,
		CharacterTitle: meta.CharacterTitle,
		AvatarStyle:    meta.AvatarStyle,
		SpeechStyle:    meta.SpeechStyle,
		Intro:          briefingNavigatorPreviewIntro(persona),
		GeneratedAt:    &generatedAt,
		Picks:          picks,
	}
}

type briefingNavigatorPersonaPresentation struct {
	CharacterName  string
	CharacterTitle string
	AvatarStyle    string
	SpeechStyle    string
}

func briefingNavigatorPersonaMeta(persona string) briefingNavigatorPersonaPresentation {
	switch strings.TrimSpace(persona) {
	case "hype":
		return briefingNavigatorPersonaPresentation{
			CharacterName:  "ルカ",
			CharacterTitle: "Momentum Guide",
			AvatarStyle:    "hype",
			SpeechStyle:    "energetic",
		}
	case "analyst":
		return briefingNavigatorPersonaPresentation{
			CharacterName:  "藍",
			CharacterTitle: "Context Analyst",
			AvatarStyle:    "analyst",
			SpeechStyle:    "measured",
		}
	case "concierge":
		return briefingNavigatorPersonaPresentation{
			CharacterName:  "凪",
			CharacterTitle: "Briefing Concierge",
			AvatarStyle:    "concierge",
			SpeechStyle:    "gentle",
		}
	case "snark":
		return briefingNavigatorPersonaPresentation{
			CharacterName:  "ジン",
			CharacterTitle: "Snark Guide",
			AvatarStyle:    "snark",
			SpeechStyle:    "wry",
		}
	default:
		return briefingNavigatorPersonaPresentation{
			CharacterName:  "水城",
			CharacterTitle: "Editor Navigator",
			AvatarStyle:    "editor",
			SpeechStyle:    "editorial",
		}
	}
}

func resolveBriefingNavigatorModel(settings *model.UserSettings) *string {
	if settings == nil {
		return nil
	}
	if modelName := chooseNavigatorModelOverride(settings.NavigatorModel, settings); modelName != nil {
		return modelName
	}
	if modelName := chooseNavigatorModelOverride(settings.NavigatorFallbackModel, settings); modelName != nil {
		return modelName
	}
	for _, provider := range service.CostEfficientLLMProviders("") {
		if !hasNavigatorProviderKey(settings, provider) {
			continue
		}
		v := strings.TrimSpace(service.DefaultLLMModelForPurpose(provider, "summary"))
		if v == "" {
			continue
		}
		return &v
	}
	return nil
}

func chooseNavigatorModelOverride(modelName *string, settings *model.UserSettings) *string {
	if modelName == nil || settings == nil {
		return nil
	}
	v := strings.TrimSpace(*modelName)
	if v == "" {
		return nil
	}
	if !hasNavigatorProviderKey(settings, service.LLMProviderForModel(&v)) {
		return nil
	}
	return &v
}

func hasNavigatorProviderKey(settings *model.UserSettings, provider string) bool {
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
	default:
		return settings.HasAnthropicAPIKey
	}
}

func normalizeBriefingNavigatorPersona(v string) string {
	switch strings.TrimSpace(v) {
	case "editor", "hype", "analyst", "concierge", "snark":
		return strings.TrimSpace(v)
	default:
		return "editor"
	}
}

func briefingNavigatorCandidateTitle(candidate model.BriefingNavigatorCandidate) string {
	if candidate.TranslatedTitle != nil && strings.TrimSpace(*candidate.TranslatedTitle) != "" {
		return strings.TrimSpace(*candidate.TranslatedTitle)
	}
	if candidate.Title != nil && strings.TrimSpace(*candidate.Title) != "" {
		return strings.TrimSpace(*candidate.Title)
	}
	return candidate.ItemID
}

func briefingNavigatorTimeOfDay(hour int) string {
	switch {
	case hour >= 5 && hour < 11:
		return "morning"
	case hour >= 11 && hour < 17:
		return "afternoon"
	case hour >= 17 && hour < 23:
		return "evening"
	default:
		return "late_night"
	}
}

func briefingNavigatorSeasonHint(now time.Time) string {
	month := now.Month()
	day := now.Day()
	switch month {
	case time.March:
		if day < 15 {
			return "early_spring"
		}
		return "spring"
	case time.April, time.May:
		return "spring"
	case time.June:
		if day < 10 {
			return "early_summer"
		}
		return "rainy_season"
	case time.July:
		if day < 20 {
			return "rainy_season"
		}
		return "mid_summer"
	case time.August:
		if day < 20 {
			return "mid_summer"
		}
		return "late_summer"
	case time.September, time.October:
		return "autumn"
	case time.November:
		if day < 20 {
			return "late_autumn"
		}
		return "early_winter"
	case time.December:
		return "mid_winter"
	case time.January:
		return "mid_winter"
	case time.February:
		return "late_winter"
	default:
		return "seasonal"
	}
}

func briefingNavigatorPreviewIntro(persona string) string {
	switch persona {
	case "hype":
		return "プレビュー表示です。本番のコメント生成前ですが、勢いと見た目はこのトーンで出ます。"
	case "analyst":
		return "プレビュー表示です。本番では各記事の背景や含意を短く添えて案内します。"
	case "concierge":
		return "プレビュー表示です。実際の生成では、やわらかい案内文で未読記事を紹介します。"
	case "snark":
		return "プレビュー表示です。本番では軽口を混ぜつつ、読む価値のある未読記事を拾います。"
	default:
		return "プレビュー表示です。本番では編集者っぽいトーンで未読記事を3本前後おすすめします。"
	}
}

func briefingNavigatorPreviewComment(persona string, rank int) string {
	switch persona {
	case "hype":
		return "ここはプレビュー枠ですが、実運用ではテンポよく背中を押すコメントが入ります。"
	case "analyst":
		return "ここに記事の文脈や見るべきポイントを短く整理したコメントが入ります。"
	case "concierge":
		return "ここにやわらかく読みどころを添える案内コメントが入ります。"
	case "snark":
		if rank == 1 {
			return "ここ、本番なら『後回しにするとあとで追うのが面倒そう』くらいの軽口で勧めます。"
		}
		return "ここに少しだけ皮肉を混ぜた推薦コメントが入ります。"
	default:
		return "ここに編集者目線で読みどころを一言にまとめたコメントが入ります。"
	}
}

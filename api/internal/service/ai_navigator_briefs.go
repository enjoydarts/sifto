package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
)

const aiNavigatorBriefNotificationKind = "ai_navigator_brief"

type aiNavigatorBriefUserLookup interface {
	GetByID(ctx context.Context, userID string) (*model.User, error)
}

type AINavigatorBriefService struct {
	briefs   *repository.AINavigatorBriefRepo
	items    *repository.ItemRepo
	settings *repository.UserSettingsRepo
	users    aiNavigatorBriefUserLookup
	pushLogs *repository.PushNotificationLogRepo
	worker   *WorkerClient
	cipher   *SecretCipher
	sender   audioBriefingPublishedSender
	now      func() time.Time
	pageURL  func(path string) string
}

func NewAINavigatorBriefService(
	briefs *repository.AINavigatorBriefRepo,
	items *repository.ItemRepo,
	settings *repository.UserSettingsRepo,
	users aiNavigatorBriefUserLookup,
	pushLogs *repository.PushNotificationLogRepo,
	worker *WorkerClient,
	cipher *SecretCipher,
	sender audioBriefingPublishedSender,
	now func() time.Time,
) *AINavigatorBriefService {
	if now == nil {
		now = timeutil.NowJST
	}
	return &AINavigatorBriefService{
		briefs:   briefs,
		items:    items,
		settings: settings,
		users:    users,
		pushLogs: pushLogs,
		worker:   worker,
		cipher:   cipher,
		sender:   sender,
		now:      now,
		pageURL:  AudioBriefingPageURLFromEnv,
	}
}

type aiNavigatorBriefSlotConfig struct {
	slot        string
	triggerHour int
}

var aiNavigatorBriefSlotConfigs = []aiNavigatorBriefSlotConfig{
	{slot: model.AINavigatorBriefSlotMorning, triggerHour: 8},
	{slot: model.AINavigatorBriefSlotNoon, triggerHour: 12},
	{slot: model.AINavigatorBriefSlotEvening, triggerHour: 18},
}

func ResolveAINavigatorBriefSlotWindow(now time.Time, slot string) (time.Time, time.Time, error) {
	now = now.In(timeutil.JST)
	var start time.Time
	end := now
	switch strings.TrimSpace(slot) {
	case model.AINavigatorBriefSlotMorning:
		start = time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 0, 0, timeutil.JST).AddDate(0, 0, -1)
	case model.AINavigatorBriefSlotNoon:
		start = time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, timeutil.JST)
	case model.AINavigatorBriefSlotEvening:
		start = time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, timeutil.JST)
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("invalid slot")
	}
	return start, end, nil
}

func ResolveAINavigatorBriefSlot(now time.Time) (string, error) {
	now = now.In(timeutil.JST)
	hour := now.Hour()
	switch {
	case hour < 12:
		return model.AINavigatorBriefSlotMorning, nil
	case hour < 18:
		return model.AINavigatorBriefSlotNoon, nil
	default:
		return model.AINavigatorBriefSlotEvening, nil
	}
}

func (s *AINavigatorBriefService) ListBriefsByUser(ctx context.Context, userID, slot string, limit int) ([]model.AINavigatorBrief, error) {
	return s.briefs.ListBriefsByUser(ctx, userID, slot, limit)
}

func (s *AINavigatorBriefService) GetBriefDetail(ctx context.Context, userID, briefID string) (*model.AINavigatorBrief, error) {
	return s.briefs.GetBriefDetail(ctx, userID, briefID)
}

func (s *AINavigatorBriefService) GenerateManual(ctx context.Context, userID string) (*model.AINavigatorBrief, error) {
	slot, err := ResolveAINavigatorBriefSlot(s.now())
	if err != nil {
		return nil, err
	}
	return s.GenerateBriefForSlot(ctx, userID, slot)
}

func (s *AINavigatorBriefService) GenerateBriefForSlot(ctx context.Context, userID, slot string) (*model.AINavigatorBrief, error) {
	if s.briefs == nil || s.items == nil || s.settings == nil || s.worker == nil || s.cipher == nil {
		return nil, fmt.Errorf("ai navigator brief service unavailable")
	}
	settings, err := s.settings.EnsureDefaults(ctx, userID)
	if err != nil {
		return nil, err
	}
	if settings == nil || !settings.AINavigatorBriefEnabled || !settings.NavigatorEnabled {
		return nil, fmt.Errorf("ai navigator brief disabled")
	}
	modelName := resolveAINavigatorBriefModel(settings)
	if modelName == nil {
		return nil, fmt.Errorf("navigator model not configured")
	}
	persona := ResolvePersona(settings.NavigatorPersonaMode, settings.NavigatorPersona)
	now := s.now().In(timeutil.JST)
	windowStart, windowEnd, err := ResolveAINavigatorBriefSlotWindow(now, slot)
	if err != nil {
		return nil, err
	}
	candidates, err := s.items.AINavigatorBriefCandidatesInWindow(ctx, userID, windowStart, windowEnd, 24)
	if err != nil {
		return nil, err
	}
	if len(candidates) < 10 {
		failed := &model.AINavigatorBrief{
			UserID:            userID,
			Slot:              slot,
			Status:            model.AINavigatorBriefStatusFailed,
			Persona:           persona,
			Model:             strings.TrimSpace(*modelName),
			SourceWindowStart: &windowStart,
			SourceWindowEnd:   &windowEnd,
			GeneratedAt:       &now,
			ErrorMessage:      "not enough candidates",
		}
		if err := s.briefs.CreateBrief(ctx, failed); err != nil {
			return nil, err
		}
		return failed, fmt.Errorf("not enough candidates")
	}

	anthropicKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, s.settings.GetAnthropicAPIKeyEncrypted, s.cipher, userID, "")
	googleKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, s.settings.GetGoogleAPIKeyEncrypted, s.cipher, userID, "")
	groqKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, s.settings.GetGroqAPIKeyEncrypted, s.cipher, userID, "")
	fireworksKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, s.settings.GetFireworksAPIKeyEncrypted, s.cipher, userID, "")
	deepseekKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, s.settings.GetDeepSeekAPIKeyEncrypted, s.cipher, userID, "")
	alibabaKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, s.settings.GetAlibabaAPIKeyEncrypted, s.cipher, userID, "")
	mistralKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, s.settings.GetMistralAPIKeyEncrypted, s.cipher, userID, "")
	xaiKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, s.settings.GetXAIAPIKeyEncrypted, s.cipher, userID, "")
	zaiKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, s.settings.GetZAIAPIKeyEncrypted, s.cipher, userID, "")
	openRouterKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, s.settings.GetOpenRouterAPIKeyEncrypted, s.cipher, userID, "")
	poeKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, s.settings.GetPoeAPIKeyEncrypted, s.cipher, userID, "")
	openAIKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, s.settings.GetOpenAIAPIKeyEncrypted, s.cipher, userID, "")
	switch LLMProviderForModel(modelName) {
	case "openrouter":
		openAIKey = openRouterKey
	case "poe":
		openAIKey = poeKey
	}
	workerCandidates := make([]BriefingNavigatorCandidate, 0, len(candidates))
	candidateByID := make(map[string]model.BriefingNavigatorCandidate, len(candidates))
	for _, candidate := range candidates {
		candidateByID[candidate.ItemID] = candidate
		var publishedAt *string
		if candidate.PublishedAt != nil {
			v := candidate.PublishedAt.Format(time.RFC3339)
			publishedAt = &v
		}
		workerCandidates = append(workerCandidates, BriefingNavigatorCandidate{
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
	workerCtx := WithWorkerTraceMetadata(ctx, "ai_navigator_brief", &userID, nil, nil, nil)
	resp, err := s.worker.ComposeAINavigatorBriefWithModel(
		workerCtx,
		persona,
		workerCandidates,
		buildAINavigatorBriefIntroContext(now, slot),
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
		failed := &model.AINavigatorBrief{
			UserID:            userID,
			Slot:              slot,
			Status:            model.AINavigatorBriefStatusFailed,
			Persona:           persona,
			Model:             strings.TrimSpace(*modelName),
			SourceWindowStart: &windowStart,
			SourceWindowEnd:   &windowEnd,
			GeneratedAt:       &now,
			ErrorMessage:      err.Error(),
		}
		if createErr := s.briefs.CreateBrief(ctx, failed); createErr != nil {
			return nil, createErr
		}
		return failed, err
	}
	brief := &model.AINavigatorBrief{
		UserID:            userID,
		Slot:              slot,
		Status:            model.AINavigatorBriefStatusGenerated,
		Title:             strings.TrimSpace(resp.Title),
		Intro:             strings.TrimSpace(resp.Intro),
		Summary:           strings.TrimSpace(resp.Summary),
		Persona:           persona,
		Model:             strings.TrimSpace(*modelName),
		SourceWindowStart: &windowStart,
		SourceWindowEnd:   &windowEnd,
		GeneratedAt:       &now,
	}
	if err := s.briefs.CreateBrief(ctx, brief); err != nil {
		return nil, err
	}
	items := make([]model.AINavigatorBriefItem, 0, 10)
	seen := map[string]struct{}{}
	for idx, row := range resp.Items {
		itemID := strings.TrimSpace(row.ItemID)
		comment := strings.TrimSpace(row.Comment)
		if itemID == "" || comment == "" {
			continue
		}
		candidate, ok := candidateByID[itemID]
		if !ok {
			continue
		}
		if _, ok := seen[itemID]; ok {
			continue
		}
		seen[itemID] = struct{}{}
		items = append(items, model.AINavigatorBriefItem{
			BriefID:                 brief.ID,
			Rank:                    len(items) + 1,
			ItemID:                  itemID,
			TitleSnapshot:           strings.TrimSpace(ptrString(candidate.Title)),
			TranslatedTitleSnapshot: strings.TrimSpace(ptrString(candidate.TranslatedTitle)),
			SourceTitleSnapshot:     strings.TrimSpace(ptrString(candidate.SourceTitle)),
			Comment:                 comment,
		})
		if idx >= 9 || len(items) >= 10 {
			break
		}
	}
	if len(items) < 10 {
		if err := s.briefs.MarkBriefFailed(ctx, brief.ID, "llm returned fewer than 10 valid items"); err != nil {
			return nil, err
		}
		brief.Status = model.AINavigatorBriefStatusFailed
		brief.ErrorMessage = "llm returned fewer than 10 valid items"
		return brief, fmt.Errorf("llm returned fewer than 10 valid items")
	}
	if err := s.briefs.AddBriefItems(ctx, brief.ID, items); err != nil {
		return nil, err
	}
	brief.Items = items
	return brief, nil
}

func (s *AINavigatorBriefService) NotifyBrief(ctx context.Context, brief *model.AINavigatorBrief) error {
	if s == nil || brief == nil || s.sender == nil || !s.sender.Enabled() || s.users == nil {
		return nil
	}
	user, err := s.users.GetByID(ctx, brief.UserID)
	if err != nil {
		return err
	}
	if user == nil || strings.TrimSpace(user.Email) == "" {
		return nil
	}
	targetPath := "/ai-navigator-briefs/" + brief.ID
	targetURL := ""
	if s.pageURL != nil {
		targetURL = s.pageURL(targetPath)
	}
	title := strings.TrimSpace(brief.Title)
	if title == "" {
		title = defaultAINavigatorBriefTitle(brief.Slot)
	}
	body := shortenAINavigatorBriefNotificationBody(brief)
	pushRes, err := s.sender.SendToExternalID(ctx, user.Email, title, body, targetURL, map[string]any{
		"type":                   aiNavigatorBriefNotificationKind,
		"ai_navigator_brief_id":  brief.ID,
		"ai_navigator_brief_url": targetURL,
		"slot":                   brief.Slot,
	})
	if err != nil {
		return err
	}
	now := s.now()
	if err := s.briefs.MarkBriefNotified(ctx, brief.ID, now); err != nil {
		return err
	}
	if s.pushLogs != nil {
		var oneSignalID *string
		recipients := 0
		if pushRes != nil {
			if id := strings.TrimSpace(pushRes.ID); id != "" {
				oneSignalID = &id
			}
			recipients = pushRes.Recipients
		}
		itemID := brief.ID
		if err := s.pushLogs.Insert(ctx, repository.PushNotificationLogInput{
			UserID:                  brief.UserID,
			Kind:                    aiNavigatorBriefNotificationKind,
			ItemID:                  &itemID,
			DayJST:                  timeutil.StartOfDayJST(now),
			Title:                   title,
			Message:                 body,
			OneSignalNotificationID: oneSignalID,
			Recipients:              recipients,
		}); err != nil {
			return err
		}
	}
	brief.Status = model.AINavigatorBriefStatusNotified
	brief.NotificationSentAt = &now
	return nil
}

func buildAINavigatorBriefIntroContext(now time.Time, slot string) BriefingNavigatorIntroContext {
	now = now.In(timeutil.JST)
	timeOfDay := slot
	if strings.TrimSpace(timeOfDay) == "" {
		timeOfDay = "morning"
	}
	return BriefingNavigatorIntroContext{
		NowJST:     now.Format(time.RFC3339),
		DateJST:    now.Format("2006-01-02"),
		WeekdayJST: now.Weekday().String(),
		TimeOfDay:  timeOfDay,
		SeasonHint: seasonHintForMonth(now.Month()),
	}
}

func seasonHintForMonth(month time.Month) string {
	switch month {
	case 3, 4, 5:
		return "spring"
	case 6, 7, 8:
		return "summer"
	case 9, 10, 11:
		return "autumn"
	default:
		return "winter"
	}
}

func defaultAINavigatorBriefTitle(slot string) string {
	switch slot {
	case model.AINavigatorBriefSlotMorning:
		return "朝のAIナビ brief"
	case model.AINavigatorBriefSlotNoon:
		return "昼のAIナビ brief"
	default:
		return "夜のAIナビ brief"
	}
}

func shortenAINavigatorBriefNotificationBody(brief *model.AINavigatorBrief) string {
	if brief == nil {
		return ""
	}
	body := strings.TrimSpace(brief.Intro)
	if body == "" {
		body = strings.TrimSpace(brief.Summary)
	}
	body = strings.ReplaceAll(body, "\n", " ")
	body = strings.Join(strings.Fields(body), " ")
	if len(body) > 120 {
		body = body[:120]
	}
	if body == "" {
		body = defaultAINavigatorBriefTitle(brief.Slot)
	}
	return body
}

func resolveAINavigatorBriefModel(settings *model.UserSettings) *string {
	if settings == nil {
		return nil
	}
	if modelName := chooseAINavigatorBriefModelOverride(settings.NavigatorModel, settings); modelName != nil {
		return modelName
	}
	if modelName := chooseAINavigatorBriefModelOverride(settings.NavigatorFallbackModel, settings); modelName != nil {
		return modelName
	}
	for _, provider := range CostEfficientLLMProviders("") {
		if !hasAINavigatorBriefProviderKey(settings, provider) {
			continue
		}
		v := strings.TrimSpace(DefaultLLMModelForPurpose(provider, "summary"))
		if v == "" {
			continue
		}
		return &v
	}
	return nil
}

func chooseAINavigatorBriefModelOverride(modelName *string, settings *model.UserSettings) *string {
	if modelName == nil || settings == nil {
		return nil
	}
	v := strings.TrimSpace(*modelName)
	if v == "" {
		return nil
	}
	if !hasAINavigatorBriefProviderKey(settings, LLMProviderForModel(&v)) {
		return nil
	}
	return &v
}

func hasAINavigatorBriefProviderKey(settings *model.UserSettings, provider string) bool {
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

func appPageURL(path string) string {
	base := strings.TrimSpace(os.Getenv("NEXTAUTH_URL"))
	if base == "" {
		base = strings.TrimSpace(os.Getenv("NEXT_PUBLIC_APP_URL"))
	}
	if base == "" {
		return ""
	}
	base = strings.TrimRight(base, "/")
	path = "/" + strings.TrimLeft(strings.TrimSpace(path), "/")
	return base + path
}

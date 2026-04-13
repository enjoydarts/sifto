package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
)

type ProviderModelSnapshotSyncSummary struct {
	Providers int `json:"providers"`
	Changes   int `json:"changes"`
}

type ProviderModelSnapshotSyncService struct {
	users     *repository.UserRepo
	settings  *repository.UserSettingsRepo
	updates   *repository.ProviderModelUpdateRepo
	pushLogs  *repository.PushNotificationLogRepo
	oneSignal *OneSignalClient
	cipher    *SecretCipher
}

func NewProviderModelSnapshotSyncService(
	users *repository.UserRepo,
	settings *repository.UserSettingsRepo,
	updates *repository.ProviderModelUpdateRepo,
	pushLogs *repository.PushNotificationLogRepo,
	oneSignal *OneSignalClient,
	cipher *SecretCipher,
) *ProviderModelSnapshotSyncService {
	return &ProviderModelSnapshotSyncService{
		users:     users,
		settings:  settings,
		updates:   updates,
		pushLogs:  pushLogs,
		oneSignal: oneSignal,
		cipher:    cipher,
	}
}

func (s *ProviderModelSnapshotSyncService) SyncCommonProviders(ctx context.Context, trigger string) (*ProviderModelSnapshotSyncSummary, error) {
	discovery, err := s.buildDiscoveryService(ctx)
	if err != nil {
		return nil, err
	}
	results, err := discovery.DiscoverAll(ctx)
	if err != nil {
		return nil, err
	}

	now := timeutil.NowJST()
	events := make([]model.ProviderModelChangeEvent, 0)
	syncedProviders := 0

	for _, res := range results {
		if model.ExcludeFromProviderModelSnapshots(res.Provider) {
			continue
		}
		if res.Error != nil {
			if err := s.syncSnapshotProviderFailure(ctx, res.Provider, *res.Error); err != nil {
				return nil, err
			}
			syncedProviders++
			continue
		}
		if err := s.syncSnapshotProvider(ctx, res.Provider, res.Models, "provider_api", trigger, now, &events); err != nil {
			return nil, err
		}
		syncedProviders++
	}

	if len(events) > 0 {
		if err := s.updates.InsertChangeEvents(ctx, events); err != nil {
			return nil, err
		}
		if err := s.sendNotifications(ctx, now, events); err != nil {
			return nil, err
		}
	}

	return &ProviderModelSnapshotSyncSummary{
		Providers: syncedProviders,
		Changes:   len(events),
	}, nil
}

func (s *ProviderModelSnapshotSyncService) syncSnapshotProvider(
	ctx context.Context,
	provider string,
	models []string,
	source string,
	trigger string,
	now time.Time,
	events *[]model.ProviderModelChangeEvent,
) error {
	prev, err := s.updates.GetSnapshot(ctx, provider)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	if prev != nil {
		prevSet := make(map[string]struct{}, len(prev.Models))
		for _, modelID := range prev.Models {
			prevSet[modelID] = struct{}{}
		}
		nextSet := make(map[string]struct{}, len(models))
		for _, modelID := range models {
			nextSet[modelID] = struct{}{}
			if _, ok := prevSet[modelID]; !ok {
				*events = append(*events, model.ProviderModelChangeEvent{
					Provider:   provider,
					ChangeType: "added",
					ModelID:    modelID,
					DetectedAt: now,
					Metadata:   map[string]any{"source": source, "trigger": trigger},
				})
			}
		}
		for _, modelID := range prev.Models {
			if _, ok := nextSet[modelID]; !ok {
				*events = append(*events, model.ProviderModelChangeEvent{
					Provider:   provider,
					ChangeType: "removed",
					ModelID:    modelID,
					DetectedAt: now,
					Metadata:   map[string]any{"source": source, "trigger": trigger},
				})
			}
		}
	}
	return s.updates.UpsertSnapshot(ctx, provider, models, "ok", nil)
}

func (s *ProviderModelSnapshotSyncService) syncSnapshotProviderFailure(
	ctx context.Context,
	provider string,
	errText string,
) error {
	msg := strings.TrimSpace(errText)
	var models []string
	prev, err := s.updates.GetSnapshot(ctx, provider)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	if prev != nil {
		models = prev.Models
	}
	return s.updates.UpsertSnapshot(ctx, provider, models, "failed", &msg)
}

func (s *ProviderModelSnapshotSyncService) sendNotifications(ctx context.Context, now time.Time, events []model.ProviderModelChangeEvent) error {
	if s.oneSignal == nil || !s.oneSignal.Enabled() || s.users == nil || s.pushLogs == nil {
		return nil
	}

	users, err := s.users.ListAll(ctx)
	if err != nil {
		return err
	}

	added := 0
	removed := 0
	providers := make(map[string]struct{})
	for _, ev := range events {
		providers[ev.Provider] = struct{}{}
		if ev.ChangeType == "added" {
			added++
		}
		if ev.ChangeType == "removed" {
			removed++
		}
	}

	title := "Sifto: LLMモデル更新を検知しました"
	message := fmt.Sprintf("追加%d件 / 削除%d件。%dプロバイダーで変更があります。", added, removed, len(providers))
	day := timeutil.StartOfDayJST(now)
	targetURL := appPageURL("/provider-model-snapshots")

	for _, u := range users {
		alreadyNotified, err := s.pushLogs.CountByUserKindDay(ctx, u.ID, "provider_model_update", day)
		if err != nil || alreadyNotified > 0 {
			continue
		}
		pushRes, sendErr := s.oneSignal.SendToExternalID(
			ctx,
			u.Email,
			title,
			message,
			targetURL,
			map[string]any{
				"type":       "provider_model_update",
				"url":        targetURL,
				"target_url": targetURL,
				"added":      added,
				"removed":    removed,
			},
		)
		if sendErr != nil {
			log.Printf("provider-model-snapshot-sync push user=%s: %v", u.ID, sendErr)
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
		if err := s.pushLogs.Insert(ctx, repository.PushNotificationLogInput{
			UserID:                  u.ID,
			Kind:                    "provider_model_update",
			ItemID:                  nil,
			DayJST:                  day,
			Title:                   title,
			Message:                 message,
			OneSignalNotificationID: oneSignalID,
			Recipients:              recipients,
		}); err != nil {
			log.Printf("provider-model-snapshot-sync push log user=%s: %v", u.ID, err)
		}
	}
	return nil
}

func (s *ProviderModelSnapshotSyncService) buildDiscoveryService(ctx context.Context) (*ProviderModelDiscoveryService, error) {
	if s.users == nil || s.settings == nil || s.cipher == nil || !s.cipher.Enabled() {
		return NewProviderModelDiscoveryService(), nil
	}
	users, err := s.users.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	keys := ProviderModelDiscoveryKeys{}
	for _, user := range users {
		if keys.OpenAI == "" {
			keys.OpenAI = s.loadUserKey(ctx, user.ID, s.settings.GetOpenAIAPIKeyEncrypted)
		}
		if keys.Anthropic == "" {
			keys.Anthropic = s.loadUserKey(ctx, user.ID, s.settings.GetAnthropicAPIKeyEncrypted)
		}
		if keys.Google == "" {
			keys.Google = s.loadUserKey(ctx, user.ID, s.settings.GetGoogleAPIKeyEncrypted)
		}
		if keys.Groq == "" {
			keys.Groq = s.loadUserKey(ctx, user.ID, s.settings.GetGroqAPIKeyEncrypted)
		}
		if keys.DeepSeek == "" {
			keys.DeepSeek = s.loadUserKey(ctx, user.ID, s.settings.GetDeepSeekAPIKeyEncrypted)
		}
		if keys.Alibaba == "" {
			keys.Alibaba = s.loadUserKey(ctx, user.ID, s.settings.GetAlibabaAPIKeyEncrypted)
		}
		if keys.Mistral == "" {
			keys.Mistral = s.loadUserKey(ctx, user.ID, s.settings.GetMistralAPIKeyEncrypted)
		}
		if keys.Moonshot == "" {
			keys.Moonshot = s.loadUserKey(ctx, user.ID, s.settings.GetMoonshotAPIKeyEncrypted)
		}
		if keys.SiliconFlow == "" {
			keys.SiliconFlow = s.loadUserKey(ctx, user.ID, s.settings.GetSiliconFlowAPIKeyEncrypted)
		}
		if keys.XAI == "" {
			keys.XAI = s.loadUserKey(ctx, user.ID, s.settings.GetXAIAPIKeyEncrypted)
		}
		if keys.ZAI == "" {
			keys.ZAI = s.loadUserKey(ctx, user.ID, s.settings.GetZAIAPIKeyEncrypted)
		}
		if keys.Fireworks == "" {
			keys.Fireworks = s.loadUserKey(ctx, user.ID, s.settings.GetFireworksAPIKeyEncrypted)
		}
		if keys.Together == "" {
			keys.Together = s.loadUserKey(ctx, user.ID, s.settings.GetTogetherAPIKeyEncrypted)
		}
	}
	return NewProviderModelDiscoveryServiceWithKeys(keys), nil
}

func (s *ProviderModelSnapshotSyncService) loadUserKey(
	ctx context.Context,
	userID string,
	getter func(context.Context, string) (*string, error),
) string {
	if getter == nil || strings.TrimSpace(userID) == "" || s.cipher == nil || !s.cipher.Enabled() {
		return ""
	}
	enc, err := getter(ctx, userID)
	if err != nil || enc == nil || strings.TrimSpace(*enc) == "" {
		return ""
	}
	plain, err := s.cipher.DecryptString(*enc)
	if err != nil {
		log.Printf("provider-model-snapshot-sync decrypt user=%s: %v", userID, err)
		return ""
	}
	return strings.TrimSpace(plain)
}

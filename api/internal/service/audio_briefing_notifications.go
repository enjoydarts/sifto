package service

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
)

const audioBriefingPublishedNotificationKind = "audio_briefing_published"

type audioBriefingPublishedUserLookup interface {
	GetByID(ctx context.Context, userID string) (*model.User, error)
}

type audioBriefingPublishedRuleRepo interface {
	EnsureDefaults(ctx context.Context, userID string) (*model.NotificationPriorityRule, error)
}

type audioBriefingPublishedPushLogRepo interface {
	ExistsByUserKindItem(ctx context.Context, userID, kind, itemID string) (bool, error)
	CountByUserKindsDay(ctx context.Context, userID string, kinds []string, dayJST time.Time) (int, error)
	Insert(ctx context.Context, in repository.PushNotificationLogInput) error
}

type audioBriefingPublishedSender interface {
	Enabled() bool
	SendToExternalID(ctx context.Context, externalID, title, body, targetURL string, data map[string]any) (*OneSignalSendResult, error)
}

type AudioBriefingPublishedNotifier struct {
	users     audioBriefingPublishedUserLookup
	rules     audioBriefingPublishedRuleRepo
	pushLogs  audioBriefingPublishedPushLogRepo
	sender    audioBriefingPublishedSender
	now       func() time.Time
	pageURLFn func(path string) string
}

func NewAudioBriefingPublishedNotifier(
	users audioBriefingPublishedUserLookup,
	rules audioBriefingPublishedRuleRepo,
	pushLogs audioBriefingPublishedPushLogRepo,
	sender audioBriefingPublishedSender,
	now func() time.Time,
) *AudioBriefingPublishedNotifier {
	if now == nil {
		now = timeutil.NowJST
	}
	return &AudioBriefingPublishedNotifier{
		users:     users,
		rules:     rules,
		pushLogs:  pushLogs,
		sender:    sender,
		now:       now,
		pageURLFn: AudioBriefingPageURLFromEnv,
	}
}

func (n *AudioBriefingPublishedNotifier) NotifyPublished(ctx context.Context, job *model.AudioBriefingJob) error {
	if n == nil || job == nil || strings.TrimSpace(job.UserID) == "" || strings.TrimSpace(job.ID) == "" {
		return nil
	}
	if strings.TrimSpace(job.Status) != "published" || n.sender == nil || !n.sender.Enabled() {
		return nil
	}

	rule := &model.NotificationPriorityRule{DailyCap: 3, BriefingEnabled: true}
	if n.rules != nil {
		next, err := n.rules.EnsureDefaults(ctx, job.UserID)
		if err != nil {
			return err
		}
		if next != nil {
			rule = next
		}
	}
	if !rule.BriefingEnabled {
		return nil
	}
	if n.pushLogs != nil {
		alreadyNotified, err := n.pushLogs.ExistsByUserKindItem(ctx, job.UserID, audioBriefingPublishedNotificationKind, job.ID)
		if err != nil {
			return err
		}
		if alreadyNotified {
			return nil
		}
		if rule.DailyCap > 0 {
			dayJST := timeutil.StartOfDayJST(n.now())
			countToday, err := n.pushLogs.CountByUserKindsDay(ctx, job.UserID, []string{"briefing_ready", audioBriefingPublishedNotificationKind}, dayJST)
			if err != nil {
				return err
			}
			if countToday >= rule.DailyCap {
				return nil
			}
		}
	}

	if n.users == nil {
		return nil
	}
	user, err := n.users.GetByID(ctx, job.UserID)
	if err != nil {
		return err
	}
	if user == nil || strings.TrimSpace(user.Email) == "" {
		return nil
	}

	title := "Sifto: 音声ブリーフィングの準備ができました"
	message := strings.TrimSpace(ptrString(job.Title))
	if message == "" {
		message = "新しい音声ブリーフィングを再生できます。"
	}
	if len(message) > 120 {
		message = message[:120]
	}
	targetURL := ""
	if n.pageURLFn != nil {
		targetURL = n.pageURLFn("/audio-briefings/" + job.ID)
	}
	pushRes, err := n.sender.SendToExternalID(ctx, user.Email, title, message, targetURL, map[string]any{
		"type":               audioBriefingPublishedNotificationKind,
		"audio_briefing_id":  job.ID,
		"audio_briefing_url": targetURL,
	})
	if err != nil {
		return err
	}
	if n.pushLogs == nil {
		return nil
	}
	var oneSignalID *string
	recipients := 0
	if pushRes != nil {
		if id := strings.TrimSpace(pushRes.ID); id != "" {
			oneSignalID = &id
		}
		recipients = pushRes.Recipients
	}
	itemID := job.ID
	return n.pushLogs.Insert(ctx, repository.PushNotificationLogInput{
		UserID:                  job.UserID,
		Kind:                    audioBriefingPublishedNotificationKind,
		ItemID:                  &itemID,
		DayJST:                  timeutil.StartOfDayJST(n.now()),
		Title:                   title,
		Message:                 message,
		OneSignalNotificationID: oneSignalID,
		Recipients:              recipients,
	})
}

func AudioBriefingPageURLFromEnv(path string) string {
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

func ptrString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

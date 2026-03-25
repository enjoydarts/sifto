package service

import (
	"context"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
)

type stubAudioBriefingPublishedUserLookup struct {
	user *model.User
	err  error
}

func (s *stubAudioBriefingPublishedUserLookup) GetByID(_ context.Context, _ string) (*model.User, error) {
	return s.user, s.err
}

type stubAudioBriefingPublishedNotificationRepo struct {
	rule *model.NotificationPriorityRule
	err  error
}

func (s *stubAudioBriefingPublishedNotificationRepo) EnsureDefaults(_ context.Context, _ string) (*model.NotificationPriorityRule, error) {
	return s.rule, s.err
}

type stubAudioBriefingPublishedPushLogRepo struct {
	exists     bool
	existsErr  error
	count      int
	countErr   error
	inserted   *repository.PushNotificationLogInput
	insertErr  error
	countKinds []string
}

func (s *stubAudioBriefingPublishedPushLogRepo) ExistsByUserKindItem(_ context.Context, _, _, _ string) (bool, error) {
	return s.exists, s.existsErr
}

func (s *stubAudioBriefingPublishedPushLogRepo) CountByUserKindsDay(_ context.Context, _ string, kinds []string, _ time.Time) (int, error) {
	s.countKinds = append([]string(nil), kinds...)
	return s.count, s.countErr
}

func (s *stubAudioBriefingPublishedPushLogRepo) Insert(_ context.Context, in repository.PushNotificationLogInput) error {
	s.inserted = &in
	return s.insertErr
}

type stubAudioBriefingPublishedSender struct {
	sendCalls int
	external  string
	title     string
	body      string
	targetURL string
	data      map[string]any
	result    *OneSignalSendResult
	err       error
}

func (s *stubAudioBriefingPublishedSender) Enabled() bool { return true }

func (s *stubAudioBriefingPublishedSender) SendToExternalID(_ context.Context, externalID, title, body, targetURL string, data map[string]any) (*OneSignalSendResult, error) {
	s.sendCalls++
	s.external = externalID
	s.title = title
	s.body = body
	s.targetURL = targetURL
	s.data = data
	return s.result, s.err
}

func TestAudioBriefingPublishedNotifierSendsPushAndLogs(t *testing.T) {
	t.Setenv("NEXTAUTH_URL", "https://app.example.com/")
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	title := "朝の音声ブリーフィング"
	job := &model.AudioBriefingJob{
		ID:        "job-1",
		UserID:    "user-1",
		Status:    "published",
		Title:     &title,
		CreatedAt: now,
	}
	logRepo := &stubAudioBriefingPublishedPushLogRepo{}
	sender := &stubAudioBriefingPublishedSender{
		result: &OneSignalSendResult{ID: "onesignal-1", Recipients: 1},
	}
	notifier := NewAudioBriefingPublishedNotifier(
		&stubAudioBriefingPublishedUserLookup{user: &model.User{ID: "user-1", Email: "user@example.com"}},
		&stubAudioBriefingPublishedNotificationRepo{rule: &model.NotificationPriorityRule{UserID: "user-1", DailyCap: 5, BriefingEnabled: true}},
		logRepo,
		sender,
		func() time.Time { return now },
	)

	if err := notifier.NotifyPublished(context.Background(), job); err != nil {
		t.Fatalf("NotifyPublished(...) error = %v", err)
	}
	if sender.sendCalls != 1 {
		t.Fatalf("sendCalls = %d, want 1", sender.sendCalls)
	}
	if sender.external != "user@example.com" {
		t.Fatalf("externalID = %q, want user@example.com", sender.external)
	}
	if sender.targetURL != "https://app.example.com/audio-briefings/job-1" {
		t.Fatalf("targetURL = %q, want detail page", sender.targetURL)
	}
	if sender.data["type"] != "audio_briefing_published" {
		t.Fatalf("data[type] = %v, want audio_briefing_published", sender.data["type"])
	}
	if logRepo.inserted == nil {
		t.Fatal("push log was not inserted")
	}
	if logRepo.inserted.Kind != "audio_briefing_published" {
		t.Fatalf("push log kind = %q, want audio_briefing_published", logRepo.inserted.Kind)
	}
	if logRepo.inserted.ItemID == nil || *logRepo.inserted.ItemID != "job-1" {
		t.Fatalf("push log item_id = %v, want job-1", logRepo.inserted.ItemID)
	}
	if len(logRepo.countKinds) != 2 || logRepo.countKinds[0] != "briefing_ready" || logRepo.countKinds[1] != "audio_briefing_published" {
		t.Fatalf("count kinds = %#v, want briefing kinds", logRepo.countKinds)
	}
}

func TestAudioBriefingPublishedNotifierSkipsWhenBriefingNotificationDisabled(t *testing.T) {
	job := &model.AudioBriefingJob{
		ID:     "job-1",
		UserID: "user-1",
		Status: "published",
	}
	sender := &stubAudioBriefingPublishedSender{}
	logRepo := &stubAudioBriefingPublishedPushLogRepo{}
	notifier := NewAudioBriefingPublishedNotifier(
		&stubAudioBriefingPublishedUserLookup{user: &model.User{ID: "user-1", Email: "user@example.com"}},
		&stubAudioBriefingPublishedNotificationRepo{rule: &model.NotificationPriorityRule{UserID: "user-1", DailyCap: 5, BriefingEnabled: false}},
		logRepo,
		sender,
		func() time.Time { return time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC) },
	)

	if err := notifier.NotifyPublished(context.Background(), job); err != nil {
		t.Fatalf("NotifyPublished(...) error = %v", err)
	}
	if sender.sendCalls != 0 {
		t.Fatalf("sendCalls = %d, want 0", sender.sendCalls)
	}
	if logRepo.inserted != nil {
		t.Fatalf("push log inserted = %#v, want nil", logRepo.inserted)
	}
}

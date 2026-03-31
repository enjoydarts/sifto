package service

import (
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func TestBuildAINavigatorBriefPushLogInputUsesNilItemID(t *testing.T) {
	now := time.Date(2026, 3, 29, 9, 30, 0, 0, time.UTC)
	brief := &model.AINavigatorBrief{
		ID:     "brief-1",
		UserID: "user-1",
	}
	oneSignalID := "onesignal-brief-1"

	got := buildAINavigatorBriefPushLogInput(brief, now, "朝のAIナビブリーフ", "本文", &oneSignalID, 1)

	if got.UserID != "user-1" {
		t.Fatalf("UserID = %q, want user-1", got.UserID)
	}
	if got.Kind != aiNavigatorBriefNotificationKind {
		t.Fatalf("Kind = %q, want %q", got.Kind, aiNavigatorBriefNotificationKind)
	}
	if got.ItemID != nil {
		t.Fatalf("ItemID = %v, want nil", got.ItemID)
	}
	if got.OneSignalNotificationID == nil || *got.OneSignalNotificationID != oneSignalID {
		t.Fatalf("OneSignalNotificationID = %v, want %q", got.OneSignalNotificationID, oneSignalID)
	}
	if got.Recipients != 1 {
		t.Fatalf("Recipients = %d, want 1", got.Recipients)
	}
	if got.DayJST.Format("2006-01-02") != "2026-03-29" {
		t.Fatalf("DayJST = %s, want 2026-03-29", got.DayJST.Format("2006-01-02"))
	}
}

func TestFormatAINavigatorBriefModelLabelPrefersProviderAndResolvedModel(t *testing.T) {
	got := formatAINavigatorBriefModelLabel("kimi-k2.5", &LLMUsage{
		Provider:      "zai",
		Model:         "glm-4.5-air",
		ResolvedModel: "kimi-k2.5",
	})

	if got != "zai / kimi-k2.5" {
		t.Fatalf("formatAINavigatorBriefModelLabel(...) = %q, want %q", got, "zai / kimi-k2.5")
	}
}

func TestFormatAINavigatorBriefModelLabelFallsBackToConfiguredModelWithProvider(t *testing.T) {
	got := formatAINavigatorBriefModelLabel("openrouter::openai/gpt-oss-120b", nil)

	if got != "openrouter / openai/gpt-oss-120b" {
		t.Fatalf("formatAINavigatorBriefModelLabel(...) = %q, want provider-prefixed configured model", got)
	}
}

func TestResolveAINavigatorBriefExecutionModelRestoresProviderAliasedModel(t *testing.T) {
	got := resolveAINavigatorBriefExecutionModel("openrouter / openai/gpt-oss-120b")

	if got != "openrouter::openai/gpt-oss-120b" {
		t.Fatalf("resolveAINavigatorBriefExecutionModel(...) = %q, want %q", got, "openrouter::openai/gpt-oss-120b")
	}
}

func TestResolveAINavigatorBriefExecutionModelRestoresPlainProviderModel(t *testing.T) {
	got := resolveAINavigatorBriefExecutionModel("zai / glm-5")

	if got != "glm-5" {
		t.Fatalf("resolveAINavigatorBriefExecutionModel(...) = %q, want %q", got, "glm-5")
	}
}

package service

import (
	"context"
	"errors"
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

func TestFormatAINavigatorBriefModelLabelFallsBackToConfiguredPlainProviderModel(t *testing.T) {
	got := formatAINavigatorBriefModelLabel("glm-5", nil)

	if got != "zai / glm-5" {
		t.Fatalf("formatAINavigatorBriefModelLabel(...) = %q, want %q", got, "zai / glm-5")
	}
}

func TestHasAINavigatorBriefProviderKeySupportsFeatherless(t *testing.T) {
	settings := &model.UserSettings{HasFeatherlessAPIKey: true}

	if !hasAINavigatorBriefProviderKey(settings, "featherless") {
		t.Fatal("hasAINavigatorBriefProviderKey(featherless) = false, want true")
	}
}

func TestHasAINavigatorBriefProviderKeySupportsDeepInfra(t *testing.T) {
	settings := &model.UserSettings{HasDeepInfraAPIKey: true}

	if !hasAINavigatorBriefProviderKey(settings, "deepinfra") {
		t.Fatal("hasAINavigatorBriefProviderKey(deepinfra) = false, want true")
	}
}

func TestResolveAINavigatorBriefModelsPrefersPrimaryThenFallback(t *testing.T) {
	primary := "openrouter::openai/gpt-oss-120b"
	fallback := "deepinfra::moonshotai/Kimi-K2.5"
	settings := &model.UserSettings{
		AINavigatorBriefModel:         &primary,
		AINavigatorBriefFallbackModel: &fallback,
		HasOpenRouterAPIKey:           true,
		HasDeepInfraAPIKey:            true,
	}

	got := resolveAINavigatorBriefModels(settings)
	if len(got) != 2 {
		t.Fatalf("len(models) = %d, want 2 (%v)", len(got), got)
	}
	if got[0] != primary || got[1] != fallback {
		t.Fatalf("models = %v, want [%q %q]", got, primary, fallback)
	}
}

func TestResolveAINavigatorBriefRunModelsUsesSavedModelThenFallback(t *testing.T) {
	primary := "openrouter::openai/gpt-oss-120b"
	fallback := "deepinfra::moonshotai/Kimi-K2.5"
	settings := &model.UserSettings{
		AINavigatorBriefModel:         &primary,
		AINavigatorBriefFallbackModel: &fallback,
		HasOpenRouterAPIKey:           true,
		HasDeepInfraAPIKey:            true,
	}

	got := resolveAINavigatorBriefRunModels("openrouter / openai/gpt-oss-120b", settings)
	if len(got) != 2 {
		t.Fatalf("len(models) = %d, want 2 (%v)", len(got), got)
	}
	if got[0] != primary || got[1] != fallback {
		t.Fatalf("models = %v, want [%q %q]", got, primary, fallback)
	}
}

func TestComposeAINavigatorBriefWithFallbackUsesSecondModelAfterFailure(t *testing.T) {
	models := []string{"openrouter::openai/gpt-oss-120b", "deepinfra::moonshotai/Kimi-K2.5"}
	var called []string

	resp, usedModel, err := composeAINavigatorBriefWithFallback(context.Background(), "user-1", models, func(modelName *string) (*AINavigatorBriefResponse, error) {
		called = append(called, *modelName)
		if len(called) == 1 {
			return nil, errors.New("status 403")
		}
		return &AINavigatorBriefResponse{Title: "fallback result"}, nil
	})
	if err != nil {
		t.Fatalf("composeAINavigatorBriefWithFallback(...) error = %v", err)
	}
	if resp == nil || resp.Title != "fallback result" {
		t.Fatalf("response = %#v, want fallback response", resp)
	}
	if usedModel != models[1] {
		t.Fatalf("usedModel = %q, want %q", usedModel, models[1])
	}
	if len(called) != 2 || called[0] != models[0] || called[1] != models[1] {
		t.Fatalf("called models = %v, want %v", called, models)
	}
}

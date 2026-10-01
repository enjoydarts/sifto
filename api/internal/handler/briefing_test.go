package handler

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/service"
)

func TestBuildBriefingNavigatorIntroContext(t *testing.T) {
	now := time.Date(2026, 3, 23, 19, 30, 0, 0, time.FixedZone("JST", 9*60*60))
	got := buildBriefingNavigatorIntroContext(now)

	if got.TimeOfDay != "evening" {
		t.Fatalf("time_of_day = %q", got.TimeOfDay)
	}
	if got.WeekdayJST != "Monday" {
		t.Fatalf("weekday_jst = %q", got.WeekdayJST)
	}
	if got.SeasonHint == "" {
		t.Fatal("season_hint is empty")
	}
	if got.NowJST == "" || got.DateJST == "" {
		t.Fatalf("missing now/date: %+v", got)
	}
}

func TestCacheKeyBriefingNavigatorVariesByPersonaModelAndPreview(t *testing.T) {
	k1 := cacheKeyBriefingNavigator("u1", "editor", "gpt-5", false)
	k2 := cacheKeyBriefingNavigator("u1", "snark", "gpt-5", false)
	k3 := cacheKeyBriefingNavigator("u1", "editor", "gpt-5-mini", false)
	k4 := cacheKeyBriefingNavigator("u1", "editor", "gpt-5", true)

	if k1 == k2 || k1 == k3 || k1 == k4 {
		t.Fatalf("cache key should vary, got k1=%q k2=%q k3=%q k4=%q", k1, k2, k3, k4)
	}
}

func TestShouldCacheBriefingNavigatorResponse(t *testing.T) {
	if shouldCacheBriefingNavigatorResponse(nil) {
		t.Fatal("nil response should not be cached")
	}
	if shouldCacheBriefingNavigatorResponse(&model.BriefingNavigatorEnvelope{}) {
		t.Fatal("empty response should not be cached")
	}
	if !shouldCacheBriefingNavigatorResponse(&model.BriefingNavigatorEnvelope{
		Navigator: &model.BriefingNavigator{
			Persona: "editor",
			Picks: []model.BriefingNavigatorPick{
				{ItemID: "item-1", Rank: 1, Title: "title", Comment: "comment"},
			},
		},
	}) {
		t.Fatal("response with picks should be cached")
	}
	if !shouldCacheBriefingNavigatorResponse(&model.BriefingNavigatorEnvelope{
		Navigator: &model.BriefingNavigator{
			Persona: "editor",
			Intro:   "こんばんは。新しい未読が届いたらまた案内します。",
			Picks:   []model.BriefingNavigatorPick{},
		},
	}) {
		t.Fatal("response with intro only should be cached")
	}
}

func TestHasNavigatorProviderKeySupportsFeatherless(t *testing.T) {
	settings := &model.UserSettings{HasFeatherlessAPIKey: true}

	if !hasNavigatorProviderKey(settings, "featherless") {
		t.Fatal("hasNavigatorProviderKey(featherless) = false, want true")
	}
}

func TestHasNavigatorProviderKeySupportsDeepInfra(t *testing.T) {
	settings := &model.UserSettings{HasDeepInfraAPIKey: true}

	if !hasNavigatorProviderKey(settings, "deepinfra") {
		t.Fatal("hasNavigatorProviderKey(deepinfra) = false, want true")
	}
}

func TestResolveBriefingNavigatorModelsIncludesConfiguredFallback(t *testing.T) {
	primary, fallback := "mimo-v2.6-flash", "deepinfra::zai-org/GLM-5.3-Flash"
	settings := &model.UserSettings{
		NavigatorModel: &primary, NavigatorFallbackModel: &fallback,
		HasXiaomiMiMoTokenPlanAPIKey: true, HasDeepInfraAPIKey: true,
	}
	got := resolveBriefingNavigatorModels(settings)
	if len(got) != 2 || got[0] != primary || got[1] != fallback {
		t.Fatalf("models = %v, want [%s %s]", got, primary, fallback)
	}
	settings.NavigatorFallbackModel = &primary
	if got := resolveBriefingNavigatorModels(settings); len(got) != 1 || got[0] != primary {
		t.Fatalf("duplicate models = %v", got)
	}
	settings.NavigatorFallbackModel = &fallback
	settings.HasXiaomiMiMoTokenPlanAPIKey = false
	if got := resolveBriefingNavigatorModels(settings); len(got) != 1 || got[0] != fallback {
		t.Fatalf("models with missing primary key = %v", got)
	}
}

func TestGenerateBriefingNavigatorWithFallbackAfterWorkerError(t *testing.T) {
	for _, message := range []string{
		"worker /briefing-navigator: status 500 detail=provider status=429 quota exhausted",
		"worker /briefing-navigator: status 502 detail=Bad Gateway",
		"worker /briefing-navigator: context deadline exceeded",
	} {
		t.Run(message, func(t *testing.T) {
			models := []string{"mimo-v2.6-flash", "deepinfra::zai-org/GLM-5.3-Flash"}
			var called []string
			want := &service.BriefingNavigatorResponse{Intro: "fallback result"}
			resp, usedModel, err := generateNavigatorWithFallback(context.Background(), "u1", "briefing_navigator", models, func(modelName *string) (*service.BriefingNavigatorResponse, error) {
				called = append(called, *modelName)
				if len(called) == 1 {
					return nil, errors.New(message)
				}
				return want, nil
			})
			if err != nil || resp != want || usedModel != models[1] || len(called) != 2 || called[0] != models[0] || called[1] != models[1] {
				t.Fatalf("resp=%v model=%s calls=%v err=%v", resp, usedModel, called, err)
			}
		})
	}
}

func TestGenerateBriefingNavigatorWithFallbackStopsAfterSuccess(t *testing.T) {
	calls := 0
	_, usedModel, err := generateNavigatorWithFallback(context.Background(), "u1", "briefing_navigator", []string{"primary", "fallback"}, func(modelName *string) (*service.BriefingNavigatorResponse, error) {
		calls++
		return &service.BriefingNavigatorResponse{Intro: "primary result"}, nil
	})
	if err != nil || calls != 1 || usedModel != "primary" {
		t.Fatalf("model=%s calls=%d err=%v", usedModel, calls, err)
	}
}

func TestGenerateBriefingNavigatorWithFallbackPreservesBothErrors(t *testing.T) {
	resp, _, err := generateNavigatorWithFallback(context.Background(), "u1", "briefing_navigator", []string{"primary", "fallback"}, func(modelName *string) (*service.BriefingNavigatorResponse, error) {
		return nil, errors.New(*modelName + " unavailable")
	})
	if resp != nil || err == nil || !strings.Contains(err.Error(), "primary unavailable") || !strings.Contains(err.Error(), "fallback unavailable") {
		t.Fatalf("resp=%v err=%v", resp, err)
	}
}

func TestGenerateBriefingNavigatorWithFallbackStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	_, _, err := generateNavigatorWithFallback(ctx, "u1", "briefing_navigator", []string{"primary", "fallback"}, func(modelName *string) (*service.BriefingNavigatorResponse, error) {
		calls++
		cancel()
		return nil, context.Canceled
	})
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

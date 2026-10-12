package inngest

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/inngest/inngest/pkg/enums"
	"github.com/inngest/inngestgo"
)

func TestConcurrencyFactoriesPreserveLimitsAndFlowControl(t *testing.T) {
	// Construction reads only a synthetic public model catalog. No function
	// body, database call, event send or external API request is executed.
	catalog := filepath.Join(t.TempDir(), "catalog.json")
	if err := os.WriteFile(catalog, []byte(`{"default_model":"fixture-model","models":[{"id":"fixture-model"}],"gate_policy":{"version":"fixture"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("D1_CATALOG_PATH", catalog)
	t.Setenv("JEV_CATALOG_PATH", catalog)
	t.Setenv("INNGEST_DEV", "1")
	client := stubInngestClient{}
	for _, test := range []struct {
		id       string
		name     string
		create   func() (inngestgo.ServableFunction, error)
		limits   []inngestgo.ConfigStepConcurrency
		throttle *inngestgo.ConfigThrottle
	}{
		{"evaluate-d1-shadow", "Evaluate D1 Shadow", func() (inngestgo.ServableFunction, error) {
			return d1ShadowFn(client, nil, nil, nil)
		}, []inngestgo.ConfigStepConcurrency{{Limit: 1}}, nil},
		{"search-suggestion-topics-refresh", "Refresh Search Suggestion Topics", func() (inngestgo.ServableFunction, error) {
			return searchSuggestionTopicsRefreshFn(client, nil, nil)
		}, []inngestgo.ConfigStepConcurrency{{Limit: 1, Key: inngestgo.StrPtr("event.data.user_id"), Scope: enums.ConcurrencyScopeFn}}, nil},
		{"run-ai-navigator-brief-pipeline", "Run AI Navigator Brief Pipeline", func() (inngestgo.ServableFunction, error) {
			return runAINavigatorBriefPipelineFn(client, nil, nil, nil, nil)
		}, []inngestgo.ConfigStepConcurrency{{Limit: 1, Key: inngestgo.StrPtr("event.data.brief_id"), Scope: enums.ConcurrencyScopeFn}}, nil},
		{"run-audio-briefing-pipeline", "Run Audio Briefing Pipeline", func() (inngestgo.ServableFunction, error) {
			return runAudioBriefingPipelineFn(client, nil, nil, nil)
		}, []inngestgo.ConfigStepConcurrency{{Limit: 1, Key: inngestgo.StrPtr("event.data.job_id"), Scope: enums.ConcurrencyScopeFn}}, nil},
		{"run-item-bulk-job", "Run Item Bulk Job", func() (inngestgo.ServableFunction, error) {
			return runItemBulkJobFn(client, nil, nil)
		}, []inngestgo.ConfigStepConcurrency{{Limit: 1}, {Limit: 1, Key: inngestgo.StrPtr("event.data.job_id")}},
			&inngestgo.ConfigThrottle{Limit: 1, Period: 30 * time.Second, Burst: 1}},
		{"process-item", "Process Item", func() (inngestgo.ServableFunction, error) {
			return processItemFn(client, nil, nil, nil, nil, nil, nil)
		}, []inngestgo.ConfigStepConcurrency{{Limit: 5}, {Limit: 1, Key: inngestgo.StrPtr("event.data.item_id")}},
			&inngestgo.ConfigThrottle{Limit: 30, Period: time.Minute, Burst: 6}},
	} {
		t.Run(test.id, func(t *testing.T) {
			function, err := test.create()
			if err != nil {
				t.Fatal(err)
			}
			config := function.Config()
			assertStepConcurrency(t, config, test.limits)
			if config.ID != test.id || config.Name != test.name {
				t.Fatalf("identity changed: %+v", config)
			}
			if (config.Throttle == nil) != (test.throttle == nil) {
				t.Fatalf("throttle presence changed: %+v", config.Throttle)
			}
			if test.throttle != nil && *config.Throttle != *test.throttle {
				t.Fatalf("throttle = %+v, want %+v", config.Throttle, test.throttle)
			}
		})
	}
}

package inngest

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/inngest/inngest/pkg/enums"
	"github.com/inngest/inngestgo"
)

type stubInngestClient struct{}

func (stubInngestClient) AppID() string                                     { return "test-app" }
func (stubInngestClient) Send(context.Context, any) (string, error)         { return "", nil }
func (stubInngestClient) SendMany(context.Context, []any) ([]string, error) { return nil, nil }
func (stubInngestClient) Options() inngestgo.ClientOpts                     { return inngestgo.ClientOpts{} }
func (stubInngestClient) Serve() http.Handler                               { return nil }
func (stubInngestClient) ServeWithOpts(inngestgo.ServeOpts) http.Handler    { return nil }
func (stubInngestClient) SetOptions(inngestgo.ClientOpts) error             { return nil }
func (stubInngestClient) SetURL(*url.URL)                                   {}

func TestRunAudioBriefingPipelineFnConfiguresJobScopedConcurrency(t *testing.T) {
	fn, err := runAudioBriefingPipelineFn(stubInngestClient{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("runAudioBriefingPipelineFn(...) error = %v", err)
	}

	cfg := fn.Config()
	if len(cfg.Concurrency) != 1 {
		t.Fatalf("len(concurrency) = %d, want 1", len(cfg.Concurrency))
	}
	if cfg.Concurrency[0].Limit != 1 {
		t.Fatalf("concurrency limit = %d, want 1", cfg.Concurrency[0].Limit)
	}
	if cfg.Concurrency[0].Key == nil || *cfg.Concurrency[0].Key != "event.data.job_id" {
		t.Fatalf("concurrency key = %v, want event.data.job_id", cfg.Concurrency[0].Key)
	}
	if cfg.Concurrency[0].Scope != enums.ConcurrencyScopeFn {
		t.Fatalf("concurrency scope = %v, want function scope", cfg.Concurrency[0].Scope)
	}
}

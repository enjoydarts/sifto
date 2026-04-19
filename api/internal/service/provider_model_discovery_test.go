package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

func TestProviderModelDiscoveryFetchListAPIProviders(t *testing.T) {
	cases := []struct {
		name      string
		fetchFunc func(context.Context, *ProviderModelDiscoveryService) ([]string, error)
		apiKey    string
		baseKey   string
		baseURL   string
		wantPath  string
	}{
		{
			name: "alibaba",
			fetchFunc: func(ctx context.Context, svc *ProviderModelDiscoveryService) ([]string, error) {
				return svc.fetchAlibabaModels(ctx)
			},
			apiKey:   "test-alibaba-key",
			baseKey:  "ALIBABA_API_BASE_URL",
			baseURL:  "/compatible-mode/v1/chat/completions",
			wantPath: "/compatible-mode/v1/models",
		},
		{
			name: "moonshot",
			fetchFunc: func(ctx context.Context, svc *ProviderModelDiscoveryService) ([]string, error) {
				return svc.fetchMoonshotModels(ctx)
			},
			apiKey:   "test-moonshot-key",
			baseKey:  "MOONSHOT_API_BASE_URL",
			baseURL:  "/v1/chat/completions",
			wantPath: "/v1/models",
		},
		{
			name: "zai",
			fetchFunc: func(ctx context.Context, svc *ProviderModelDiscoveryService) ([]string, error) {
				return svc.fetchZAIModels(ctx)
			},
			apiKey:   "test-zai-key",
			baseKey:  "ZAI_API_BASE_URL",
			baseURL:  "/api/paas/v4/chat/completions",
			wantPath: "/api/paas/v4/models",
		},
		{
			name: "poe",
			fetchFunc: func(ctx context.Context, svc *ProviderModelDiscoveryService) ([]string, error) {
				return svc.fetchPoeModels(ctx)
			},
			apiKey:   "test-poe-key",
			baseKey:  "POE_API_BASE_URL",
			baseURL:  "/v1",
			wantPath: "/v1/models",
		},
		{
			name: "siliconflow",
			fetchFunc: func(ctx context.Context, svc *ProviderModelDiscoveryService) ([]string, error) {
				return svc.fetchSiliconFlowModels(ctx)
			},
			apiKey:   "test-siliconflow-key",
			baseKey:  "SILICONFLOW_API_BASE_URL",
			baseURL:  "/v1/chat/completions",
			wantPath: "/v1/models",
		},
		{
			name: "together",
			fetchFunc: func(ctx context.Context, svc *ProviderModelDiscoveryService) ([]string, error) {
				return svc.fetchTogetherModels(ctx)
			},
			apiKey:   "test-together-key",
			baseKey:  "TOGETHER_API_BASE_URL",
			baseURL:  "/v1",
			wantPath: "/v1/models",
		},
		{
			name: "xiaomi_mimo_token_plan",
			fetchFunc: func(ctx context.Context, svc *ProviderModelDiscoveryService) ([]string, error) {
				return svc.fetchXiaomiMiMoTokenPlanModels(ctx)
			},
			apiKey:   "test-mimo-key",
			baseKey:  "XIAOMI_MIMO_TOKEN_PLAN_API_BASE_URL",
			baseURL:  "/v1",
			wantPath: "/v1/models",
		},
		{
			name: "featherless",
			fetchFunc: func(ctx context.Context, svc *ProviderModelDiscoveryService) ([]string, error) {
				return svc.fetchFeatherlessModels(ctx)
			},
			apiKey:   "test-featherless-key",
			baseKey:  "FEATHERLESS_API_BASE_URL",
			baseURL:  "/v1",
			wantPath: "/v1/models",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c := c
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Fatalf("method = %s, want %s", r.Method, http.MethodGet)
				}
				if r.URL.Path != c.wantPath {
					t.Fatalf("path = %s, want %s", r.URL.Path, c.wantPath)
				}
				if c.name == "xiaomi_mimo_token_plan" {
					if got := r.Header.Get("api-key"); got != c.apiKey {
						t.Fatalf("api-key = %q, want %q", got, c.apiKey)
					}
				} else if c.name != "minimax" {
					if got := r.Header.Get("Authorization"); got != "Bearer "+c.apiKey {
						t.Fatalf("authorization = %q, want %q", got, "Bearer "+c.apiKey)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": []map[string]string{
						{"id": c.name + "-model-1"},
						{"id": c.name + "-model-2"},
						{"id": c.name + "-model-1"},
					},
				})
			}))
			defer server.Close()

			if c.baseKey != "" {
				t.Setenv(c.baseKey, server.URL+c.baseURL)
			}
			t.Setenv(strings.ToUpper(c.name)+"_API_KEY", c.apiKey)
			svc := NewProviderModelDiscoveryService()
			svc.http = server.Client()

			models, err := c.fetchFunc(context.Background(), svc)
			if err != nil {
				t.Fatalf("fetch failed: %v", err)
			}
			if len(models) != 2 || models[0] != c.name+"-model-1" || models[1] != c.name+"-model-2" {
				t.Fatalf("models = %#v, want sorted unique two models", models)
			}
		})
	}
}

func TestProviderModelDiscoveryFetchFeatherlessSnapshots(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %s, want %s", r.URL.Path, "/v1/models")
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-featherless-key" {
			t.Fatalf("authorization = %q, want %q", got, "Bearer test-featherless-key")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":                        "meta-llama/llama-3.3-70b",
					"name":                      "Llama 3.3 70B",
					"model_class":               "text",
					"context_length":            131072,
					"max_completion_tokens":     8192,
					"is_gated":                  true,
					"available_on_current_plan": false,
				},
			},
		})
	}))
	defer server.Close()

	t.Setenv("FEATHERLESS_API_BASE_URL", server.URL+"/v1")
	svc := NewProviderModelDiscoveryServiceWithKeys(ProviderModelDiscoveryKeys{
		Featherless: "test-featherless-key",
	})
	svc.http = server.Client()

	models, err := svc.fetchFeatherlessSnapshots(context.Background())
	if err != nil {
		t.Fatalf("fetchFeatherlessSnapshots() error = %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(models))
	}
	if models[0].ModelID != "meta-llama/llama-3.3-70b" {
		t.Fatalf("model_id = %q, want %q", models[0].ModelID, "meta-llama/llama-3.3-70b")
	}
	if models[0].DisplayName != "Llama 3.3 70B" {
		t.Fatalf("display_name = %q, want %q", models[0].DisplayName, "Llama 3.3 70B")
	}
	if models[0].ModelClass != "text" {
		t.Fatalf("model_class = %q, want %q", models[0].ModelClass, "text")
	}
	if models[0].ContextLength == nil || *models[0].ContextLength != 131072 {
		t.Fatalf("context_length = %v, want 131072", models[0].ContextLength)
	}
	if models[0].MaxCompletionTokens == nil || *models[0].MaxCompletionTokens != 8192 {
		t.Fatalf("max_completion_tokens = %v, want 8192", models[0].MaxCompletionTokens)
	}
	if !models[0].IsGated {
		t.Fatal("is_gated = false, want true")
	}
	if models[0].AvailableOnCurrentPlan {
		t.Fatal("available_on_current_plan = true, want false")
	}
}

func TestProviderModelDiscoveryFetchMiniMaxModelsFromDocs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/docs/api-reference/api-overview" {
			t.Fatalf("path = %s, want %s", r.URL.Path, "/docs/api-reference/api-overview")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`
			<html>
				<body>
					<div>MiniMax-M2.7</div>
					<div>MiniMax-M2.7-highspeed</div>
					<div>MiniMax-M2.5</div>
					<div>MiniMax-M2.7</div>
				</body>
			</html>
		`))
	}))
	defer server.Close()

	svc := NewProviderModelDiscoveryService()
	svc.http = server.Client()
	original := miniMaxDiscoveryDocsURLs
	miniMaxDiscoveryDocsURLs = []string{server.URL + "/docs/api-reference/api-overview"}
	defer func() { miniMaxDiscoveryDocsURLs = original }()

	models, err := svc.fetchMiniMaxModels(context.Background())
	if err != nil {
		t.Fatalf("fetchMiniMaxModels failed: %v", err)
	}
	want := []string{"MiniMax-M2.5", "MiniMax-M2.7", "MiniMax-M2.7-highspeed"}
	if !slices.Equal(models, want) {
		t.Fatalf("models = %#v, want %#v", models, want)
	}
}

func TestNormalizeMiniMaxAPIBaseURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty", raw: "", want: "https://api.minimax.io"},
		{name: "base", raw: "https://api.minimax.io", want: "https://api.minimax.io"},
		{name: "v1", raw: "https://api.minimax.io/v1", want: "https://api.minimax.io"},
		{name: "chat completions", raw: "https://api.minimax.io/chat/completions", want: "https://api.minimax.io"},
		{name: "v1 chat completions", raw: "https://api.minimax.io/v1/chat/completions", want: "https://api.minimax.io"},
		{name: "anthropic", raw: "https://api.minimax.io/anthropic", want: "https://api.minimax.io"},
		{name: "anthropic v1", raw: "https://api.minimax.io/anthropic/v1", want: "https://api.minimax.io"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeMiniMaxAPIBaseURL(tt.raw); got != tt.want {
				t.Fatalf("normalizeMiniMaxAPIBaseURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestExtractMiniMaxModelIDs(t *testing.T) {
	body := `
		<div>MiniMax-M2.7</div>
		<div>MiniMax-M2.7-highspeed</div>
		<div>MiniMax-M2.5</div>
		<div>MiniMax-M2-her</div>
		<div>MiniMax-M2</div>
	`

	got := normalizeModelIDs(extractMiniMaxModelIDs(body))
	want := []string{"MiniMax-M2", "MiniMax-M2-her", "MiniMax-M2.5", "MiniMax-M2.7", "MiniMax-M2.7-highspeed"}
	if !slices.Equal(got, want) {
		t.Fatalf("extractMiniMaxModelIDs() = %#v, want %#v", got, want)
	}
}

func TestNormalizeTogetherAPIBaseURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty", raw: "", want: "https://api.together.xyz"},
		{name: "base", raw: "https://api.together.xyz", want: "https://api.together.xyz"},
		{name: "v1", raw: "https://api.together.xyz/v1", want: "https://api.together.xyz"},
		{name: "chat completions", raw: "https://api.together.xyz/chat/completions", want: "https://api.together.xyz"},
		{name: "v1 chat completions", raw: "https://api.together.xyz/v1/chat/completions", want: "https://api.together.xyz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeTogetherAPIBaseURL(tt.raw); got != tt.want {
				t.Fatalf("normalizeTogetherAPIBaseURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestNormalizeFeatherlessAPIBaseURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty", raw: "", want: "https://api.featherless.ai"},
		{name: "base", raw: "https://api.featherless.ai", want: "https://api.featherless.ai"},
		{name: "v1", raw: "https://api.featherless.ai/v1", want: "https://api.featherless.ai"},
		{name: "chat completions", raw: "https://api.featherless.ai/chat/completions", want: "https://api.featherless.ai"},
		{name: "v1 chat completions", raw: "https://api.featherless.ai/v1/chat/completions", want: "https://api.featherless.ai"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeFeatherlessAPIBaseURL(tt.raw); got != tt.want {
				t.Fatalf("normalizeFeatherlessAPIBaseURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestProviderModelDiscoveryDiscoverAllSkipsMissingKeysAndReturnsConfiguredProviders(t *testing.T) {
	moonshotKey := "test-moonshot-key"
	poeKey := "test-poe-key"
	siliconFlowKey := "test-siliconflow-key"
	xiaomiKey := "test-mimo-key"
	featherlessKey := "test-featherless-key"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/compatible-mode/v1/models":
			_ = json.NewEncoder(w).Encode(struct {
				Data []struct {
					ID string `json:"id"`
				} `json:"data"`
			}{Data: []struct {
				ID string `json:"id"`
			}{{ID: "alibaba-model-1"}}})
		case "/v1/models":
			switch {
			case r.Header.Get("api-key") == xiaomiKey:
				_ = json.NewEncoder(w).Encode(struct {
					Data []struct {
						ID string `json:"id"`
					} `json:"data"`
				}{Data: []struct {
					ID string `json:"id"`
				}{{ID: "xiaomi_mimo_token_plan-model-1"}}})
			case r.Header.Get("Authorization") == "Bearer "+moonshotKey:
				_ = json.NewEncoder(w).Encode(struct {
					Data []struct {
						ID string `json:"id"`
					} `json:"data"`
				}{Data: []struct {
					ID string `json:"id"`
				}{{ID: "moonshot-model-1"}}})
			case r.Header.Get("Authorization") == "Bearer "+poeKey:
				_ = json.NewEncoder(w).Encode(struct {
					Data []struct {
						ID string `json:"id"`
					} `json:"data"`
				}{Data: []struct {
					ID string `json:"id"`
				}{{ID: "poe-model-1"}}})
			case r.Header.Get("Authorization") == "Bearer "+siliconFlowKey:
				_ = json.NewEncoder(w).Encode(struct {
					Data []struct {
						ID string `json:"id"`
					} `json:"data"`
				}{Data: []struct {
					ID string `json:"id"`
				}{{ID: "siliconflow-model-1"}}})
			case r.Header.Get("Authorization") == "Bearer "+featherlessKey:
				_ = json.NewEncoder(w).Encode(struct {
					Data []struct {
						ID string `json:"id"`
					} `json:"data"`
				}{Data: []struct {
					ID string `json:"id"`
				}{{ID: "featherless-model-1"}}})
			default:
				t.Fatalf("unexpected auth for /v1/models: authorization=%q api-key=%q", r.Header.Get("Authorization"), r.Header.Get("api-key"))
			}
		case "/docs/api-reference/api-overview":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`
				<div>MiniMax-M2.7</div>
				<div>MiniMax-M2.7-highspeed</div>
				<div>MiniMax-M2.5</div>
			`))
		case "/api/paas/v4/models":
			_ = json.NewEncoder(w).Encode(struct {
				Data []struct {
					ID string `json:"id"`
				} `json:"data"`
			}{Data: []struct {
				ID string `json:"id"`
			}{{ID: "zai-model-1"}}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("GROQ_API_KEY", "")
	t.Setenv("DEEPSEEK_API_KEY", "")
	t.Setenv("MISTRAL_API_KEY", "")
	t.Setenv("XAI_API_KEY", "")
	t.Setenv("FIREWORKS_API_KEY", "")
	t.Setenv("POE_API_KEY", poeKey)
	t.Setenv("POE_API_BASE_URL", server.URL+"/v1")
	t.Setenv("ALIBABA_API_KEY", "test-alibaba-key")
	t.Setenv("ALIBABA_API_BASE_URL", server.URL+"/compatible-mode/v1/chat/completions")
	t.Setenv("MOONSHOT_API_KEY", moonshotKey)
	t.Setenv("MOONSHOT_API_BASE_URL", server.URL+"/v1/chat/completions")
	t.Setenv("SILICONFLOW_API_KEY", siliconFlowKey)
	t.Setenv("SILICONFLOW_API_BASE_URL", server.URL+"/v1/chat/completions")
	t.Setenv("XIAOMI_MIMO_TOKEN_PLAN_API_KEY", "test-mimo-key")
	t.Setenv("XIAOMI_MIMO_TOKEN_PLAN_API_BASE_URL", server.URL+"/v1")
	t.Setenv("ZAI_API_KEY", "test-zai-key")
	t.Setenv("ZAI_API_BASE_URL", server.URL+"/api/paas/v4/chat/completions")
	t.Setenv("FEATHERLESS_API_KEY", featherlessKey)
	t.Setenv("FEATHERLESS_API_BASE_URL", server.URL+"/v1/chat/completions")

	svc := NewProviderModelDiscoveryService()
	svc.http = server.Client()
	originalMiniMaxURLs := miniMaxDiscoveryDocsURLs
	miniMaxDiscoveryDocsURLs = []string{server.URL + "/docs/api-reference/api-overview"}
	defer func() { miniMaxDiscoveryDocsURLs = originalMiniMaxURLs }()
	results, err := svc.DiscoverAll(context.Background())
	if err != nil {
		t.Fatalf("DiscoverAll failed: %v", err)
	}
	if len(results) != 8 {
		t.Fatalf("providers count = %d, want 8", len(results))
	}
	got := make([]string, 0, len(results))
	for _, item := range results {
		got = append(got, item.Provider)
		if item.Provider == "alibaba" {
			if item.Models[0] != "alibaba-model-1" {
				t.Fatalf("alibaba model = %q, want %q", item.Models[0], "alibaba-model-1")
			}
		}
		if item.Provider == "moonshot" {
			if item.Models[0] != "moonshot-model-1" {
				t.Fatalf("moonshot model = %q, want %q", item.Models[0], "moonshot-model-1")
			}
		}
		if item.Provider == "zai" {
			if item.Models[0] != "zai-model-1" {
				t.Fatalf("zai model = %q, want %q", item.Models[0], "zai-model-1")
			}
		}
		if item.Provider == "poe" {
			if item.Models[0] != "poe-model-1" {
				t.Fatalf("poe model = %q, want %q", item.Models[0], "poe-model-1")
			}
		}
		if item.Provider == "siliconflow" {
			if item.Models[0] != "siliconflow-model-1" {
				t.Fatalf("siliconflow model = %q, want %q", item.Models[0], "siliconflow-model-1")
			}
		}
		if item.Provider == "xiaomi_mimo_token_plan" {
			if item.Models[0] != "xiaomi_mimo_token_plan-model-1" {
				t.Fatalf("xiaomi_mimo_token_plan model = %q, want %q", item.Models[0], "xiaomi_mimo_token_plan-model-1")
			}
		}
		if item.Provider == "featherless" {
			if item.Models[0] != "featherless-model-1" {
				t.Fatalf("featherless model = %q, want %q", item.Models[0], "featherless-model-1")
			}
		}
		if item.Provider == "minimax" {
			if item.Models[0] != "MiniMax-M2.5" {
				t.Fatalf("minimax model = %q, want %q", item.Models[0], "MiniMax-M2.5")
			}
		}
	}
	want := []string{"alibaba", "featherless", "minimax", "moonshot", "poe", "siliconflow", "xiaomi_mimo_token_plan", "zai"}
	slices.Sort(got)
	slices.Sort(want)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("providers = %#v, want %#v", got, want)
		}
	}
}

func TestProviderModelDiscoveryFetchAlibabaModelsRetriesTransientServerError(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/compatible-mode/v1/models" {
			t.Fatalf("path = %s, want %s", r.URL.Path, "/compatible-mode/v1/models")
		}
		current := attempts.Add(1)
		if current < 2 {
			http.Error(w, `{"error":"temporary"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"id": "qwen-max"}},
		})
	}))
	defer server.Close()

	t.Setenv("ALIBABA_API_KEY", "test-alibaba-key")
	t.Setenv("ALIBABA_API_BASE_URL", server.URL+"/compatible-mode/v1/chat/completions")

	svc := NewProviderModelDiscoveryService()
	svc.http = server.Client()

	models, err := svc.fetchAlibabaModels(context.Background())
	if err != nil {
		t.Fatalf("fetchAlibabaModels failed: %v", err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
	if len(models) != 1 || models[0] != "qwen-max" {
		t.Fatalf("models = %#v, want %#v", models, []string{"qwen-max"})
	}
}

func TestProviderModelDiscoveryFetchFireworksModelsRetriesTransientServerError(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/accounts/fireworks/models" {
			t.Fatalf("path = %s, want %s", r.URL.Path, "/v1/accounts/fireworks/models")
		}
		if r.URL.Query().Get("pageSize") != "100" {
			t.Fatalf("pageSize = %q, want %q", r.URL.Query().Get("pageSize"), "100")
		}
		if got := r.URL.Query().Get("readMask"); !strings.Contains(got, "models.name") || !strings.Contains(got, "models.displayName") {
			t.Fatalf("readMask = %q, want models.name/models.displayName", got)
		}
		current := attempts.Add(1)
		if current < 2 {
			http.Error(w, `{"error":"temporary"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"name": "accounts/fireworks/models/fireworks/glm-5", "displayName": "GLM-5 Instruct", "description": "LLM text model", "public": true},
			},
		})
	}))
	defer server.Close()

	t.Setenv("FIREWORKS_API_KEY", "test-fireworks-key")
	t.Setenv("FIREWORKS_API_BASE_URL", server.URL)

	svc := NewProviderModelDiscoveryService()
	svc.http = server.Client()

	models, err := svc.fetchFireworksModels(context.Background())
	if err != nil {
		t.Fatalf("fetchFireworksModels failed: %v", err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
	if len(models) != 1 || models[0] != "fireworks/glm-5" {
		t.Fatalf("models = %#v, want %#v", models, []string{"fireworks/glm-5"})
	}
}

func TestProviderModelDiscoveryDiscoverAllReturnsProviderErrorsWithoutFailingWholeSync(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/compatible-mode/v1/models":
			http.Error(w, `{"error":"temporary"}`, http.StatusInternalServerError)
		case "/v1/models":
			switch r.Header.Get("Authorization") {
			case "Bearer test-moonshot-key":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": []map[string]string{{"id": "moonshot-model-1"}},
				})
			default:
				t.Fatalf("unexpected authorization for /v1/models: %q", r.Header.Get("Authorization"))
			}
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("ALIBABA_API_KEY", "test-alibaba-key")
	t.Setenv("ALIBABA_API_BASE_URL", server.URL+"/compatible-mode/v1/chat/completions")
	t.Setenv("MOONSHOT_API_KEY", "test-moonshot-key")
	t.Setenv("MOONSHOT_API_BASE_URL", server.URL+"/v1/chat/completions")

	svc := NewProviderModelDiscoveryService()
	svc.http = server.Client()

	results, err := svc.DiscoverAll(context.Background())
	if err != nil {
		t.Fatalf("DiscoverAll failed: %v", err)
	}

	resultByProvider := make(map[string]ProviderModelsResult, len(results))
	for _, result := range results {
		resultByProvider[result.Provider] = result
	}

	if moonshot, ok := resultByProvider["moonshot"]; !ok {
		t.Fatal("moonshot result missing")
	} else if moonshot.Error != nil || len(moonshot.Models) != 1 || moonshot.Models[0] != "moonshot-model-1" {
		t.Fatalf("moonshot result = %#v, want successful discovery", moonshot)
	}

	alibaba, ok := resultByProvider["alibaba"]
	if !ok {
		t.Fatal("alibaba result missing")
	}
	if alibaba.Error == nil {
		t.Fatal("alibaba error = nil, want provider error")
	}
	if !strings.Contains(*alibaba.Error, "alibaba model discovery: status 500") {
		t.Fatalf("alibaba error = %q, want status 500 detail", *alibaba.Error)
	}
	if len(alibaba.Models) != 0 {
		t.Fatalf("alibaba models = %#v, want empty on failure", alibaba.Models)
	}
}

func TestProviderModelDiscoveryHTTPStatus(t *testing.T) {
	status, ok := providerModelDiscoveryHTTPStatus(fmt.Errorf("status 502 body=boom"))
	if !ok || status != 502 {
		t.Fatalf("status = %d, ok = %v, want 502/true", status, ok)
	}
}

func TestIsFireworksTextModel(t *testing.T) {
	t.Run("excludes obvious non text models", func(t *testing.T) {
		item := fireworksModelListItem{
			Name:        "fireworks/whisper-v3",
			DisplayName: "Whisper",
			Description: "Speech to text model",
		}
		if isFireworksTextModel(item) {
			t.Fatal("expected whisper model to be excluded")
		}
	})

	t.Run("keeps instruct text models", func(t *testing.T) {
		item := fireworksModelListItem{
			Name:        "fireworks/glm-5",
			DisplayName: "GLM-5 Instruct",
			Description: "LLM text model",
		}
		if !isFireworksTextModel(item) {
			t.Fatal("expected glm-5 instruct model to be treated as text")
		}
	})
}

func TestFireworksModelID(t *testing.T) {
	got := fireworksModelID("accounts/fireworks/models/fireworks/glm-5")
	if got != "fireworks/glm-5" {
		t.Fatalf("model id = %q, want %q", got, "fireworks/glm-5")
	}
}

func TestFireworksSupportsServerless(t *testing.T) {
	t.Run("accepts explicit serverless models", func(t *testing.T) {
		item := fireworksModelListItem{Name: "fireworks/glm-5", SupportsServerless: true}
		if !fireworksSupportsServerless(item) {
			t.Fatal("expected explicit serverless model to be accepted")
		}
	})

	t.Run("accepts public models when serverless flag is absent", func(t *testing.T) {
		item := fireworksModelListItem{Name: "accounts/fireworks/models/fireworks/glm-5", Public: true}
		if !fireworksSupportsServerless(item) {
			t.Fatal("expected public fireworks model to be accepted")
		}
	})

	t.Run("rejects non-public models without serverless flag", func(t *testing.T) {
		item := fireworksModelListItem{Name: "accounts/fireworks/models/private/model-1"}
		if fireworksSupportsServerless(item) {
			t.Fatal("expected non-public model without serverless flag to be rejected")
		}
	})
}

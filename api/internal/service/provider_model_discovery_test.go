package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
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
				if got := r.Header.Get("Authorization"); got != "Bearer "+c.apiKey {
					t.Fatalf("authorization = %q, want %q", got, "Bearer "+c.apiKey)
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

func TestProviderModelDiscoveryDiscoverAllSkipsMissingKeysAndReturnsConfiguredProviders(t *testing.T) {
	moonshotKey := "test-moonshot-key"
	poeKey := "test-poe-key"
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
			switch r.Header.Get("Authorization") {
			case "Bearer " + moonshotKey:
				_ = json.NewEncoder(w).Encode(struct {
					Data []struct {
						ID string `json:"id"`
					} `json:"data"`
				}{Data: []struct {
					ID string `json:"id"`
				}{{ID: "moonshot-model-1"}}})
			case "Bearer " + poeKey:
				_ = json.NewEncoder(w).Encode(struct {
					Data []struct {
						ID string `json:"id"`
					} `json:"data"`
				}{Data: []struct {
					ID string `json:"id"`
				}{{ID: "poe-model-1"}}})
			default:
				t.Fatalf("unexpected authorization for /v1/models: %q", r.Header.Get("Authorization"))
			}
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
	t.Setenv("ZAI_API_KEY", "test-zai-key")
	t.Setenv("ZAI_API_BASE_URL", server.URL+"/api/paas/v4/chat/completions")

	svc := NewProviderModelDiscoveryService()
	svc.http = server.Client()
	results, err := svc.DiscoverAll(context.Background())
	if err != nil {
		t.Fatalf("DiscoverAll failed: %v", err)
	}
	if len(results) != 4 {
		t.Fatalf("providers count = %d, want 4", len(results))
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
	}
	want := []string{"alibaba", "moonshot", "poe", "zai"}
	slices.Sort(got)
	slices.Sort(want)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("providers = %#v, want %#v", got, want)
		}
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

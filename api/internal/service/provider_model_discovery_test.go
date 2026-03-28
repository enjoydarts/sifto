package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
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

			t.Setenv(c.baseKey, server.URL+c.baseURL)
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
			_ = json.NewEncoder(w).Encode(struct {
				Data []struct {
					ID string `json:"id"`
				} `json:"data"`
			}{Data: []struct {
				ID string `json:"id"`
			}{{ID: "moonshot-model-1"}}})
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
	t.Setenv("ALIBABA_API_KEY", "test-alibaba-key")
	t.Setenv("ALIBABA_API_BASE_URL", server.URL+"/compatible-mode/v1/chat/completions")
	t.Setenv("MOONSHOT_API_KEY", "test-moonshot-key")
	t.Setenv("MOONSHOT_API_BASE_URL", server.URL+"/v1/chat/completions")
	t.Setenv("ZAI_API_KEY", "test-zai-key")
	t.Setenv("ZAI_API_BASE_URL", server.URL+"/api/paas/v4/chat/completions")

	svc := NewProviderModelDiscoveryService()
	svc.http = server.Client()
	results, err := svc.DiscoverAll(context.Background())
	if err != nil {
		t.Fatalf("DiscoverAll failed: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("providers count = %d, want 3", len(results))
	}
	want := []string{"alibaba", "moonshot", "zai"}
	for i := range want {
		if results[i].Provider != want[i] {
			t.Fatalf("results[%d].Provider = %q, want %q", i, results[i].Provider, want[i])
		}
	}
	if got := results[0].Models[0]; got != "alibaba-model-1" {
		t.Fatalf("alibaba model = %q, want %q", got, "alibaba-model-1")
	}
	if got := results[1].Models[0]; got != "moonshot-model-1" {
		t.Fatalf("moonshot model = %q, want %q", got, "moonshot-model-1")
	}
	if got := results[2].Models[0]; got != "zai-model-1" {
		t.Fatalf("zai model = %q, want %q", got, "zai-model-1")
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

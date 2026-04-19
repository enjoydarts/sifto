package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFeatherlessCatalogServiceFetchModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %s, want /v1/models", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer featherless-test-key" {
			t.Fatalf("authorization = %q, want %q", got, "Bearer featherless-test-key")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"meta-llama/llama-3.3-70b","name":"Llama 3.3 70B","model_class":"text","context_length":131072,"max_completion_tokens":8192,"is_gated":true,"available_on_current_plan":false}]}`))
	}))
	defer server.Close()

	t.Setenv("FEATHERLESS_API_BASE_URL", server.URL+"/v1")
	svc := NewFeatherlessCatalogService()
	svc.http = server.Client()

	models, err := svc.FetchModels(context.Background(), "featherless-test-key")
	if err != nil {
		t.Fatalf("FetchModels() error = %v", err)
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

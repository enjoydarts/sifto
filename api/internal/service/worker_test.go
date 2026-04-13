package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func workerTestStringPtr(v string) *string { return &v }

func TestSelectOpenAICompatibleKeyPrefersProviderSpecificKey(t *testing.T) {
	togetherKey := workerTestStringPtr("together-key")
	moonshotKey := workerTestStringPtr("moonshot-key")
	openRouterKey := workerTestStringPtr("openrouter-key")
	poeKey := workerTestStringPtr("poe-key")
	siliconFlowKey := workerTestStringPtr("siliconflow-key")
	minimaxKey := workerTestStringPtr("minimax-key")
	openAIKey := workerTestStringPtr("openai-key")

	tests := []struct {
		name  string
		model *string
		want  *string
	}{
		{name: "together", model: workerTestStringPtr("together::openai/gpt-oss-20b"), want: togetherKey},
		{name: "moonshot", model: workerTestStringPtr("kimi-k2-turbo-preview"), want: moonshotKey},
		{name: "openrouter", model: workerTestStringPtr("openrouter::openai/gpt-5.4-mini"), want: openRouterKey},
		{name: "poe", model: workerTestStringPtr("poe::claude-sonnet-4"), want: poeKey},
		{name: "siliconflow", model: workerTestStringPtr("siliconflow::Qwen/Qwen3-Next-80B-A3B-Instruct"), want: siliconFlowKey},
		{name: "minimax", model: workerTestStringPtr("MiniMax-M2.5"), want: minimaxKey},
		{name: "openai fallback", model: workerTestStringPtr("gpt-5.4-mini"), want: openAIKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectOpenAICompatibleKey(tt.model, togetherKey, moonshotKey, openRouterKey, poeKey, siliconFlowKey, minimaxKey, openAIKey)
			if got == nil || tt.want == nil || *got != *tt.want {
				t.Fatalf("got %v, want %v", workerTestDerefString(got), workerTestDerefString(tt.want))
			}
		})
	}
}

func TestWorkerHeadersUsesMinimaxHeaderForMiniMaxModels(t *testing.T) {
	headers := workerHeadersForModel(
		workerTestStringPtr("MiniMax-M2.5"),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		workerTestStringPtr("minimax-key"),
		nil,
		nil,
		nil,
		"",
	)
	if got := headers["X-Minimax-Api-Key"]; got != "minimax-key" {
		t.Fatalf("X-Minimax-Api-Key = %q, want %q", got, "minimax-key")
	}
	if _, ok := headers["X-Openai-Api-Key"]; ok {
		t.Fatalf("X-Openai-Api-Key should not be set for MiniMax models")
	}
}

func TestExtractFactsWithModelUsesMinimaxHeader(t *testing.T) {
	var gotMinimax string
	var gotOpenAI string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMinimax = r.Header.Get("X-Minimax-Api-Key")
		gotOpenAI = r.Header.Get("X-Openai-Api-Key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"facts":["ok"]}`))
	}))
	defer server.Close()

	client := &WorkerClient{
		baseURL: server.URL,
		http:    server.Client(),
	}
	model := "MiniMax-M2.5"
	key := "minimax-key"

	if _, err := client.ExtractFactsWithModel(context.Background(), nil, "content", nil, nil, nil, nil, nil, nil, nil, nil, nil, &key, &model, nil); err != nil {
		t.Fatalf("ExtractFactsWithModel: %v", err)
	}
	if gotMinimax != "minimax-key" {
		t.Fatalf("X-Minimax-Api-Key = %q, want %q", gotMinimax, "minimax-key")
	}
	if gotOpenAI != "" {
		t.Fatalf("X-Openai-Api-Key = %q, want empty", gotOpenAI)
	}
}

func workerTestDerefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

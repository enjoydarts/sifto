package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAITranslateTextsToJAGPT62(t *testing.T) {
	for _, model := range []string{"gpt-6.2-astra", "gpt-6.2-sol"} {
		t.Run(model, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Model        string `json:"model"`
					Instructions string `json:"instructions"`
					Input        []struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"input"`
					Reasoning struct {
						Effort string `json:"effort"`
					} `json:"reasoning"`
				}
				if r.URL.Path != "/v1/responses" {
					t.Errorf("path = %q, want /v1/responses", r.URL.Path)
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decode request: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if body.Model != model || body.Reasoning.Effort != "low" {
					t.Errorf("model = %q, reasoning effort = %q", body.Model, body.Reasoning.Effort)
				}
				if body.Instructions != "" || len(body.Input) != 2 || body.Input[0].Role != "developer" ||
					body.Input[0].Content == "" || body.Input[1].Role != "user" || body.Input[1].Content == "" {
					t.Errorf("unexpected instructions or input: %+v", body)
				}
				_, _ = io.WriteString(w, `{"output_text":"{\"items\":[{\"id\":\"one\",\"translated\":\"説明\"}]}"}`)
			}))
			defer server.Close()

			client := &OpenAIClient{baseURL: server.URL, http: server.Client()}
			got, err := client.TranslateTextsToJA(context.Background(), "test-key", model, map[string]string{"one": "Description"})
			if err != nil || got["one"] != "説明" {
				t.Fatalf("TranslateTextsToJA = %v, %v", got, err)
			}
		})
	}
}

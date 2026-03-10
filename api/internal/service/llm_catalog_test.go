package service

import "testing"

func TestLLMCatalogIncludesExpectedModels(t *testing.T) {
	catalog := LLMCatalogData()
	if catalog == nil {
		t.Fatal("catalog is nil")
	}
	if got := findModelCatalog("gpt-5.4-pro"); got == nil {
		t.Fatal("gpt-5.4-pro not found in catalog")
	}
	if got := findModelCatalog("deepseek-chat"); got == nil {
		t.Fatal("deepseek-chat not found in catalog")
	}
	if got := findModelCatalog("text-embedding-3-small"); got == nil {
		t.Fatal("text-embedding-3-small not found in catalog")
	}
}

func TestCatalogProviderAndDefaults(t *testing.T) {
	tests := []struct {
		model    string
		provider string
	}{
		{model: "claude-sonnet-4-6", provider: "anthropic"},
		{model: "gemini-2.5-flash", provider: "google"},
		{model: "openai/gpt-oss-20b", provider: "groq"},
		{model: "deepseek-chat", provider: "deepseek"},
		{model: "gpt-5-mini", provider: "openai"},
	}
	for _, tt := range tests {
		if got := CatalogProviderForModel(tt.model); got != tt.provider {
			t.Fatalf("CatalogProviderForModel(%q) = %q, want %q", tt.model, got, tt.provider)
		}
	}

	defaults := []struct {
		provider string
		purpose  string
		want     string
	}{
		{provider: "openai", purpose: "digest", want: "gpt-5.4"},
		{provider: "openai", purpose: "facts", want: "gpt-5-mini"},
		{provider: "deepseek", purpose: "summary", want: "deepseek-reasoner"},
		{provider: "groq", purpose: "ask", want: "openai/gpt-oss-20b"},
	}
	for _, tt := range defaults {
		if got := DefaultLLMModelForPurpose(tt.provider, tt.purpose); got != tt.want {
			t.Fatalf("DefaultLLMModelForPurpose(%q, %q) = %q, want %q", tt.provider, tt.purpose, got, tt.want)
		}
	}
}

func TestLLMCatalogCommentsAreFilled(t *testing.T) {
	catalog := LLMCatalogData()
	for _, item := range catalog.ChatModels {
		if item.Comment == "" {
			t.Fatalf("chat model %q has empty comment", item.ID)
		}
	}
	for _, item := range catalog.EmbeddingModels {
		if item.Comment == "" {
			t.Fatalf("embedding model %q has empty comment", item.ID)
		}
	}
}

func TestLLMCatalogPricingIsFilled(t *testing.T) {
	catalog := LLMCatalogData()
	for _, item := range catalog.ChatModels {
		if item.Pricing == nil {
			t.Fatalf("chat model %q has nil pricing", item.ID)
		}
	}
	for _, item := range catalog.EmbeddingModels {
		if item.Pricing == nil {
			t.Fatalf("embedding model %q has nil pricing", item.ID)
		}
	}
}

func TestLLMCatalogProvidersAndCapabilitiesAreFilled(t *testing.T) {
	catalog := LLMCatalogData()
	for _, provider := range catalog.Providers {
		if provider.APIKeyHeader == "" {
			t.Fatalf("provider %q has empty api_key_header", provider.ID)
		}
	}
	for _, item := range catalog.ChatModels {
		if item.Capabilities == nil {
			t.Fatalf("chat model %q has nil capabilities", item.ID)
		}
	}
}

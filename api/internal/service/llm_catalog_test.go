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
		{model: "qwen3.5-plus", provider: "alibaba"},
		{model: "mistral-small-2506", provider: "mistral"},
		{model: "grok-4-fast-non-reasoning", provider: "xai"},
		{model: "glm-4.7-flash", provider: "zai"},
		{model: "gpt-5-mini", provider: "openai"},
		{model: "openrouter::openai/gpt-oss-120b", provider: "openrouter"},
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
		{provider: "openai", purpose: "digest", want: "gpt-5"},
		{provider: "openai", purpose: "facts", want: "gpt-5-mini"},
		{provider: "deepseek", purpose: "summary", want: "deepseek-chat"},
		{provider: "groq", purpose: "ask", want: "openai/gpt-oss-20b"},
		{provider: "google", purpose: "facts", want: "gemini-2.5-flash-lite"},
		{provider: "alibaba", purpose: "source_suggestion", want: "qwen3.5-flash"},
		{provider: "mistral", purpose: "digest", want: "mistral-medium-2508"},
		{provider: "xai", purpose: "facts", want: "grok-4-fast-non-reasoning"},
		{provider: "zai", purpose: "ask", want: "glm-5-turbo"},
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

func TestCatalogModelSupportsPurpose(t *testing.T) {
	tests := []struct {
		model   string
		purpose string
		want    bool
	}{
		{model: "gpt-5-mini", purpose: "summary", want: true},
		{model: "gpt-5-mini", purpose: "source_suggestion", want: true},
		{model: "text-embedding-3-small", purpose: "summary", want: false},
		{model: "text-embedding-3-small", purpose: "embedding", want: true},
		{model: "does-not-exist", purpose: "summary", want: false},
	}
	for _, tt := range tests {
		if got := CatalogModelSupportsPurpose(tt.model, tt.purpose); got != tt.want {
			t.Fatalf("CatalogModelSupportsPurpose(%q, %q) = %v, want %v", tt.model, tt.purpose, got, tt.want)
		}
	}
}

func TestCatalogIsEmbeddingModel(t *testing.T) {
	if !CatalogIsEmbeddingModel("text-embedding-3-small") {
		t.Fatal("text-embedding-3-small should be recognized as embedding model")
	}
	if CatalogIsEmbeddingModel("gpt-5-mini") {
		t.Fatal("gpt-5-mini should not be recognized as embedding model")
	}
}

func TestCatalogModelSupportsCapability(t *testing.T) {
	if !CatalogModelSupportsCapability("gpt-5-mini", "structured_output") {
		t.Fatal("gpt-5-mini should support structured_output")
	}
	if CatalogModelSupportsCapability("text-embedding-3-small", "structured_output") {
		t.Fatal("text-embedding-3-small should not support structured_output")
	}
	if CatalogModelSupportsCapability("does-not-exist", "structured_output") {
		t.Fatal("unknown model should not support structured_output")
	}
}

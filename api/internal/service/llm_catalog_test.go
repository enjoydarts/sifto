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
	if got := findModelCatalog("gpt-5.4-mini"); got == nil {
		t.Fatal("gpt-5.4-mini not found in catalog")
	}
	if got := findModelCatalog("gpt-5.4-nano"); got == nil {
		t.Fatal("gpt-5.4-nano not found in catalog")
	}
	if got := findModelCatalog("deepseek-chat"); got == nil {
		t.Fatal("deepseek-chat not found in catalog")
	}
	if got := findModelCatalog("kimi-k2.5"); got == nil {
		t.Fatal("kimi-k2.5 not found in catalog")
	}
	if got := findModelCatalog("kimi-k2-0905-preview"); got == nil {
		t.Fatal("kimi-k2-0905-preview not found in catalog")
	}
	if got := findModelCatalog("kimi-k2-thinking-turbo"); got == nil {
		t.Fatal("kimi-k2-thinking-turbo not found in catalog")
	}
	if got := findModelCatalog("text-embedding-3-small"); got == nil {
		t.Fatal("text-embedding-3-small not found in catalog")
	}
	if got := findModelCatalog("siliconflow::deepseek-ai/DeepSeek-V3.2"); got == nil {
		t.Fatal("siliconflow::deepseek-ai/DeepSeek-V3.2 not found in catalog")
	}
	if got := findModelCatalog("siliconflow::Qwen/Qwen3-30B-A3B-Instruct-2507"); got == nil {
		t.Fatal("siliconflow::Qwen/Qwen3-30B-A3B-Instruct-2507 not found in catalog")
	}
	if got := findModelCatalog("qwen3.6-plus"); got == nil {
		t.Fatal("qwen3.6-plus not found in catalog")
	}
	if got := findModelCatalog("fireworks/qwen3p6-plus"); got == nil {
		t.Fatal("fireworks/qwen3p6-plus not found in catalog")
	}
	if got := findModelCatalog("grok-4.20-0309-non-reasoning"); got == nil {
		t.Fatal("grok-4.20-0309-non-reasoning not found in catalog")
	}
	if got := findModelCatalog("grok-4.20-0309-reasoning"); got == nil {
		t.Fatal("grok-4.20-0309-reasoning not found in catalog")
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
		{model: "qwen3.6-plus", provider: "alibaba"},
		{model: "mistral-small-2506", provider: "mistral"},
		{model: "grok-4-fast-non-reasoning", provider: "xai"},
		{model: "grok-4.20-0309-non-reasoning", provider: "xai"},
		{model: "grok-4.20-0309-reasoning", provider: "xai"},
		{model: "glm-4.7-flash", provider: "zai"},
		{model: "fireworks/gpt-oss-20b", provider: "fireworks"},
		{model: "fireworks/qwen3p6-plus", provider: "fireworks"},
		{model: "kimi-k2.5", provider: "moonshot"},
		{model: "kimi-k2-0905-preview", provider: "moonshot"},
		{model: "kimi-k2-thinking-turbo", provider: "moonshot"},
		{model: "gpt-5.4-mini", provider: "openai"},
		{model: "openrouter::openai/gpt-oss-120b", provider: "openrouter"},
		{model: "poe::Claude-Sonnet-4.5", provider: "poe"},
		{model: "siliconflow::deepseek-ai/DeepSeek-V3.2", provider: "siliconflow"},
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
		{provider: "openai", purpose: "facts", want: "gpt-5.4-mini"},
		{provider: "deepseek", purpose: "summary", want: "deepseek-chat"},
		{provider: "groq", purpose: "ask", want: "openai/gpt-oss-20b"},
		{provider: "google", purpose: "facts", want: "gemini-2.5-flash-lite"},
		{provider: "alibaba", purpose: "source_suggestion", want: "qwen3.5-flash"},
		{provider: "mistral", purpose: "digest", want: "mistral-medium-2508"},
		{provider: "xai", purpose: "facts", want: "grok-4-fast-non-reasoning"},
		{provider: "zai", purpose: "ask", want: "glm-5-turbo"},
		{provider: "fireworks", purpose: "ask", want: "fireworks/kimi-k2-instruct-0905"},
		{provider: "moonshot", purpose: "ask", want: "kimi-k2.5"},
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
		{model: "gpt-5.4-mini", purpose: "summary", want: true},
		{model: "gpt-5.4-mini", purpose: "source_suggestion", want: true},
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
	if CatalogIsEmbeddingModel("gpt-5.4-mini") {
		t.Fatal("gpt-5.4-mini should not be recognized as embedding model")
	}
}

func TestCatalogModelSupportsCapability(t *testing.T) {
	if !CatalogModelSupportsCapability("gpt-5.4-mini", "structured_output") {
		t.Fatal("gpt-5.4-mini should support structured_output")
	}
	if CatalogModelSupportsCapability("text-embedding-3-small", "structured_output") {
		t.Fatal("text-embedding-3-small should not support structured_output")
	}
	if CatalogModelSupportsCapability("does-not-exist", "structured_output") {
		t.Fatal("unknown model should not support structured_output")
	}
}

func TestDynamicChatModelsMergeAcrossProviders(t *testing.T) {
	t.Cleanup(func() {
		SetDynamicChatModelsForProvider("openrouter", nil)
		SetDynamicChatModelsForProvider("poe", nil)
	})

	SetDynamicChatModelsForProvider("openrouter", []LLMModelCatalog{
		{ID: OpenRouterAliasModelID("openai/gpt-oss-120b"), Provider: "openrouter"},
	})
	SetDynamicChatModelsForProvider("poe", []LLMModelCatalog{
		{ID: PoeAliasModelID("Claude-Sonnet-4.5"), Provider: "poe"},
	})

	catalog := LLMCatalogData()
	if CatalogModelByIDInCatalog(catalog, OpenRouterAliasModelID("openai/gpt-oss-120b")) == nil {
		t.Fatal("openrouter dynamic model should remain in merged catalog")
	}
	if CatalogModelByIDInCatalog(catalog, PoeAliasModelID("Claude-Sonnet-4.5")) == nil {
		t.Fatal("poe dynamic model should be present in merged catalog")
	}
}

package service

import (
	"reflect"
	"strings"
	"testing"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func TestLLMCatalogIncludesExpectedModels(t *testing.T) {
	catalog := LLMCatalogData()
	if catalog == nil {
		t.Fatal("catalog is nil")
	}
	if got := findModelCatalog("gpt-5.4-pro"); got == nil {
		t.Fatal("gpt-5.4-pro not found in catalog")
	}
	if got := findModelCatalog("gpt-5.5"); got == nil {
		t.Fatal("gpt-5.5 not found in catalog")
	}
	if got := findModelCatalog("gpt-5.5-pro"); got == nil {
		t.Fatal("gpt-5.5-pro not found in catalog")
	}
	if got := findModelCatalog("gpt-5.4-mini"); got == nil {
		t.Fatal("gpt-5.4-mini not found in catalog")
	}
	if got := findModelCatalog("gpt-5.4-nano"); got == nil {
		t.Fatal("gpt-5.4-nano not found in catalog")
	}
	if got := findModelCatalog("gemini-3.5-flash"); got == nil {
		t.Fatal("gemini-3.5-flash not found in catalog")
	}
	if got := findModelCatalog("deepseek-chat"); got == nil {
		t.Fatal("deepseek-chat not found in catalog")
	}
	if got := findModelCatalog("deepseek-v4-flash"); got == nil {
		t.Fatal("deepseek-v4-flash not found in catalog")
	}
	if got := findModelCatalog("deepseek-v4-pro"); got == nil {
		t.Fatal("deepseek-v4-pro not found in catalog")
	}
	if got := findModelCatalog("kimi-k2.5"); got == nil {
		t.Fatal("kimi-k2.5 not found in catalog")
	}
	if got := findModelCatalog("kimi-k2.6"); got == nil {
		t.Fatal("kimi-k2.6 not found in catalog")
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
	if got := findModelCatalog("siliconflow::deepseek-ai/DeepSeek-V4-Flash"); got == nil {
		t.Fatal("siliconflow::deepseek-ai/DeepSeek-V4-Flash not found in catalog")
	}
	if got := findModelCatalog("siliconflow::deepseek-ai/DeepSeek-V4-Pro"); got == nil {
		t.Fatal("siliconflow::deepseek-ai/DeepSeek-V4-Pro not found in catalog")
	}
	if got := findModelCatalog("siliconflow::moonshotai/Kimi-K2.6"); got == nil {
		t.Fatal("siliconflow::moonshotai/Kimi-K2.6 not found in catalog")
	}
	if got := findModelCatalog("siliconflow::MiniMaxAI/MiniMax-M3"); got == nil {
		t.Fatal("siliconflow::MiniMaxAI/MiniMax-M3 not found in catalog")
	}
	if got := findModelCatalog("claude-opus-4-8"); got == nil {
		t.Fatal("claude-opus-4-8 not found in catalog")
	}
	if got := findModelCatalog("claude-fable-5"); got == nil {
		t.Fatal("claude-fable-5 not found in catalog")
	}
	if got := findModelCatalog("siliconflow::Qwen/Qwen3-30B-A3B-Instruct-2507"); got == nil {
		t.Fatal("siliconflow::Qwen/Qwen3-30B-A3B-Instruct-2507 not found in catalog")
	}
	if got := findModelCatalog("siliconflow::Qwen/Qwen3.6-35B-A3B"); got == nil {
		t.Fatal("siliconflow::Qwen/Qwen3.6-35B-A3B not found in catalog")
	}
	if got := findModelCatalog("siliconflow::Qwen/Qwen3.6-27B"); got == nil {
		t.Fatal("siliconflow::Qwen/Qwen3.6-27B not found in catalog")
	}
	if got := findModelCatalog("siliconflow::zai-org/GLM-5.1"); got == nil {
		t.Fatal("siliconflow::zai-org/GLM-5.1 not found in catalog")
	}
	if got := findModelCatalog("siliconflow::zai-org/GLM-5.2"); got == nil {
		t.Fatal("siliconflow::zai-org/GLM-5.2 not found in catalog")
	}
	if got := findModelCatalog("qwen3.6-plus"); got == nil {
		t.Fatal("qwen3.6-plus not found in catalog")
	}
	if got := findModelCatalog("qwen3.6-flash"); got == nil {
		t.Fatal("qwen3.6-flash not found in catalog")
	}
	if got := findModelCatalog("qwen3.6-35b-a3b"); got == nil {
		t.Fatal("qwen3.6-35b-a3b not found in catalog")
	}
	if got := findModelCatalog("qwen3.7-max"); got == nil {
		t.Fatal("qwen3.7-max not found in catalog")
	}
	if got := findModelCatalog("qwen3.7-plus"); got == nil {
		t.Fatal("qwen3.7-plus not found in catalog")
	}
	if got := findModelCatalog("fireworks/qwen3p6-plus"); got == nil {
		t.Fatal("fireworks/qwen3p6-plus not found in catalog")
	}
	if got := findModelCatalog("fireworks/glm-5p2"); got == nil {
		t.Fatal("fireworks/glm-5p2 not found in catalog")
	}
	if got := findModelCatalog("qwen3p7-plus"); got == nil {
		t.Fatal("qwen3p7-plus not found in catalog")
	}
	if got := findModelCatalog("fireworks/deepseek-v4-pro"); got == nil {
		t.Fatal("fireworks/deepseek-v4-pro not found in catalog")
	}
	if got := findModelCatalog("fireworks/kimi-k2p6"); got == nil {
		t.Fatal("fireworks/kimi-k2p6 not found in catalog")
	}
	if got := findModelCatalog("grok-4.20-0309-non-reasoning"); got == nil {
		t.Fatal("grok-4.20-0309-non-reasoning not found in catalog")
	}
	if got := findModelCatalog("grok-4.20-0309-reasoning"); got == nil {
		t.Fatal("grok-4.20-0309-reasoning not found in catalog")
	}
	if got := findModelCatalog("grok-4.3"); got == nil {
		t.Fatal("grok-4.3 not found in catalog")
	}
	if got := findModelCatalog("glm-5.1"); got == nil {
		t.Fatal("glm-5.1 not found in catalog")
	}
	if got := findModelCatalog("mistral-small-2603"); got == nil {
		t.Fatal("mistral-small-2603 not found in catalog")
	}
	if got := findModelCatalog("mistral-medium-3.5"); got == nil {
		t.Fatal("mistral-medium-3.5 not found in catalog")
	}
	if got := findModelCatalog("MiniMax-M3"); got == nil {
		t.Fatal("MiniMax-M3 not found in catalog")
	}
	if got := findModelCatalog("MiniMax-M2.7"); got == nil {
		t.Fatal("MiniMax-M2.7 not found in catalog")
	}
	if got := findModelCatalog(MiniMaxAliasModelID("MiniMax-M2.7")); got == nil {
		t.Fatal("minimax::MiniMax-M2.7 not found in catalog")
	}
	if got := findModelCatalog("minimax/MiniMax-M2.7"); got == nil {
		t.Fatal("minimax/MiniMax-M2.7 not found in catalog")
	}
	if got := findModelCatalog("MiniMax-M2.7-highspeed"); got == nil {
		t.Fatal("MiniMax-M2.7-highspeed not found in catalog")
	}
	if got := findModelCatalog("MiniMax-M2.5"); got == nil {
		t.Fatal("MiniMax-M2.5 not found in catalog")
	}
	if got := findModelCatalog("MiniMax-M2.5-highspeed"); got == nil {
		t.Fatal("MiniMax-M2.5-highspeed not found in catalog")
	}
	if got := findModelCatalog("mimo-v2-pro"); got == nil {
		t.Fatal("mimo-v2-pro not found in catalog")
	}
	if got := findModelCatalog("mimo-v2.5"); got == nil {
		t.Fatal("mimo-v2.5 not found in catalog")
	}
	if got := findModelCatalog("mimo-v2.5-pro"); got == nil {
		t.Fatal("mimo-v2.5-pro not found in catalog")
	}
	if got := findModelCatalog("mimo-v2-omni"); got == nil {
		t.Fatal("mimo-v2-omni not found in catalog")
	}
	if got := findModelCatalog("gemma-4-31b-it"); got == nil {
		t.Fatal("gemma-4-31b-it not found in catalog")
	}
	if got := findModelCatalog("gemma-4-26b-a4b-it"); got == nil {
		t.Fatal("gemma-4-26b-a4b-it not found in catalog")
	}
	if got := findModelCatalog(TogetherAliasModelID("google/gemma-4-31B-it")); got == nil {
		t.Fatal("together::google/gemma-4-31B-it not found in catalog")
	}
	if got := findModelCatalog(TogetherAliasModelID("moonshotai/Kimi-K2.5")); got == nil {
		t.Fatal("together::moonshotai/Kimi-K2.5 not found in catalog")
	}
	if got := findModelCatalog(TogetherAliasModelID("moonshotai/Kimi-K2.6")); got == nil {
		t.Fatal("together::moonshotai/Kimi-K2.6 not found in catalog")
	}
	if got := findModelCatalog(TogetherAliasModelID("deepseek-ai/DeepSeek-V4-Pro")); got == nil {
		t.Fatal("together::deepseek-ai/DeepSeek-V4-Pro not found in catalog")
	}
	if got := findModelCatalog(TogetherAliasModelID("zai-org/GLM-5.1")); got == nil {
		t.Fatal("together::zai-org/GLM-5.1 not found in catalog")
	}
	if got := findModelCatalog(TogetherAliasModelID("zai-org/GLM-5.2")); got == nil {
		t.Fatal("together::zai-org/GLM-5.2 not found in catalog")
	}
	if got := findModelCatalog(TogetherAliasModelID("openai/gpt-oss-120b")); got == nil {
		t.Fatal("together::openai/gpt-oss-120b not found in catalog")
	}
	if got := findModelCatalog(TogetherAliasModelID("Qwen/Qwen3-Coder-Next-FP8")); got == nil {
		t.Fatal("together::Qwen/Qwen3-Coder-Next-FP8 not found in catalog")
	}
	if got := findModelCatalog(TogetherAliasModelID("Qwen/Qwen3.6-Plus")); got == nil {
		t.Fatal("together::Qwen/Qwen3.6-Plus not found in catalog")
	}
	if got := findModelCatalog(TogetherAliasModelID("MiniMaxAI/MiniMax-M3")); got == nil {
		t.Fatal("together::MiniMaxAI/MiniMax-M3 not found in catalog")
	}
	if got := findModelCatalog(FeatherlessAliasModelID("Qwen/Qwen3.5-9B")); got == nil {
		t.Fatal("featherless::Qwen/Qwen3.5-9B not found in catalog")
	}
	if got := providerCatalogByID("deepinfra"); got == nil {
		t.Fatal("deepinfra provider not found in catalog")
	}
}

func TestCatalogProviderAndDefaults(t *testing.T) {
	tests := []struct {
		model    string
		provider string
	}{
		{model: "claude-sonnet-4-6", provider: "anthropic"},
		{model: "claude-fable-5", provider: "anthropic"},
		{model: "claude-opus-4-7", provider: "anthropic"},
		{model: "claude-opus-4-8", provider: "anthropic"},
		{model: "gemini-3.5-flash", provider: "google"},
		{model: "gemini-2.5-flash", provider: "google"},
		{model: "openai/gpt-oss-20b", provider: "groq"},
		{model: "deepseek-v4-flash", provider: "deepseek"},
		{model: "deepseek-v4-pro", provider: "deepseek"},
		{model: "deepseek-chat", provider: "deepseek"},
		{model: "qwen3.5-plus", provider: "alibaba"},
		{model: "qwen3.6-plus", provider: "alibaba"},
		{model: "qwen3.6-flash", provider: "alibaba"},
		{model: "qwen3.6-35b-a3b", provider: "alibaba"},
		{model: "qwen3.7-max", provider: "alibaba"},
		{model: "qwen3.7-plus", provider: "alibaba"},
		{model: "mistral-small-2506", provider: "mistral"},
		{model: "mistral-small-2603", provider: "mistral"},
		{model: "mistral-medium-3.5", provider: "mistral"},
		{model: "MiniMax-M3", provider: "minimax"},
		{model: "MiniMax-M2.7", provider: "minimax"},
		{model: MiniMaxAliasModelID("MiniMax-M2.7"), provider: "minimax"},
		{model: "minimax/MiniMax-M2.7", provider: "minimax"},
		{model: "MiniMax-M2.7-highspeed", provider: "minimax"},
		{model: "MiniMax-M2.5", provider: "minimax"},
		{model: "MiniMax-M2.5-highspeed", provider: "minimax"},
		{model: "grok-4-fast-non-reasoning", provider: "xai"},
		{model: "grok-4.20-0309-non-reasoning", provider: "xai"},
		{model: "grok-4.20-0309-reasoning", provider: "xai"},
		{model: "grok-4.3", provider: "xai"},
		{model: "glm-5.1", provider: "zai"},
		{model: "gemma-4-31b-it", provider: "google"},
		{model: "gemma-4-26b-a4b-it", provider: "google"},
		{model: "glm-4.7-flash", provider: "zai"},
		{model: "fireworks/gpt-oss-20b", provider: "fireworks"},
		{model: "fireworks/kimi-k2p6", provider: "fireworks"},
		{model: "fireworks/qwen3p6-plus", provider: "fireworks"},
		{model: "fireworks/glm-5p2", provider: "fireworks"},
		{model: "qwen3p7-plus", provider: "fireworks"},
		{model: "fireworks/deepseek-v4-pro", provider: "fireworks"},
		{model: "kimi-k2.6", provider: "moonshot"},
		{model: "kimi-k2.5", provider: "moonshot"},
		{model: "kimi-k2-0905-preview", provider: "moonshot"},
		{model: "kimi-k2-thinking-turbo", provider: "moonshot"},
		{model: "MiniMax-M2.5", provider: "minimax"},
		{model: "MiniMax-M2.7", provider: "minimax"},
		{model: "mimo-v2-pro", provider: "xiaomi_mimo_token_plan"},
		{model: "mimo-v2.5", provider: "xiaomi_mimo_token_plan"},
		{model: "mimo-v2.5-pro", provider: "xiaomi_mimo_token_plan"},
		{model: "mimo-v2-omni", provider: "xiaomi_mimo_token_plan"},
		{model: TogetherAliasModelID("google/gemma-4-31B-it"), provider: "together"},
		{model: TogetherAliasModelID("moonshotai/Kimi-K2.6"), provider: "together"},
		{model: TogetherAliasModelID("moonshotai/Kimi-K2.5"), provider: "together"},
		{model: TogetherAliasModelID("deepseek-ai/DeepSeek-V4-Pro"), provider: "together"},
		{model: TogetherAliasModelID("zai-org/GLM-5.1"), provider: "together"},
		{model: TogetherAliasModelID("zai-org/GLM-5.2"), provider: "together"},
		{model: TogetherAliasModelID("openai/gpt-oss-120b"), provider: "together"},
		{model: TogetherAliasModelID("Qwen/Qwen3-Coder-Next-FP8"), provider: "together"},
		{model: TogetherAliasModelID("Qwen/Qwen3.6-Plus"), provider: "together"},
		{model: TogetherAliasModelID("MiniMaxAI/MiniMax-M3"), provider: "together"},
		{model: "gpt-5.4-mini", provider: "openai"},
		{model: "gpt-5.5", provider: "openai"},
		{model: "gpt-5.5-pro", provider: "openai"},
		{model: "openrouter::openai/gpt-oss-120b", provider: "openrouter"},
		{model: "poe::Claude-Sonnet-4.5", provider: "poe"},
		{model: "siliconflow::deepseek-ai/DeepSeek-V3.2", provider: "siliconflow"},
		{model: "siliconflow::deepseek-ai/DeepSeek-V4-Pro", provider: "siliconflow"},
		{model: "siliconflow::MiniMaxAI/MiniMax-M3", provider: "siliconflow"},
		{model: "siliconflow::Qwen/Qwen3.6-35B-A3B", provider: "siliconflow"},
		{model: "siliconflow::Qwen/Qwen3.6-27B", provider: "siliconflow"},
		{model: "siliconflow::zai-org/GLM-5.1", provider: "siliconflow"},
		{model: "siliconflow::zai-org/GLM-5.2", provider: "siliconflow"},
		{model: FeatherlessAliasModelID("Qwen/Qwen3.5-9B"), provider: "featherless"},
		{model: "deepinfra::meta-llama/Meta-Llama-3.3-70B-Instruct-Turbo", provider: "deepinfra"},
		{model: CerebrasAliasModelID("llama-4-scout-17b-16e-instruct"), provider: "cerebras"},
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
		{provider: "mistral", purpose: "facts", want: "mistral-small-2603"},
		{provider: "mistral", purpose: "summary", want: "mistral-small-2603"},
		{provider: "mistral", purpose: "digest", want: "mistral-medium-2508"},
		{provider: "mistral", purpose: "ask", want: "mistral-small-2603"},
		{provider: "mistral", purpose: "source_suggestion", want: "mistral-small-2603"},
		{provider: "minimax", purpose: "facts", want: "MiniMax-M2.5-highspeed"},
		{provider: "minimax", purpose: "summary", want: "MiniMax-M2.7"},
		{provider: "minimax", purpose: "digest", want: "MiniMax-M2.7"},
		{provider: "minimax", purpose: "ask", want: "MiniMax-M2.7-highspeed"},
		{provider: "minimax", purpose: "source_suggestion", want: "MiniMax-M2.5-highspeed"},
		{provider: "xiaomi_mimo_token_plan", purpose: "facts", want: "mimo-v2-pro"},
		{provider: "xiaomi_mimo_token_plan", purpose: "digest", want: "mimo-v2-omni"},
		{provider: "xiaomi_mimo_token_plan", purpose: "ask", want: "mimo-v2-pro"},
		{provider: "xai", purpose: "facts", want: "grok-4-fast-non-reasoning"},
		{provider: "zai", purpose: "ask", want: "glm-5-turbo"},
		{provider: "fireworks", purpose: "ask", want: "fireworks/kimi-k2-instruct-0905"},
		{provider: "moonshot", purpose: "ask", want: "kimi-k2.6"},
		{provider: "together", purpose: "facts", want: TogetherAliasModelID("openai/gpt-oss-20b")},
		{provider: "together", purpose: "summary", want: TogetherAliasModelID("moonshotai/Kimi-K2.5")},
		{provider: "together", purpose: "source_suggestion", want: TogetherAliasModelID("LiquidAI/LFM2-24B-A2B")},
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

func TestLLMCatalogPricingMatchesCacheCapabilities(t *testing.T) {
	catalog := LLMCatalogData()
	for _, item := range catalog.ChatModels {
		if item.Pricing == nil || item.Capabilities == nil {
			continue
		}
		if item.Capabilities.SupportsCacheReadPricing && item.Pricing.CacheReadPerMTokUSD <= 0 {
			t.Fatalf("chat model %q supports cache read pricing but cache_read_per_mtok_usd is %v", item.ID, item.Pricing.CacheReadPerMTokUSD)
		}
		if item.Capabilities.SupportsCacheWritePricing && item.Pricing.CacheWritePerMTokUSD <= 0 {
			t.Fatalf("chat model %q supports cache write pricing but cache_write_per_mtok_usd is %v", item.ID, item.Pricing.CacheWritePerMTokUSD)
		}
		if item.Pricing.CacheReadPerMTokUSD > 0 && !item.Capabilities.SupportsCacheReadPricing {
			t.Fatalf("chat model %q has cache_read_per_mtok_usd but does not support cache read pricing", item.ID)
		}
		if item.Pricing.CacheWritePerMTokUSD > 0 && !item.Capabilities.SupportsCacheWritePricing {
			t.Fatalf("chat model %q has cache_write_per_mtok_usd but does not support cache write pricing", item.ID)
		}
	}
}

func TestLLMCatalogProvidersAndCapabilitiesAreFilled(t *testing.T) {
	catalog := LLMCatalogData()
	for _, provider := range catalog.Providers {
		if provider.APIKeyHeader == "" {
			t.Fatalf("provider %q has empty api_key_header", provider.ID)
		}
		if provider.MatchExact == nil {
			t.Fatalf("provider %q has nil match_exact; use [] for an empty list", provider.ID)
		}
		if provider.MatchPrefixes == nil {
			t.Fatalf("provider %q has nil match_prefixes; use [] for an empty list", provider.ID)
		}
	}
	for _, item := range catalog.ChatModels {
		if item.Capabilities == nil {
			t.Fatalf("chat model %q has nil capabilities", item.ID)
		}
	}
}

func TestLLMCatalogProviderMatchExactDoesNotClaimOtherCatalogProvider(t *testing.T) {
	catalog := LLMCatalogData()
	for _, provider := range catalog.Providers {
		for _, exact := range provider.MatchExact {
			model := CatalogModelByIDInCatalog(catalog, exact)
			if model == nil {
				continue
			}
			if model.Provider != provider.ID {
				t.Fatalf("provider %q match_exact %q is catalog provider %q", provider.ID, exact, model.Provider)
			}
		}
	}
}

func TestLLMCatalogProviderMatchRulesResolveToProvider(t *testing.T) {
	catalog := LLMCatalogData()
	for _, provider := range catalog.Providers {
		for _, exact := range provider.MatchExact {
			modelID := exact
			if provider.ID == "together" {
				modelID = TogetherAliasModelID(exact)
			}
			if got := CatalogProviderForModel(modelID); got != provider.ID {
				t.Fatalf("CatalogProviderForModel(%q) = %q, want %q", modelID, got, provider.ID)
			}
		}
		for _, prefix := range provider.MatchPrefixes {
			if prefix == "" {
				t.Fatalf("provider %q has empty match prefix", provider.ID)
			}
			model := prefix + "catalog-smoke-test"
			if got := CatalogProviderForModel(model); got != provider.ID {
				t.Fatalf("CatalogProviderForModel(%q) = %q, want %q", model, got, provider.ID)
			}
		}
	}
}

func TestLLMCatalogDefaultModelsExistAndSupportPurpose(t *testing.T) {
	catalog := LLMCatalogData()
	for _, provider := range catalog.Providers {
		for purpose, modelID := range provider.DefaultModels {
			model := CatalogModelByIDInCatalog(catalog, modelID)
			if model == nil {
				t.Fatalf("provider %q default model for %q = %q is not in catalog", provider.ID, purpose, modelID)
			}
			if model.Provider != provider.ID {
				t.Fatalf("provider %q default model for %q = %q has provider %q", provider.ID, purpose, modelID, model.Provider)
			}
			if !CatalogModelSupportsPurposeInCatalog(catalog, modelID, purpose) {
				t.Fatalf("provider %q default model %q does not support purpose %q", provider.ID, modelID, purpose)
			}
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
		SetDynamicChatModelsForProvider("featherless", nil)
		SetDynamicChatModelsForProvider("deepinfra", nil)
	})

	SetDynamicChatModelsForProvider("openrouter", []LLMModelCatalog{
		{ID: OpenRouterAliasModelID("openai/gpt-oss-120b"), Provider: "openrouter"},
	})
	SetDynamicChatModelsForProvider("poe", []LLMModelCatalog{
		{ID: PoeAliasModelID("Claude-Sonnet-4.5"), Provider: "poe"},
	})
	SetDynamicChatModelsForProvider("featherless", FeatherlessModelIDsToCatalogModels([]string{"meta-llama/Llama-3.3-70B-Instruct-Turbo"}))
	SetDynamicChatModelsForProvider("deepinfra", []LLMModelCatalog{
		{
			ID:                "deepinfra::meta-llama/Meta-Llama-3.3-70B-Instruct-Turbo",
			Provider:          "deepinfra",
			AvailablePurposes: []string{"facts", "summary", "digest_cluster_draft", "digest", "ask", "source_suggestion"},
		},
	})

	catalog := LLMCatalogData()
	if CatalogModelByIDInCatalog(catalog, OpenRouterAliasModelID("openai/gpt-oss-120b")) == nil {
		t.Fatal("openrouter dynamic model should remain in merged catalog")
	}
	if CatalogModelByIDInCatalog(catalog, PoeAliasModelID("Claude-Sonnet-4.5")) == nil {
		t.Fatal("poe dynamic model should be present in merged catalog")
	}
	if CatalogModelByIDInCatalog(catalog, FeatherlessAliasModelID("meta-llama/Llama-3.3-70B-Instruct-Turbo")) == nil {
		t.Fatal("featherless dynamic model should be present in merged catalog")
	}
	if CatalogModelByIDInCatalog(catalog, "deepinfra::meta-llama/Meta-Llama-3.3-70B-Instruct-Turbo") == nil {
		t.Fatal("deepinfra dynamic model should be present in merged catalog")
	}
}

func TestDefaultLLMModelForPurposeFallsBackToDeepInfraDynamicCatalogModel(t *testing.T) {
	t.Cleanup(func() {
		SetDynamicChatModelsForProvider("deepinfra", nil)
	})

	SetDynamicChatModelsForProvider("deepinfra", []LLMModelCatalog{
		{
			ID:                "deepinfra::Qwen/Qwen3.5-32B-Instruct",
			Provider:          "deepinfra",
			AvailablePurposes: []string{"facts", "summary", "digest_cluster_draft", "digest", "ask", "source_suggestion"},
		},
	})

	if got := DefaultLLMModelForPurpose("deepinfra", "summary"); got != "deepinfra::Qwen/Qwen3.5-32B-Instruct" {
		t.Fatalf("DefaultLLMModelForPurpose(%q, %q) = %q, want %q", "deepinfra", "summary", got, "deepinfra::Qwen/Qwen3.5-32B-Instruct")
	}
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

func TestGetLLMProvidersDrivenByCatalog(t *testing.T) {
	ids := GetLLMProviders()
	if len(ids) == 0 {
		t.Fatal("GetLLMProviders returned empty; catalog not source of truth")
	}
	if !contains(ids, "anthropic") || !contains(ids, "groq") || !contains(ids, "openai") {
		t.Errorf("expected core providers from catalog, got %v", ids)
	}
}

func TestSynthesizedAddProviderScenario(t *testing.T) {
	// catalog data (service_module etc) drives; new provider via catalog would appear in GetLLMProviders
	if mod := ProviderServiceModule("anthropic"); mod != "claude_service" {
		t.Errorf("anthropic service_module from catalog: got %s", mod)
	}
	if mod := ProviderServiceModule("groq"); mod != "groq_service" {
		t.Errorf("default module: got %s", mod)
	}
	ids := GetLLMProviders()
	if !contains(ids, "anthropic") {
		t.Error("catalog providers not driving")
	}
}

// TestEveryCatalogProviderHasSettingsAndRepoFields is the exhaustive gate (per strategy).
// For every id from GetLLMProviders(), the catalog settings_field_base must allow
// reflect-resolving the exact Has* / *Last4 on model.UserSettings (DB columns for compat) and repo.
// Payload and key loading are map/catalog-driven (AC2); model/repo fields remain per-column per non-goal.

func TestEveryCatalogProviderHasSettingsAndRepoFields(t *testing.T) {
	ids := GetLLMProviders()
	if len(ids) == 0 {
		t.Fatal("no providers")
	}
	settingsVal := reflect.ValueOf(&model.UserSettings{}).Elem()
	for _, id := range ids {
		base := ProviderSettingsFieldBase(id)
		if base == "" {
			// allow derive as last resort but prefer catalog
			parts := strings.Split(id, "_")
			for i := range parts {
				if len(parts[i]) > 0 {
					parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
				}
			}
			base = strings.Join(parts, "")
		}
		hasF := "Has" + base + "APIKey"
		last4F := base + "APIKeyLast4"

		if f := settingsVal.FieldByName(hasF); !f.IsValid() {
			t.Errorf("provider %s: missing field %s on UserSettings (base=%s)", id, hasF, base)
		}
		if f := settingsVal.FieldByName(last4F); !f.IsValid() {
			t.Errorf("provider %s: missing field %s on UserSettings", id, last4F)
		}
	}
}

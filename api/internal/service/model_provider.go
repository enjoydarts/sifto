package service

import "strings"

var costEfficientProviderPriority = []string{"groq", "zai", "fireworks", "together", "moonshot", "alibaba", "google", "mistral", "xai", "deepseek", "siliconflow", "openrouter", "openai", "anthropic"}

func isModelByProvider(model *string, provider string) bool {
	if model == nil {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(*model))
	if v == "" {
		return false
	}
	return CatalogProviderForModel(v) == provider
}

func IsGeminiModel(model *string) bool   { return isModelByProvider(model, "google") }
func IsGroqModel(model *string) bool     { return isModelByProvider(model, "groq") }
func IsDeepSeekModel(model *string) bool { return isModelByProvider(model, "deepseek") }
func IsAlibabaModel(model *string) bool  { return isModelByProvider(model, "alibaba") }
func IsMistralModel(model *string) bool  { return isModelByProvider(model, "mistral") }
func IsXAIModel(model *string) bool      { return isModelByProvider(model, "xai") }
func IsOpenAIModel(model *string) bool   { return isModelByProvider(model, "openai") }

func LLMProviderForModel(model *string) string {
	if model == nil {
		return "anthropic"
	}
	if provider := CatalogProviderForModel(strings.TrimSpace(*model)); provider != "" {
		return provider
	}
	return "anthropic"
}

func DefaultLLMModelForPurpose(provider, purpose string) string {
	if v := CatalogDefaultModelForPurpose(provider, purpose); v != "" {
		return v
	}
	if v := CatalogDefaultModelForPurpose("anthropic", purpose); v != "" {
		return v
	}
	return "claude-sonnet-4-6"
}

func CostEfficientLLMProviders(exclude string) []string {
	out := make([]string, 0, len(costEfficientProviderPriority))
	for _, provider := range costEfficientProviderPriority {
		if provider == exclude {
			continue
		}
		out = append(out, provider)
	}
	return out
}

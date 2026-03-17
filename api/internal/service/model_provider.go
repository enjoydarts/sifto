package service

import "strings"

var costEfficientProviderPriority = []string{"groq", "zai", "alibaba", "google", "mistral", "xai", "deepseek", "openai", "anthropic"}

func IsGeminiModel(model *string) bool {
	if model == nil {
		return false
	}
	return CatalogProviderForModel(strings.TrimSpace(*model)) == "google"
}

func IsGroqModel(model *string) bool {
	if model == nil {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(*model))
	if v == "" {
		return false
	}
	return CatalogProviderForModel(v) == "groq"
}

func IsDeepSeekModel(model *string) bool {
	if model == nil {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(*model))
	if v == "" {
		return false
	}
	return CatalogProviderForModel(v) == "deepseek"
}

func IsAlibabaModel(model *string) bool {
	if model == nil {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(*model))
	if v == "" {
		return false
	}
	return CatalogProviderForModel(v) == "alibaba"
}

func IsMistralModel(model *string) bool {
	if model == nil {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(*model))
	if v == "" {
		return false
	}
	return CatalogProviderForModel(v) == "mistral"
}

func IsXAIModel(model *string) bool {
	if model == nil {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(*model))
	if v == "" {
		return false
	}
	return CatalogProviderForModel(v) == "xai"
}

func IsOpenAIModel(model *string) bool {
	if model == nil {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(*model))
	if v == "" {
		return false
	}
	return CatalogProviderForModel(v) == "openai"
}

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

package service

import "strings"

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

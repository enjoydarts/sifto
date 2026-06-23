package service

import "strings"

// costEfficientProviderPriority is empty/deprecated. Real provider list + order comes exclusively
// from GetLLMProviders() (catalog json order). No long provider enumeration here.
var costEfficientProviderPriority []string

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
func IsMiniMaxModel(model *string) bool  { return isModelByProvider(model, "minimax") }
func IsXAIModel(model *string) bool      { return isModelByProvider(model, "xai") }
func IsOpenAIModel(model *string) bool   { return isModelByProvider(model, "openai") }

func LLMProviderForModel(model *string) string {
	if model == nil {
		if p := GetLLMProviders(); len(p) > 0 {
			return p[0]
		}
		return "openai"
	}
	if provider := CatalogProviderForModel(strings.TrimSpace(*model)); provider != "" {
		return provider
	}
	if p := GetLLMProviders(); len(p) > 0 {
		return p[0]
	}
	return "openai"
}

func DefaultLLMModelForPurpose(provider, purpose string) string {
	if v := CatalogDefaultModelForPurpose(provider, purpose); v != "" {
		return v
	}
	if catalog := LLMCatalogData(); catalog != nil {
		for _, item := range catalog.ChatModels {
			if strings.TrimSpace(item.Provider) != strings.TrimSpace(provider) {
				continue
			}
			for _, availablePurpose := range item.AvailablePurposes {
				if strings.TrimSpace(availablePurpose) == strings.TrimSpace(purpose) {
					return item.ID
				}
			}
		}
	}
	if ps := GetLLMProviders(); len(ps) > 0 {
		if v := CatalogDefaultModelForPurpose(ps[0], purpose); v != "" {
			return v
		}
	}
	return "claude-sonnet-4-6"
}

func CostEfficientLLMProviders(exclude string) []string {
	// exclusively from catalog via GetLLMProviders(); no static provider list
	ids := GetLLMProviders()
	out := make([]string, 0, len(ids))
	for _, p := range ids {
		if p == exclude {
			continue
		}
		out = append(out, p)
	}
	return out
}

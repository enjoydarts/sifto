package service

import "strings"

func NormalizeCatalogPricedUsage(purpose string, usage *LLMUsage) *LLMUsage {
	if usage == nil {
		return nil
	}
	modelID := strings.TrimSpace(usage.Model)
	if strings.TrimSpace(usage.Provider) == "openrouter" && strings.TrimSpace(usage.ResolvedModel) != "" {
		modelID = OpenRouterAliasModelID(strings.TrimSpace(usage.ResolvedModel))
	}
	if modelID == "" {
		return usage
	}
	entry := CatalogModelByID(modelID)
	if entry == nil || entry.Pricing == nil {
		return usage
	}
	normalized := *usage
	normalized.Provider = strings.TrimSpace(entry.Provider)
	if normalized.Provider == "" {
		normalized.Provider = usage.Provider
	}
	normalized.PricingModelFamily = modelID
	normalized.PricingSource = strings.TrimSpace(entry.Pricing.PricingSource)
	nonCachedInput := normalized.InputTokens - normalized.CacheReadInputTokens
	if nonCachedInput < 0 {
		nonCachedInput = 0
	}
	estimated := 0.0
	estimated += float64(nonCachedInput) / 1_000_000 * entry.Pricing.InputPerMTokUSD
	estimated += float64(normalized.OutputTokens) / 1_000_000 * entry.Pricing.OutputPerMTokUSD
	estimated += float64(normalized.CacheReadInputTokens) / 1_000_000 * entry.Pricing.CacheReadPerMTokUSD
	estimated += float64(normalized.CacheCreationInputTokens) / 1_000_000 * entry.Pricing.CacheWritePerMTokUSD
	normalized.EstimatedCostUSD = estimated
	return &normalized
}

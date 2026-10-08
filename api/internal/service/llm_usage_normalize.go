package service

import (
	"math"
	"strings"
)

func NormalizeCatalogPricedUsage(purpose string, usage *LLMUsage) *LLMUsage {
	if usage == nil {
		return nil
	}
	if strings.TrimSpace(usage.Provider) == "openrouter" && usage.OpenRouterCostUSD != nil {
		normalized := *usage
		if resolvedModelID := OpenRouterAliasModelID(CanonicalizeOpenRouterModelID(strings.TrimSpace(usage.ResolvedModel))); resolvedModelID != "" {
			if entry := CatalogModelByID(resolvedModelID); entry != nil {
				normalized.PricingModelFamily = resolvedModelID
			}
		}
		normalized.PricingSource = "openrouter_billed"
		normalized.EstimatedCostUSD = *usage.OpenRouterCostUSD
		return &normalized
	}
	modelID := strings.TrimSpace(usage.Model)
	if strings.TrimSpace(usage.Provider) == "openrouter" {
		if resolvedModelID := OpenRouterAliasModelID(CanonicalizeOpenRouterModelID(strings.TrimSpace(usage.ResolvedModel))); resolvedModelID != "" {
			if entry := CatalogModelByID(resolvedModelID); entry != nil && entry.Pricing != nil {
				modelID = resolvedModelID
			}
		}
	}
	if modelID == "" {
		return usage
	}
	entry := CatalogModelByID(modelID)
	if entry == nil || entry.Pricing == nil {
		return usage
	}
	// A prompt-length tier applies per request, not to the total of merged calls.
	if entry.Pricing.LongContext != nil && usage.Calls > 1 {
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
	promptTokens := normalized.InputTokens + normalized.CacheCreationInputTokens
	if normalized.Provider == "anthropic" {
		// Anthropic reports uncached input, cache writes, and cache reads separately.
		nonCachedInput = normalized.InputTokens
		promptTokens += normalized.CacheReadInputTokens
	}
	if nonCachedInput < 0 {
		nonCachedInput = 0
	}
	pricing := *entry.Pricing
	if longContext := pricing.LongContext; longContext != nil && promptTokens > longContext.InputTokenThreshold {
		pricing.InputPerMTokUSD = longContext.InputPerMTokUSD
		pricing.OutputPerMTokUSD = longContext.OutputPerMTokUSD
		pricing.CacheReadPerMTokUSD = longContext.CacheReadPerMTokUSD
		pricing.CacheWritePerMTokUSD = longContext.CacheWritePerMTokUSD
	}
	estimated := 0.0
	estimated += float64(nonCachedInput) / 1_000_000 * pricing.InputPerMTokUSD
	estimated += float64(normalized.OutputTokens) / 1_000_000 * pricing.OutputPerMTokUSD
	estimated += float64(normalized.CacheReadInputTokens) / 1_000_000 * pricing.CacheReadPerMTokUSD
	estimated += float64(normalized.CacheCreationInputTokens) / 1_000_000 * pricing.CacheWritePerMTokUSD
	normalized.EstimatedCostUSD = math.Round(estimated*1e8) / 1e8
	return &normalized
}

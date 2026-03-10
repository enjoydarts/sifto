package service

import (
	"fmt"
	"strings"
)

type OpenAIEmbeddingCostEstimate struct {
	Provider           string
	Model              string
	PricingModelFamily string
	PricingSource      string
	InputTokens        int
	EstimatedCostUSD   float64
}

func SupportedOpenAIEmbeddingModels() []string {
	return CatalogSupportedEmbeddingModels("openai")
}

func IsSupportedOpenAIEmbeddingModel(model string) bool {
	for _, v := range SupportedOpenAIEmbeddingModels() {
		if v == strings.TrimSpace(model) {
			return true
		}
	}
	return false
}

func EstimateOpenAIEmbeddingCostUSD(model string, inputTokens int) (*OpenAIEmbeddingCostEstimate, error) {
	if inputTokens < 0 {
		return nil, fmt.Errorf("inputTokens must be >= 0")
	}
	entry := findModelCatalog(model)
	if entry == nil || entry.Pricing == nil {
		return nil, fmt.Errorf("unsupported openai embedding model: %s", model)
	}
	cost := (float64(inputTokens) / 1_000_000.0) * entry.Pricing.InputPerMTokUSD
	return &OpenAIEmbeddingCostEstimate{
		Provider:           "openai",
		Model:              model,
		PricingModelFamily: model,
		PricingSource:      entry.Pricing.PricingSource,
		InputTokens:        inputTokens,
		EstimatedCostUSD:   cost,
	}, nil
}

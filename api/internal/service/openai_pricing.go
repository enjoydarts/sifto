package service

import "fmt"

type OpenAIEmbeddingCostEstimate struct {
	Provider           string
	Model              string
	PricingModelFamily string
	PricingSource      string
	InputTokens        int
	EstimatedCostUSD   float64
}

const (
	openAIEmbeddingPricingSource = "openai_static_embeddings"
)

var openAIEmbeddingPricePer1MTokensUSD = map[string]float64{
	"text-embedding-3-small": 0.02,
	"text-embedding-3-large": 0.13,
}

func SupportedOpenAIEmbeddingModels() []string {
	return []string{"text-embedding-3-small", "text-embedding-3-large"}
}

func IsSupportedOpenAIEmbeddingModel(model string) bool {
	_, ok := openAIEmbeddingPricePer1MTokensUSD[model]
	return ok
}

func EstimateOpenAIEmbeddingCostUSD(model string, inputTokens int) (*OpenAIEmbeddingCostEstimate, error) {
	if inputTokens < 0 {
		return nil, fmt.Errorf("inputTokens must be >= 0")
	}
	pricePer1M, ok := openAIEmbeddingPricePer1MTokensUSD[model]
	if !ok {
		return nil, fmt.Errorf("unsupported openai embedding model: %s", model)
	}
	cost := (float64(inputTokens) / 1_000_000.0) * pricePer1M
	return &OpenAIEmbeddingCostEstimate{
		Provider:           "openai",
		Model:              model,
		PricingModelFamily: model,
		PricingSource:      openAIEmbeddingPricingSource,
		InputTokens:        inputTokens,
		EstimatedCostUSD:   cost,
	}, nil
}

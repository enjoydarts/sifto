package inngest

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/service"
)

func appPageURL(path string) string {
	base := strings.TrimSpace(os.Getenv("NEXTAUTH_URL"))
	if base == "" {
		return ""
	}
	base = strings.TrimRight(base, "/")
	path = "/" + strings.TrimLeft(strings.TrimSpace(path), "/")
	return base + path
}

func loadUserAPIKey(ctx context.Context, keyProvider *service.UserKeyProvider, userID *string, provider string) (*string, error) {
	if keyProvider == nil || userID == nil || *userID == "" {
		return nil, fmt.Errorf("user %s api key is required", provider)
	}
	key, err := keyProvider.GetAPIKey(ctx, *userID, provider)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, fmt.Errorf("user %s api key is required", provider)
	}
	return key, nil
}

func ptrStringOrNil(v *string) *string {
	if v == nil || *v == "" {
		return nil
	}
	s := *v
	return &s
}

func loadLLMKeysForModel(ctx context.Context, keyProvider *service.UserKeyProvider, userID *string, model *string, purpose string) (*llmRuntime, error) {
	provider := service.LLMProviderForModel(model)
	resolvedModel := model
	if resolvedModel == nil || strings.TrimSpace(*resolvedModel) == "" {
		if userID != nil && *userID != "" {
			for _, candidateProvider := range service.CostEfficientLLMProviders("") {
				if key, err := loadUserAPIKey(ctx, keyProvider, userID, candidateProvider); err == nil && key != nil && strings.TrimSpace(*key) != "" {
					fallback := service.DefaultLLMModelForPurpose(candidateProvider, purpose)
					return llmKeysTuple(candidateProvider, key, &fallback)
				}
			}
		}
	}
	key, err := loadUserAPIKey(ctx, keyProvider, userID, provider)
	if err != nil {
		return nil, err
	}
	return llmKeysTuple(provider, key, model)
}

func llmKeysTuple(provider string, key, model *string) (*llmRuntime, error) {
	rt := &llmRuntime{Model: model}
	switch provider {
	case "google":
		rt.GoogleKey = key
	case "groq":
		rt.GroqKey = key
	case "deepseek":
		rt.DeepSeekKey = key
	case "alibaba":
		rt.AlibabaKey = key
	case "mistral":
		rt.MistralKey = key
	case "moonshot":
		rt.OpenAIKey = key
	case "together":
		rt.OpenAIKey = key
	case "minimax":
		rt.OpenAIKey = key
	case "xiaomi_mimo_token_plan":
		rt.OpenAIKey = key
	case "xai":
		rt.XAIKey = key
	case "zai":
		rt.ZAIKey = key
	case "fireworks":
		rt.FireworksKey = key
	case "openai":
		rt.OpenAIKey = key
	case "openrouter":
		rt.OpenAIKey = key
	case "poe":
		rt.OpenAIKey = key
	case "siliconflow":
		rt.OpenAIKey = key
	default:
		rt.AnthropicKey = key
	}
	return rt, nil
}

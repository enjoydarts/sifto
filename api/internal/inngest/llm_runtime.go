package inngest

import (
	"context"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/service"
)

type llmRuntime struct {
	AnthropicKey *string
	GoogleKey    *string
	GroqKey      *string
	DeepSeekKey  *string
	AlibabaKey   *string
	MistralKey   *string
	XAIKey       *string
	ZAIKey       *string
	FireworksKey *string
	OpenAIKey    *string
	Model        *string
}

func resolveLLMRuntime(ctx context.Context, keyProvider *service.UserKeyProvider, userID *string, model *string, purpose string) (*llmRuntime, error) {
	return loadLLMKeysForModel(ctx, keyProvider, userID, model, purpose)
}

func chooseModelOverride(primary, fallback *string) *string {
	if primary != nil && strings.TrimSpace(*primary) != "" {
		return primary
	}
	return fallback
}

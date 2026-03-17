package inngest

import (
	"context"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/repository"
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

func resolveLLMRuntime(ctx context.Context, settingsRepo *repository.UserSettingsRepo, cipher *service.SecretCipher, userID *string, model *string, purpose string) (*llmRuntime, error) {
	anthropicKey, googleKey, groqKey, deepseekKey, alibabaKey, mistralKey, xaiKey, zaiKey, fireworksKey, openAIKey, resolvedModel, err := loadLLMKeysForModel(ctx, settingsRepo, cipher, userID, model, purpose)
	if err != nil {
		return nil, err
	}
	return &llmRuntime{
		AnthropicKey: anthropicKey,
		GoogleKey:    googleKey,
		GroqKey:      groqKey,
		DeepSeekKey:  deepseekKey,
		AlibabaKey:   alibabaKey,
		MistralKey:   mistralKey,
		XAIKey:       xaiKey,
		ZAIKey:       zaiKey,
		FireworksKey: fireworksKey,
		OpenAIKey:    openAIKey,
		Model:        resolvedModel,
	}, nil
}

func chooseModelOverride(primary, fallback *string) *string {
	if primary != nil && strings.TrimSpace(*primary) != "" {
		return primary
	}
	return fallback
}

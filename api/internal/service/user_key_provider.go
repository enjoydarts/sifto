package service

import (
	"context"
	"log"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

type UserKeyProvider struct {
	settingsRepo *repository.UserSettingsRepo
	cipher       *SecretCipher
	loaders      map[string]func(ctx context.Context, userID string) (*string, error)
}

func NewUserKeyProvider(settingsRepo *repository.UserSettingsRepo, cipher *SecretCipher) *UserKeyProvider {
	p := &UserKeyProvider{
		settingsRepo: settingsRepo,
		cipher:       cipher,
		loaders:      make(map[string]func(ctx context.Context, userID string) (*string, error)),
	}
	p.registerLoaders()
	return p
}

func (p *UserKeyProvider) registerLoaders() {
	if p.settingsRepo == nil {
		return
	}
	p.loaders["anthropic"] = p.settingsRepo.GetAnthropicAPIKeyEncrypted
	p.loaders["google"] = p.settingsRepo.GetGoogleAPIKeyEncrypted
	p.loaders["groq"] = p.settingsRepo.GetGroqAPIKeyEncrypted
	p.loaders["deepseek"] = p.settingsRepo.GetDeepSeekAPIKeyEncrypted
	p.loaders["alibaba"] = p.settingsRepo.GetAlibabaAPIKeyEncrypted
	p.loaders["minimax"] = p.settingsRepo.GetMiniMaxAPIKeyEncrypted
	p.loaders["xiaomi_mimo_token_plan"] = p.settingsRepo.GetXiaomiMiMoTokenPlanAPIKeyEncrypted
	p.loaders["deepinfra"] = p.settingsRepo.GetDeepInfraAPIKeyEncrypted
	p.loaders["featherless"] = p.settingsRepo.GetFeatherlessAPIKeyEncrypted
	p.loaders["mistral"] = p.settingsRepo.GetMistralAPIKeyEncrypted
	p.loaders["moonshot"] = p.settingsRepo.GetMoonshotAPIKeyEncrypted
	p.loaders["xai"] = p.settingsRepo.GetXAIAPIKeyEncrypted
	p.loaders["zai"] = p.settingsRepo.GetZAIAPIKeyEncrypted
	p.loaders["fireworks"] = p.settingsRepo.GetFireworksAPIKeyEncrypted
	p.loaders["together"] = p.settingsRepo.GetTogetherAPIKeyEncrypted
	p.loaders["openrouter"] = p.settingsRepo.GetOpenRouterAPIKeyEncrypted
	p.loaders["poe"] = p.settingsRepo.GetPoeAPIKeyEncrypted
	p.loaders["siliconflow"] = p.settingsRepo.GetSiliconFlowAPIKeyEncrypted
	p.loaders["openai"] = p.settingsRepo.GetOpenAIAPIKeyEncrypted
}

func (p *UserKeyProvider) GetAPIKey(ctx context.Context, userID, provider string) (*string, error) {
	if p.settingsRepo == nil || p.cipher == nil || !p.cipher.Enabled() {
		return nil, ErrSecretEncryptionNotConfigured
	}
	loader, ok := p.loaders[provider]
	if !ok {
		return nil, nil
	}
	enc, err := loader(ctx, userID)
	if err != nil {
		return nil, err
	}
	if enc == nil || strings.TrimSpace(*enc) == "" {
		return nil, nil
	}
	plain, err := p.cipher.DecryptString(*enc)
	if err != nil {
		return nil, err
	}
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return nil, nil
	}
	return &plain, nil
}

func (p *UserKeyProvider) GetAllKeys(ctx context.Context, userID string) map[string]*string {
	keys := make(map[string]*string, len(p.loaders))
	for provider := range p.loaders {
		key, err := p.GetAPIKey(ctx, userID, provider)
		if err != nil {
			log.Printf("failed to load key for provider %s: %v", provider, err)
		}
		keys[provider] = key
	}
	return keys
}

func (p *UserKeyProvider) ResolveOpenAIKey(keys map[string]*string, model *string) *string {
	provider := LLMProviderForModel(model)
	switch provider {
	case "openrouter", "together", "moonshot", "poe", "siliconflow", "minimax", "xiaomi_mimo_token_plan", "featherless", "deepinfra":
		return keys[provider]
	default:
		return keys["openai"]
	}
}

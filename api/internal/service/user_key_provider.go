package service

import (
	"context"
	"log"
	"reflect"
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
	// iterate over catalog providers (GetLLMProviders).
	// Use reflect to lookup repo method "Get" + base + "APIKeyEncrypted" so adding provider does not require case here (reduction per AC2).
	// Repo methods and DB columns still needed (non-goal).
	repoVal := reflect.ValueOf(p.settingsRepo)
	for _, pid := range GetLLMProviders() {
		base := ProviderSettingsFieldBase(pid)
		if base == "" {
			// fallback (for non-catalog or legacy)
			parts := strings.Split(pid, "_")
			for i := range parts {
				if len(parts[i]) > 0 {
					parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
				}
			}
			base = strings.Join(parts, "")
		}
		methName := "Get" + base + "APIKeyEncrypted"
		m := repoVal.MethodByName(methName)
		if m.IsValid() {
			p.loaders[pid] = func(ctx context.Context, userID string) (*string, error) {
				res := m.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(userID)})
				if len(res) != 2 {
					return nil, nil
				}
				if errIface := res[1].Interface(); errIface != nil {
					if err, ok := errIface.(error); ok && err != nil {
						return nil, err
					}
				}
				if res[0].IsNil() {
					return nil, nil
				}
				if s, ok := res[0].Interface().(*string); ok {
					return s, nil
				}
				return nil, nil
			}
		}
		// else: not registered (no method yet)
	}
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
	case "openrouter", "together", "moonshot", "poe", "siliconflow", "minimax", "plamo", "xiaomi_mimo_token_plan", "featherless", "deepinfra", "cerebras":
		return keys[provider]
	default:
		return keys["openai"]
	}
}

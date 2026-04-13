package handler

import (
	"context"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/service"
)

type navigatorKeys struct {
	anthropicKey *string
	googleKey    *string
	groqKey      *string
	fireworksKey *string
	deepseekKey  *string
	alibabaKey   *string
	mistralKey   *string
	xaiKey       *string
	zaiKey       *string
	minimaxKey   *string
	openAIKey    *string
}

func loadNavigatorKeys(ctx context.Context, keyProvider *service.UserKeyProvider, userID string, model *string) navigatorKeys {
	keys := keyProvider.GetAllKeys(ctx, userID)
	return navigatorKeys{
		anthropicKey: keys["anthropic"],
		googleKey:    keys["google"],
		groqKey:      keys["groq"],
		fireworksKey: keys["fireworks"],
		deepseekKey:  keys["deepseek"],
		alibabaKey:   keys["alibaba"],
		mistralKey:   keys["mistral"],
		xaiKey:       keys["xai"],
		zaiKey:       keys["zai"],
		minimaxKey:   keys["minimax"],
		openAIKey:    keyProvider.ResolveOpenAIKey(keys, model),
	}
}

func (k navigatorKeys) hasAny() bool {
	return stringPtrNonEmpty(k.anthropicKey) ||
		stringPtrNonEmpty(k.googleKey) ||
		stringPtrNonEmpty(k.groqKey) ||
		stringPtrNonEmpty(k.fireworksKey) ||
		stringPtrNonEmpty(k.deepseekKey) ||
		stringPtrNonEmpty(k.alibabaKey) ||
		stringPtrNonEmpty(k.mistralKey) ||
		stringPtrNonEmpty(k.xaiKey) ||
		stringPtrNonEmpty(k.zaiKey) ||
		stringPtrNonEmpty(k.minimaxKey) ||
		stringPtrNonEmpty(k.openAIKey)
}

func stringPtrNonEmpty(s *string) bool {
	return s != nil && strings.TrimSpace(*s) != ""
}

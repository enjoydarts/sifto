package service

import (
	"context"
	"errors"
	"testing"
)

func TestUserKeyProviderGetAPIKeyReturnsConfigErrorWhenCipherUnavailable(t *testing.T) {
	provider := NewUserKeyProvider(nil, nil)

	_, err := provider.GetAPIKey(context.Background(), "user-1", "openai")
	if !errors.Is(err, ErrSecretEncryptionNotConfigured) {
		t.Fatalf("expected ErrSecretEncryptionNotConfigured, got %v", err)
	}
}

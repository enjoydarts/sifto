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

func TestUserKeyProviderGetAPIKeyLoadsMiniMaxKey(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "user-key-provider-minimax-key")

	svc := newSettingsServiceForTest(t)
	svc.cipher = NewSecretCipher()
	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000021"

	if _, err := svc.SetAPIKey(ctx, userID, "minimax", "minimax-secret-value"); err != nil {
		t.Fatalf("SetAPIKey(minimax) error = %v", err)
	}

	provider := NewUserKeyProvider(svc.repo, svc.cipher)
	got, err := provider.GetAPIKey(ctx, userID, "minimax")
	if err != nil {
		t.Fatalf("GetAPIKey(minimax) error = %v", err)
	}
	if got == nil || *got != "minimax-secret-value" {
		t.Fatalf("GetAPIKey(minimax) = %#v, want %q", got, "minimax-secret-value")
	}
}

func TestUserKeyProviderGetAPIKeyLoadsFeatherlessKey(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "user-key-provider-featherless-key")

	svc := newSettingsServiceForTest(t)
	svc.cipher = NewSecretCipher()
	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000021"

	if _, err := svc.SetAPIKey(ctx, userID, "featherless", "featherless-secret-value"); err != nil {
		t.Fatalf("SetAPIKey(featherless) error = %v", err)
	}

	provider := NewUserKeyProvider(svc.repo, svc.cipher)
	got, err := provider.GetAPIKey(ctx, userID, "featherless")
	if err != nil {
		t.Fatalf("GetAPIKey(featherless) error = %v", err)
	}
	if got == nil || *got != "featherless-secret-value" {
		t.Fatalf("GetAPIKey(featherless) = %#v, want %q", got, "featherless-secret-value")
	}
}

func TestUserKeyProviderGetAPIKeyLoadsDeepInfraKey(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "user-key-provider-deepinfra-key")

	svc := newSettingsServiceForTest(t)
	svc.cipher = NewSecretCipher()
	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000021"

	if _, err := svc.SetAPIKey(ctx, userID, "deepinfra", "deepinfra-secret-value"); err != nil {
		t.Fatalf("SetAPIKey(deepinfra) error = %v", err)
	}

	provider := NewUserKeyProvider(svc.repo, svc.cipher)
	got, err := provider.GetAPIKey(ctx, userID, "deepinfra")
	if err != nil {
		t.Fatalf("GetAPIKey(deepinfra) error = %v", err)
	}
	if got == nil || *got != "deepinfra-secret-value" {
		t.Fatalf("GetAPIKey(deepinfra) = %#v, want %q", got, "deepinfra-secret-value")
	}
}

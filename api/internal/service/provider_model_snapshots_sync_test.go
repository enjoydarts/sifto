package service

import (
	"context"
	"testing"
)

func TestProviderModelSnapshotSyncBuildDiscoveryServiceLoadsMiniMaxKey(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "provider-model-sync-minimax-key")

	svc := newSettingsServiceForTest(t)
	svc.cipher = NewSecretCipher()
	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000021"

	if _, err := svc.SetAPIKey(ctx, userID, "minimax", "snapshot-minimax-secret"); err != nil {
		t.Fatalf("SetAPIKey(minimax) error = %v", err)
	}

	syncer := NewProviderModelSnapshotSyncService(svc.userRepo, svc.repo, nil, nil, nil, svc.cipher)
	discovery, err := syncer.buildDiscoveryService(ctx)
	if err != nil {
		t.Fatalf("buildDiscoveryService() error = %v", err)
	}
	if discovery.keys.MiniMax != "snapshot-minimax-secret" {
		t.Fatalf("MiniMax key = %q, want %q", discovery.keys.MiniMax, "snapshot-minimax-secret")
	}
}

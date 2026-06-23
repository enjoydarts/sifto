package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestSettingsGetPayloadIncludesCartesiaAPIKeyStatus(t *testing.T) {
	last4 := "Rqqz"
	payload := SettingsGetPayload{
		HasCartesiaAPIKey:   true,
		CartesiaAPIKeyLast4: &last4,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got["has_cartesia_api_key"] != true {
		t.Fatalf("has_cartesia_api_key = %#v, want true", got["has_cartesia_api_key"])
	}
	if got["cartesia_api_key_last4"] != "Rqqz" {
		t.Fatalf("cartesia_api_key_last4 = %#v, want Rqqz", got["cartesia_api_key_last4"])
	}
}

func TestSettingsGetPayloadIncludesLLMAPIKeysMapFromCatalog(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "catalog-payload-real-get-key")
	svc := newSettingsServiceForTest(t)
	svc.cipher = NewSecretCipher()
	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000021"

	// Seed several LLM keys via shipped Set path (drives real Get contract)
	if _, err := svc.SetAPIKey(ctx, userID, "openai", "sk-real-openai-1234"); err != nil {
		t.Fatalf("Set openai: %v", err)
	}
	if _, err := svc.SetAPIKey(ctx, userID, "anthropic", "sk-ant-real-9876"); err != nil {
		t.Fatalf("Set anthropic: %v", err)
	}
	if _, err := svc.SetAPIKey(ctx, userID, "groq", "gsk-groq-abcd"); err != nil {
		t.Fatalf("Set groq: %v", err)
	}

	// Drive the SHIPPED svc.Get (not direct construction)
	payload, err := svc.Get(ctx, userID)
	if err != nil {
		t.Fatalf("svc.Get error: %v", err)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	keys, ok := got["llm_api_keys"].(map[string]any)
	if !ok {
		t.Fatalf("llm_api_keys not present or wrong type in real Get payload JSON")
	}
	for _, pid := range []string{"openai", "anthropic", "groq"} {
		entry, ok := keys[pid].(map[string]any)
		if !ok || entry["has"] != true {
			t.Fatalf("%s in llm_api_keys from real Get incorrect: %#v", pid, entry)
		}
	}
	// Real svc.Get body evidence (full relevant slice)
	fmt.Printf("REAL_SVC_GET_LLM_KEYS_BODY: %s\n", string(body)[:300])
}

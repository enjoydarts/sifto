package service

import (
	"encoding/json"
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

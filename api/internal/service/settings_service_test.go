package service

import (
	"testing"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func strptr(v string) *string { return &v }

func TestValidateCatalogModelForPurpose(t *testing.T) {
	tests := []struct {
		name    string
		model   *string
		purpose string
		wantErr bool
	}{
		{name: "nil allowed", model: nil, purpose: "summary", wantErr: false},
		{name: "valid summary model", model: strptr("gpt-5.4-mini"), purpose: "summary", wantErr: false},
		{name: "invalid purpose", model: strptr("text-embedding-3-small"), purpose: "summary", wantErr: true},
		{name: "unknown model", model: strptr("unknown-model"), purpose: "summary", wantErr: true},
	}
	for _, tt := range tests {
		err := validateCatalogModelForPurpose(LLMCatalogData(), tt.model, tt.purpose)
		if (err != nil) != tt.wantErr {
			t.Fatalf("%s: validateCatalogModelForPurpose(%v, %q) err=%v, wantErr=%v", tt.name, tt.model, tt.purpose, err, tt.wantErr)
		}
	}
}

func TestLLMModelSettingsPayloadIncludesFallbackModels(t *testing.T) {
	settings := &model.UserSettings{
		FactsModel:           strptr("gpt-5.4-mini"),
		FactsFallbackModel:   strptr("google/gemini-2.5-flash"),
		SummaryModel:         strptr("gpt-5.4"),
		SummaryFallbackModel: strptr("openrouter::openai/gpt-oss-120b"),
		HasPoeAPIKey:         true,
		PoeAPIKeyLast4:       strptr("abcd"),
	}

	got := LLMModelSettingsPayload(settings)

	if gotFactsFallback, _ := got["facts_fallback"].(*string); gotFactsFallback == nil || *gotFactsFallback != "google/gemini-2.5-flash" {
		t.Fatalf("facts_fallback = %v, want %q", got["facts_fallback"], "google/gemini-2.5-flash")
	}
	if gotSummaryFallback, _ := got["summary_fallback"].(*string); gotSummaryFallback == nil || *gotSummaryFallback != "openrouter::openai/gpt-oss-120b" {
		t.Fatalf("summary_fallback = %v, want %q", got["summary_fallback"], "openrouter::openai/gpt-oss-120b")
	}
}

func TestSettingsGetPayloadSupportsPoeFields(t *testing.T) {
	payload := &SettingsGetPayload{
		HasPoeAPIKey:   true,
		PoeAPIKeyLast4: strptr("abcd"),
	}

	if !payload.HasPoeAPIKey {
		t.Fatal("HasPoeAPIKey should be true")
	}
	if payload.PoeAPIKeyLast4 == nil || *payload.PoeAPIKeyLast4 != "abcd" {
		t.Fatalf("PoeAPIKeyLast4 = %v, want %q", payload.PoeAPIKeyLast4, "abcd")
	}
}

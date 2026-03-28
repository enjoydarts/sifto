package service

import (
	"encoding/json"
	"testing"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
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
		FactsModel:                       strptr("gpt-5.4-mini"),
		FactsFallbackModel:               strptr("google/gemini-2.5-flash"),
		SummaryModel:                     strptr("gpt-5.4"),
		SummaryFallbackModel:             strptr("openrouter::openai/gpt-oss-120b"),
		NavigatorPersonaMode:             PersonaModeRandom,
		NavigatorPersona:                 "editor",
		AINavigatorBriefModel:            strptr("kimi-k2.5"),
		AINavigatorBriefFallbackModel:    strptr("google/gemini-2.5-flash"),
		AudioBriefingScriptModel:         strptr("gpt-5.4"),
		AudioBriefingScriptFallbackModel: strptr("google/gemini-2.5-flash"),
		HasPoeAPIKey:                     true,
		PoeAPIKeyLast4:                   strptr("abcd"),
	}

	got := LLMModelSettingsPayload(settings)

	if gotFactsFallback, _ := got["facts_fallback"].(*string); gotFactsFallback == nil || *gotFactsFallback != "google/gemini-2.5-flash" {
		t.Fatalf("facts_fallback = %v, want %q", got["facts_fallback"], "google/gemini-2.5-flash")
	}
	if gotSummaryFallback, _ := got["summary_fallback"].(*string); gotSummaryFallback == nil || *gotSummaryFallback != "openrouter::openai/gpt-oss-120b" {
		t.Fatalf("summary_fallback = %v, want %q", got["summary_fallback"], "openrouter::openai/gpt-oss-120b")
	}
	if gotAudioBriefingScript, _ := got["audio_briefing_script"].(*string); gotAudioBriefingScript == nil || *gotAudioBriefingScript != "gpt-5.4" {
		t.Fatalf("audio_briefing_script = %v, want %q", got["audio_briefing_script"], "gpt-5.4")
	}
	if gotAudioBriefingScriptFallback, _ := got["audio_briefing_script_fallback"].(*string); gotAudioBriefingScriptFallback == nil || *gotAudioBriefingScriptFallback != "google/gemini-2.5-flash" {
		t.Fatalf("audio_briefing_script_fallback = %v, want %q", got["audio_briefing_script_fallback"], "google/gemini-2.5-flash")
	}
	if gotNavigatorPersonaMode, _ := got["navigator_persona_mode"].(string); gotNavigatorPersonaMode != PersonaModeRandom {
		t.Fatalf("navigator_persona_mode = %v, want %q", got["navigator_persona_mode"], PersonaModeRandom)
	}
	if gotBriefModel, _ := got["ai_navigator_brief"].(*string); gotBriefModel == nil || *gotBriefModel != "kimi-k2.5" {
		t.Fatalf("ai_navigator_brief = %v, want %q", got["ai_navigator_brief"], "kimi-k2.5")
	}
	if gotBriefFallback, _ := got["ai_navigator_brief_fallback"].(*string); gotBriefFallback == nil || *gotBriefFallback != "google/gemini-2.5-flash" {
		t.Fatalf("ai_navigator_brief_fallback = %v, want %q", got["ai_navigator_brief_fallback"], "google/gemini-2.5-flash")
	}
}

func TestResolveAINavigatorBriefModelUsesBriefSpecificOverrides(t *testing.T) {
	settings := &model.UserSettings{
		NavigatorModel:                strptr("gpt-5.4"),
		NavigatorFallbackModel:        strptr("google/gemini-2.5-flash"),
		AINavigatorBriefModel:         strptr("kimi-k2.5"),
		AINavigatorBriefFallbackModel: strptr("google/gemini-2.5-flash"),
		HasOpenAIAPIKey:               true,
		HasGoogleAPIKey:               true,
		HasMoonshotAPIKey:             true,
	}

	got := resolveAINavigatorBriefModel(settings)
	if got == nil || *got != "kimi-k2.5" {
		t.Fatalf("resolveAINavigatorBriefModel(...) = %v, want %q", got, "kimi-k2.5")
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

func TestSettingsGetPayloadSupportsMoonshotFields(t *testing.T) {
	payload := &SettingsGetPayload{
		HasMoonshotAPIKey:   true,
		MoonshotAPIKeyLast4: strptr("k25x"),
	}

	if !payload.HasMoonshotAPIKey {
		t.Fatal("HasMoonshotAPIKey should be true")
	}
	if payload.MoonshotAPIKeyLast4 == nil || *payload.MoonshotAPIKeyLast4 != "k25x" {
		t.Fatalf("MoonshotAPIKeyLast4 = %v, want %q", payload.MoonshotAPIKeyLast4, "k25x")
	}
}

func TestSettingsGetPayloadSupportsAivisFields(t *testing.T) {
	payload := &SettingsGetPayload{
		HasAivisAPIKey:          true,
		AivisAPIKeyLast4:        strptr("wxyz"),
		AivisUserDictionaryUUID: strptr("5b6f7aa3-2c34-4ad7-aad0-4e1d683d7861"),
	}

	if !payload.HasAivisAPIKey {
		t.Fatal("HasAivisAPIKey should be true")
	}
	if payload.AivisAPIKeyLast4 == nil || *payload.AivisAPIKeyLast4 != "wxyz" {
		t.Fatalf("AivisAPIKeyLast4 = %v, want %q", payload.AivisAPIKeyLast4, "wxyz")
	}
	if payload.AivisUserDictionaryUUID == nil || *payload.AivisUserDictionaryUUID != "5b6f7aa3-2c34-4ad7-aad0-4e1d683d7861" {
		t.Fatalf("AivisUserDictionaryUUID = %v, want expected uuid", payload.AivisUserDictionaryUUID)
	}
}

func TestPodcastSettingsPayloadSupportsPodcastFields(t *testing.T) {
	slug := "p_123"
	payload := PodcastSettingsPayload(&model.UserSettings{
		PodcastEnabled:     true,
		PodcastFeedSlug:    &slug,
		PodcastTitle:       strptr("Sifto Daily"),
		PodcastDescription: strptr("opening\n\noverall summary"),
		PodcastAuthor:      strptr("Sifto"),
		PodcastLanguage:    "ja",
		PodcastCategory:    strptr("Technology"),
		PodcastExplicit:    false,
		PodcastArtworkURL:  strptr("https://audio.example.com/podcasts/artwork/u1/current.jpg"),
	})

	if enabled, _ := payload["enabled"].(bool); !enabled {
		t.Fatalf("enabled = %v, want true", payload["enabled"])
	}
	if gotSlug, _ := payload["feed_slug"].(*string); gotSlug == nil || *gotSlug != "p_123" {
		t.Fatalf("feed_slug = %v, want p_123", payload["feed_slug"])
	}
	if gotLanguage, _ := payload["language"].(string); gotLanguage != "ja" {
		t.Fatalf("language = %v, want ja", payload["language"])
	}
	if gotCategory, _ := payload["category"].(*string); gotCategory == nil || *gotCategory != "Technology" {
		t.Fatalf("category = %v, want Technology", payload["category"])
	}
	if gotDefs, _ := payload["available_categories"].([]PodcastCategoryDefinition); len(gotDefs) == 0 {
		t.Fatalf("available_categories = %v, want non-empty", payload["available_categories"])
	}
}

func TestNormalizePodcastLanguage(t *testing.T) {
	if got := normalizePodcastLanguage(nil); got != "ja" {
		t.Fatalf("normalizePodcastLanguage(nil) = %q, want ja", got)
	}
	if got := normalizePodcastLanguage(strptr(" en ")); got != "en" {
		t.Fatalf("normalizePodcastLanguage(en) = %q, want en", got)
	}
}

func TestNormalizePodcastCategorySelection(t *testing.T) {
	category, subcategory, err := normalizePodcastCategorySelection(strptr("News"), strptr("Tech News"))
	if err != nil {
		t.Fatalf("normalizePodcastCategorySelection(...) error = %v", err)
	}
	if category == nil || *category != "News" {
		t.Fatalf("category = %v, want News", category)
	}
	if subcategory == nil || *subcategory != "Tech News" {
		t.Fatalf("subcategory = %v, want Tech News", subcategory)
	}
	if _, _, err := normalizePodcastCategorySelection(strptr("Technology"), strptr("Tech News")); err == nil {
		t.Fatal("expected invalid category/subcategory combination to fail")
	}
}

func TestAudioBriefingSettingsPayload(t *testing.T) {
	settings := &model.AudioBriefingSettings{
		UserID:                "u1",
		Enabled:               true,
		IntervalHours:         3,
		ArticlesPerEpisode:    6,
		TargetDurationMinutes: 20,
		DefaultPersona:        "editor",
	}

	got := AudioBriefingSettingsPayload(settings)

	if enabled, _ := got["enabled"].(bool); !enabled {
		t.Fatalf("enabled = %v, want true", got["enabled"])
	}
	if interval, _ := got["interval_hours"].(int); interval != 3 {
		t.Fatalf("interval_hours = %v, want 3", got["interval_hours"])
	}
	if persona, _ := got["default_persona"].(string); persona != "editor" {
		t.Fatalf("default_persona = %v, want editor", got["default_persona"])
	}
}

func TestAudioBriefingPersonaVoicesPayload(t *testing.T) {
	rows := []model.AudioBriefingPersonaVoice{
		{
			UserID:                  "u1",
			Persona:                 "editor",
			TTSProvider:             "aivis",
			VoiceModel:              "speaker-a",
			VoiceStyle:              "calm",
			SpeechRate:              1.1,
			EmotionalIntensity:      1.0,
			TempoDynamics:           1.0,
			LineBreakSilenceSeconds: 0.4,
		},
	}

	got := AudioBriefingPersonaVoicesPayload(rows)
	if len(got) != 1 {
		t.Fatalf("len(AudioBriefingPersonaVoicesPayload) = %d, want 1", len(got))
	}
	if got[0]["persona"] != "editor" {
		t.Fatalf("persona = %v, want editor", got[0]["persona"])
	}
	if got[0]["tts_provider"] != "aivis" {
		t.Fatalf("tts_provider = %v, want aivis", got[0]["tts_provider"])
	}
}

func TestNormalizeAudioBriefingDefaultPersona(t *testing.T) {
	if got := normalizeAudioBriefingDefaultPersona(nil); got != "editor" {
		t.Fatalf("normalizeAudioBriefingDefaultPersona(nil) = %q, want editor", got)
	}
	if got := normalizeAudioBriefingDefaultPersona(strptr(" snark ")); got != "snark" {
		t.Fatalf("normalizeAudioBriefingDefaultPersona(snark) = %q, want snark", got)
	}
	if got := normalizeAudioBriefingDefaultPersona(strptr("unknown")); got != "editor" {
		t.Fatalf("normalizeAudioBriefingDefaultPersona(unknown) = %q, want editor", got)
	}
}

func TestAudioBriefingSettingsPayloadIncludesPersonaMode(t *testing.T) {
	got := AudioBriefingSettingsPayload(&model.AudioBriefingSettings{
		Enabled:               true,
		IntervalHours:         6,
		ArticlesPerEpisode:    5,
		TargetDurationMinutes: 20,
		DefaultPersonaMode:    PersonaModeRandom,
		DefaultPersona:        "editor",
	})

	if gotMode, _ := got["default_persona_mode"].(string); gotMode != PersonaModeRandom {
		t.Fatalf("default_persona_mode = %v, want %q", got["default_persona_mode"], PersonaModeRandom)
	}
}

func TestValidateAudioBriefingPersonaVoiceInputs(t *testing.T) {
	valid, err := validateAudioBriefingPersonaVoiceInputs([]UpdateAudioBriefingPersonaVoiceInput{
		{
			Persona:                 "editor",
			TTSProvider:             "aivis",
			VoiceModel:              "speaker-a",
			VoiceStyle:              "calm",
			SpeechRate:              1.0,
			EmotionalIntensity:      1.0,
			TempoDynamics:           1.0,
			LineBreakSilenceSeconds: 0.4,
			Pitch:                   0,
			VolumeGain:              0,
		},
	})
	if err != nil {
		t.Fatalf("validateAudioBriefingPersonaVoiceInputs(valid) err=%v", err)
	}
	if len(valid) != 1 || valid[0].Persona != "editor" {
		t.Fatalf("validated rows = %#v, want editor row", valid)
	}

	t.Run("allows completely unset rows", func(t *testing.T) {
		rows, err := validateAudioBriefingPersonaVoiceInputs([]UpdateAudioBriefingPersonaVoiceInput{
			{
				Persona:                 "editor",
				TTSProvider:             "aivis",
				VoiceModel:              "",
				VoiceStyle:              "",
				SpeechRate:              1.0,
				EmotionalIntensity:      1.0,
				TempoDynamics:           1.0,
				LineBreakSilenceSeconds: 0.4,
			},
			{
				Persona:                 "analyst",
				TTSProvider:             "aivis",
				VoiceModel:              "speaker-a",
				VoiceStyle:              "speaker-uuid:1",
				SpeechRate:              1.0,
				EmotionalIntensity:      1.0,
				TempoDynamics:           1.0,
				LineBreakSilenceSeconds: 0.4,
			},
		})
		if err != nil {
			t.Fatalf("validateAudioBriefingPersonaVoiceInputs(unset row) err=%v", err)
		}
		if len(rows) != 1 || rows[0].Persona != "analyst" {
			t.Fatalf("validated rows = %#v, want only analyst row", rows)
		}
	})

	_, err = validateAudioBriefingPersonaVoiceInputs([]UpdateAudioBriefingPersonaVoiceInput{
		{
			Persona:                 "editor",
			TTSProvider:             " ",
			VoiceModel:              "speaker-a",
			VoiceStyle:              "calm",
			SpeechRate:              1.0,
			EmotionalIntensity:      1.0,
			TempoDynamics:           1.0,
			LineBreakSilenceSeconds: 0.4,
		},
	})
	if err == nil || err.Error() != "invalid tts_provider for editor" {
		t.Fatalf("missing provider err = %v, want invalid tts_provider for editor", err)
	}

	_, err = validateAudioBriefingPersonaVoiceInputs([]UpdateAudioBriefingPersonaVoiceInput{
		{
			Persona:                 "editor",
			TTSProvider:             "aivis",
			VoiceModel:              "",
			VoiceStyle:              "speaker-uuid:1",
			SpeechRate:              1.0,
			EmotionalIntensity:      1.0,
			TempoDynamics:           1.0,
			LineBreakSilenceSeconds: 0.4,
		},
	})
	if err == nil || err.Error() != "invalid voice_model for editor" {
		t.Fatalf("partial voice model err = %v, want invalid voice_model for editor", err)
	}

	_, err = validateAudioBriefingPersonaVoiceInputs([]UpdateAudioBriefingPersonaVoiceInput{
		{
			Persona:                 "editor",
			TTSProvider:             "aivis",
			VoiceModel:              "speaker-a",
			VoiceStyle:              "calm",
			SpeechRate:              1.0,
			EmotionalIntensity:      1.0,
			TempoDynamics:           1.0,
			LineBreakSilenceSeconds: 0.4,
		},
		{
			Persona:                 "editor",
			TTSProvider:             "aivis",
			VoiceModel:              "speaker-b",
			VoiceStyle:              "bright",
			SpeechRate:              1.0,
			EmotionalIntensity:      1.0,
			TempoDynamics:           1.0,
			LineBreakSilenceSeconds: 0.4,
		},
	})
	if err == nil || err.Error() != "duplicate persona voice: editor" {
		t.Fatalf("duplicate persona err = %v, want duplicate persona voice: editor", err)
	}
}

func TestParseAivisVoiceStyle(t *testing.T) {
	t.Run("colon format", func(t *testing.T) {
		got, err := parseAivisVoiceStyle("speaker-uuid:3")
		if err != nil {
			t.Fatalf("parseAivisVoiceStyle(colon) err=%v", err)
		}
		if got.SpeakerUUID != "speaker-uuid" || got.StyleID != 3 {
			t.Fatalf("parseAivisVoiceStyle(colon) = %#v", got)
		}
	})

	t.Run("json format", func(t *testing.T) {
		got, err := parseAivisVoiceStyle(`{"speaker_uuid":"speaker-json","style_id":4}`)
		if err != nil {
			t.Fatalf("parseAivisVoiceStyle(json) err=%v", err)
		}
		if got.SpeakerUUID != "speaker-json" || got.StyleID != 4 {
			t.Fatalf("parseAivisVoiceStyle(json) = %#v", got)
		}
	})
}

func TestValidateAivisVoiceSelectionAgainstSnapshots(t *testing.T) {
	speakersJSON, err := json.Marshal([]map[string]any{
		{
			"aivm_speaker_uuid": "speaker-1",
			"styles": []map[string]any{
				{"local_id": 0},
				{"local_id": 1},
			},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal(speakers) err=%v", err)
	}

	snapshots := []repository.AivisModelSnapshot{
		{
			AivmModelUUID: "model-1",
			SpeakersJSON:  speakersJSON,
		},
	}

	if err := validateAivisVoiceSelectionAgainstSnapshots(snapshots, "model-1", "speaker-1:1"); err != nil {
		t.Fatalf("validateAivisVoiceSelectionAgainstSnapshots(valid) err=%v", err)
	}
	if err := validateAivisVoiceSelectionAgainstSnapshots(snapshots, "model-x", "speaker-1:1"); err == nil {
		t.Fatal("validateAivisVoiceSelectionAgainstSnapshots should fail for unknown model")
	}
	if err := validateAivisVoiceSelectionAgainstSnapshots(snapshots, "model-1", "speaker-x:1"); err == nil {
		t.Fatal("validateAivisVoiceSelectionAgainstSnapshots should fail for unknown speaker")
	}
}

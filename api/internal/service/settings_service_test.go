package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

func strptr(v string) *string { return &v }

func newSettingsServiceForTest(t *testing.T) *SettingsService {
	t.Helper()

	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	lockSettingsServiceTestDB(t, db)

	const userID = "00000000-0000-4000-8000-000000000021"
	if _, err := db.Exec(context.Background(), `DELETE FROM audio_briefing_persona_voices WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset persona voices: %v", err)
	}
	if _, err := db.Exec(context.Background(), `DELETE FROM audio_briefing_preset_voices WHERE preset_id IN (SELECT id FROM audio_briefing_presets WHERE user_id = $1)`, userID); err != nil {
		t.Fatalf("reset preset voices: %v", err)
	}
	if _, err := db.Exec(context.Background(), `DELETE FROM audio_briefing_presets WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset presets: %v", err)
	}
	if _, err := db.Exec(context.Background(), `DELETE FROM summary_audio_voice_settings WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset summary audio voice settings: %v", err)
	}
	if _, err := db.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("reset settings service test tables: %v", err)
	}
	if _, err := db.Exec(context.Background(), `
		INSERT INTO users (id, email, name) VALUES ($1, $2, $3)
	`, userID, "settings-service@example.com", "Settings Service"); err != nil {
		t.Fatalf("insert test user: %v", err)
	}

	svc := NewSettingsService(
		repository.NewUserSettingsRepo(db),
		repository.NewUserRepo(db),
		repository.NewAudioBriefingRepo(db),
		repository.NewSummaryAudioVoiceSettingsRepo(db),
		nil,
		repository.NewObsidianExportRepo(db),
		repository.NewLLMUsageLogRepo(db),
		nil,
		nil,
		nil,
	)
	svc.SetAudioBriefingPresetRepo(repository.NewAudioBriefingPresetRepo(db))
	return svc
}

func TestAudioBriefingPresetPayloadIncludesVoices(t *testing.T) {
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	preset := model.AudioBriefingPreset{
		ID:                 "preset-1",
		UserID:             "user-1",
		Name:               "Morning Briefing",
		DefaultPersonaMode: PersonaModeRandom,
		DefaultPersona:     "editor",
		ConversationMode:   "duo",
		Voices: []model.AudioBriefingPersonaVoice{
			{
				Persona:     "editor",
				TTSProvider: "xai",
				VoiceModel:  "voice-1",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	got := AudioBriefingPresetPayload(preset)
	if got["name"] != "Morning Briefing" {
		t.Fatalf("name = %v, want Morning Briefing", got["name"])
	}
	if got["conversation_mode"] != "duo" {
		t.Fatalf("conversation_mode = %v, want duo", got["conversation_mode"])
	}
	voices, ok := got["voices"].([]map[string]any)
	if !ok || len(voices) != 1 {
		t.Fatalf("voices = %#v, want 1 voice", got["voices"])
	}
}

func TestCreateAudioBriefingPresetRejectsDuplicateName(t *testing.T) {
	svc := newSettingsServiceForTest(t)
	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000021"

	_, err := svc.CreateAudioBriefingPreset(ctx, userID, SaveAudioBriefingPresetInput{
		Name:   "morning",
		Voices: []UpdateAudioBriefingPersonaVoiceInput{},
	})
	if err != nil {
		t.Fatalf("CreateAudioBriefingPreset() error = %v", err)
	}

	_, err = svc.CreateAudioBriefingPreset(ctx, userID, SaveAudioBriefingPresetInput{
		Name:   "morning",
		Voices: []UpdateAudioBriefingPersonaVoiceInput{},
	})
	if !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("CreateAudioBriefingPreset() error = %v, want conflict", err)
	}
}

func TestCreateAudioBriefingPresetValidatesVoices(t *testing.T) {
	svc := newSettingsServiceForTest(t)
	_, err := svc.CreateAudioBriefingPreset(context.Background(), "00000000-0000-4000-8000-000000000021", SaveAudioBriefingPresetInput{
		Name: "voice-check",
		Voices: []UpdateAudioBriefingPersonaVoiceInput{
			{
				Persona:     "editor",
				TTSProvider: "openai",
				VoiceModel:  "alloy",
			},
		},
	})
	if err == nil || err.Error() != "invalid tts_model for editor" {
		t.Fatalf("CreateAudioBriefingPreset() error = %v, want invalid tts_model for editor", err)
	}
}

func lockSettingsServiceTestDB(t *testing.T, db *pgxpool.Pool) {
	t.Helper()

	const key int64 = 74231002
	if _, err := db.Exec(context.Background(), `SELECT pg_advisory_lock($1)`, key); err != nil {
		t.Fatalf("pg_advisory_lock() error = %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, key); err != nil {
			t.Fatalf("pg_advisory_unlock() error = %v", err)
		}
	})
}

func TestValidateCatalogModelForPurpose(t *testing.T) {
	tests := []struct {
		name    string
		model   *string
		purpose string
		key     string
		wantErr bool
	}{
		{name: "nil allowed", model: nil, purpose: "summary", key: "summary", wantErr: false},
		{name: "valid summary model", model: strptr("gpt-5.4-mini"), purpose: "summary", key: "summary", wantErr: false},
		{name: "google alias with models prefix", model: strptr("models/gemini-2.5-flash"), purpose: "summary", key: "summary", wantErr: false},
		{name: "latest alias", model: strptr("gpt-5.4-mini-latest"), purpose: "summary", key: "summary", wantErr: false},
		{name: "invalid purpose", model: strptr("text-embedding-3-small"), purpose: "summary", key: "faithfulness_check", wantErr: true},
		{name: "unknown model", model: strptr("unknown-model"), purpose: "summary", key: "summary_fallback", wantErr: true},
	}
	for _, tt := range tests {
		err := validateCatalogModelForPurpose(LLMCatalogData(), tt.model, tt.purpose, tt.key)
		if (err != nil) != tt.wantErr {
			t.Fatalf("%s: validateCatalogModelForPurpose(%v, %q, %q) err=%v, wantErr=%v", tt.name, tt.model, tt.purpose, tt.key, err, tt.wantErr)
		}
		if tt.wantErr && err != nil && err.Error() != "invalid model for "+tt.key {
			t.Fatalf("%s: err=%q, want %q", tt.name, err.Error(), "invalid model for "+tt.key)
		}
	}
}

func TestUpdateAudioBriefingPersonaVoicesAllowsXAI(t *testing.T) {
	svc := newSettingsServiceForTest(t)
	rows, err := svc.UpdateAudioBriefingPersonaVoices(context.Background(), "00000000-0000-4000-8000-000000000021", []UpdateAudioBriefingPersonaVoiceInput{
		{
			Persona:                 "editor",
			TTSProvider:             "xai",
			VoiceModel:              "voice-1",
			VoiceStyle:              "",
			SpeechRate:              0,
			EmotionalIntensity:      0,
			TempoDynamics:           0,
			LineBreakSilenceSeconds: 0,
			Pitch:                   0,
			VolumeGain:              0,
		},
	})
	if err != nil {
		t.Fatalf("UpdateAudioBriefingPersonaVoices() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0].Persona != "editor" {
		t.Fatalf("rows[0].Persona = %q, want editor", rows[0].Persona)
	}
	if rows[0].TTSProvider != "xai" {
		t.Fatalf("rows[0].TTSProvider = %q, want xai", rows[0].TTSProvider)
	}
	if rows[0].VoiceModel != "voice-1" {
		t.Fatalf("rows[0].VoiceModel = %q, want voice-1", rows[0].VoiceModel)
	}
	if rows[0].VoiceStyle != "" {
		t.Fatalf("rows[0].VoiceStyle = %q, want empty", rows[0].VoiceStyle)
	}
	if rows[0].SpeechRate != 0 {
		t.Fatalf("rows[0].SpeechRate = %v, want 0", rows[0].SpeechRate)
	}
	if rows[0].EmotionalIntensity != 0 {
		t.Fatalf("rows[0].EmotionalIntensity = %v, want 0", rows[0].EmotionalIntensity)
	}
	if rows[0].TempoDynamics != 0 {
		t.Fatalf("rows[0].TempoDynamics = %v, want 0", rows[0].TempoDynamics)
	}
	if rows[0].LineBreakSilenceSeconds != 0 {
		t.Fatalf("rows[0].LineBreakSilenceSeconds = %v, want 0", rows[0].LineBreakSilenceSeconds)
	}
	if rows[0].Pitch != 0 {
		t.Fatalf("rows[0].Pitch = %v, want 0", rows[0].Pitch)
	}
	if rows[0].VolumeGain != 0 {
		t.Fatalf("rows[0].VolumeGain = %v, want 0", rows[0].VolumeGain)
	}
}

func TestUpdateAudioBriefingPersonaVoicesRequiresTTSModelForOpenAI(t *testing.T) {
	svc := newSettingsServiceForTest(t)
	_, err := svc.UpdateAudioBriefingPersonaVoices(context.Background(), "00000000-0000-4000-8000-000000000021", []UpdateAudioBriefingPersonaVoiceInput{
		{
			Persona:                 "editor",
			TTSProvider:             "openai",
			TTSModel:                "",
			VoiceModel:              "alloy",
			VoiceStyle:              "",
			SpeechRate:              0,
			EmotionalIntensity:      0,
			TempoDynamics:           0,
			LineBreakSilenceSeconds: 0,
			Pitch:                   0,
			VolumeGain:              0,
		},
	})
	if err == nil || err.Error() != "invalid tts_model for editor" {
		t.Fatalf("UpdateAudioBriefingPersonaVoices() error = %v, want invalid tts_model for editor", err)
	}
}

func TestUpdateAudioBriefingPersonaVoicesAllowsGeminiTTS(t *testing.T) {
	svc := newSettingsServiceForTest(t)
	rows, err := svc.UpdateAudioBriefingPersonaVoices(context.Background(), "00000000-0000-4000-8000-000000000021", []UpdateAudioBriefingPersonaVoiceInput{
		{
			Persona:                 "editor",
			TTSProvider:             "gemini_tts",
			TTSModel:                "gemini-2.5-flash-tts",
			VoiceModel:              "Kore",
			VoiceStyle:              "",
			SpeechRate:              0,
			EmotionalIntensity:      0,
			TempoDynamics:           0,
			LineBreakSilenceSeconds: 0,
			Pitch:                   0,
			VolumeGain:              0,
		},
	})
	if err != nil {
		t.Fatalf("UpdateAudioBriefingPersonaVoices() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0].TTSProvider != "gemini_tts" {
		t.Fatalf("rows[0].TTSProvider = %q, want gemini_tts", rows[0].TTSProvider)
	}
	if rows[0].TTSModel != "gemini-2.5-flash-tts" {
		t.Fatalf("rows[0].TTSModel = %q, want gemini-2.5-flash-tts", rows[0].TTSModel)
	}
	if rows[0].VoiceModel != "Kore" {
		t.Fatalf("rows[0].VoiceModel = %q, want Kore", rows[0].VoiceModel)
	}
	if rows[0].VoiceStyle != "" {
		t.Fatalf("rows[0].VoiceStyle = %q, want empty", rows[0].VoiceStyle)
	}
}

func TestLLMModelSettingsPayloadIncludesFallbackModels(t *testing.T) {
	settings := &model.UserSettings{
		FactsModel:                       strptr("gpt-5.4-mini"),
		FactsSecondaryModel:              strptr("google/gemini-2.5-flash"),
		FactsSecondaryRatePercent:        33,
		FactsFallbackModel:               strptr("google/gemini-2.5-flash"),
		SummaryModel:                     strptr("gpt-5.4"),
		SummarySecondaryModel:            strptr("openrouter::openai/gpt-oss-120b"),
		SummarySecondaryRatePercent:      25,
		SummaryFallbackModel:             strptr("openrouter::openai/gpt-oss-120b"),
		NavigatorPersonaMode:             PersonaModeRandom,
		NavigatorPersona:                 "editor",
		AINavigatorBriefModel:            strptr("kimi-k2.5"),
		AINavigatorBriefFallbackModel:    strptr("google/gemini-2.5-flash"),
		AudioBriefingScriptModel:         strptr("gpt-5.4"),
		AudioBriefingScriptFallbackModel: strptr("google/gemini-2.5-flash"),
		TTSMarkupPreprocessModel:         strptr("gpt-5.4-mini"),
		HasPoeAPIKey:                     true,
		PoeAPIKeyLast4:                   strptr("abcd"),
	}

	got := LLMModelSettingsPayload(settings)

	if gotFactsFallback, _ := got["facts_fallback"].(*string); gotFactsFallback == nil || *gotFactsFallback != "google/gemini-2.5-flash" {
		t.Fatalf("facts_fallback = %v, want %q", got["facts_fallback"], "google/gemini-2.5-flash")
	}
	if gotFactsSecondary, _ := got["facts_secondary"].(*string); gotFactsSecondary == nil || *gotFactsSecondary != "google/gemini-2.5-flash" {
		t.Fatalf("facts_secondary = %v, want %q", got["facts_secondary"], "google/gemini-2.5-flash")
	}
	if gotFactsSecondaryRate, _ := got["facts_secondary_rate_percent"].(int); gotFactsSecondaryRate != 33 {
		t.Fatalf("facts_secondary_rate_percent = %v, want 33", got["facts_secondary_rate_percent"])
	}
	if gotSummaryFallback, _ := got["summary_fallback"].(*string); gotSummaryFallback == nil || *gotSummaryFallback != "openrouter::openai/gpt-oss-120b" {
		t.Fatalf("summary_fallback = %v, want %q", got["summary_fallback"], "openrouter::openai/gpt-oss-120b")
	}
	if gotSummarySecondary, _ := got["summary_secondary"].(*string); gotSummarySecondary == nil || *gotSummarySecondary != "openrouter::openai/gpt-oss-120b" {
		t.Fatalf("summary_secondary = %v, want %q", got["summary_secondary"], "openrouter::openai/gpt-oss-120b")
	}
	if gotSummarySecondaryRate, _ := got["summary_secondary_rate_percent"].(int); gotSummarySecondaryRate != 25 {
		t.Fatalf("summary_secondary_rate_percent = %v, want 25", got["summary_secondary_rate_percent"])
	}
	if gotAudioBriefingScript, _ := got["audio_briefing_script"].(*string); gotAudioBriefingScript == nil || *gotAudioBriefingScript != "gpt-5.4" {
		t.Fatalf("audio_briefing_script = %v, want %q", got["audio_briefing_script"], "gpt-5.4")
	}
	if gotAudioBriefingScriptFallback, _ := got["audio_briefing_script_fallback"].(*string); gotAudioBriefingScriptFallback == nil || *gotAudioBriefingScriptFallback != "google/gemini-2.5-flash" {
		t.Fatalf("audio_briefing_script_fallback = %v, want %q", got["audio_briefing_script_fallback"], "google/gemini-2.5-flash")
	}
	if gotTTSMarkupPreprocessModel, _ := got["tts_markup_preprocess_model"].(*string); gotTTSMarkupPreprocessModel == nil || *gotTTSMarkupPreprocessModel != "gpt-5.4-mini" {
		t.Fatalf("tts_markup_preprocess_model = %v, want %q", got["tts_markup_preprocess_model"], "gpt-5.4-mini")
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

func TestUpdateLLMModelsAcceptsTTSMarkupPreprocessModel(t *testing.T) {
	svc := newSettingsServiceForTest(t)
	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000021"

	settings, err := svc.UpdateLLMModels(ctx, userID, UpdateLLMModelsInput{
		TTSMarkupPreprocessModel: strptr("gpt-5.4-mini"),
	})
	if err != nil {
		t.Fatalf("UpdateLLMModels() error = %v", err)
	}
	if settings.TTSMarkupPreprocessModel == nil || *settings.TTSMarkupPreprocessModel != "gpt-5.4-mini" {
		t.Fatalf("TTSMarkupPreprocessModel = %v, want gpt-5.4-mini", settings.TTSMarkupPreprocessModel)
	}
	got := LLMModelSettingsPayload(settings)
	if gotTTSMarkupPreprocessModel, _ := got["tts_markup_preprocess_model"].(*string); gotTTSMarkupPreprocessModel == nil || *gotTTSMarkupPreprocessModel != "gpt-5.4-mini" {
		t.Fatalf("tts_markup_preprocess_model payload = %v, want gpt-5.4-mini", got["tts_markup_preprocess_model"])
	}
}

func TestSettingsGetPayloadIncludesUIFontKeys(t *testing.T) {
	payload := &SettingsGetPayload{
		UIFontSansKey:  "biz-udgothic",
		UIFontSerifKey: "biz-udmincho",
	}

	if payload.UIFontSansKey != "biz-udgothic" {
		t.Fatalf("UIFontSansKey = %q, want biz-udgothic", payload.UIFontSansKey)
	}
	if payload.UIFontSerifKey != "biz-udmincho" {
		t.Fatalf("UIFontSerifKey = %q, want biz-udmincho", payload.UIFontSerifKey)
	}
}

func TestUpdateUIFontSettingsAcceptsSelectableKeys(t *testing.T) {
	svc := newSettingsServiceForTest(t)
	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000031"

	settings, err := svc.UpdateUIFontSettings(ctx, userID, UpdateUIFontSettingsInput{
		UIFontSansKey:  "biz-udgothic",
		UIFontSerifKey: "biz-udmincho",
	})
	if err != nil {
		t.Fatalf("UpdateUIFontSettings() error = %v", err)
	}
	if settings.UIFontSansKey != "biz-udgothic" {
		t.Fatalf("UIFontSansKey = %q, want biz-udgothic", settings.UIFontSansKey)
	}
	if settings.UIFontSerifKey != "biz-udmincho" {
		t.Fatalf("UIFontSerifKey = %q, want biz-udmincho", settings.UIFontSerifKey)
	}
}

func TestUpdateUIFontSettingsRejectsDisplayKey(t *testing.T) {
	svc := newSettingsServiceForTest(t)
	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000032"

	_, err := svc.UpdateUIFontSettings(ctx, userID, UpdateUIFontSettingsInput{
		UIFontSansKey:  "dotgothic16",
		UIFontSerifKey: "biz-udmincho",
	})
	if err == nil || err.Error() != "invalid ui_font_sans_key" {
		t.Fatalf("UpdateUIFontSettings() error = %v, want invalid ui_font_sans_key", err)
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

func TestSettingsGetPayloadSupportsSiliconFlowFields(t *testing.T) {
	payload := &SettingsGetPayload{
		HasSiliconFlowAPIKey:   true,
		SiliconFlowAPIKeyLast4: strptr("sf42"),
	}

	if !payload.HasSiliconFlowAPIKey {
		t.Fatal("HasSiliconFlowAPIKey should be true")
	}
	if payload.SiliconFlowAPIKeyLast4 == nil || *payload.SiliconFlowAPIKeyLast4 != "sf42" {
		t.Fatalf("SiliconFlowAPIKeyLast4 = %v, want %q", payload.SiliconFlowAPIKeyLast4, "sf42")
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

func TestSummaryAudioVoiceSettingsPayloadDefaults(t *testing.T) {
	got := SummaryAudioVoiceSettingsPayload(nil)

	if got == nil {
		t.Fatal("SummaryAudioVoiceSettingsPayload(nil) = nil, want map")
	}
	if provider, _ := got["tts_provider"].(string); provider != "" {
		t.Fatalf("tts_provider = %v, want empty", got["tts_provider"])
	}
	if model, _ := got["tts_model"].(string); model != "" {
		t.Fatalf("tts_model = %v, want empty", got["tts_model"])
	}
	if voice, _ := got["voice_model"].(string); voice != "" {
		t.Fatalf("voice_model = %v, want empty", got["voice_model"])
	}
	if dict, ok := got["aivis_user_dictionary_uuid"]; ok && dict != nil {
		t.Fatalf("aivis_user_dictionary_uuid = %v, want nil", dict)
	}
}

func TestSettingsGetIncludesSummaryAudio(t *testing.T) {
	svc := newSettingsServiceForTest(t)

	payload, err := svc.Get(context.Background(), "00000000-0000-4000-8000-000000000021")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if payload.SummaryAudio == nil {
		t.Fatal("SummaryAudio should not be nil")
	}
	if provider, _ := payload.SummaryAudio["tts_provider"].(string); provider != "" {
		t.Fatalf("tts_provider = %v, want empty", payload.SummaryAudio["tts_provider"])
	}
}

func TestUpdateSummaryAudioVoiceSettingsAllowsGeminiTTS(t *testing.T) {
	svc := newSettingsServiceForTest(t)
	row, err := svc.UpdateSummaryAudioVoiceSettings(context.Background(), "00000000-0000-4000-8000-000000000021", UpdateSummaryAudioVoiceSettingsInput{
		TTSProvider:             "gemini_tts",
		TTSModel:                "gemini-2.5-flash-tts",
		VoiceModel:              "Kore",
		VoiceStyle:              "",
		SpeechRate:              1.0,
		EmotionalIntensity:      1.0,
		TempoDynamics:           1.0,
		LineBreakSilenceSeconds: 0.4,
		Pitch:                   0.0,
		VolumeGain:              0.0,
		AivisUserDictionaryUUID: nil,
	})
	if err != nil {
		t.Fatalf("UpdateSummaryAudioVoiceSettings() error = %v", err)
	}
	if row == nil {
		t.Fatal("UpdateSummaryAudioVoiceSettings() = nil, want row")
	}
	if row.TTSProvider != "gemini_tts" {
		t.Fatalf("TTSProvider = %q, want gemini_tts", row.TTSProvider)
	}
	if row.TTSModel != "gemini-2.5-flash-tts" {
		t.Fatalf("TTSModel = %q, want gemini-2.5-flash-tts", row.TTSModel)
	}
	if row.VoiceModel != "Kore" {
		t.Fatalf("VoiceModel = %q, want Kore", row.VoiceModel)
	}
}

func TestUpdateSummaryAudioVoiceSettingsRequiresTTSModelForGeminiTTS(t *testing.T) {
	svc := newSettingsServiceForTest(t)
	_, err := svc.UpdateSummaryAudioVoiceSettings(context.Background(), "00000000-0000-4000-8000-000000000021", UpdateSummaryAudioVoiceSettingsInput{
		TTSProvider: "gemini_tts",
		VoiceModel:  "Kore",
	})
	if err == nil {
		t.Fatal("UpdateSummaryAudioVoiceSettings() error = nil, want validation error")
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
		ScheduleMode:          "fixed_slots_3x",
		IntervalHours:         3,
		ArticlesPerEpisode:    6,
		TargetDurationMinutes: 20,
		DefaultPersona:        "editor",
		ProgramName:           strptr("Morning Sifto"),
		BGMEnabled:            true,
		BGMR2Prefix:           strptr("audio/bgm"),
	}

	got := AudioBriefingSettingsPayload(settings)

	if enabled, _ := got["enabled"].(bool); !enabled {
		t.Fatalf("enabled = %v, want true", got["enabled"])
	}
	if scheduleMode, _ := got["schedule_mode"].(string); scheduleMode != "fixed_slots_3x" {
		t.Fatalf("schedule_mode = %v, want fixed_slots_3x", got["schedule_mode"])
	}
	if interval, _ := got["interval_hours"].(int); interval != 3 {
		t.Fatalf("interval_hours = %v, want 3", got["interval_hours"])
	}
	if persona, _ := got["default_persona"].(string); persona != "editor" {
		t.Fatalf("default_persona = %v, want editor", got["default_persona"])
	}
	if programName, _ := got["program_name"].(*string); programName == nil || *programName != "Morning Sifto" {
		t.Fatalf("program_name = %v, want Morning Sifto", got["program_name"])
	}
	if bgmEnabled, _ := got["bgm_enabled"].(bool); !bgmEnabled {
		t.Fatalf("bgm_enabled = %v, want true", got["bgm_enabled"])
	}
	if bgmPrefix, _ := got["bgm_r2_prefix"].(*string); bgmPrefix == nil || *bgmPrefix != "audio/bgm" {
		t.Fatalf("bgm_r2_prefix = %v, want audio/bgm", got["bgm_r2_prefix"])
	}
}

func TestAudioBriefingSettingsPayloadDefaultsScheduleMode(t *testing.T) {
	got := AudioBriefingSettingsPayload(nil)
	if scheduleMode, _ := got["schedule_mode"].(string); scheduleMode != AudioBriefingScheduleModeInterval {
		t.Fatalf("schedule_mode = %v, want %q", got["schedule_mode"], AudioBriefingScheduleModeInterval)
	}
}

func TestUpdateAudioBriefingSettingsAllowsFixedSlots3x(t *testing.T) {
	svc := newSettingsServiceForTest(t)
	row, err := svc.UpdateAudioBriefingSettings(context.Background(), "00000000-0000-4000-8000-000000000021", UpdateAudioBriefingSettingsInput{
		Enabled:                     true,
		ScheduleMode:                AudioBriefingScheduleModeFixedSlots3x,
		IntervalHours:               6,
		ArticlesPerEpisode:          5,
		TargetDurationMinutes:       10,
		ChunkTrailingSilenceSeconds: 1,
		DefaultPersonaMode:          strptr("fixed"),
		DefaultPersona:              strptr("editor"),
	})
	if err != nil {
		t.Fatalf("UpdateAudioBriefingSettings() error = %v", err)
	}
	if row.ScheduleMode != AudioBriefingScheduleModeFixedSlots3x {
		t.Fatalf("ScheduleMode = %q, want %q", row.ScheduleMode, AudioBriefingScheduleModeFixedSlots3x)
	}
}

func TestUpdateAudioBriefingSettingsRejectsInvalidScheduleMode(t *testing.T) {
	svc := newSettingsServiceForTest(t)
	_, err := svc.UpdateAudioBriefingSettings(context.Background(), "00000000-0000-4000-8000-000000000021", UpdateAudioBriefingSettingsInput{
		Enabled:                     true,
		ScheduleMode:                "weird",
		IntervalHours:               6,
		ArticlesPerEpisode:          5,
		TargetDurationMinutes:       10,
		ChunkTrailingSilenceSeconds: 1,
		DefaultPersonaMode:          strptr("fixed"),
		DefaultPersona:              strptr("editor"),
	})
	if err == nil || err.Error() != "invalid schedule_mode" {
		t.Fatalf("err = %v, want invalid schedule_mode", err)
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
		{
			UserID:      "u2",
			Persona:     "snark",
			TTSProvider: "gemini_tts",
			TTSModel:    "gemini-2.5-flash-tts",
			VoiceModel:  "Kore",
			VoiceStyle:  "",
		},
	}

	got := AudioBriefingPersonaVoicesPayload(rows)
	if len(got) != 2 {
		t.Fatalf("len(AudioBriefingPersonaVoicesPayload) = %d, want 2", len(got))
	}
	if got[0]["persona"] != "editor" {
		t.Fatalf("persona = %v, want editor", got[0]["persona"])
	}
	if got[0]["tts_provider"] != "aivis" {
		t.Fatalf("tts_provider = %v, want aivis", got[0]["tts_provider"])
	}
	if got[0]["tts_model"] != "" {
		t.Fatalf("tts_model = %v, want empty", got[0]["tts_model"])
	}
	if got[1]["persona"] != "snark" {
		t.Fatalf("persona = %v, want snark", got[1]["persona"])
	}
	if got[1]["tts_provider"] != "gemini_tts" {
		t.Fatalf("tts_provider = %v, want gemini_tts", got[1]["tts_provider"])
	}
	if got[1]["tts_model"] != "gemini-2.5-flash-tts" {
		t.Fatalf("tts_model = %v, want gemini-2.5-flash-tts", got[1]["tts_model"])
	}
	if got[1]["voice_model"] != "Kore" {
		t.Fatalf("voice_model = %v, want Kore", got[1]["voice_model"])
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

func TestAudioBriefingSettingsPayloadIncludesConversationMode(t *testing.T) {
	got := AudioBriefingSettingsPayload(&model.AudioBriefingSettings{
		Enabled:               true,
		IntervalHours:         6,
		ArticlesPerEpisode:    5,
		TargetDurationMinutes: 20,
		DefaultPersonaMode:    PersonaModeRandom,
		DefaultPersona:        "editor",
		ConversationMode:      "duo",
	})

	if gotMode, _ := got["conversation_mode"].(string); gotMode != "duo" {
		t.Fatalf("conversation_mode = %v, want %q", got["conversation_mode"], "duo")
	}
}

func TestUpdateAudioBriefingSettingsPersistsScheduleMode(t *testing.T) {
	svc := newSettingsServiceForTest(t)
	got, err := svc.UpdateAudioBriefingSettings(context.Background(), "00000000-0000-4000-8000-000000000021", UpdateAudioBriefingSettingsInput{
		Enabled:               true,
		ScheduleMode:          "fixed_slots_3x",
		IntervalHours:         3,
		ArticlesPerEpisode:    5,
		TargetDurationMinutes: 20,
	})
	if err != nil {
		t.Fatalf("UpdateAudioBriefingSettings() error = %v", err)
	}
	if got == nil {
		t.Fatal("UpdateAudioBriefingSettings() returned nil settings")
	}
	if got.ScheduleMode != "fixed_slots_3x" {
		t.Fatalf("ScheduleMode = %q, want fixed_slots_3x", got.ScheduleMode)
	}
	payload := AudioBriefingSettingsPayload(got)
	if scheduleMode, _ := payload["schedule_mode"].(string); scheduleMode != "fixed_slots_3x" {
		t.Fatalf("payload schedule_mode = %v, want fixed_slots_3x", payload["schedule_mode"])
	}
}

func TestNormalizeAudioBriefingProgramName(t *testing.T) {
	if got := normalizeAudioBriefingProgramName(nil); got != nil {
		t.Fatalf("normalizeAudioBriefingProgramName(nil) = %v, want nil", got)
	}
	if got := normalizeAudioBriefingProgramName(strptr("   ")); got != nil {
		t.Fatalf("normalizeAudioBriefingProgramName(blank) = %v, want nil", got)
	}
	if got := normalizeAudioBriefingProgramName(strptr("  Morning Sifto  ")); got == nil || *got != "Morning Sifto" {
		t.Fatalf("normalizeAudioBriefingProgramName(trim) = %v, want Morning Sifto", got)
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

	t.Run("allows openai rows with tts model", func(t *testing.T) {
		rows, err := validateAudioBriefingPersonaVoiceInputs([]UpdateAudioBriefingPersonaVoiceInput{
			{
				Persona:                 "editor",
				TTSProvider:             "openai",
				TTSModel:                "gpt-4o-mini-tts",
				VoiceModel:              "alloy",
				VoiceStyle:              "",
				SpeechRate:              0,
				EmotionalIntensity:      0,
				TempoDynamics:           0,
				LineBreakSilenceSeconds: 0,
				Pitch:                   0,
				VolumeGain:              0,
			},
		})
		if err != nil {
			t.Fatalf("validateAudioBriefingPersonaVoiceInputs(openai) err=%v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("len(rows) = %d, want 1", len(rows))
		}
		if rows[0].TTSModel != "gpt-4o-mini-tts" {
			t.Fatalf("rows[0].TTSModel = %q, want gpt-4o-mini-tts", rows[0].TTSModel)
		}
		if rows[0].TTSProvider != "openai" {
			t.Fatalf("rows[0].TTSProvider = %q, want openai", rows[0].TTSProvider)
		}
	})

	t.Run("allows gemini rows with tts model", func(t *testing.T) {
		rows, err := validateAudioBriefingPersonaVoiceInputs([]UpdateAudioBriefingPersonaVoiceInput{
			{
				Persona:                 "editor",
				TTSProvider:             "gemini_tts",
				TTSModel:                "gemini-2.5-flash-tts",
				VoiceModel:              "Kore",
				VoiceStyle:              "",
				SpeechRate:              0,
				EmotionalIntensity:      0,
				TempoDynamics:           0,
				LineBreakSilenceSeconds: 0,
				Pitch:                   0,
				VolumeGain:              0,
			},
		})
		if err != nil {
			t.Fatalf("validateAudioBriefingPersonaVoiceInputs(gemini) err=%v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("len(rows) = %d, want 1", len(rows))
		}
		if rows[0].TTSProvider != "gemini_tts" {
			t.Fatalf("rows[0].TTSProvider = %q, want gemini_tts", rows[0].TTSProvider)
		}
		if rows[0].TTSModel != "gemini-2.5-flash-tts" {
			t.Fatalf("rows[0].TTSModel = %q, want gemini-2.5-flash-tts", rows[0].TTSModel)
		}
		if rows[0].VoiceStyle != "" {
			t.Fatalf("rows[0].VoiceStyle = %q, want empty", rows[0].VoiceStyle)
		}
	})

	t.Run("requires tts model for gemini", func(t *testing.T) {
		_, err := validateAudioBriefingPersonaVoiceInputs([]UpdateAudioBriefingPersonaVoiceInput{
			{
				Persona:                 "editor",
				TTSProvider:             "gemini_tts",
				TTSModel:                "",
				VoiceModel:              "Kore",
				VoiceStyle:              "",
				SpeechRate:              0,
				EmotionalIntensity:      0,
				TempoDynamics:           0,
				LineBreakSilenceSeconds: 0,
				Pitch:                   0,
				VolumeGain:              0,
			},
		})
		if err == nil || err.Error() != "invalid tts_model for editor" {
			t.Fatalf("validateAudioBriefingPersonaVoiceInputs(gemini missing tts model) err=%v, want invalid tts_model for editor", err)
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
			TTSProvider:             "openai",
			TTSModel:                "",
			VoiceModel:              "alloy",
			VoiceStyle:              "",
			SpeechRate:              0,
			EmotionalIntensity:      0,
			TempoDynamics:           0,
			LineBreakSilenceSeconds: 0,
		},
	})
	if err == nil || err.Error() != "invalid tts_model for editor" {
		t.Fatalf("missing tts_model err = %v, want invalid tts_model for editor", err)
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

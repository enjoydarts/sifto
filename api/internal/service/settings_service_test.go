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
	if got.Name != "Morning Briefing" {
		t.Fatalf("name = %v, want Morning Briefing", got.Name)
	}
	if got.ConversationMode != "duo" {
		t.Fatalf("conversation_mode = %v, want duo", got.ConversationMode)
	}
	if len(got.Voices) != 1 {
		t.Fatalf("voices = %d, want 1 voice", len(got.Voices))
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

func TestSettingsServiceSetAndDeleteCerebrasAPIKey(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "settings-service-cerebras-test-key")

	svc := newSettingsServiceForTest(t)
	svc.cipher = NewSecretCipher()
	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000021"

	settings, err := svc.SetAPIKey(ctx, userID, "cerebras", "cerebras-secret-value")
	if err != nil {
		t.Fatalf("SetAPIKey(cerebras) error = %v", err)
	}
	if !settings.HasCerebrasAPIKey {
		t.Fatal("HasCerebrasAPIKey = false, want true")
	}
	if settings.CerebrasAPIKeyLast4 == nil || *settings.CerebrasAPIKeyLast4 != "alue" {
		t.Fatalf("CerebrasAPIKeyLast4 = %#v, want %q", settings.CerebrasAPIKeyLast4, "alue")
	}
	enc, err := svc.repo.GetCerebrasAPIKeyEncrypted(ctx, userID)
	if err != nil {
		t.Fatalf("GetCerebrasAPIKeyEncrypted() error = %v", err)
	}
	if enc == nil || *enc == "" {
		t.Fatalf("GetCerebrasAPIKeyEncrypted() = %#v, want encrypted value", enc)
	}

	settings, err = svc.DeleteAPIKey(ctx, userID, "cerebras")
	if err != nil {
		t.Fatalf("DeleteAPIKey(cerebras) error = %v", err)
	}
	if settings.HasCerebrasAPIKey {
		t.Fatal("HasCerebrasAPIKey = true, want false")
	}
	if settings.CerebrasAPIKeyLast4 != nil {
		t.Fatalf("CerebrasAPIKeyLast4 = %#v, want nil", settings.CerebrasAPIKeyLast4)
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
	if _, err := db.Exec(context.Background(), `ALTER TABLE items ADD COLUMN IF NOT EXISTS user_genre text`); err != nil {
		t.Fatalf("ensure items.user_genre: %v", err)
	}
	if _, err := db.Exec(context.Background(), `ALTER TABLE items ADD COLUMN IF NOT EXISTS user_other_genre_label text`); err != nil {
		t.Fatalf("ensure items.user_other_genre_label: %v", err)
	}
	if _, err := db.Exec(context.Background(), `ALTER TABLE item_summaries ADD COLUMN IF NOT EXISTS genre text`); err != nil {
		t.Fatalf("ensure item_summaries.genre: %v", err)
	}
	if _, err := db.Exec(context.Background(), `ALTER TABLE item_summaries ADD COLUMN IF NOT EXISTS other_genre_label text`); err != nil {
		t.Fatalf("ensure item_summaries.other_genre_label: %v", err)
	}
	if _, err := db.Exec(context.Background(), `ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS featherless_api_key_enc text`); err != nil {
		t.Fatalf("ensure user_settings.featherless_api_key_enc: %v", err)
	}
	if _, err := db.Exec(context.Background(), `ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS featherless_api_key_last4 text`); err != nil {
		t.Fatalf("ensure user_settings.featherless_api_key_last4: %v", err)
	}
	if _, err := db.Exec(context.Background(), `ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS deepinfra_api_key_enc text`); err != nil {
		t.Fatalf("ensure user_settings.deepinfra_api_key_enc: %v", err)
	}
	if _, err := db.Exec(context.Background(), `ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS deepinfra_api_key_last4 text`); err != nil {
		t.Fatalf("ensure user_settings.deepinfra_api_key_last4: %v", err)
	}
	if _, err := db.Exec(context.Background(), `ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS cerebras_api_key_enc text`); err != nil {
		t.Fatalf("ensure user_settings.cerebras_api_key_enc: %v", err)
	}
	if _, err := db.Exec(context.Background(), `ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS cerebras_api_key_last4 text`); err != nil {
		t.Fatalf("ensure user_settings.cerebras_api_key_last4: %v", err)
	}
	if _, err := db.Exec(context.Background(), `ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS facts_check_fallback_model text`); err != nil {
		t.Fatalf("ensure user_settings.facts_check_fallback_model: %v", err)
	}
	if _, err := db.Exec(context.Background(), `ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS faithfulness_check_fallback_model text`); err != nil {
		t.Fatalf("ensure user_settings.faithfulness_check_fallback_model: %v", err)
	}
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

func TestSettingsServiceSetAndDeleteDeepInfraAPIKey(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "settings-service-deepinfra-test-key")

	svc := newSettingsServiceForTest(t)
	svc.cipher = NewSecretCipher()
	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000021"

	settings, err := svc.SetAPIKey(ctx, userID, "deepinfra", "deepinfra-secret")
	if err != nil {
		t.Fatalf("SetAPIKey(deepinfra) error = %v", err)
	}
	if !settings.HasDeepInfraAPIKey {
		t.Fatal("HasDeepInfraAPIKey = false, want true")
	}
	if settings.DeepInfraAPIKeyLast4 == nil || *settings.DeepInfraAPIKeyLast4 != "cret" {
		t.Fatalf("DeepInfraAPIKeyLast4 = %#v, want %q", settings.DeepInfraAPIKeyLast4, "cret")
	}

	settings, err = svc.DeleteAPIKey(ctx, userID, "deepinfra")
	if err != nil {
		t.Fatalf("DeleteAPIKey(deepinfra) error = %v", err)
	}
	if settings.HasDeepInfraAPIKey {
		t.Fatal("HasDeepInfraAPIKey = true, want false")
	}
	if settings.DeepInfraAPIKeyLast4 != nil {
		t.Fatalf("DeepInfraAPIKeyLast4 = %#v, want nil", settings.DeepInfraAPIKeyLast4)
	}
}

func TestSettingsPayloadIncludesDeepInfraAPIKeyState(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "settings-service-deepinfra-payload-key")

	svc := newSettingsServiceForTest(t)
	svc.cipher = NewSecretCipher()
	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000021"

	if _, err := svc.SetAPIKey(ctx, userID, "deepinfra", "payload-deepinfra-secret"); err != nil {
		t.Fatalf("SetAPIKey(deepinfra) error = %v", err)
	}

	payload, err := svc.Get(ctx, userID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !payload.HasDeepInfraAPIKey {
		t.Fatal("HasDeepInfraAPIKey = false, want true")
	}
	if payload.DeepInfraAPIKeyLast4 == nil || *payload.DeepInfraAPIKeyLast4 != "cret" {
		t.Fatalf("DeepInfraAPIKeyLast4 = %#v, want %q", payload.DeepInfraAPIKeyLast4, "cret")
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
		FactsCheckModel:                  strptr("featherless::Qwen/Qwen3.5-9B"),
		FactsCheckFallbackModel:          strptr("google/gemini-2.5-flash"),
		FaithfulnessCheckModel:           strptr("featherless::Qwen/Qwen3.5-9B"),
		FaithfulnessCheckFallbackModel:   strptr("gpt-5.4-mini"),
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

	if got.FactsFallback == nil || *got.FactsFallback != "google/gemini-2.5-flash" {
		t.Fatalf("facts_fallback = %v, want %q", got.FactsFallback, "google/gemini-2.5-flash")
	}
	if got.FactsSecondary == nil || *got.FactsSecondary != "google/gemini-2.5-flash" {
		t.Fatalf("facts_secondary = %v, want %q", got.FactsSecondary, "google/gemini-2.5-flash")
	}
	if got.FactsSecondaryRatePercent != 33 {
		t.Fatalf("facts_secondary_rate_percent = %v, want 33", got.FactsSecondaryRatePercent)
	}
	if got.SummaryFallback == nil || *got.SummaryFallback != "openrouter::openai/gpt-oss-120b" {
		t.Fatalf("summary_fallback = %v, want %q", got.SummaryFallback, "openrouter::openai/gpt-oss-120b")
	}
	if got.SummarySecondary == nil || *got.SummarySecondary != "openrouter::openai/gpt-oss-120b" {
		t.Fatalf("summary_secondary = %v, want %q", got.SummarySecondary, "openrouter::openai/gpt-oss-120b")
	}
	if got.SummarySecondaryRatePercent != 25 {
		t.Fatalf("summary_secondary_rate_percent = %v, want 25", got.SummarySecondaryRatePercent)
	}
	if got.FactsCheckFallback == nil || *got.FactsCheckFallback != "google/gemini-2.5-flash" {
		t.Fatalf("facts_check_fallback = %v, want %q", got.FactsCheckFallback, "google/gemini-2.5-flash")
	}
	if got.FaithfulnessCheckFallback == nil || *got.FaithfulnessCheckFallback != "gpt-5.4-mini" {
		t.Fatalf("faithfulness_check_fallback = %v, want %q", got.FaithfulnessCheckFallback, "gpt-5.4-mini")
	}
	if got.AudioBriefingScript == nil || *got.AudioBriefingScript != "gpt-5.4" {
		t.Fatalf("audio_briefing_script = %v, want %q", got.AudioBriefingScript, "gpt-5.4")
	}
	if got.AudioBriefingScriptFallback == nil || *got.AudioBriefingScriptFallback != "google/gemini-2.5-flash" {
		t.Fatalf("audio_briefing_script_fallback = %v, want %q", got.AudioBriefingScriptFallback, "google/gemini-2.5-flash")
	}
	if got.TTSMarkupPreprocessModel == nil || *got.TTSMarkupPreprocessModel != "gpt-5.4-mini" {
		t.Fatalf("tts_markup_preprocess_model = %v, want %q", got.TTSMarkupPreprocessModel, "gpt-5.4-mini")
	}
	if got.NavigatorPersonaMode != PersonaModeRandom {
		t.Fatalf("navigator_persona_mode = %v, want %q", got.NavigatorPersonaMode, PersonaModeRandom)
	}
	if got.AINavigatorBrief == nil || *got.AINavigatorBrief != "kimi-k2.5" {
		t.Fatalf("ai_navigator_brief = %v, want %q", got.AINavigatorBrief, "kimi-k2.5")
	}
	if got.AINavigatorBriefFallback == nil || *got.AINavigatorBriefFallback != "google/gemini-2.5-flash" {
		t.Fatalf("ai_navigator_brief_fallback = %v, want %q", got.AINavigatorBriefFallback, "google/gemini-2.5-flash")
	}
}

func TestSettingsServiceSetAndDeleteMiniMaxAPIKey(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "settings-service-minimax-test-key")

	svc := newSettingsServiceForTest(t)
	svc.cipher = NewSecretCipher()
	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000021"

	settings, err := svc.SetAPIKey(ctx, userID, "minimax", "mini-max-secret")
	if err != nil {
		t.Fatalf("SetAPIKey(minimax) error = %v", err)
	}
	if !settings.HasMiniMaxAPIKey {
		t.Fatal("HasMiniMaxAPIKey = false, want true")
	}
	if settings.MiniMaxAPIKeyLast4 == nil || *settings.MiniMaxAPIKeyLast4 != "cret" {
		t.Fatalf("MiniMaxAPIKeyLast4 = %#v, want %q", settings.MiniMaxAPIKeyLast4, "cret")
	}

	settings, err = svc.DeleteAPIKey(ctx, userID, "minimax")
	if err != nil {
		t.Fatalf("DeleteAPIKey(minimax) error = %v", err)
	}
	if settings.HasMiniMaxAPIKey {
		t.Fatal("HasMiniMaxAPIKey = true, want false")
	}
	if settings.MiniMaxAPIKeyLast4 != nil {
		t.Fatalf("MiniMaxAPIKeyLast4 = %#v, want nil", settings.MiniMaxAPIKeyLast4)
	}
}

func TestSettingsServiceGetIncludesMiniMaxAPIKeyPayload(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "settings-service-minimax-payload-key")

	svc := newSettingsServiceForTest(t)
	svc.cipher = NewSecretCipher()
	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000021"

	if _, err := svc.SetAPIKey(ctx, userID, "minimax", "payload-minimax-secret"); err != nil {
		t.Fatalf("SetAPIKey(minimax) error = %v", err)
	}

	payload, err := svc.Get(ctx, userID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !payload.HasMiniMaxAPIKey {
		t.Fatal("HasMiniMaxAPIKey = false, want true")
	}
	if payload.MiniMaxAPIKeyLast4 == nil || *payload.MiniMaxAPIKeyLast4 != "cret" {
		t.Fatalf("MiniMaxAPIKeyLast4 = %#v, want %q", payload.MiniMaxAPIKeyLast4, "cret")
	}
}

func TestSettingsServiceSetAndDeleteFeatherlessAPIKey(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "settings-service-featherless-test-key")

	svc := newSettingsServiceForTest(t)
	svc.cipher = NewSecretCipher()
	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000021"

	settings, err := svc.SetAPIKey(ctx, userID, "featherless", "featherless-secret")
	if err != nil {
		t.Fatalf("SetAPIKey(featherless) error = %v", err)
	}
	if !settings.HasFeatherlessAPIKey {
		t.Fatal("HasFeatherlessAPIKey = false, want true")
	}
	if settings.FeatherlessAPIKeyLast4 == nil || *settings.FeatherlessAPIKeyLast4 != "cret" {
		t.Fatalf("FeatherlessAPIKeyLast4 = %#v, want %q", settings.FeatherlessAPIKeyLast4, "cret")
	}

	settings, err = svc.DeleteAPIKey(ctx, userID, "featherless")
	if err != nil {
		t.Fatalf("DeleteAPIKey(featherless) error = %v", err)
	}
	if settings.HasFeatherlessAPIKey {
		t.Fatal("HasFeatherlessAPIKey = true, want false")
	}
	if settings.FeatherlessAPIKeyLast4 != nil {
		t.Fatalf("FeatherlessAPIKeyLast4 = %#v, want nil", settings.FeatherlessAPIKeyLast4)
	}
}

func TestSettingsServiceGetIncludesFeatherlessAPIKeyPayload(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "settings-service-featherless-payload-key")

	svc := newSettingsServiceForTest(t)
	svc.cipher = NewSecretCipher()
	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000021"

	if _, err := svc.SetAPIKey(ctx, userID, "featherless", "payload-featherless-secret"); err != nil {
		t.Fatalf("SetAPIKey(featherless) error = %v", err)
	}

	payload, err := svc.Get(ctx, userID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !payload.HasFeatherlessAPIKey {
		t.Fatal("HasFeatherlessAPIKey = false, want true")
	}
	if payload.FeatherlessAPIKeyLast4 == nil || *payload.FeatherlessAPIKeyLast4 != "cret" {
		t.Fatalf("FeatherlessAPIKeyLast4 = %#v, want %q", payload.FeatherlessAPIKeyLast4, "cret")
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
	if got.TTSMarkupPreprocessModel == nil || *got.TTSMarkupPreprocessModel != "gpt-5.4-mini" {
		t.Fatalf("tts_markup_preprocess_model payload = %v, want gpt-5.4-mini", got.TTSMarkupPreprocessModel)
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

	if got.TTSProvider != "" {
		t.Fatalf("tts_provider = %v, want empty", got.TTSProvider)
	}
	if got.TTSModel != "" {
		t.Fatalf("tts_model = %v, want empty", got.TTSModel)
	}
	if got.VoiceModel != "" {
		t.Fatalf("voice_model = %v, want empty", got.VoiceModel)
	}
	if got.AivisUserDictionaryUUID != nil {
		t.Fatalf("aivis_user_dictionary_uuid = %v, want nil", got.AivisUserDictionaryUUID)
	}
}

func TestSettingsGetIncludesSummaryAudio(t *testing.T) {
	svc := newSettingsServiceForTest(t)

	payload, err := svc.Get(context.Background(), "00000000-0000-4000-8000-000000000021")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if payload.SummaryAudio.TTSProvider != "" {
		t.Fatalf("tts_provider = %v, want empty", payload.SummaryAudio.TTSProvider)
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

func TestUpdateSummaryAudioVoiceSettingsTTSModelRequirementsByProvider(t *testing.T) {
	tests := []struct {
		name       string
		provider   string
		ttsModel   string
		voiceModel string
		wantErr    string
	}{
		{name: "xai does not require tts model", provider: "xai", voiceModel: "alloy"},
		{name: "openai requires tts model", provider: "openai", voiceModel: "alloy", wantErr: "invalid tts_model"},
		{name: "fish requires tts model", provider: "fish", voiceModel: "fish-voice", wantErr: "invalid tts_model"},
		{name: "gemini_tts requires tts model", provider: "gemini_tts", voiceModel: "Kore", wantErr: "invalid tts_model"},
		{name: "elevenlabs requires tts model", provider: "elevenlabs", voiceModel: "voice-1", wantErr: "invalid tts_model"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newSettingsServiceForTest(t)
			row, err := svc.UpdateSummaryAudioVoiceSettings(context.Background(), "00000000-0000-4000-8000-000000000021", UpdateSummaryAudioVoiceSettingsInput{
				TTSProvider:             tt.provider,
				TTSModel:                tt.ttsModel,
				VoiceModel:              tt.voiceModel,
				VoiceStyle:              "",
				SpeechRate:              1.0,
				EmotionalIntensity:      1.0,
				TempoDynamics:           1.0,
				LineBreakSilenceSeconds: 0.4,
				Pitch:                   0.0,
				VolumeGain:              0.0,
				AivisUserDictionaryUUID: nil,
			})
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("UpdateSummaryAudioVoiceSettings() error = %v, want %q", err, tt.wantErr)
				}
				if row != nil {
					t.Fatalf("UpdateSummaryAudioVoiceSettings() row = %#v, want nil", row)
				}
				return
			}
			if err != nil {
				t.Fatalf("UpdateSummaryAudioVoiceSettings() error = %v", err)
			}
			if row == nil {
				t.Fatal("UpdateSummaryAudioVoiceSettings() = nil, want row")
			}
			if row.TTSProvider != tt.provider {
				t.Fatalf("TTSProvider = %q, want %q", row.TTSProvider, tt.provider)
			}
			if row.TTSModel != tt.ttsModel {
				t.Fatalf("TTSModel = %q, want %q", row.TTSModel, tt.ttsModel)
			}
		})
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

	if !payload.Enabled {
		t.Fatalf("enabled = %v, want true", payload.Enabled)
	}
	if payload.FeedSlug == nil || *payload.FeedSlug != "p_123" {
		t.Fatalf("feed_slug = %v, want p_123", payload.FeedSlug)
	}
	if payload.Language != "ja" {
		t.Fatalf("language = %v, want ja", payload.Language)
	}
	if payload.Category == nil || *payload.Category != "Technology" {
		t.Fatalf("category = %v, want Technology", payload.Category)
	}
	if len(payload.AvailableCategories) == 0 {
		t.Fatalf("available_categories = %v, want non-empty", payload.AvailableCategories)
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

	if !got.Enabled {
		t.Fatalf("enabled = %v, want true", got.Enabled)
	}
	if got.ScheduleMode != "fixed_slots_3x" {
		t.Fatalf("schedule_mode = %v, want fixed_slots_3x", got.ScheduleMode)
	}
	if got.IntervalHours != 3 {
		t.Fatalf("interval_hours = %v, want 3", got.IntervalHours)
	}
	if got.DefaultPersona != "editor" {
		t.Fatalf("default_persona = %v, want editor", got.DefaultPersona)
	}
	if got.ProgramName == nil || *got.ProgramName != "Morning Sifto" {
		t.Fatalf("program_name = %v, want Morning Sifto", got.ProgramName)
	}
	if !got.BGMEnabled {
		t.Fatalf("bgm_enabled = %v, want true", got.BGMEnabled)
	}
	if got.BGMR2Prefix == nil || *got.BGMR2Prefix != "audio/bgm" {
		t.Fatalf("bgm_r2_prefix = %v, want audio/bgm", got.BGMR2Prefix)
	}
}

func TestAudioBriefingSettingsPayloadDefaultsScheduleMode(t *testing.T) {
	got := AudioBriefingSettingsPayload(nil)
	if got.ScheduleMode != AudioBriefingScheduleModeInterval {
		t.Fatalf("schedule_mode = %v, want %q", got.ScheduleMode, AudioBriefingScheduleModeInterval)
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
	if got[0].Persona != "editor" {
		t.Fatalf("persona = %v, want editor", got[0].Persona)
	}
	if got[0].TTSProvider != "aivis" {
		t.Fatalf("tts_provider = %v, want aivis", got[0].TTSProvider)
	}
	if got[0].TTSModel != "" {
		t.Fatalf("tts_model = %v, want empty", got[0].TTSModel)
	}
	if got[1].Persona != "snark" {
		t.Fatalf("persona = %v, want snark", got[1].Persona)
	}
	if got[1].TTSProvider != "gemini_tts" {
		t.Fatalf("tts_provider = %v, want gemini_tts", got[1].TTSProvider)
	}
	if got[1].TTSModel != "gemini-2.5-flash-tts" {
		t.Fatalf("tts_model = %v, want gemini-2.5-flash-tts", got[1].TTSModel)
	}
	if got[1].VoiceModel != "Kore" {
		t.Fatalf("voice_model = %v, want Kore", got[1].VoiceModel)
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

	if got.DefaultPersonaMode != PersonaModeRandom {
		t.Fatalf("default_persona_mode = %v, want %q", got.DefaultPersonaMode, PersonaModeRandom)
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

	if got.ConversationMode != "duo" {
		t.Fatalf("conversation_mode = %v, want %q", got.ConversationMode, "duo")
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
	if payload.ScheduleMode != "fixed_slots_3x" {
		t.Fatalf("payload schedule_mode = %v, want fixed_slots_3x", payload.ScheduleMode)
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

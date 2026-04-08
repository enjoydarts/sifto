package service

import (
	"context"
	"testing"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
)

type summaryAudioSynthStub struct {
	gotText string
	resp    *SummaryAudioSynthesizeResponse
}

func (s *summaryAudioSynthStub) SynthesizeSummaryAudio(
	_ context.Context,
	_ string,
	_ string,
	_ string,
	_ string,
	text string,
	_ float64,
	_ float64,
	_ float64,
	_ float64,
	_ float64,
	_ float64,
	_ float64,
	_ *string,
	_ *string,
	_ *string,
	_ *string,
	_ *string,
	_ *string,
	_ *string,
) (*SummaryAudioSynthesizeResponse, error) {
	s.gotText = text
	if s.resp != nil {
		return s.resp, nil
	}
	return &SummaryAudioSynthesizeResponse{
		AudioBase64:  "Zm9v",
		ContentType:  "audio/mpeg",
		DurationSec:  12,
		ResolvedText: text,
	}, nil
}

type summaryAudioPreprocessStub struct {
	gotUserID   string
	gotItemID   string
	gotProvider string
	gotText     string
	resp        *TTSMarkupPreprocessResult
}

func (s *summaryAudioPreprocessStub) PreprocessSummaryAudioText(_ context.Context, userID, itemID, text string) (*TTSMarkupPreprocessResult, error) {
	s.gotUserID = userID
	s.gotItemID = itemID
	s.gotText = text
	if s.resp != nil {
		return s.resp, nil
	}
	return &TTSMarkupPreprocessResult{Text: text}, nil
}

func (s *summaryAudioPreprocessStub) PreprocessSummaryAudioTextForProvider(_ context.Context, userID, itemID, provider, text string) (*TTSMarkupPreprocessResult, error) {
	s.gotUserID = userID
	s.gotItemID = itemID
	s.gotProvider = provider
	s.gotText = text
	if s.resp != nil {
		return s.resp, nil
	}
	return &TTSMarkupPreprocessResult{Text: text}, nil
}

func TestSummaryAudioPlayerUsesFishPreprocessOutput(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "summary-audio-fish-test-key")
	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	lockSettingsServiceTestDB(t, db)

	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000021"
	sourceID := "00000000-0000-4000-8000-000000000031"
	itemID := "00000000-0000-4000-8000-000000000041"
	if _, err := db.Exec(ctx, `DELETE FROM llm_usage_logs WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset llm_usage_logs: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM summary_audio_voice_settings WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset summary_audio_voice_settings: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM item_summaries WHERE item_id = $1`, itemID); err != nil {
		t.Fatalf("reset item_summaries: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM items WHERE id = $1`, itemID); err != nil {
		t.Fatalf("reset items: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM sources WHERE id = $1`, sourceID); err != nil {
		t.Fatalf("reset sources: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM user_settings WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset user_settings: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("reset users: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO users (id, email, name) VALUES ($1, $2, $3)`, userID, "summary-audio@example.com", "Summary Audio"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO sources (id, user_id, url, type, title) VALUES ($1, $2, $3, 'manual', $4)`, sourceID, userID, "https://example.com/feed", "Example Source"); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO items (id, source_id, url, title, status, content_text) VALUES ($1, $2, $3, $4, 'summarized', $5)`, itemID, sourceID, "https://example.com/items/1", "Original Title", "body"); err != nil {
		t.Fatalf("insert item: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO item_summaries (item_id, summary, topics, translated_title, score, score_breakdown, score_reason, score_policy_version) VALUES ($1, $2, $3, $4, $5, '{}'::jsonb, '', '')`,
		itemID, "Summary body", []string{"ai"}, "Translated Title", 0.9); err != nil {
		t.Fatalf("insert summary: %v", err)
	}

	userSettingsRepo := repository.NewUserSettingsRepo(db)
	summaryAudioRepo := repository.NewSummaryAudioVoiceSettingsRepo(db)
	if _, err := summaryAudioRepo.Upsert(ctx, model.SummaryAudioVoiceSettings{
		UserID:                  userID,
		TTSProvider:             "fish",
		TTSModel:                "s1",
		VoiceModel:              "fish-voice",
		SpeechRate:              1,
		EmotionalIntensity:      1,
		TempoDynamics:           1,
		LineBreakSilenceSeconds: 0.4,
	}); err != nil {
		t.Fatalf("summaryAudioRepo.Upsert() error = %v", err)
	}
	cipher := NewSecretCipher()
	fishEnc, err := cipher.EncryptString("fish-secret")
	if err != nil {
		t.Fatalf("EncryptString(fish): %v", err)
	}
	if _, err := userSettingsRepo.SetFishAudioAPIKey(ctx, userID, fishEnc, "cret"); err != nil {
		t.Fatalf("SetFishAudioAPIKey() error = %v", err)
	}

	synth := &summaryAudioSynthStub{}
	preprocess := &summaryAudioPreprocessStub{
		resp: &TTSMarkupPreprocessResult{Text: "[自然に] Translated Title\n\n[落ち着いて] Summary body"},
	}
	service := NewSummaryAudioPlayerService(
		repository.NewItemRepo(db),
		summaryAudioRepo,
		repository.NewUserRepo(db),
		userSettingsRepo,
		cipher,
		synth,
		preprocess,
	)

	result, err := service.Synthesize(ctx, userID, itemID)
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}
	if preprocess.gotUserID != userID {
		t.Fatalf("preprocess userID = %q, want %q", preprocess.gotUserID, userID)
	}
	if preprocess.gotItemID != itemID {
		t.Fatalf("preprocess itemID = %q, want %q", preprocess.gotItemID, itemID)
	}
	if preprocess.gotProvider != "fish" {
		t.Fatalf("preprocess provider = %q, want fish", preprocess.gotProvider)
	}
	if preprocess.gotText != "Translated Title\n\nSummary body" {
		t.Fatalf("preprocess text = %q, want narration", preprocess.gotText)
	}
	if synth.gotText != "[自然に] Translated Title\n\n[落ち着いて] Summary body" {
		t.Fatalf("synth text = %q, want preprocessed narration", synth.gotText)
	}
	if result == nil || result.ResolvedText != "[自然に] Translated Title\n\n[落ち着いて] Summary body" {
		t.Fatalf("result = %#v, want resolved preprocessed text", result)
	}
}

func TestSummaryAudioPlayerUsesGeminiPreprocessOutput(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "summary-audio-gemini-test-key")
	t.Setenv("GEMINI_TTS_ALLOWED_EMAILS", "summary-audio-gemini@example.com")
	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	lockSettingsServiceTestDB(t, db)

	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000051"
	sourceID := "00000000-0000-4000-8000-000000000061"
	itemID := "00000000-0000-4000-8000-000000000071"
	if _, err := db.Exec(ctx, `DELETE FROM summary_audio_voice_settings WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset summary_audio_voice_settings: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM item_summaries WHERE item_id = $1`, itemID); err != nil {
		t.Fatalf("reset item_summaries: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM items WHERE id = $1`, itemID); err != nil {
		t.Fatalf("reset items: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM sources WHERE id = $1`, sourceID); err != nil {
		t.Fatalf("reset sources: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM user_settings WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset user_settings: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("reset users: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO users (id, email, name) VALUES ($1, $2, $3)`, userID, "summary-audio-gemini@example.com", "Summary Audio Gemini"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO sources (id, user_id, url, type, title) VALUES ($1, $2, $3, 'manual', $4)`, sourceID, userID, "https://example.com/feed", "Example Source"); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO items (id, source_id, url, title, status, content_text) VALUES ($1, $2, $3, $4, 'summarized', $5)`, itemID, sourceID, "https://example.com/items/2", "Original Title", "body"); err != nil {
		t.Fatalf("insert item: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO item_summaries (item_id, summary, topics, translated_title, score, score_breakdown, score_reason, score_policy_version) VALUES ($1, $2, $3, $4, $5, '{}'::jsonb, '', '')`,
		itemID, "Summary body", []string{"ai"}, "Translated Title", 0.9); err != nil {
		t.Fatalf("insert summary: %v", err)
	}

	userSettingsRepo := repository.NewUserSettingsRepo(db)
	summaryAudioRepo := repository.NewSummaryAudioVoiceSettingsRepo(db)
	if _, err := summaryAudioRepo.Upsert(ctx, model.SummaryAudioVoiceSettings{
		UserID:                  userID,
		TTSProvider:             "gemini_tts",
		TTSModel:                "gemini-2.5-flash-preview-tts",
		VoiceModel:              "Kore",
		SpeechRate:              1,
		EmotionalIntensity:      1,
		TempoDynamics:           1,
		LineBreakSilenceSeconds: 0.4,
	}); err != nil {
		t.Fatalf("summaryAudioRepo.Upsert() error = %v", err)
	}

	cipher := NewSecretCipher()
	googleEnc, err := cipher.EncryptString("google-secret")
	if err != nil {
		t.Fatalf("EncryptString(google): %v", err)
	}
	if _, err := userSettingsRepo.SetGoogleAPIKey(ctx, userID, googleEnc, "cret"); err != nil {
		t.Fatalf("SetGoogleAPIKey() error = %v", err)
	}

	synth := &summaryAudioSynthStub{}
	preprocess := &summaryAudioPreprocessStub{
		resp: &TTSMarkupPreprocessResult{Text: "[short pause] Translated Title\n\n[medium pause] Summary body"},
	}
	service := NewSummaryAudioPlayerService(
		repository.NewItemRepo(db),
		summaryAudioRepo,
		repository.NewUserRepo(db),
		userSettingsRepo,
		cipher,
		synth,
		preprocess,
	)

	result, err := service.Synthesize(ctx, userID, itemID)
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}
	if preprocess.gotProvider != "gemini_tts" {
		t.Fatalf("preprocess provider = %q, want gemini_tts", preprocess.gotProvider)
	}
	if synth.gotText != "[short pause] Translated Title\n\n[medium pause] Summary body" {
		t.Fatalf("synth text = %q, want preprocessed narration", synth.gotText)
	}
	if result == nil || result.PreprocessedText == nil || *result.PreprocessedText == "" {
		t.Fatalf("result.PreprocessedText = %#v, want non-empty", result)
	}
}

func TestSummaryAudioPlayerUsesElevenLabsPreprocessOutput(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "summary-audio-elevenlabs-test-key")
	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	lockSettingsServiceTestDB(t, db)

	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000081"
	sourceID := "00000000-0000-4000-8000-000000000082"
	itemID := "00000000-0000-4000-8000-000000000083"
	if _, err := db.Exec(ctx, `DELETE FROM summary_audio_voice_settings WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset summary_audio_voice_settings: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM item_summaries WHERE item_id = $1`, itemID); err != nil {
		t.Fatalf("reset item_summaries: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM items WHERE id = $1`, itemID); err != nil {
		t.Fatalf("reset items: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM sources WHERE id = $1`, sourceID); err != nil {
		t.Fatalf("reset sources: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM user_settings WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("reset user_settings: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("reset users: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO users (id, email, name) VALUES ($1, $2, $3)`, userID, "summary-audio-elevenlabs@example.com", "Summary Audio ElevenLabs"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO sources (id, user_id, url, type, title) VALUES ($1, $2, $3, 'manual', $4)`, sourceID, userID, "https://example.com/feed", "Example Source"); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO items (id, source_id, url, title, status, content_text) VALUES ($1, $2, $3, $4, 'summarized', $5)`, itemID, sourceID, "https://example.com/items/3", "Original Title", "body"); err != nil {
		t.Fatalf("insert item: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO item_summaries (item_id, summary, topics, translated_title, score, score_breakdown, score_reason, score_policy_version) VALUES ($1, $2, $3, $4, $5, '{}'::jsonb, '', '')`,
		itemID, "Summary body", []string{"ai"}, "Translated Title", 0.9); err != nil {
		t.Fatalf("insert summary: %v", err)
	}

	userSettingsRepo := repository.NewUserSettingsRepo(db)
	summaryAudioRepo := repository.NewSummaryAudioVoiceSettingsRepo(db)
	if _, err := summaryAudioRepo.Upsert(ctx, model.SummaryAudioVoiceSettings{
		UserID:                  userID,
		TTSProvider:             "elevenlabs",
		TTSModel:                "eleven_multilingual_v2",
		VoiceModel:              "eleven-voice",
		SpeechRate:              1,
		EmotionalIntensity:      1,
		TempoDynamics:           1,
		LineBreakSilenceSeconds: 0.4,
	}); err != nil {
		t.Fatalf("summaryAudioRepo.Upsert() error = %v", err)
	}

	cipher := NewSecretCipher()
	elevenEnc, err := cipher.EncryptString("eleven-secret")
	if err != nil {
		t.Fatalf("EncryptString(elevenlabs): %v", err)
	}
	if _, err := userSettingsRepo.SetElevenLabsAPIKey(ctx, userID, elevenEnc, "cret"); err != nil {
		t.Fatalf("SetElevenLabsAPIKey() error = %v", err)
	}

	synth := &summaryAudioSynthStub{}
	preprocess := &summaryAudioPreprocessStub{
		resp: &TTSMarkupPreprocessResult{Text: "[自然に] Translated Title\n\n[落ち着いて] Summary body"},
	}
	service := NewSummaryAudioPlayerService(
		repository.NewItemRepo(db),
		summaryAudioRepo,
		repository.NewUserRepo(db),
		userSettingsRepo,
		cipher,
		synth,
		preprocess,
	)

	result, err := service.Synthesize(ctx, userID, itemID)
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}
	if preprocess.gotProvider != "elevenlabs" {
		t.Fatalf("preprocess provider = %q, want elevenlabs", preprocess.gotProvider)
	}
	if synth.gotText != "[自然に] Translated Title\n\n[落ち着いて] Summary body" {
		t.Fatalf("synth text = %q, want preprocessed narration", synth.gotText)
	}
	if result == nil || result.PreprocessedText == nil || *result.PreprocessedText == "" {
		t.Fatalf("result.PreprocessedText = %#v, want non-empty", result)
	}
}

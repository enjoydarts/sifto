package repository

import (
	"context"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SummaryAudioVoiceSettingsRepo struct{ db *pgxpool.Pool }

func NewSummaryAudioVoiceSettingsRepo(db *pgxpool.Pool) *SummaryAudioVoiceSettingsRepo {
	return &SummaryAudioVoiceSettingsRepo{db: db}
}

func (r *SummaryAudioVoiceSettingsRepo) EnsureDefaults(ctx context.Context, userID string) (*model.SummaryAudioVoiceSettings, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO summary_audio_voice_settings (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING
	`, userID)
	if err != nil {
		return nil, err
	}
	return r.GetByUserID(ctx, userID)
}

func (r *SummaryAudioVoiceSettingsRepo) GetByUserID(ctx context.Context, userID string) (*model.SummaryAudioVoiceSettings, error) {
	var row model.SummaryAudioVoiceSettings
	err := r.db.QueryRow(ctx, `
		SELECT user_id, tts_provider, tts_model, voice_model, voice_style, COALESCE(provider_voice_label, ''), COALESCE(provider_voice_description, ''), speech_rate, emotional_intensity, tempo_dynamics, line_break_silence_seconds, pitch, volume_gain, aivis_user_dictionary_uuid, created_at, updated_at
		FROM summary_audio_voice_settings
		WHERE user_id = $1
	`, userID).Scan(
		&row.UserID,
		&row.TTSProvider,
		&row.TTSModel,
		&row.VoiceModel,
		&row.VoiceStyle,
		&row.ProviderVoiceLabel,
		&row.ProviderVoiceDescription,
		&row.SpeechRate,
		&row.EmotionalIntensity,
		&row.TempoDynamics,
		&row.LineBreakSilenceSeconds,
		&row.Pitch,
		&row.VolumeGain,
		&row.AivisUserDictionaryUUID,
		&row.CreatedAt,
		&row.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *SummaryAudioVoiceSettingsRepo) Upsert(ctx context.Context, row model.SummaryAudioVoiceSettings) (*model.SummaryAudioVoiceSettings, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO summary_audio_voice_settings (
			user_id, tts_provider, tts_model, voice_model, voice_style, provider_voice_label, provider_voice_description, speech_rate, emotional_intensity, tempo_dynamics, line_break_silence_seconds, pitch, volume_gain, aivis_user_dictionary_uuid
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)
		ON CONFLICT (user_id) DO UPDATE
		SET tts_provider = EXCLUDED.tts_provider,
		    tts_model = EXCLUDED.tts_model,
		    voice_model = EXCLUDED.voice_model,
		    voice_style = EXCLUDED.voice_style,
		    provider_voice_label = EXCLUDED.provider_voice_label,
		    provider_voice_description = EXCLUDED.provider_voice_description,
		    speech_rate = EXCLUDED.speech_rate,
		    emotional_intensity = EXCLUDED.emotional_intensity,
		    tempo_dynamics = EXCLUDED.tempo_dynamics,
		    line_break_silence_seconds = EXCLUDED.line_break_silence_seconds,
		    pitch = EXCLUDED.pitch,
		    volume_gain = EXCLUDED.volume_gain,
		    aivis_user_dictionary_uuid = EXCLUDED.aivis_user_dictionary_uuid,
		    updated_at = NOW()
	`, row.UserID, row.TTSProvider, row.TTSModel, row.VoiceModel, row.VoiceStyle, row.ProviderVoiceLabel, row.ProviderVoiceDescription, row.SpeechRate, row.EmotionalIntensity, row.TempoDynamics, row.LineBreakSilenceSeconds, row.Pitch, row.VolumeGain, row.AivisUserDictionaryUUID)
	if err != nil {
		return nil, err
	}
	return r.GetByUserID(ctx, row.UserID)
}

func (r *SummaryAudioVoiceSettingsRepo) Delete(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM summary_audio_voice_settings WHERE user_id = $1`, userID)
	return err
}

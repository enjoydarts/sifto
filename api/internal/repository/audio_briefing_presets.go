package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AudioBriefingPresetRepo struct{ db *pgxpool.Pool }

func NewAudioBriefingPresetRepo(db *pgxpool.Pool) *AudioBriefingPresetRepo {
	return &AudioBriefingPresetRepo{db: db}
}

func (r *AudioBriefingPresetRepo) ListByUser(ctx context.Context, userID string) ([]model.AudioBriefingPreset, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, name, default_persona_mode, default_persona, conversation_mode, created_at, updated_at
		FROM audio_briefing_presets
		WHERE user_id = $1
		ORDER BY updated_at DESC, created_at DESC, name ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.AudioBriefingPreset, 0)
	for rows.Next() {
		preset, err := scanAudioBriefingPresetRow(rows)
		if err != nil {
			return nil, err
		}
		voices, err := r.listVoicesByPresetID(ctx, preset.ID)
		if err != nil {
			return nil, err
		}
		preset.Voices = voices
		out = append(out, preset)
	}
	return out, rows.Err()
}

func (r *AudioBriefingPresetRepo) GetByID(ctx context.Context, userID, presetID string) (*model.AudioBriefingPreset, error) {
	preset, err := r.getByQueryRow(ctx, `
		SELECT id, user_id, name, default_persona_mode, default_persona, conversation_mode, created_at, updated_at
		FROM audio_briefing_presets
		WHERE user_id = $1 AND id = $2
	`, userID, presetID)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil || preset == nil {
		return preset, err
	}
	voices, err := r.listVoicesByPresetID(ctx, preset.ID)
	if err != nil {
		return nil, err
	}
	preset.Voices = voices
	return preset, nil
}

func (r *AudioBriefingPresetRepo) GetByName(ctx context.Context, userID, name string) (*model.AudioBriefingPreset, error) {
	preset, err := r.getByQueryRow(ctx, `
		SELECT id, user_id, name, default_persona_mode, default_persona, conversation_mode, created_at, updated_at
		FROM audio_briefing_presets
		WHERE user_id = $1 AND name = $2
	`, userID, name)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil || preset == nil {
		return preset, err
	}
	voices, err := r.listVoicesByPresetID(ctx, preset.ID)
	if err != nil {
		return nil, err
	}
	preset.Voices = voices
	return preset, nil
}

func (r *AudioBriefingPresetRepo) Create(ctx context.Context, preset model.AudioBriefingPreset) (*model.AudioBriefingPreset, error) {
	preset.Name = strings.TrimSpace(preset.Name)
	preset.DefaultPersonaMode = normalizeAudioBriefingPresetPersonaMode(preset.DefaultPersonaMode)
	preset.DefaultPersona = normalizeAudioBriefingPresetPersona(preset.DefaultPersona)
	preset.ConversationMode = normalizeAudioBriefingPresetConversationMode(preset.ConversationMode)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
		INSERT INTO audio_briefing_presets (
			user_id, name, default_persona_mode, default_persona, conversation_mode
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, name, default_persona_mode, default_persona, conversation_mode, created_at, updated_at
	`, preset.UserID, preset.Name, preset.DefaultPersonaMode, preset.DefaultPersona, preset.ConversationMode)
	stored, err := scanAudioBriefingPresetRow(row)
	if err != nil {
		return nil, mapDBError(err)
	}
	if err := r.upsertPresetVoicesTx(ctx, tx, stored.ID, preset.Voices); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, stored.UserID, stored.ID)
}

func (r *AudioBriefingPresetRepo) Update(ctx context.Context, preset model.AudioBriefingPreset) (*model.AudioBriefingPreset, error) {
	preset.Name = strings.TrimSpace(preset.Name)
	preset.DefaultPersonaMode = normalizeAudioBriefingPresetPersonaMode(preset.DefaultPersonaMode)
	preset.DefaultPersona = normalizeAudioBriefingPresetPersona(preset.DefaultPersona)
	preset.ConversationMode = normalizeAudioBriefingPresetConversationMode(preset.ConversationMode)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE audio_briefing_presets
		SET name = $3,
		    default_persona_mode = $4,
		    default_persona = $5,
		    conversation_mode = $6,
		    updated_at = NOW()
		WHERE user_id = $1 AND id = $2
	`, preset.UserID, preset.ID, preset.Name, preset.DefaultPersonaMode, preset.DefaultPersona, preset.ConversationMode)
	if err != nil {
		return nil, mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	if _, err := tx.Exec(ctx, `DELETE FROM audio_briefing_preset_voices WHERE preset_id = $1`, preset.ID); err != nil {
		return nil, mapDBError(err)
	}
	if err := r.insertPresetVoicesTx(ctx, tx, preset.ID, preset.Voices); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, preset.UserID, preset.ID)
}

func (r *AudioBriefingPresetRepo) Delete(ctx context.Context, userID, presetID string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM audio_briefing_presets
		WHERE user_id = $1 AND id = $2
	`, userID, presetID)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *AudioBriefingPresetRepo) listVoicesByPresetID(ctx context.Context, presetID string) ([]model.AudioBriefingPersonaVoice, error) {
	rows, err := r.db.Query(ctx, `
		SELECT ''::text, persona, tts_provider, tts_model, voice_model, voice_style, COALESCE(provider_voice_label, ''), COALESCE(provider_voice_description, ''), speech_rate, emotional_intensity, tempo_dynamics, line_break_silence_seconds, pitch, volume_gain, created_at, updated_at
		FROM audio_briefing_preset_voices
		WHERE preset_id = $1
		ORDER BY persona ASC
	`, presetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.AudioBriefingPersonaVoice, 0)
	for rows.Next() {
		var presetID string
		var row model.AudioBriefingPersonaVoice
		if err := rows.Scan(
			&presetID,
			&row.Persona,
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
			&row.CreatedAt,
			&row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *AudioBriefingPresetRepo) upsertPresetVoicesTx(ctx context.Context, tx pgx.Tx, presetID string, voices []model.AudioBriefingPersonaVoice) error {
	return r.insertPresetVoicesTx(ctx, tx, presetID, voices)
}

func (r *AudioBriefingPresetRepo) insertPresetVoicesTx(ctx context.Context, tx pgx.Tx, presetID string, voices []model.AudioBriefingPersonaVoice) error {
	for _, row := range voices {
		if _, err := tx.Exec(ctx, `
			INSERT INTO audio_briefing_preset_voices (
				preset_id, persona, tts_provider, tts_model, voice_model, voice_style, provider_voice_label, provider_voice_description, speech_rate, emotional_intensity, tempo_dynamics, line_break_silence_seconds, pitch, volume_gain
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		`, presetID, row.Persona, row.TTSProvider, row.TTSModel, row.VoiceModel, row.VoiceStyle, row.ProviderVoiceLabel, row.ProviderVoiceDescription, row.SpeechRate, row.EmotionalIntensity, row.TempoDynamics, row.LineBreakSilenceSeconds, row.Pitch, row.VolumeGain); err != nil {
			return mapDBError(err)
		}
	}
	return nil
}

func (r *AudioBriefingPresetRepo) getByQueryRow(ctx context.Context, query string, args ...any) (*model.AudioBriefingPreset, error) {
	preset, err := scanAudioBriefingPresetRow(r.db.QueryRow(ctx, query, args...))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, mapDBError(err)
	}
	return &preset, nil
}

func scanAudioBriefingPresetRow(row audioBriefingPresetScanner) (model.AudioBriefingPreset, error) {
	var preset model.AudioBriefingPreset
	if err := row.Scan(
		&preset.ID,
		&preset.UserID,
		&preset.Name,
		&preset.DefaultPersonaMode,
		&preset.DefaultPersona,
		&preset.ConversationMode,
		&preset.CreatedAt,
		&preset.UpdatedAt,
	); err != nil {
		return model.AudioBriefingPreset{}, err
	}
	return preset, nil
}

type audioBriefingPresetScanner interface {
	Scan(dest ...any) error
}

func normalizeAudioBriefingPresetPersonaMode(v string) string {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "random":
		return "random"
	default:
		return "fixed"
	}
}

func normalizeAudioBriefingPresetPersona(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "editor"
	}
	return v
}

func normalizeAudioBriefingPresetConversationMode(v string) string {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "duo":
		return "duo"
	default:
		return "single"
	}
}

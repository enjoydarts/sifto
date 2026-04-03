package repository

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AudioBriefingRepo struct{ db *pgxpool.Pool }

func NewAudioBriefingRepo(db *pgxpool.Pool) *AudioBriefingRepo { return &AudioBriefingRepo{db: db} }

type AudioBriefingPromptMetadata struct {
	PromptKey             *string
	PromptSource          *string
	PromptVersionID       *string
	PromptVersionNumber   *int
	PromptExperimentID    *string
	PromptExperimentArmID *string
}

func (r *AudioBriefingRepo) EnsureSettingsDefaults(ctx context.Context, userID string) (*model.AudioBriefingSettings, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO audio_briefing_settings (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING
	`, userID)
	if err != nil {
		return nil, err
	}
	return r.GetSettings(ctx, userID)
}

func (r *AudioBriefingRepo) GetSettings(ctx context.Context, userID string) (*model.AudioBriefingSettings, error) {
	var v model.AudioBriefingSettings
	err := r.db.QueryRow(ctx, `
		SELECT user_id, enabled, interval_hours, articles_per_episode, target_duration_minutes, chunk_trailing_silence_seconds, program_name, default_persona_mode, default_persona, conversation_mode, bgm_enabled, bgm_r2_prefix, created_at, updated_at
		FROM audio_briefing_settings
		WHERE user_id = $1
	`, userID).Scan(
		&v.UserID,
		&v.Enabled,
		&v.IntervalHours,
		&v.ArticlesPerEpisode,
		&v.TargetDurationMinutes,
		&v.ChunkTrailingSilenceSeconds,
		&v.ProgramName,
		&v.DefaultPersonaMode,
		&v.DefaultPersona,
		&v.ConversationMode,
		&v.BGMEnabled,
		&v.BGMR2Prefix,
		&v.CreatedAt,
		&v.UpdatedAt,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &v, nil
}

func (r *AudioBriefingRepo) ListEnabledSettings(ctx context.Context) ([]model.AudioBriefingSettings, error) {
	rows, err := r.db.Query(ctx, `
		SELECT user_id, enabled, interval_hours, articles_per_episode, target_duration_minutes, chunk_trailing_silence_seconds, program_name, default_persona_mode, default_persona, conversation_mode, bgm_enabled, bgm_r2_prefix, created_at, updated_at
		FROM audio_briefing_settings
		WHERE enabled = TRUE
		ORDER BY updated_at ASC, user_id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.AudioBriefingSettings, 0)
	for rows.Next() {
		var row model.AudioBriefingSettings
		if err := rows.Scan(
			&row.UserID,
			&row.Enabled,
			&row.IntervalHours,
			&row.ArticlesPerEpisode,
			&row.TargetDurationMinutes,
			&row.ChunkTrailingSilenceSeconds,
			&row.ProgramName,
			&row.DefaultPersonaMode,
			&row.DefaultPersona,
			&row.ConversationMode,
			&row.BGMEnabled,
			&row.BGMR2Prefix,
			&row.CreatedAt,
			&row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *AudioBriefingRepo) UpsertSettings(ctx context.Context, userID string, enabled bool, intervalHours, articlesPerEpisode, targetDurationMinutes int, chunkTrailingSilenceSeconds float64, programName *string, defaultPersonaMode string, defaultPersona string, conversationMode string, bgmEnabled bool, bgmR2Prefix *string) (*model.AudioBriefingSettings, error) {
	if strings.TrimSpace(defaultPersona) == "" {
		defaultPersona = "editor"
	}
	if strings.TrimSpace(defaultPersonaMode) == "" {
		defaultPersonaMode = "fixed"
	}
	if strings.TrimSpace(conversationMode) == "" {
		conversationMode = "single"
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO audio_briefing_settings (
			user_id, enabled, interval_hours, articles_per_episode, target_duration_minutes, chunk_trailing_silence_seconds, program_name, default_persona_mode, default_persona, conversation_mode, bgm_enabled, bgm_r2_prefix
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (user_id) DO UPDATE
		SET enabled = EXCLUDED.enabled,
		    interval_hours = EXCLUDED.interval_hours,
		    articles_per_episode = EXCLUDED.articles_per_episode,
		    target_duration_minutes = EXCLUDED.target_duration_minutes,
		    chunk_trailing_silence_seconds = EXCLUDED.chunk_trailing_silence_seconds,
		    program_name = EXCLUDED.program_name,
		    default_persona_mode = EXCLUDED.default_persona_mode,
		    default_persona = EXCLUDED.default_persona,
		    conversation_mode = EXCLUDED.conversation_mode,
		    bgm_enabled = EXCLUDED.bgm_enabled,
		    bgm_r2_prefix = EXCLUDED.bgm_r2_prefix,
		    updated_at = NOW()
	`, userID, enabled, intervalHours, articlesPerEpisode, targetDurationMinutes, chunkTrailingSilenceSeconds, programName, defaultPersonaMode, defaultPersona, conversationMode, bgmEnabled, bgmR2Prefix)
	if err != nil {
		return nil, err
	}
	return r.GetSettings(ctx, userID)
}

func (r *AudioBriefingRepo) ListPersonaVoicesByUser(ctx context.Context, userID string) ([]model.AudioBriefingPersonaVoice, error) {
	rows, err := r.db.Query(ctx, `
		SELECT user_id, persona, tts_provider, tts_model, voice_model, voice_style, speech_rate, emotional_intensity, tempo_dynamics, line_break_silence_seconds, pitch, volume_gain, created_at, updated_at
		FROM audio_briefing_persona_voices
		WHERE user_id = $1
		ORDER BY persona
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.AudioBriefingPersonaVoice, 0)
	for rows.Next() {
		var row model.AudioBriefingPersonaVoice
		if err := rows.Scan(
			&row.UserID,
			&row.Persona,
			&row.TTSProvider,
			&row.TTSModel,
			&row.VoiceModel,
			&row.VoiceStyle,
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

func (r *AudioBriefingRepo) UpsertPersonaVoices(ctx context.Context, userID string, rows []model.AudioBriefingPersonaVoice) ([]model.AudioBriefingPersonaVoice, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM audio_briefing_persona_voices WHERE user_id = $1`, userID); err != nil {
		return nil, err
	}
	for _, row := range rows {
		if _, err := tx.Exec(ctx, `
			INSERT INTO audio_briefing_persona_voices (
				user_id, persona, tts_provider, tts_model, voice_model, voice_style, speech_rate, emotional_intensity, tempo_dynamics, line_break_silence_seconds, pitch, volume_gain
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`, userID, row.Persona, row.TTSProvider, row.TTSModel, row.VoiceModel, row.VoiceStyle, row.SpeechRate, row.EmotionalIntensity, row.TempoDynamics, row.LineBreakSilenceSeconds, row.Pitch, row.VolumeGain); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.ListPersonaVoicesByUser(ctx, userID)
}

func (r *AudioBriefingRepo) GetPersonaVoice(ctx context.Context, userID, persona string) (*model.AudioBriefingPersonaVoice, error) {
	var row model.AudioBriefingPersonaVoice
	err := r.db.QueryRow(ctx, `
		SELECT user_id, persona, tts_provider, tts_model, voice_model, voice_style, speech_rate, emotional_intensity, tempo_dynamics, line_break_silence_seconds, pitch, volume_gain, created_at, updated_at
		FROM audio_briefing_persona_voices
		WHERE user_id = $1 AND persona = $2
	`, userID, persona).Scan(
		&row.UserID,
		&row.Persona,
		&row.TTSProvider,
		&row.TTSModel,
		&row.VoiceModel,
		&row.VoiceStyle,
		&row.SpeechRate,
		&row.EmotionalIntensity,
		&row.TempoDynamics,
		&row.LineBreakSilenceSeconds,
		&row.Pitch,
		&row.VolumeGain,
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

func (r *AudioBriefingRepo) ListJobsByUser(ctx context.Context, userID string, limit int) ([]model.AudioBriefingJob, error) {
	if limit <= 0 {
		limit = 24
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, slot_started_at_jst, slot_key, persona, conversation_mode, partner_persona, pipeline_stage, status, archive_status,
		       source_item_count, reused_item_count, script_char_count, script_llm_models,
               prompt_key, prompt_source, prompt_version_id, prompt_version_number, prompt_experiment_id, prompt_experiment_arm_id,
               audio_duration_sec,
		       title, r2_audio_object_key, r2_manifest_object_key, bgm_object_key, r2_storage_bucket, podcast_public_object_key, podcast_public_bucket, podcast_public_deleted_at, provider_job_id, idempotency_key,
		       error_code, error_message, published_at, failed_at, created_at, updated_at
		FROM audio_briefing_jobs
		WHERE user_id = $1
		ORDER BY slot_started_at_jst DESC, created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.AudioBriefingJob, 0, limit)
	for rows.Next() {
		row, err := scanAudioBriefingJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *AudioBriefingRepo) ListRecentPersonasByUser(ctx context.Context, userID string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 3
	}
	if limit > 20 {
		limit = 20
	}
	rows, err := r.db.Query(ctx, `
		SELECT persona
		FROM audio_briefing_jobs
		WHERE user_id = $1
		  AND persona <> ''
		ORDER BY slot_started_at_jst DESC, created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0, limit)
	for rows.Next() {
		var persona string
		if err := rows.Scan(&persona); err != nil {
			return nil, err
		}
		out = append(out, persona)
	}
	return out, rows.Err()
}

func (r *AudioBriefingRepo) ListPodcastPublishedJobsByUser(ctx context.Context, userID string, publishedAfter time.Time, limit int) ([]model.AudioBriefingJob, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, listPodcastPublishedJobsByUserQuery(), userID, publishedAfter, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.AudioBriefingJob, 0, limit)
	for rows.Next() {
		row, err := scanAudioBriefingJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func listPodcastPublishedJobsByUserQuery() string {
	return `
		SELECT id, user_id, slot_started_at_jst, slot_key, persona, conversation_mode, partner_persona, pipeline_stage, status, archive_status,
		       source_item_count, reused_item_count, script_char_count, script_llm_models,
               prompt_key, prompt_source, prompt_version_id, prompt_version_number, prompt_experiment_id, prompt_experiment_arm_id,
               audio_duration_sec,
		       title, r2_audio_object_key, r2_manifest_object_key, bgm_object_key, r2_storage_bucket, podcast_public_object_key, podcast_public_bucket, podcast_public_deleted_at, provider_job_id, idempotency_key,
		       error_code, error_message, published_at, failed_at, created_at, updated_at
		FROM audio_briefing_jobs
		WHERE user_id = $1
		  AND status = 'published'
		  AND archive_status = 'active'
		  AND published_at IS NOT NULL
		  AND published_at >= $2
		  AND podcast_public_object_key IS NOT NULL
		ORDER BY created_at DESC, id DESC
		LIMIT $3
	`
}

func (r *AudioBriefingRepo) GetJobByID(ctx context.Context, userID, jobID string) (*model.AudioBriefingJob, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, user_id, slot_started_at_jst, slot_key, persona, conversation_mode, partner_persona, pipeline_stage, status, archive_status,
		       source_item_count, reused_item_count, script_char_count, script_llm_models,
               prompt_key, prompt_source, prompt_version_id, prompt_version_number, prompt_experiment_id, prompt_experiment_arm_id,
               audio_duration_sec,
		       title, r2_audio_object_key, r2_manifest_object_key, bgm_object_key, r2_storage_bucket, podcast_public_object_key, podcast_public_bucket, podcast_public_deleted_at, provider_job_id, idempotency_key,
		       error_code, error_message, published_at, failed_at, created_at, updated_at
		FROM audio_briefing_jobs
		WHERE user_id = $1 AND id = $2
	`, userID, jobID)
	job, err := scanAudioBriefingJob(row)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &job, nil
}

func (r *AudioBriefingRepo) GetJobBySlotKey(ctx context.Context, userID, slotKey string) (*model.AudioBriefingJob, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, user_id, slot_started_at_jst, slot_key, persona, conversation_mode, partner_persona, pipeline_stage, status, archive_status,
		       source_item_count, reused_item_count, script_char_count, script_llm_models,
               prompt_key, prompt_source, prompt_version_id, prompt_version_number, prompt_experiment_id, prompt_experiment_arm_id,
               audio_duration_sec,
		       title, r2_audio_object_key, r2_manifest_object_key, bgm_object_key, r2_storage_bucket, podcast_public_object_key, podcast_public_bucket, podcast_public_deleted_at, provider_job_id, idempotency_key,
		       error_code, error_message, published_at, failed_at, created_at, updated_at
		FROM audio_briefing_jobs
		WHERE user_id = $1 AND slot_key = $2
	`, userID, slotKey)
	job, err := scanAudioBriefingJob(row)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &job, nil
}

func (r *AudioBriefingRepo) ListJobItems(ctx context.Context, userID, jobID string) ([]model.AudioBriefingJobItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT ji.id, ji.job_id, ji.item_id, ji.rank, ji.segment_title, ji.summary_snapshot, ji.created_at,
		       i.title, sm.translated_title, s.title AS source_title, i.published_at
		FROM audio_briefing_job_items ji
		JOIN audio_briefing_jobs j ON j.id = ji.job_id
		JOIN items i ON i.id = ji.item_id
		JOIN sources s ON s.id = i.source_id
		LEFT JOIN item_summaries sm ON sm.item_id = i.id
		WHERE j.user_id = $1 AND ji.job_id = $2
		ORDER BY ji.rank ASC
	`, userID, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.AudioBriefingJobItem, 0)
	for rows.Next() {
		var row model.AudioBriefingJobItem
		if err := rows.Scan(
			&row.ID,
			&row.JobID,
			&row.ItemID,
			&row.Rank,
			&row.SegmentTitle,
			&row.SummarySnapshot,
			&row.CreatedAt,
			&row.Title,
			&row.TranslatedTitle,
			&row.SourceTitle,
			&row.PublishedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *AudioBriefingRepo) ListJobChunks(ctx context.Context, userID, jobID string) ([]model.AudioBriefingScriptChunk, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.id, c.job_id, c.seq, c.part_type, c.speaker, c.text, c.char_count, c.tts_status, c.attempt_count, c.last_error_code,
		       c.tts_provider, c.voice_model, c.voice_style, c.r2_audio_object_key,
		       c.r2_storage_bucket, c.duration_sec, c.error_message, c.heartbeat_token, c.last_heartbeat_at, c.started_at, c.completed_at, c.created_at, c.updated_at
		FROM audio_briefing_script_chunks c
		JOIN audio_briefing_jobs j ON j.id = c.job_id
		WHERE j.user_id = $1 AND c.job_id = $2
		ORDER BY c.seq ASC
	`, userID, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.AudioBriefingScriptChunk, 0)
	for rows.Next() {
		row, err := scanAudioBriefingScriptChunk(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *AudioBriefingRepo) DeleteJob(ctx context.Context, userID, jobID string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM audio_briefing_jobs
		WHERE user_id = $1 AND id = $2
	`, userID, jobID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *AudioBriefingRepo) ListIAMoveCandidates(ctx context.Context, cutoff time.Time, limit int) ([]model.AudioBriefingJob, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, slot_started_at_jst, slot_key, persona, conversation_mode, partner_persona, pipeline_stage, status, archive_status,
		       source_item_count, reused_item_count, script_char_count, script_llm_models,
               prompt_key, prompt_source, prompt_version_id, prompt_version_number, prompt_experiment_id, prompt_experiment_arm_id,
               audio_duration_sec,
		       title, r2_audio_object_key, r2_manifest_object_key, bgm_object_key, r2_storage_bucket, podcast_public_object_key, podcast_public_bucket, podcast_public_deleted_at, provider_job_id, idempotency_key,
		       error_code, error_message, published_at, failed_at, created_at, updated_at
		FROM audio_briefing_jobs
		WHERE status = 'published'
		  AND published_at IS NOT NULL
		  AND published_at <= $1
		  AND r2_audio_object_key IS NOT NULL
		ORDER BY published_at ASC, id ASC
		LIMIT $2
	`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.AudioBriefingJob, 0, limit)
	for rows.Next() {
		row, err := scanAudioBriefingJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *AudioBriefingRepo) ListStaleVoicingJobs(ctx context.Context, cutoff time.Time, limit int) ([]model.AudioBriefingJob, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(ctx, listStaleVoicingJobsQuery(), cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.AudioBriefingJob, 0, limit)
	for rows.Next() {
		row, err := scanAudioBriefingJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func listStaleVoicingJobsQuery() string {
	return `
		SELECT j.id, j.user_id, j.slot_started_at_jst, j.slot_key, j.persona, j.conversation_mode, j.partner_persona, j.pipeline_stage, j.status, j.archive_status,
		       j.source_item_count, j.reused_item_count, j.script_char_count, j.script_llm_models,
               j.prompt_key, j.prompt_source, j.prompt_version_id, j.prompt_version_number, j.prompt_experiment_id, j.prompt_experiment_arm_id,
               j.audio_duration_sec,
		       j.title, j.r2_audio_object_key, j.r2_manifest_object_key, j.bgm_object_key, j.r2_storage_bucket, j.podcast_public_object_key, j.podcast_public_bucket, j.podcast_public_deleted_at, j.provider_job_id, j.idempotency_key,
		       j.error_code, j.error_message, j.published_at, j.failed_at, j.created_at, j.updated_at
		FROM audio_briefing_jobs j
		WHERE j.status = 'voicing'
		  AND EXISTS (
		    SELECT 1
		    FROM audio_briefing_script_chunks c
		    WHERE c.job_id = j.id
		      AND c.tts_status = 'generating'
		      AND COALESCE(c.last_heartbeat_at, c.updated_at) <= $1
		  )
		ORDER BY j.updated_at ASC, j.id ASC
		LIMIT $2
	`
}

func (r *AudioBriefingRepo) UpdateStorageBucketForJobAndChunks(ctx context.Context, jobID string, bucket string) error {
	bucket = strings.TrimSpace(bucket)
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		UPDATE audio_briefing_script_chunks
		SET r2_storage_bucket = $2,
		    updated_at = NOW()
		WHERE job_id = $1
		  AND r2_audio_object_key IS NOT NULL
	`, jobID, bucket); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE audio_briefing_jobs
		SET r2_storage_bucket = $2,
		    updated_at = NOW()
		WHERE id = $1
	`, jobID, bucket)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

func (r *AudioBriefingRepo) UpdateArchiveStatus(ctx context.Context, userID, jobID, archiveStatus string) (*model.AudioBriefingJob, error) {
	archiveStatus = strings.TrimSpace(archiveStatus)
	if archiveStatus != model.AudioBriefingArchiveStatusArchived {
		archiveStatus = model.AudioBriefingArchiveStatusActive
	}
	job, err := scanAudioBriefingJob(r.db.QueryRow(ctx, `
		UPDATE audio_briefing_jobs
		SET archive_status = $3,
		    updated_at = NOW()
		WHERE id = $1
		  AND user_id = $2
		  AND status = 'published'
		RETURNING id, user_id, slot_started_at_jst, slot_key, persona, conversation_mode, partner_persona, pipeline_stage, status, archive_status,
		          source_item_count, reused_item_count, script_char_count, script_llm_models,
               prompt_key, prompt_source, prompt_version_id, prompt_version_number, prompt_experiment_id, prompt_experiment_arm_id,
               audio_duration_sec,
		          title, r2_audio_object_key, r2_manifest_object_key, bgm_object_key, r2_storage_bucket, podcast_public_object_key, podcast_public_bucket, podcast_public_deleted_at, provider_job_id, idempotency_key,
		          error_code, error_message, published_at, failed_at, created_at, updated_at
	`, jobID, userID, archiveStatus))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrInvalidState
		}
		return nil, err
	}
	return &job, nil
}

func (r *AudioBriefingRepo) SetPodcastPublicObject(ctx context.Context, jobID string, bucket string, objectKey string) (*model.AudioBriefingJob, error) {
	bucket = strings.TrimSpace(bucket)
	objectKey = strings.TrimSpace(objectKey)
	if bucket == "" || objectKey == "" {
		return nil, ErrInvalidState
	}
	job, err := scanAudioBriefingJob(r.db.QueryRow(ctx, `
		UPDATE audio_briefing_jobs
		SET podcast_public_bucket = $2,
		    podcast_public_object_key = $3,
		    podcast_public_deleted_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, user_id, slot_started_at_jst, slot_key, persona, conversation_mode, partner_persona, pipeline_stage, status, archive_status,
		          source_item_count, reused_item_count, script_char_count, script_llm_models,
               prompt_key, prompt_source, prompt_version_id, prompt_version_number, prompt_experiment_id, prompt_experiment_arm_id,
               audio_duration_sec,
		          title, r2_audio_object_key, r2_manifest_object_key, bgm_object_key, r2_storage_bucket, podcast_public_object_key, podcast_public_bucket, podcast_public_deleted_at, provider_job_id, idempotency_key,
		          error_code, error_message, published_at, failed_at, created_at, updated_at
	`, jobID, bucket, objectKey))
	if err != nil {
		return nil, mapDBError(err)
	}
	return &job, nil
}

func (r *AudioBriefingRepo) MarkPodcastPublicObjectDeleted(ctx context.Context, jobID string) (*model.AudioBriefingJob, error) {
	job, err := scanAudioBriefingJob(r.db.QueryRow(ctx, `
		UPDATE audio_briefing_jobs
		SET podcast_public_deleted_at = COALESCE(podcast_public_deleted_at, NOW()),
		    podcast_public_object_key = NULL,
		    podcast_public_bucket = '',
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, user_id, slot_started_at_jst, slot_key, persona, conversation_mode, partner_persona, pipeline_stage, status, archive_status,
		          source_item_count, reused_item_count, script_char_count, script_llm_models,
               prompt_key, prompt_source, prompt_version_id, prompt_version_number, prompt_experiment_id, prompt_experiment_arm_id,
               audio_duration_sec,
		          title, r2_audio_object_key, r2_manifest_object_key, bgm_object_key, r2_storage_bucket, podcast_public_object_key, podcast_public_bucket, podcast_public_deleted_at, provider_job_id, idempotency_key,
		          error_code, error_message, published_at, failed_at, created_at, updated_at
	`, jobID))
	if err != nil {
		return nil, mapDBError(err)
	}
	return &job, nil
}

func audioBriefingCandidateItemsQuery(userID string, intervalHours int, limit int) (string, []any) {
	if intervalHours <= 0 {
		intervalHours = 6
	}
	query := `
		SELECT i.id,
		       i.title,
		       sm.translated_title,
		       s.title AS source_title,
		       sm.summary,
		       COALESCE(i.published_at, i.created_at) AS effective_published_at
		FROM items i
		JOIN sources s ON s.id = i.source_id
		JOIN item_summaries sm ON sm.item_id = i.id
		WHERE s.user_id = $1
		  AND i.deleted_at IS NULL
		  AND i.status = 'summarized'
		  AND NULLIF(BTRIM(sm.summary), '') IS NOT NULL
		ORDER BY CASE
		           WHEN COALESCE(i.fetched_at, i.created_at) >= NOW() - make_interval(hours => $2::int) THEN 0
		           ELSE 1
		         END,
		         COALESCE(i.fetched_at, i.created_at) DESC,
		         COALESCE(i.published_at, i.created_at) DESC,
		         sm.score DESC NULLS LAST
		LIMIT $3`
	return query, []any{userID, intervalHours, limit}
}

func (r *AudioBriefingRepo) ListCandidateItems(ctx context.Context, userID string, intervalHours int, limit int) ([]model.AudioBriefingJobItem, error) {
	if limit <= 0 {
		limit = 6
	}
	if limit > 30 {
		limit = 30
	}
	query, args := audioBriefingCandidateItemsQuery(userID, intervalHours, limit)
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.AudioBriefingJobItem, 0, limit)
	rank := 1
	for rows.Next() {
		var row model.AudioBriefingJobItem
		if err := rows.Scan(
			&row.ItemID,
			&row.Title,
			&row.TranslatedTitle,
			&row.SourceTitle,
			&row.SummarySnapshot,
			&row.PublishedAt,
		); err != nil {
			return nil, err
		}
		row.Rank = rank
		rank++
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *AudioBriefingRepo) CreateJobWithContent(
	ctx context.Context,
	userID string,
	slotStartedAtJST time.Time,
	slotKey string,
	persona string,
	status string,
	title *string,
	scriptCharCount int,
	items []model.AudioBriefingJobItem,
	chunks []model.AudioBriefingScriptChunk,
) (*model.AudioBriefingJob, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var jobID string
	standardBucket := audioBriefingStandardBucket()
	err = tx.QueryRow(ctx, `
		INSERT INTO audio_briefing_jobs (
			user_id, slot_started_at_jst, slot_key, persona, conversation_mode, status,
			source_item_count, reused_item_count, script_char_count, title, r2_storage_bucket
		) VALUES ($1, $2, $3, $4, 'single', $5, $6, 0, $7, $8, $9)
		RETURNING id
	`, userID, slotStartedAtJST, slotKey, persona, status, len(items), scriptCharCount, title, standardBucket).Scan(&jobID)
	if err != nil {
		return nil, mapDBError(err)
	}

	for _, item := range items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO audio_briefing_job_items (
				job_id, item_id, rank, segment_title, summary_snapshot
			) VALUES ($1, $2, $3, $4, $5)
		`, jobID, item.ItemID, item.Rank, item.SegmentTitle, item.SummarySnapshot); err != nil {
			return nil, err
		}
	}

	for _, chunk := range chunks {
		if _, err := tx.Exec(ctx, `
			INSERT INTO audio_briefing_script_chunks (
				job_id, seq, part_type, speaker, text, char_count, tts_status, tts_provider, voice_model, voice_style, r2_storage_bucket
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`, jobID, chunk.Seq, chunk.PartType, chunk.Speaker, chunk.Text, chunk.CharCount, chunk.TTSStatus, chunk.TTSProvider, chunk.VoiceModel, chunk.VoiceStyle, firstNonEmpty(chunk.R2StorageBucket, standardBucket)); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.GetJobByID(ctx, userID, jobID)
}

func (r *AudioBriefingRepo) CreatePendingJob(
	ctx context.Context,
	userID string,
	slotStartedAtJST time.Time,
	slotKey string,
	persona string,
	conversationMode string,
	pipelineStage string,
) (*model.AudioBriefingJob, error) {
	if strings.TrimSpace(conversationMode) == "" {
		conversationMode = "single"
	}
	if strings.TrimSpace(pipelineStage) == "" {
		pipelineStage = "single_script"
	}
	var jobID string
	standardBucket := audioBriefingStandardBucket()
	err := r.db.QueryRow(ctx, `
		INSERT INTO audio_briefing_jobs (
			user_id, slot_started_at_jst, slot_key, persona, conversation_mode, status, pipeline_stage, r2_storage_bucket
		) VALUES ($1, $2, $3, $4, $5, 'pending', $6, $7)
		RETURNING id
	`, userID, slotStartedAtJST, slotKey, persona, conversationMode, pipelineStage, standardBucket).Scan(&jobID)
	if err != nil {
		return nil, mapDBError(err)
	}
	return r.GetJobByID(ctx, userID, jobID)
}

func (r *AudioBriefingRepo) SetPartnerPersona(ctx context.Context, jobID string, partnerPersona string) (*model.AudioBriefingJob, error) {
	partnerPersona = strings.TrimSpace(partnerPersona)
	if partnerPersona == "" {
		return nil, ErrInvalidState
	}
	job, err := scanAudioBriefingJob(r.db.QueryRow(ctx, `
		UPDATE audio_briefing_jobs
		SET partner_persona = $2,
		    updated_at = NOW()
		WHERE id = $1
		  AND status IN ('pending', 'scripting', 'failed')
		RETURNING id, user_id, slot_started_at_jst, slot_key, persona, conversation_mode, partner_persona, pipeline_stage, status, archive_status,
		          source_item_count, reused_item_count, script_char_count, script_llm_models,
               prompt_key, prompt_source, prompt_version_id, prompt_version_number, prompt_experiment_id, prompt_experiment_arm_id,
               audio_duration_sec,
		          title, r2_audio_object_key, r2_manifest_object_key, bgm_object_key, r2_storage_bucket, podcast_public_object_key, podcast_public_bucket, podcast_public_deleted_at, provider_job_id, idempotency_key,
		          error_code, error_message, published_at, failed_at, created_at, updated_at
	`, jobID, partnerPersona))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrInvalidState
		}
		return nil, err
	}
	return &job, nil
}

func (r *AudioBriefingRepo) StartScriptingJob(ctx context.Context, jobID string) (*model.AudioBriefingJob, error) {
	job, err := scanAudioBriefingJob(r.db.QueryRow(ctx, `
		UPDATE audio_briefing_jobs
		SET status = 'scripting',
		    pipeline_stage = CASE
		      WHEN conversation_mode = 'duo' THEN 'duo_script'
		      ELSE 'single_script'
		    END,
		    error_code = NULL,
		    error_message = NULL,
		    failed_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
		  AND (
		    status = 'pending'
		    OR status = 'scripting'
		    OR (
		      status = 'failed'
		      AND NOT EXISTS (
		        SELECT 1
		        FROM audio_briefing_script_chunks
		        WHERE job_id = $1
		      )
		    )
		  )
		RETURNING id, user_id, slot_started_at_jst, slot_key, persona, conversation_mode, partner_persona, pipeline_stage, status, archive_status,
		          source_item_count, reused_item_count, script_char_count, script_llm_models,
               prompt_key, prompt_source, prompt_version_id, prompt_version_number, prompt_experiment_id, prompt_experiment_arm_id,
               audio_duration_sec,
		          title, r2_audio_object_key, r2_manifest_object_key, bgm_object_key, r2_storage_bucket, podcast_public_object_key, podcast_public_bucket, podcast_public_deleted_at, provider_job_id, idempotency_key,
		          error_code, error_message, published_at, failed_at, created_at, updated_at
	`, jobID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrInvalidState
		}
		return nil, err
	}
	return &job, nil
}

func (r *AudioBriefingRepo) CompleteScriptingJob(
	ctx context.Context,
	jobID string,
	status string,
	title *string,
	scriptCharCount int,
	scriptLLMModels *string,
	prompt AudioBriefingPromptMetadata,
	items []model.AudioBriefingJobItem,
	chunks []model.AudioBriefingScriptChunk,
) (*model.AudioBriefingJob, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	job, err := scanAudioBriefingJob(tx.QueryRow(ctx, `
		UPDATE audio_briefing_jobs
		SET status = $2,
		    source_item_count = $3,
		    reused_item_count = 0,
		    script_char_count = $4,
		    script_llm_models = NULLIF($5, ''),
		    prompt_key = NULLIF($6, ''),
		    prompt_source = NULLIF($7, ''),
		    prompt_version_id = $8,
		    prompt_version_number = $9,
		    prompt_experiment_id = $10,
		    prompt_experiment_arm_id = $11,
		    title = $12,
		    error_code = NULL,
		    error_message = NULL,
		    failed_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
		  AND status = 'scripting'
		RETURNING id, user_id, slot_started_at_jst, slot_key, persona, conversation_mode, partner_persona, pipeline_stage, status, archive_status,
		          source_item_count, reused_item_count, script_char_count, script_llm_models,
               prompt_key, prompt_source, prompt_version_id, prompt_version_number, prompt_experiment_id, prompt_experiment_arm_id,
               audio_duration_sec,
		          title, r2_audio_object_key, r2_manifest_object_key, bgm_object_key, r2_storage_bucket, podcast_public_object_key, podcast_public_bucket, podcast_public_deleted_at, provider_job_id, idempotency_key,
		          error_code, error_message, published_at, failed_at, created_at, updated_at
	`, jobID, strings.TrimSpace(status), len(items), scriptCharCount, strings.TrimSpace(valueOrEmpty(scriptLLMModels)), strings.TrimSpace(valueOrEmpty(prompt.PromptKey)), strings.TrimSpace(valueOrEmpty(prompt.PromptSource)), prompt.PromptVersionID, prompt.PromptVersionNumber, prompt.PromptExperimentID, prompt.PromptExperimentArmID, title))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrInvalidState
		}
		return nil, err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM audio_briefing_job_items WHERE job_id = $1`, jobID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM audio_briefing_script_chunks WHERE job_id = $1`, jobID); err != nil {
		return nil, err
	}

	for _, item := range items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO audio_briefing_job_items (
				job_id, item_id, rank, segment_title, summary_snapshot
			) VALUES ($1, $2, $3, $4, $5)
		`, jobID, item.ItemID, item.Rank, item.SegmentTitle, item.SummarySnapshot); err != nil {
			return nil, err
		}
	}

	for _, chunk := range chunks {
		if _, err := tx.Exec(ctx, `
			INSERT INTO audio_briefing_script_chunks (
				job_id, seq, part_type, speaker, text, char_count, tts_status, tts_provider, voice_model, voice_style, r2_storage_bucket
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`, jobID, chunk.Seq, chunk.PartType, chunk.Speaker, chunk.Text, chunk.CharCount, chunk.TTSStatus, chunk.TTSProvider, chunk.VoiceModel, chunk.VoiceStyle, firstNonEmpty(chunk.R2StorageBucket, audioBriefingStandardBucket())); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *AudioBriefingRepo) FailScriptingJob(ctx context.Context, jobID string, errorCode string, errorMessage string) (*model.AudioBriefingJob, error) {
	errorCode = strings.TrimSpace(errorCode)
	if errorCode == "" {
		errorCode = "script_failed"
	}
	job, err := scanAudioBriefingJob(r.db.QueryRow(ctx, `
		UPDATE audio_briefing_jobs
		SET status = 'failed',
		    error_code = $2,
		    error_message = NULLIF($3, ''),
		    failed_at = COALESCE(failed_at, NOW()),
		    updated_at = NOW()
		WHERE id = $1
		  AND status IN ('pending', 'scripting')
		RETURNING id, user_id, slot_started_at_jst, slot_key, persona, conversation_mode, partner_persona, pipeline_stage, status, archive_status,
		          source_item_count, reused_item_count, script_char_count, script_llm_models,
               prompt_key, prompt_source, prompt_version_id, prompt_version_number, prompt_experiment_id, prompt_experiment_arm_id,
               audio_duration_sec,
		          title, r2_audio_object_key, r2_manifest_object_key, bgm_object_key, r2_storage_bucket, podcast_public_object_key, podcast_public_bucket, podcast_public_deleted_at, provider_job_id, idempotency_key,
		          error_code, error_message, published_at, failed_at, created_at, updated_at
	`, jobID, errorCode, strings.TrimSpace(errorMessage)))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrInvalidState
		}
		return nil, err
	}
	return &job, nil
}

func (r *AudioBriefingRepo) BeginConcatCallback(
	ctx context.Context,
	jobID string,
	requestID string,
	tokenHash string,
	providerJobID *string,
	manifestObjectKey *string,
	expiresAt time.Time,
) (*model.AudioBriefingJob, *model.AudioBriefingCallbackToken, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	job, err := scanAudioBriefingJob(tx.QueryRow(ctx, `
		UPDATE audio_briefing_jobs
		SET status = 'concatenating',
		    pipeline_stage = CASE
		      WHEN conversation_mode = 'duo' THEN 'duo_concat'
		      ELSE 'single_concat'
		    END,
		    provider_job_id = COALESCE($2, provider_job_id),
		    r2_manifest_object_key = COALESCE($3, r2_manifest_object_key),
		    bgm_object_key = NULL,
		    error_code = NULL,
		    error_message = NULL,
		    failed_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
		  AND (
		    status IN ('scripted', 'voiced', 'concatenating')
		    OR (
		      status = 'failed'
		      AND NOT EXISTS (
		        SELECT 1
		        FROM audio_briefing_script_chunks
		        WHERE job_id = $1
		          AND tts_status <> 'generated'
		      )
		    )
		  )
		RETURNING id, user_id, slot_started_at_jst, slot_key, persona, conversation_mode, partner_persona, pipeline_stage, status, archive_status,
		          source_item_count, reused_item_count, script_char_count, script_llm_models,
               prompt_key, prompt_source, prompt_version_id, prompt_version_number, prompt_experiment_id, prompt_experiment_arm_id,
               audio_duration_sec,
		          title, r2_audio_object_key, r2_manifest_object_key, bgm_object_key, r2_storage_bucket, podcast_public_object_key, podcast_public_bucket, podcast_public_deleted_at, provider_job_id, idempotency_key,
		          error_code, error_message, published_at, failed_at, created_at, updated_at
	`, jobID, providerJobID, manifestObjectKey))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil, ErrInvalidState
		}
		return nil, nil, err
	}

	var token model.AudioBriefingCallbackToken
	err = tx.QueryRow(ctx, `
		INSERT INTO audio_briefing_callback_tokens (
			job_id, request_id, provider_job_id, token_hash, expires_at
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING id, job_id, request_id, provider_job_id, token_hash, expires_at, used_at, created_at
	`, jobID, requestID, providerJobID, tokenHash, expiresAt).Scan(
		&token.ID,
		&token.JobID,
		&token.RequestID,
		&token.ProviderJobID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.UsedAt,
		&token.CreatedAt,
	)
	if err != nil {
		return nil, nil, mapDBError(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return &job, &token, nil
}

func (r *AudioBriefingRepo) FinalizeConcatJob(
	ctx context.Context,
	jobID string,
	requestID string,
	tokenHash string,
	providerJobID *string,
	status string,
	audioObjectKey *string,
	manifestObjectKey *string,
	bgmObjectKey *string,
	audioDurationSec *int,
	errorCode *string,
	errorMessage *string,
) (*model.AudioBriefingJob, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var token model.AudioBriefingCallbackToken
	err = tx.QueryRow(ctx, `
		SELECT id, job_id, request_id, provider_job_id, token_hash, expires_at, used_at, created_at
		FROM audio_briefing_callback_tokens
		WHERE job_id = $1 AND request_id = $2 AND token_hash = $3
		FOR UPDATE
	`, jobID, requestID, tokenHash).Scan(
		&token.ID,
		&token.JobID,
		&token.RequestID,
		&token.ProviderJobID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.UsedAt,
		&token.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	if token.ExpiresAt.Before(time.Now().UTC()) {
		return nil, ErrUnauthorized
	}
	if !audioBriefingProviderJobMatches(token.ProviderJobID, providerJobID) {
		return nil, ErrUnauthorized
	}

	if token.UsedAt != nil {
		job, err := getAudioBriefingJobByIDTx(ctx, tx, jobID)
		if err != nil {
			return nil, err
		}
		if job.Status == status {
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return job, nil
		}
		return nil, ErrConflict
	}

	if _, err := tx.Exec(ctx, `
		UPDATE audio_briefing_callback_tokens
		SET used_at = NOW()
		WHERE id = $1
	`, token.ID); err != nil {
		return nil, err
	}

	var job *model.AudioBriefingJob
	switch status {
	case "published":
		job, err = updateAudioBriefingJobPublishedTx(ctx, tx, jobID, providerJobID, audioObjectKey, manifestObjectKey, bgmObjectKey, audioDurationSec)
	case "failed":
		job, err = updateAudioBriefingJobFailedTx(ctx, tx, jobID, providerJobID, errorCode, errorMessage)
	default:
		return nil, ErrInvalidState
	}
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return job, nil
}

func (r *AudioBriefingRepo) UpdateConcatProviderJobID(ctx context.Context, jobID string, providerJobID string) (*model.AudioBriefingJob, error) {
	providerJobID = strings.TrimSpace(providerJobID)
	if providerJobID == "" {
		return nil, ErrInvalidState
	}
	job, err := scanAudioBriefingJob(r.db.QueryRow(ctx, `
		UPDATE audio_briefing_jobs
		SET provider_job_id = $2,
		    updated_at = NOW()
		WHERE id = $1
		  AND status = 'concatenating'
		RETURNING id, user_id, slot_started_at_jst, slot_key, persona, conversation_mode, partner_persona, pipeline_stage, status, archive_status,
		          source_item_count, reused_item_count, script_char_count, script_llm_models,
               prompt_key, prompt_source, prompt_version_id, prompt_version_number, prompt_experiment_id, prompt_experiment_arm_id,
               audio_duration_sec,
		          title, r2_audio_object_key, r2_manifest_object_key, bgm_object_key, r2_storage_bucket, podcast_public_object_key, podcast_public_bucket, podcast_public_deleted_at, provider_job_id, idempotency_key,
		          error_code, error_message, published_at, failed_at, created_at, updated_at
	`, jobID, providerJobID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrInvalidState
		}
		return nil, err
	}
	return &job, nil
}

func (r *AudioBriefingRepo) FailConcatLaunch(ctx context.Context, jobID string, errorCode string, errorMessage string) (*model.AudioBriefingJob, error) {
	errorCode = strings.TrimSpace(errorCode)
	if errorCode == "" {
		errorCode = "concat_launch_failed"
	}
	errorMessage = strings.TrimSpace(errorMessage)
	job, err := scanAudioBriefingJob(r.db.QueryRow(ctx, `
		UPDATE audio_briefing_jobs
		SET status = 'failed',
		    error_code = $2,
		    error_message = NULLIF($3, ''),
		    failed_at = COALESCE(failed_at, NOW()),
		    updated_at = NOW()
		WHERE id = $1
		  AND status = 'concatenating'
		RETURNING id, user_id, slot_started_at_jst, slot_key, persona, conversation_mode, partner_persona, pipeline_stage, status, archive_status,
		          source_item_count, reused_item_count, script_char_count, script_llm_models,
               prompt_key, prompt_source, prompt_version_id, prompt_version_number, prompt_experiment_id, prompt_experiment_arm_id,
               audio_duration_sec,
		          title, r2_audio_object_key, r2_manifest_object_key, bgm_object_key, r2_storage_bucket, podcast_public_object_key, podcast_public_bucket, podcast_public_deleted_at, provider_job_id, idempotency_key,
		          error_code, error_message, published_at, failed_at, created_at, updated_at
	`, jobID, errorCode, errorMessage))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrInvalidState
		}
		return nil, err
	}
	return &job, nil
}

func (r *AudioBriefingRepo) StartVoicingJob(ctx context.Context, jobID string) (*model.AudioBriefingJob, error) {
	job, err := scanAudioBriefingJob(r.db.QueryRow(ctx, `
		UPDATE audio_briefing_jobs
		SET status = 'voicing',
		    pipeline_stage = CASE
		      WHEN conversation_mode = 'duo' THEN 'duo_voicing'
		      ELSE 'single_voicing'
		    END,
		    error_code = NULL,
		    error_message = NULL,
		    failed_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
		  AND (
		    status = 'scripted'
		    OR (
		      status = 'failed'
		      AND EXISTS (
		        SELECT 1
		        FROM audio_briefing_script_chunks
		        WHERE job_id = $1
		          AND tts_status <> 'generated'
		      )
		    )
		  )
		RETURNING id, user_id, slot_started_at_jst, slot_key, persona, conversation_mode, partner_persona, pipeline_stage, status, archive_status,
		          source_item_count, reused_item_count, script_char_count, script_llm_models,
               prompt_key, prompt_source, prompt_version_id, prompt_version_number, prompt_experiment_id, prompt_experiment_arm_id,
               audio_duration_sec,
		          title, r2_audio_object_key, r2_manifest_object_key, bgm_object_key, r2_storage_bucket, podcast_public_object_key, podcast_public_bucket, podcast_public_deleted_at, provider_job_id, idempotency_key,
		          error_code, error_message, published_at, failed_at, created_at, updated_at
	`, jobID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrInvalidState
		}
		return nil, err
	}
	return &job, nil
}

func (r *AudioBriefingRepo) ResetChunksForVoicing(ctx context.Context, jobID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE audio_briefing_script_chunks
		SET tts_status = 'pending',
		    last_error_code = NULL,
		    error_message = NULL,
		    heartbeat_token = NULL,
		    last_heartbeat_at = NULL,
		    started_at = NULL,
		    completed_at = NULL,
		    updated_at = NOW()
		WHERE job_id = $1
		  AND tts_status <> 'generated'
	`, jobID)
	return err
}

func (r *AudioBriefingRepo) StartChunkGenerating(ctx context.Context, chunkID string, heartbeatTokenHash string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE audio_briefing_script_chunks
		SET tts_status = 'generating',
		    attempt_count = attempt_count + 1,
		    last_error_code = NULL,
		    error_message = NULL,
		    heartbeat_token = NULLIF($2, ''),
		    last_heartbeat_at = NOW(),
		    started_at = NOW(),
		    completed_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
		  AND tts_status IN ('pending', 'retry_wait', 'failed')
	`, chunkID, strings.TrimSpace(heartbeatTokenHash))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInvalidState
	}
	return nil
}

func (r *AudioBriefingRepo) MarkChunkGenerated(ctx context.Context, chunkID string, audioObjectKey string, durationSec int) error {
	standardBucket := audioBriefingStandardBucket()
	tag, err := r.db.Exec(ctx, `
		UPDATE audio_briefing_script_chunks
		SET tts_status = 'generated',
		    r2_audio_object_key = $2,
		    duration_sec = $3,
		    r2_storage_bucket = COALESCE(NULLIF($4, ''), r2_storage_bucket),
		    last_error_code = NULL,
		    error_message = NULL,
		    heartbeat_token = NULL,
		    last_heartbeat_at = NULL,
		    completed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
	`, chunkID, strings.TrimSpace(audioObjectKey), durationSec, standardBucket)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *AudioBriefingRepo) MarkChunkRetryWait(ctx context.Context, chunkID string, errorCode string, errorMessage string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE audio_briefing_script_chunks
		SET tts_status = 'retry_wait',
		    last_error_code = NULLIF($2, ''),
		    error_message = NULLIF($3, ''),
		    heartbeat_token = NULL,
		    last_heartbeat_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
		  AND tts_status = 'generating'
	`, chunkID, strings.TrimSpace(errorCode), strings.TrimSpace(errorMessage))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInvalidState
	}
	return nil
}

func (r *AudioBriefingRepo) MarkChunkExhausted(ctx context.Context, chunkID string, errorCode string, errorMessage string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE audio_briefing_script_chunks
		SET tts_status = 'exhausted',
		    last_error_code = NULLIF($2, ''),
		    error_message = NULLIF($3, ''),
		    heartbeat_token = NULL,
		    last_heartbeat_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
		  AND tts_status = 'generating'
	`, chunkID, strings.TrimSpace(errorCode), strings.TrimSpace(errorMessage))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInvalidState
	}
	return nil
}

func (r *AudioBriefingRepo) MarkChunkFailed(ctx context.Context, chunkID string, errorMessage string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE audio_briefing_script_chunks
		SET tts_status = 'failed',
		    last_error_code = NULL,
		    error_message = NULLIF($2, ''),
		    heartbeat_token = NULL,
		    last_heartbeat_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
	`, chunkID, strings.TrimSpace(errorMessage))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *AudioBriefingRepo) TouchChunkHeartbeat(ctx context.Context, chunkID string, heartbeatTokenHash string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE audio_briefing_script_chunks
		SET last_heartbeat_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		  AND tts_status = 'generating'
		  AND heartbeat_token = NULLIF($2, '')
	`, chunkID, strings.TrimSpace(heartbeatTokenHash))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUnauthorized
	}
	return nil
}

func (r *AudioBriefingRepo) CompleteVoicingJob(ctx context.Context, jobID string) (*model.AudioBriefingJob, error) {
	job, err := scanAudioBriefingJob(r.db.QueryRow(ctx, `
		UPDATE audio_briefing_jobs
		SET status = 'voiced',
		    error_code = NULL,
		    error_message = NULL,
		    failed_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
		  AND status = 'voicing'
		  AND NOT EXISTS (
		    SELECT 1
		    FROM audio_briefing_script_chunks
		    WHERE job_id = $1
		      AND tts_status <> 'generated'
		  )
		RETURNING id, user_id, slot_started_at_jst, slot_key, persona, conversation_mode, partner_persona, pipeline_stage, status, archive_status,
		          source_item_count, reused_item_count, script_char_count, script_llm_models,
               prompt_key, prompt_source, prompt_version_id, prompt_version_number, prompt_experiment_id, prompt_experiment_arm_id,
               audio_duration_sec,
		          title, r2_audio_object_key, r2_manifest_object_key, bgm_object_key, r2_storage_bucket, podcast_public_object_key, podcast_public_bucket, podcast_public_deleted_at, provider_job_id, idempotency_key,
		          error_code, error_message, published_at, failed_at, created_at, updated_at
	`, jobID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrInvalidState
		}
		return nil, err
	}
	return &job, nil
}

func (r *AudioBriefingRepo) FailVoicingJob(ctx context.Context, jobID string, errorCode string, errorMessage string) (*model.AudioBriefingJob, error) {
	errorCode = strings.TrimSpace(errorCode)
	if errorCode == "" {
		errorCode = "tts_failed"
	}
	job, err := scanAudioBriefingJob(r.db.QueryRow(ctx, `
		UPDATE audio_briefing_jobs
		SET status = 'failed',
		    error_code = $2,
		    error_message = NULLIF($3, ''),
		    failed_at = COALESCE(failed_at, NOW()),
		    updated_at = NOW()
		WHERE id = $1
		  AND status IN ('scripted', 'voicing')
		RETURNING id, user_id, slot_started_at_jst, slot_key, persona, conversation_mode, partner_persona, pipeline_stage, status, archive_status,
		          source_item_count, reused_item_count, script_char_count, script_llm_models,
               prompt_key, prompt_source, prompt_version_id, prompt_version_number, prompt_experiment_id, prompt_experiment_arm_id,
               audio_duration_sec,
		          title, r2_audio_object_key, r2_manifest_object_key, bgm_object_key, r2_storage_bucket, podcast_public_object_key, podcast_public_bucket, podcast_public_deleted_at, provider_job_id, idempotency_key,
		          error_code, error_message, published_at, failed_at, created_at, updated_at
	`, jobID, errorCode, strings.TrimSpace(errorMessage)))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrInvalidState
		}
		return nil, err
	}
	return &job, nil
}

type audioBriefingJobScanner interface {
	Scan(dest ...any) error
}

func scanAudioBriefingJob(row audioBriefingJobScanner) (model.AudioBriefingJob, error) {
	var job model.AudioBriefingJob
	err := row.Scan(
		&job.ID,
		&job.UserID,
		&job.SlotStartedAtJST,
		&job.SlotKey,
		&job.Persona,
		&job.ConversationMode,
		&job.PartnerPersona,
		&job.PipelineStage,
		&job.Status,
		&job.ArchiveStatus,
		&job.SourceItemCount,
		&job.ReusedItemCount,
		&job.ScriptCharCount,
		&job.ScriptLLMModels,
		&job.PromptKey,
		&job.PromptSource,
		&job.PromptVersionID,
		&job.PromptVersionNumber,
		&job.PromptExperimentID,
		&job.PromptExperimentArmID,
		&job.AudioDurationSec,
		&job.Title,
		&job.R2AudioObjectKey,
		&job.R2ManifestObjectKey,
		&job.BGMObjectKey,
		&job.R2StorageBucket,
		&job.PodcastPublicObjectKey,
		&job.PodcastPublicBucket,
		&job.PodcastPublicDeletedAt,
		&job.ProviderJobID,
		&job.IdempotencyKey,
		&job.ErrorCode,
		&job.ErrorMessage,
		&job.PublishedAt,
		&job.FailedAt,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
	return job, err
}

func scanAudioBriefingScriptChunk(row audioBriefingJobScanner) (model.AudioBriefingScriptChunk, error) {
	var chunk model.AudioBriefingScriptChunk
	err := row.Scan(
		&chunk.ID,
		&chunk.JobID,
		&chunk.Seq,
		&chunk.PartType,
		&chunk.Speaker,
		&chunk.Text,
		&chunk.CharCount,
		&chunk.TTSStatus,
		&chunk.AttemptCount,
		&chunk.LastErrorCode,
		&chunk.TTSProvider,
		&chunk.VoiceModel,
		&chunk.VoiceStyle,
		&chunk.R2AudioObjectKey,
		&chunk.R2StorageBucket,
		&chunk.DurationSec,
		&chunk.ErrorMessage,
		&chunk.HeartbeatToken,
		&chunk.LastHeartbeatAt,
		&chunk.StartedAt,
		&chunk.CompletedAt,
		&chunk.CreatedAt,
		&chunk.UpdatedAt,
	)
	return chunk, err
}

func getAudioBriefingJobByIDTx(ctx context.Context, tx pgx.Tx, jobID string) (*model.AudioBriefingJob, error) {
	job, err := scanAudioBriefingJob(tx.QueryRow(ctx, `
		SELECT id, user_id, slot_started_at_jst, slot_key, persona, conversation_mode, partner_persona, pipeline_stage, status, archive_status,
		       source_item_count, reused_item_count, script_char_count, script_llm_models,
               prompt_key, prompt_source, prompt_version_id, prompt_version_number, prompt_experiment_id, prompt_experiment_arm_id,
               audio_duration_sec,
		       title, r2_audio_object_key, r2_manifest_object_key, bgm_object_key, r2_storage_bucket, podcast_public_object_key, podcast_public_bucket, podcast_public_deleted_at, provider_job_id, idempotency_key,
		       error_code, error_message, published_at, failed_at, created_at, updated_at
		FROM audio_briefing_jobs
		WHERE id = $1
	`, jobID))
	if err != nil {
		return nil, mapDBError(err)
	}
	return &job, nil
}

func updateAudioBriefingJobPublishedTx(ctx context.Context, tx pgx.Tx, jobID string, providerJobID, audioObjectKey, manifestObjectKey, bgmObjectKey *string, audioDurationSec *int) (*model.AudioBriefingJob, error) {
	standardBucket := audioBriefingStandardBucket()
	job, err := scanAudioBriefingJob(tx.QueryRow(ctx, `
		UPDATE audio_briefing_jobs
		SET status = 'published',
		    provider_job_id = COALESCE($2, provider_job_id),
		    r2_audio_object_key = COALESCE($3, r2_audio_object_key),
		    r2_manifest_object_key = COALESCE($4, r2_manifest_object_key),
		    bgm_object_key = COALESCE($5, bgm_object_key),
		    audio_duration_sec = COALESCE($6, audio_duration_sec),
		    r2_storage_bucket = COALESCE(NULLIF($7, ''), r2_storage_bucket),
		    error_code = NULL,
		    error_message = NULL,
		    failed_at = NULL,
		    published_at = COALESCE(published_at, NOW()),
		    updated_at = NOW()
		WHERE id = $1
		  AND status IN ('scripted', 'voiced', 'concatenating')
		RETURNING id, user_id, slot_started_at_jst, slot_key, persona, conversation_mode, partner_persona, pipeline_stage, status, archive_status,
		          source_item_count, reused_item_count, script_char_count, script_llm_models,
               prompt_key, prompt_source, prompt_version_id, prompt_version_number, prompt_experiment_id, prompt_experiment_arm_id,
               audio_duration_sec,
		          title, r2_audio_object_key, r2_manifest_object_key, bgm_object_key, r2_storage_bucket, podcast_public_object_key, podcast_public_bucket, podcast_public_deleted_at, provider_job_id, idempotency_key,
		          error_code, error_message, published_at, failed_at, created_at, updated_at
	`, jobID, providerJobID, audioObjectKey, manifestObjectKey, bgmObjectKey, audioDurationSec, standardBucket))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrInvalidState
		}
		return nil, err
	}
	return &job, nil
}

func updateAudioBriefingJobFailedTx(ctx context.Context, tx pgx.Tx, jobID string, providerJobID, errorCode, errorMessage *string) (*model.AudioBriefingJob, error) {
	job, err := scanAudioBriefingJob(tx.QueryRow(ctx, `
		UPDATE audio_briefing_jobs
		SET status = 'failed',
		    provider_job_id = COALESCE($2, provider_job_id),
		    error_code = COALESCE($3, error_code, 'concat_failed'),
		    error_message = COALESCE($4, error_message),
		    failed_at = COALESCE(failed_at, NOW()),
		    updated_at = NOW()
		WHERE id = $1
		  AND status IN ('scripted', 'voiced', 'concatenating')
		RETURNING id, user_id, slot_started_at_jst, slot_key, persona, conversation_mode, partner_persona, pipeline_stage, status, archive_status,
		          source_item_count, reused_item_count, script_char_count, script_llm_models,
               prompt_key, prompt_source, prompt_version_id, prompt_version_number, prompt_experiment_id, prompt_experiment_arm_id,
               audio_duration_sec,
		          title, r2_audio_object_key, r2_manifest_object_key, bgm_object_key, r2_storage_bucket, podcast_public_object_key, podcast_public_bucket, podcast_public_deleted_at, provider_job_id, idempotency_key,
		          error_code, error_message, published_at, failed_at, created_at, updated_at
	`, jobID, providerJobID, errorCode, errorMessage))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrInvalidState
		}
		return nil, err
	}
	return &job, nil
}

func audioBriefingProviderJobMatches(expected, actual *string) bool {
	expectedValue := strings.TrimSpace(valueOrEmpty(expected))
	actualValue := strings.TrimSpace(valueOrEmpty(actual))
	if expectedValue == "" {
		return true
	}
	return expectedValue == actualValue
}

func valueOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func audioBriefingStandardBucket() string {
	return firstNonEmpty(
		os.Getenv("AUDIO_BRIEFING_R2_STANDARD_BUCKET"),
		os.Getenv("AUDIO_BRIEFING_R2_BUCKET"),
	)
}

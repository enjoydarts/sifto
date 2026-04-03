package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
)

type AudioBriefingVoiceRunResult struct {
	ProcessedChunk bool
	Completed      bool
	Waiting        bool
}

type AudioBriefingVoiceRunner struct {
	repo         *repository.AudioBriefingRepo
	userSettings *repository.UserSettingsRepo
	cipher       *SecretCipher
	worker       *WorkerClient
}

const audioBriefingChunkMaxAttempts = 3

type audioBriefingSpeechParams struct {
	SpeechRate                 float64
	EmotionalIntensity         float64
	TempoDynamics              float64
	LineBreakSilenceSeconds    float64
	ChunkTrailingSilenceSecond float64
	Pitch                      float64
	VolumeGain                 float64
}

func audioBriefingVoiceConfigComplete(provider, voiceModel, voiceStyle string) bool {
	provider = strings.TrimSpace(provider)
	voiceModel = strings.TrimSpace(voiceModel)
	voiceStyle = strings.TrimSpace(voiceStyle)
	if provider == "" || voiceModel == "" {
		return false
	}
	return voiceStyle != "" || strings.EqualFold(provider, "xai") || strings.EqualFold(provider, "mock")
}

func NewAudioBriefingVoiceRunner(repo *repository.AudioBriefingRepo, userSettings *repository.UserSettingsRepo, cipher *SecretCipher, worker *WorkerClient) *AudioBriefingVoiceRunner {
	return &AudioBriefingVoiceRunner{repo: repo, userSettings: userSettings, cipher: cipher, worker: worker}
}

func (r *AudioBriefingVoiceRunner) Start(ctx context.Context, userID string, jobID string) (result *AudioBriefingVoiceRunResult, err error) {
	if r == nil || r.repo == nil || r.worker == nil {
		return nil, fmt.Errorf("audio briefing voice runner unavailable")
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("audio briefing voice stage panic: %v", recovered)
		}
		if err != nil {
			r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
		}
	}()

	job, err := r.repo.GetJobByID(ctx, userID, jobID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(job.Status) != "voicing" {
		job, err = r.repo.StartVoicingJob(ctx, jobID)
		if err != nil {
			return nil, err
		}
		if err := r.repo.ResetChunksForVoicing(ctx, jobID); err != nil {
			r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
			return nil, err
		}
	}
	chunks, err := r.repo.ListJobChunks(ctx, userID, jobID)
	if err != nil {
		r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
		return nil, err
	}
	if len(chunks) == 0 {
		r.bestEffortFailVoicing(jobID, "tts_failed", "no script chunks")
		return nil, repository.ErrInvalidState
	}
	voice, err := r.repo.GetPersonaVoice(ctx, userID, job.Persona)
	if err != nil {
		r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
		return nil, err
	}
	settings, err := r.repo.GetSettings(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			settings = nil
		} else {
			r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
			return nil, err
		}
	}
	partnerVoice, err := r.resolvePartnerVoiceForJob(ctx, job)
	if err != nil {
		r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
		return nil, err
	}

	selection, chunk, resetGenerating := nextAudioBriefingVoicingChunk(chunks, timeutilNow())
	switch selection {
	case audioBriefingVoicingChunkSelectionComplete:
		completedJob, err := r.repo.CompleteVoicingJob(ctx, jobID)
		if err != nil {
			r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
			return nil, err
		}
		_ = completedJob
		return &AudioBriefingVoiceRunResult{Completed: true}, nil
	case audioBriefingVoicingChunkSelectionWaiting:
		return &AudioBriefingVoiceRunResult{Waiting: true}, nil
	}
	if chunk == nil {
		return &AudioBriefingVoiceRunResult{Waiting: true}, nil
	}
	if resetGenerating {
		if chunk.AttemptCount >= audioBriefingChunkMaxAttempts {
			message := "stale generating chunk exceeded retry limit"
			if err := r.repo.MarkChunkExhausted(ctx, chunk.ID, "tts_stalled", message); err != nil {
				r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
				return nil, err
			}
			r.bestEffortFailVoicing(jobID, "tts_stalled", message)
			return nil, errors.New(message)
		}
		if err := r.repo.MarkChunkRetryWait(ctx, chunk.ID, "tts_stalled", "stale generating chunk reset for retry"); err != nil {
			r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
			return nil, err
		}
	}
	rawHeartbeatToken, err := randomAudioBriefingToken(32)
	if err != nil {
		r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
		return nil, err
	}
	if err := r.repo.StartChunkGenerating(ctx, chunk.ID, HashAudioBriefingCallbackToken(rawHeartbeatToken)); err != nil {
		if err == repository.ErrInvalidState {
			return &AudioBriefingVoiceRunResult{Waiting: true}, nil
		}
		r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
		return nil, err
	}
	chunk.AttemptCount++
	provider := strings.TrimSpace(derefString(chunk.TTSProvider))
	voiceModel := strings.TrimSpace(derefString(chunk.VoiceModel))
	voiceStyle := strings.TrimSpace(derefString(chunk.VoiceStyle))
	if !audioBriefingVoiceConfigComplete(provider, voiceModel, voiceStyle) {
		err := fmt.Errorf("chunk tts config is incomplete")
		return r.handleChunkGenerationFailure(ctx, jobID, chunk, "tts_failed", err)
	}
	var aivisAPIKey *string
	var aivisUserDictionaryUUID *string
	var xaiAPIKey *string
	if provider == "aivis" {
		aivisAPIKey, err = r.loadAivisAPIKey(ctx, userID)
		if err != nil {
			return r.handleChunkGenerationFailure(ctx, jobID, chunk, "tts_failed", err)
		}
		aivisUserDictionaryUUID, err = r.userSettings.GetAivisUserDictionaryUUID(ctx, userID)
		if err != nil {
			return r.handleChunkGenerationFailure(ctx, jobID, chunk, "tts_failed", err)
		}
	} else if provider == "xai" {
		xaiAPIKey, err = r.loadXAIAPIKey(ctx, userID)
		if err != nil {
			return r.handleChunkGenerationFailure(ctx, jobID, chunk, "tts_failed", err)
		}
	}
	speechParams := audioBriefingSpeechParamsForChunk(chunk, voice, partnerVoice, settings)
	resp, err := r.worker.SynthesizeAudioBriefingUpload(
		ctx,
		provider,
		voiceModel,
		voiceStyle,
		chunk.Text,
		speechParams.SpeechRate,
		speechParams.EmotionalIntensity,
		speechParams.TempoDynamics,
		speechParams.LineBreakSilenceSeconds,
		speechParams.ChunkTrailingSilenceSecond,
		speechParams.Pitch,
		speechParams.VolumeGain,
		audioBriefingChunkObjectKey(userID, jobID, chunk.Seq),
		chunk.ID,
		audioBriefingChunkHeartbeatURL(chunk.ID),
		rawHeartbeatToken,
		aivisUserDictionaryUUID,
		aivisAPIKey,
		xaiAPIKey,
	)
	if err != nil {
		return r.handleChunkGenerationFailure(ctx, jobID, chunk, "tts_failed", annotateAudioBriefingChunkError(chunk, err))
	}
	if err := r.repo.MarkChunkGenerated(ctx, chunk.ID, resp.AudioObjectKey, resp.DurationSec); err != nil {
		r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
		return nil, err
	}
	if _, err := r.repo.CompleteVoicingJob(ctx, jobID); err == nil {
		return &AudioBriefingVoiceRunResult{ProcessedChunk: true, Completed: true}, nil
	} else if err != repository.ErrInvalidState {
		r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
		return nil, err
	}
	return &AudioBriefingVoiceRunResult{ProcessedChunk: true}, nil
}

func (r *AudioBriefingVoiceRunner) loadXAIAPIKey(ctx context.Context, userID string) (*string, error) {
	if r == nil || r.userSettings == nil || r.cipher == nil {
		return nil, errors.New("audio briefing xai key loader is not configured")
	}
	enc, err := r.userSettings.GetXAIAPIKeyEncrypted(ctx, userID)
	if err != nil {
		return nil, err
	}
	if enc == nil || strings.TrimSpace(*enc) == "" {
		return nil, errors.New("xai api key is not configured")
	}
	plain, err := r.cipher.DecryptString(*enc)
	if err != nil {
		return nil, err
	}
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return nil, errors.New("xai api key is empty")
	}
	return &plain, nil
}

func audioBriefingSpeechParamsForChunk(
	chunk *model.AudioBriefingScriptChunk,
	hostVoice *model.AudioBriefingPersonaVoice,
	partnerVoice *model.AudioBriefingPersonaVoice,
	settings *model.AudioBriefingSettings,
) audioBriefingSpeechParams {
	params := audioBriefingSpeechParams{
		SpeechRate:                 1.0,
		EmotionalIntensity:         1.0,
		TempoDynamics:              1.0,
		LineBreakSilenceSeconds:    0.4,
		ChunkTrailingSilenceSecond: 1.0,
		Pitch:                      0.0,
		VolumeGain:                 0.0,
	}
	if settings != nil && settings.ChunkTrailingSilenceSeconds >= 0 {
		params.ChunkTrailingSilenceSecond = settings.ChunkTrailingSilenceSeconds
	}
	selectedVoice := hostVoice
	if chunk != nil && strings.TrimSpace(derefString(chunk.Speaker)) == "partner" && partnerVoice != nil {
		selectedVoice = partnerVoice
	}
	if selectedVoice != nil {
		params.SpeechRate = selectedVoice.SpeechRate
		params.EmotionalIntensity = selectedVoice.EmotionalIntensity
		params.TempoDynamics = selectedVoice.TempoDynamics
		params.LineBreakSilenceSeconds = selectedVoice.LineBreakSilenceSeconds
		params.Pitch = selectedVoice.Pitch
		params.VolumeGain = selectedVoice.VolumeGain
	}
	return params
}

func (r *AudioBriefingVoiceRunner) resolvePartnerVoiceForJob(ctx context.Context, job *model.AudioBriefingJob) (*model.AudioBriefingPersonaVoice, error) {
	if r == nil || r.repo == nil || job == nil {
		return nil, nil
	}
	if strings.TrimSpace(job.ConversationMode) != "duo" {
		return nil, nil
	}
	partnerPersona := strings.TrimSpace(derefString(job.PartnerPersona))
	if partnerPersona == "" {
		return nil, nil
	}
	return r.repo.GetPersonaVoice(ctx, job.UserID, partnerPersona)
}

func (r *AudioBriefingVoiceRunner) handleChunkGenerationFailure(ctx context.Context, jobID string, chunk *model.AudioBriefingScriptChunk, errorCode string, err error) (*AudioBriefingVoiceRunResult, error) {
	if r == nil || r.repo == nil {
		return nil, err
	}
	if chunk == nil {
		r.bestEffortFailVoicing(jobID, errorCode, err.Error())
		return nil, err
	}
	errorMessage := err.Error()
	if chunk.AttemptCount >= audioBriefingChunkMaxAttempts {
		if markErr := r.repo.MarkChunkExhausted(ctx, chunk.ID, errorCode, errorMessage); markErr != nil {
			r.bestEffortFailVoicing(jobID, errorCode, markErr.Error())
			return nil, markErr
		}
		r.bestEffortFailVoicing(jobID, errorCode, errorMessage)
		return nil, err
	}
	if markErr := r.repo.MarkChunkRetryWait(ctx, chunk.ID, errorCode, errorMessage); markErr != nil {
		r.bestEffortFailVoicing(jobID, errorCode, markErr.Error())
		return nil, markErr
	}
	return &AudioBriefingVoiceRunResult{ProcessedChunk: true}, nil
}

func annotateAudioBriefingChunkError(chunk *model.AudioBriefingScriptChunk, err error) error {
	if chunk == nil || err == nil {
		return err
	}
	return fmt.Errorf(
		"chunk_id=%s seq=%d part=%s text_preview=%q: %w",
		strings.TrimSpace(chunk.ID),
		chunk.Seq,
		strings.TrimSpace(chunk.PartType),
		audioBriefingChunkTextPreview(chunk.Text),
		err,
	)
}

func audioBriefingChunkTextPreview(text string) string {
	preview := strings.TrimSpace(text)
	if preview == "" {
		return ""
	}
	runes := []rune(preview)
	if len(runes) > 120 {
		preview = string(runes[:120])
	}
	return preview
}

func (r *AudioBriefingVoiceRunner) bestEffortFailVoicing(jobID string, errorCode string, errorMessage string) {
	if r == nil || r.repo == nil || strings.TrimSpace(jobID) == "" {
		return
	}
	ctx, cancel := audioBriefingFailureContext(context.Background())
	defer cancel()
	_, _ = r.repo.FailVoicingJob(ctx, jobID, errorCode, errorMessage)
}

func (r *AudioBriefingVoiceRunner) bestEffortMarkChunkFailed(chunkID string, errorMessage string) {
	if r == nil || r.repo == nil || strings.TrimSpace(chunkID) == "" {
		return
	}
	ctx, cancel := audioBriefingFailureContext(context.Background())
	defer cancel()
	_ = r.repo.MarkChunkFailed(ctx, chunkID, errorMessage)
}

func (r *AudioBriefingVoiceRunner) loadAivisAPIKey(ctx context.Context, userID string) (*string, error) {
	if r == nil || r.userSettings == nil {
		return nil, nil
	}
	enc, err := r.userSettings.GetAivisAPIKeyEncrypted(ctx, userID)
	if err != nil || enc == nil || strings.TrimSpace(*enc) == "" {
		return nil, err
	}
	if r.cipher == nil || !r.cipher.Enabled() {
		return nil, fmt.Errorf("user secret encryption is not configured")
	}
	key, err := r.cipher.DecryptString(*enc)
	if err != nil {
		return nil, err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, nil
	}
	return &key, nil
}

func audioBriefingChunkObjectKey(userID string, jobID string, seq int) string {
	return path.Join("audio-briefings", userID, jobID, fmt.Sprintf("chunk-%03d", seq))
}

func audioBriefingChunkHeartbeatURL(chunkID string) string {
	baseURL := audioBriefingCallbackBaseURL(AudioBriefingConcatModeFromEnv())
	if baseURL == "" {
		return ""
	}
	return strings.TrimRight(baseURL, "/") + "/api/internal/audio-briefings/chunks/" + strings.TrimSpace(chunkID) + "/heartbeat"
}

type audioBriefingVoicingChunkSelection string

const (
	audioBriefingVoicingChunkSelectionProcess  audioBriefingVoicingChunkSelection = "process"
	audioBriefingVoicingChunkSelectionWaiting  audioBriefingVoicingChunkSelection = "waiting"
	audioBriefingVoicingChunkSelectionComplete audioBriefingVoicingChunkSelection = "complete"
)

func nextAudioBriefingVoicingChunk(chunks []model.AudioBriefingScriptChunk, now time.Time) (audioBriefingVoicingChunkSelection, *model.AudioBriefingScriptChunk, bool) {
	retryAfter := audioBriefingChunkRetryAfter()
	for i := range chunks {
		chunk := &chunks[i]
		if chunk.R2AudioObjectKey != nil && strings.TrimSpace(*chunk.R2AudioObjectKey) != "" && chunk.TTSStatus == "generated" {
			continue
		}
		status := strings.TrimSpace(chunk.TTSStatus)
		if status == "exhausted" {
			return audioBriefingVoicingChunkSelectionWaiting, nil, false
		}
		if status == "generating" && now.Sub(audioBriefingChunkHeartbeatAt(*chunk)) < retryAfter {
			return audioBriefingVoicingChunkSelectionWaiting, nil, false
		}
		if status == "pending" || status == "retry_wait" || status == "failed" || status == "generating" {
			return audioBriefingVoicingChunkSelectionProcess, chunk, status == "generating"
		}
	}
	return audioBriefingVoicingChunkSelectionComplete, nil, false
}

func audioBriefingChunkHeartbeatAt(chunk model.AudioBriefingScriptChunk) time.Time {
	if chunk.LastHeartbeatAt != nil && !chunk.LastHeartbeatAt.IsZero() {
		return *chunk.LastHeartbeatAt
	}
	return chunk.UpdatedAt
}

func audioBriefingChunkRetryAfter() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("AUDIO_BRIEFING_CHUNK_RETRY_AFTER_SEC")); raw != "" {
		if sec, err := strconv.Atoi(raw); err == nil && sec > 0 {
			return time.Duration(sec) * time.Second
		}
	}
	return 15 * time.Minute
}

func timeutilNow() time.Time {
	return time.Now()
}

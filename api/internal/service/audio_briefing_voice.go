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
		r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
		return nil, err
	}
	speechRate := 1.0
	emotionalIntensity := 1.0
	tempoDynamics := 1.0
	lineBreakSilenceSeconds := 0.4
	chunkTrailingSilenceSeconds := 1.0
	pitch := 0.0
	volumeGain := 0.0
	if settings != nil && settings.ChunkTrailingSilenceSeconds >= 0 {
		chunkTrailingSilenceSeconds = settings.ChunkTrailingSilenceSeconds
	}
	if voice != nil {
		speechRate = voice.SpeechRate
		emotionalIntensity = voice.EmotionalIntensity
		tempoDynamics = voice.TempoDynamics
		lineBreakSilenceSeconds = voice.LineBreakSilenceSeconds
		pitch = voice.Pitch
		volumeGain = voice.VolumeGain
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
	if provider == "" || voiceModel == "" || voiceStyle == "" {
		err := fmt.Errorf("chunk tts config is incomplete")
		return r.handleChunkGenerationFailure(ctx, jobID, chunk, "tts_failed", err)
	}
	var aivisAPIKey *string
	var aivisUserDictionaryUUID *string
	if provider == "aivis" {
		aivisAPIKey, err = r.loadAivisAPIKey(ctx, userID)
		if err != nil {
			return r.handleChunkGenerationFailure(ctx, jobID, chunk, "tts_failed", err)
		}
		aivisUserDictionaryUUID, err = r.userSettings.GetAivisUserDictionaryUUID(ctx, userID)
		if err != nil {
			return r.handleChunkGenerationFailure(ctx, jobID, chunk, "tts_failed", err)
		}
	}
	resp, err := r.worker.SynthesizeAudioBriefingUpload(
		ctx,
		provider,
		voiceModel,
		voiceStyle,
		chunk.Text,
		speechRate,
		emotionalIntensity,
		tempoDynamics,
		lineBreakSilenceSeconds,
		chunkTrailingSilenceSeconds,
		pitch,
		volumeGain,
		audioBriefingChunkObjectKey(userID, jobID, chunk.Seq),
		chunk.ID,
		audioBriefingChunkHeartbeatURL(chunk.ID),
		rawHeartbeatToken,
		aivisUserDictionaryUUID,
		aivisAPIKey,
	)
	if err != nil {
		return r.handleChunkGenerationFailure(ctx, jobID, chunk, "tts_failed", err)
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

package service

import (
	"context"
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
	speechRate := 1.0
	emotionalIntensity := 1.0
	tempoDynamics := 1.0
	lineBreakSilenceSeconds := 0.4
	pitch := 0.0
	volumeGain := 0.0
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
		if err := r.repo.MarkChunkFailed(ctx, chunk.ID, "stale generating chunk reset for retry"); err != nil {
			r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
			return nil, err
		}
	}
	if err := r.repo.MarkChunkGenerating(ctx, chunk.ID); err != nil {
		if err == repository.ErrInvalidState {
			return &AudioBriefingVoiceRunResult{Waiting: true}, nil
		}
		r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
		return nil, err
	}
	provider := strings.TrimSpace(derefString(chunk.TTSProvider))
	voiceModel := strings.TrimSpace(derefString(chunk.VoiceModel))
	voiceStyle := strings.TrimSpace(derefString(chunk.VoiceStyle))
	if provider == "" || voiceModel == "" || voiceStyle == "" {
		err := fmt.Errorf("chunk tts config is incomplete")
		r.bestEffortMarkChunkFailed(chunk.ID, err.Error())
		r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
		return nil, err
	}
	var aivisAPIKey *string
	var aivisUserDictionaryUUID *string
	if provider == "aivis" {
		aivisAPIKey, err = r.loadAivisAPIKey(ctx, userID)
		if err != nil {
			r.bestEffortMarkChunkFailed(chunk.ID, err.Error())
			r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
			return nil, err
		}
		aivisUserDictionaryUUID, err = r.userSettings.GetAivisUserDictionaryUUID(ctx, userID)
		if err != nil {
			r.bestEffortMarkChunkFailed(chunk.ID, err.Error())
			r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
			return nil, err
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
		pitch,
		volumeGain,
		audioBriefingChunkObjectKey(userID, jobID, chunk.Seq),
		aivisUserDictionaryUUID,
		aivisAPIKey,
	)
	if err != nil {
		r.bestEffortMarkChunkFailed(chunk.ID, err.Error())
		r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
		return nil, err
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
		if strings.TrimSpace(chunk.TTSStatus) == "generating" && now.Sub(chunk.UpdatedAt) < retryAfter {
			return audioBriefingVoicingChunkSelectionWaiting, nil, false
		}
		return audioBriefingVoicingChunkSelectionProcess, chunk, strings.TrimSpace(chunk.TTSStatus) == "generating"
	}
	return audioBriefingVoicingChunkSelectionComplete, nil, false
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

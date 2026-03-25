package service

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

type AudioBriefingVoiceRunner struct {
	repo         *repository.AudioBriefingRepo
	userSettings *repository.UserSettingsRepo
	cipher       *SecretCipher
	worker       *WorkerClient
}

func NewAudioBriefingVoiceRunner(repo *repository.AudioBriefingRepo, userSettings *repository.UserSettingsRepo, cipher *SecretCipher, worker *WorkerClient) *AudioBriefingVoiceRunner {
	return &AudioBriefingVoiceRunner{repo: repo, userSettings: userSettings, cipher: cipher, worker: worker}
}

func (r *AudioBriefingVoiceRunner) Start(ctx context.Context, userID string, jobID string) (err error) {
	if r == nil || r.repo == nil || r.worker == nil {
		return fmt.Errorf("audio briefing voice runner unavailable")
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("audio briefing voice stage panic: %v", recovered)
		}
		if err != nil {
			_, _ = r.repo.FailVoicingJob(ctx, jobID, "tts_failed", err.Error())
		}
	}()

	if _, err := r.repo.GetJobByID(ctx, userID, jobID); err != nil {
		return err
	}
	job, err := r.repo.StartVoicingJob(ctx, jobID)
	if err != nil {
		return err
	}
	if err := r.repo.ResetChunksForVoicing(ctx, jobID); err != nil {
		_, _ = r.repo.FailVoicingJob(ctx, jobID, "tts_failed", err.Error())
		return err
	}
	chunks, err := r.repo.ListJobChunks(ctx, userID, jobID)
	if err != nil {
		_, _ = r.repo.FailVoicingJob(ctx, jobID, "tts_failed", err.Error())
		return err
	}
	if len(chunks) == 0 {
		_, _ = r.repo.FailVoicingJob(ctx, jobID, "tts_failed", "no script chunks")
		return repository.ErrInvalidState
	}
	voice, err := r.repo.GetPersonaVoice(ctx, userID, job.Persona)
	if err != nil {
		_, _ = r.repo.FailVoicingJob(ctx, jobID, "tts_failed", err.Error())
		return err
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

	for _, chunk := range chunks {
		if chunk.R2AudioObjectKey != nil && strings.TrimSpace(*chunk.R2AudioObjectKey) != "" && chunk.TTSStatus == "generated" {
			continue
		}
		if err := r.repo.MarkChunkGenerating(ctx, chunk.ID); err != nil {
			_, _ = r.repo.FailVoicingJob(ctx, jobID, "tts_failed", err.Error())
			return err
		}
		provider := strings.TrimSpace(derefString(chunk.TTSProvider))
		voiceModel := strings.TrimSpace(derefString(chunk.VoiceModel))
		voiceStyle := strings.TrimSpace(derefString(chunk.VoiceStyle))
		if provider == "" || voiceModel == "" || voiceStyle == "" {
			err := fmt.Errorf("chunk tts config is incomplete")
			_ = r.repo.MarkChunkFailed(ctx, chunk.ID, err.Error())
			_, _ = r.repo.FailVoicingJob(ctx, jobID, "tts_failed", err.Error())
			return err
		}
		var aivisAPIKey *string
		if provider == "aivis" {
			aivisAPIKey, err = r.loadAivisAPIKey(ctx, userID)
			if err != nil {
				_ = r.repo.MarkChunkFailed(ctx, chunk.ID, err.Error())
				_, _ = r.repo.FailVoicingJob(ctx, jobID, "tts_failed", err.Error())
				return err
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
			aivisAPIKey,
		)
		if err != nil {
			_ = r.repo.MarkChunkFailed(ctx, chunk.ID, err.Error())
			_, _ = r.repo.FailVoicingJob(ctx, jobID, "tts_failed", err.Error())
			return err
		}
		if err := r.repo.MarkChunkGenerated(ctx, chunk.ID, resp.AudioObjectKey, resp.DurationSec); err != nil {
			_, _ = r.repo.FailVoicingJob(ctx, jobID, "tts_failed", err.Error())
			return err
		}
	}
	if _, err := r.repo.CompleteVoicingJob(ctx, jobID); err != nil {
		_, _ = r.repo.FailVoicingJob(ctx, jobID, "tts_failed", err.Error())
		return err
	}
	return nil
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

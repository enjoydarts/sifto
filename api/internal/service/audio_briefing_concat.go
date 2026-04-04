package service

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
)

type audioBriefingConcatRepo interface {
	GetJobByID(ctx context.Context, userID, jobID string) (*model.AudioBriefingJob, error)
	GetSettings(ctx context.Context, userID string) (*model.AudioBriefingSettings, error)
	ListJobChunks(ctx context.Context, userID, jobID string) ([]model.AudioBriefingScriptChunk, error)
	BeginConcatCallback(ctx context.Context, jobID, requestID, tokenHash string, providerJobID, audioObjectKey *string, expiresAt time.Time) (*model.AudioBriefingJob, *model.AudioBriefingCallbackToken, error)
	UpdateConcatProviderJobID(ctx context.Context, jobID string, providerJobID string) (*model.AudioBriefingJob, error)
	FailConcatLaunch(ctx context.Context, jobID string, errorCode string, errorMessage string) (*model.AudioBriefingJob, error)
}

type AudioBriefingConcatStarter struct {
	repo   audioBriefingConcatRepo
	runner AudioConcatRunner
	mode   string
}

func NewAudioBriefingConcatStarter(repo *repository.AudioBriefingRepo, runner AudioConcatRunner) *AudioBriefingConcatStarter {
	return &AudioBriefingConcatStarter{
		repo:   repo,
		runner: runner,
		mode:   AudioBriefingConcatModeFromEnv(),
	}
}

func (s *AudioBriefingConcatStarter) Start(ctx context.Context, userID string, jobID string) error {
	if s == nil || s.repo == nil || s.runner == nil {
		return fmt.Errorf("audio briefing concat starter unavailable")
	}
	if !s.runner.Enabled() {
		return ErrAudioConcatRunnerDisabled
	}

	job, err := s.repo.GetJobByID(ctx, userID, jobID)
	if err != nil {
		return err
	}
	settings, err := s.repo.GetSettings(ctx, userID)
	if err != nil {
		return err
	}
	chunks, err := s.repo.ListJobChunks(ctx, userID, jobID)
	if err != nil {
		return err
	}
	audioObjectKeys := make([]string, 0, len(chunks))
	var previousKey string
	for _, chunk := range chunks {
		if chunk.R2AudioObjectKey == nil || strings.TrimSpace(*chunk.R2AudioObjectKey) == "" {
			return repository.ErrInvalidState
		}
		key := strings.TrimSpace(*chunk.R2AudioObjectKey)
		if key == previousKey {
			continue
		}
		audioObjectKeys = append(audioObjectKeys, key)
		previousKey = key
	}
	if len(audioObjectKeys) == 0 {
		return repository.ErrInvalidState
	}

	callbackBaseURL := audioBriefingCallbackBaseURL(s.mode)
	if callbackBaseURL == "" {
		return fmt.Errorf("APP_BASE_URL is not configured")
	}
	callbackToken, requestID, tokenHash, expiresAt, err := IssueAudioBriefingCallbackToken(time.Now().UTC(), time.Hour)
	if err != nil {
		return err
	}
	if _, _, err := s.repo.BeginConcatCallback(ctx, job.ID, requestID, tokenHash, nil, nil, expiresAt); err != nil {
		return err
	}

	outputObjectKey := audioBriefingEpisodeObjectKey(userID, job.ID)
	callbackURL := callbackBaseURL + "/api/internal/audio-briefings/" + job.ID + "/concat-complete"
	runResp, err := s.runner.RunAudioConcat(ctx, AudioConcatRunRequest{
		JobID:           job.ID,
		UserID:          userID,
		RequestID:       requestID,
		CallbackURL:     callbackURL,
		CallbackToken:   callbackToken,
		AudioObjectKeys: audioObjectKeys,
		OutputObjectKey: outputObjectKey,
		BGMEnabled:      settings != nil && settings.BGMEnabled,
		BGMR2Prefix:     strings.TrimSpace(derefString(settings.BGMR2Prefix)),
	})
	if err != nil {
		_, _ = s.repo.FailConcatLaunch(ctx, job.ID, "concat_launch_failed", err.Error())
		return err
	}
	if _, err := s.repo.UpdateConcatProviderJobID(ctx, job.ID, runResp.ExecutionName); err != nil {
		if _, failErr := s.repo.FailConcatLaunch(ctx, job.ID, "concat_launch_failed", err.Error()); failErr != nil {
			return errors.Join(err, failErr)
		}
		return err
	}
	return nil
}

func audioBriefingEpisodeObjectKey(userID string, jobID string) string {
	return path.Join("audio-briefings", userID, jobID, "episode.mp3")
}

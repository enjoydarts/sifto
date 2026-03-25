package service

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
)

type audioBriefingDeleteRepo interface {
	GetJobByID(ctx context.Context, userID, jobID string) (*model.AudioBriefingJob, error)
	ListJobChunks(ctx context.Context, userID, jobID string) ([]model.AudioBriefingScriptChunk, error)
	DeleteJob(ctx context.Context, userID, jobID string) error
}

type audioBriefingObjectDeleter interface {
	DeleteAudioBriefingObjects(ctx context.Context, objectRefs []AudioBriefingObjectRef) error
}

type AudioBriefingDeleteService struct {
	repo       audioBriefingDeleteRepo
	deleter    audioBriefingObjectDeleter
	now        func() time.Time
	staleAfter time.Duration
}

func NewAudioBriefingDeleteService(repo audioBriefingDeleteRepo, deleter audioBriefingObjectDeleter) *AudioBriefingDeleteService {
	return &AudioBriefingDeleteService{
		repo:       repo,
		deleter:    deleter,
		now:        time.Now,
		staleAfter: audioBriefingStaleDeleteAfter(),
	}
}

func (s *AudioBriefingDeleteService) Delete(ctx context.Context, userID, jobID string) error {
	if s == nil || s.repo == nil {
		return repository.ErrNotFound
	}
	job, err := s.repo.GetJobByID(ctx, userID, jobID)
	if err != nil {
		return err
	}
	if !audioBriefingJobCanBeDeletedAt(job, s.currentTime(), s.staleAfter) {
		return repository.ErrInvalidState
	}
	chunks, err := s.repo.ListJobChunks(ctx, userID, jobID)
	if err != nil {
		return err
	}
	objectRefs := CollectAudioBriefingObjectRefs(job, chunks)
	if len(objectRefs) > 0 && s.deleter != nil {
		if err := s.deleter.DeleteAudioBriefingObjects(ctx, objectRefs); err != nil {
			return err
		}
	}
	return s.repo.DeleteJob(ctx, userID, jobID)
}

func (s *AudioBriefingDeleteService) currentTime() time.Time {
	if s == nil || s.now == nil {
		return time.Now()
	}
	return s.now()
}

func audioBriefingJobCanBeDeletedAt(job *model.AudioBriefingJob, now time.Time, staleAfter time.Duration) bool {
	if job == nil {
		return false
	}
	switch strings.TrimSpace(job.Status) {
	case "scripted", "voiced", "published", "failed", "cancelled", "skipped", "needs_rerun":
		return true
	case "scripting", "voicing", "concatenating":
		if staleAfter <= 0 {
			return false
		}
		return !job.UpdatedAt.IsZero() && now.Sub(job.UpdatedAt) >= staleAfter
	default:
		return false
	}
}

func AudioBriefingDeleteAllowed(job *model.AudioBriefingJob) bool {
	return audioBriefingJobCanBeDeletedAt(job, time.Now(), audioBriefingStaleDeleteAfter())
}

func audioBriefingStaleDeleteAfter() time.Duration {
	raw := strings.TrimSpace(os.Getenv("AUDIO_BRIEFING_STALE_DELETE_AFTER_MINUTES"))
	if raw == "" {
		return 30 * time.Minute
	}
	minutes, err := strconv.Atoi(raw)
	if err != nil || minutes <= 0 {
		return 30 * time.Minute
	}
	return time.Duration(minutes) * time.Minute
}

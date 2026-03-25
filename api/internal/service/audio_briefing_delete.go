package service

import (
	"context"
	"strings"

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
	repo    audioBriefingDeleteRepo
	deleter audioBriefingObjectDeleter
}

func NewAudioBriefingDeleteService(repo audioBriefingDeleteRepo, deleter audioBriefingObjectDeleter) *AudioBriefingDeleteService {
	return &AudioBriefingDeleteService{repo: repo, deleter: deleter}
}

func (s *AudioBriefingDeleteService) Delete(ctx context.Context, userID, jobID string) error {
	if s == nil || s.repo == nil {
		return repository.ErrNotFound
	}
	job, err := s.repo.GetJobByID(ctx, userID, jobID)
	if err != nil {
		return err
	}
	if !audioBriefingJobCanBeDeleted(job) {
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

func audioBriefingJobCanBeDeleted(job *model.AudioBriefingJob) bool {
	if job == nil {
		return false
	}
	switch strings.TrimSpace(job.Status) {
	case "scripted", "voiced", "published", "failed", "cancelled", "skipped", "needs_rerun":
		return true
	default:
		return false
	}
}

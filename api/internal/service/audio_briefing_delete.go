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
	DeleteAudioBriefingObjects(ctx context.Context, objectKeys []string) error
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
	objectKeys := collectAudioBriefingObjectKeys(job, chunks)
	if len(objectKeys) > 0 && s.deleter != nil {
		if err := s.deleter.DeleteAudioBriefingObjects(ctx, objectKeys); err != nil {
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

func collectAudioBriefingObjectKeys(job *model.AudioBriefingJob, chunks []model.AudioBriefingScriptChunk) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(chunks)+2)
	appendKey := func(raw string) {
		key := strings.TrimSpace(raw)
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	if job != nil {
		appendKey(ptrString(job.R2AudioObjectKey))
		appendKey(ptrString(job.R2ManifestObjectKey))
	}
	for _, chunk := range chunks {
		appendKey(ptrString(chunk.R2AudioObjectKey))
	}
	return out
}

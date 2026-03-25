package service

import (
	"context"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
)

type PodcastPublicationService struct {
	repo   *repository.AudioBriefingRepo
	worker *WorkerClient
}

func NewPodcastPublicationService(repo *repository.AudioBriefingRepo, worker *WorkerClient) *PodcastPublicationService {
	return &PodcastPublicationService{repo: repo, worker: worker}
}

func (s *PodcastPublicationService) EnsurePublicCopy(ctx context.Context, job *model.AudioBriefingJob) (*model.AudioBriefingJob, error) {
	if s == nil || s.repo == nil || s.worker == nil || job == nil {
		return job, nil
	}
	if strings.TrimSpace(job.Status) != "published" {
		return job, nil
	}
	publicBucket := strings.TrimSpace(AudioBriefingPublicBucketFromEnv())
	sourceBucket := NormalizeAudioBriefingStorageBucket(job.R2StorageBucket)
	audioObjectKey := strings.TrimSpace(ptrString(job.R2AudioObjectKey))
	if publicBucket == "" || sourceBucket == "" || audioObjectKey == "" {
		return job, nil
	}
	if strings.TrimSpace(job.PodcastPublicBucket) == publicBucket && strings.TrimSpace(ptrString(job.PodcastPublicObjectKey)) == audioObjectKey && job.PodcastPublicDeletedAt == nil {
		return job, nil
	}
	if err := s.worker.CopyAudioBriefingObjects(ctx, sourceBucket, publicBucket, []string{audioObjectKey}); err != nil {
		return nil, err
	}
	updated, err := s.repo.SetPodcastPublicObject(ctx, job.ID, publicBucket, audioObjectKey)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

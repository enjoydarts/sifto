package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

type podcastCleanupRepo interface {
	ListExpiredPodcastPublicCopies(ctx context.Context, cutoff time.Time, limit int) ([]model.AudioBriefingJob, error)
	MarkPodcastPublicObjectDeleted(ctx context.Context, jobID string) (*model.AudioBriefingJob, error)
}

type podcastCleanupStorage interface {
	DeleteAudioBriefingObjectsInBucket(ctx context.Context, bucket string, objectKeys []string) error
}

type PodcastCleanupResult struct {
	Processed int `json:"processed"`
	Deleted   int `json:"deleted"`
	Failed    int `json:"failed"`
}

type PodcastCleanupService struct {
	repo    podcastCleanupRepo
	storage podcastCleanupStorage
	now     func() time.Time
}

func NewPodcastCleanupService(repo podcastCleanupRepo, storage podcastCleanupStorage) *PodcastCleanupService {
	return &PodcastCleanupService{repo: repo, storage: storage, now: time.Now}
}

func (s *PodcastCleanupService) DeleteExpiredPublicCopies(ctx context.Context) (*PodcastCleanupResult, error) {
	result := &PodcastCleanupResult{}
	if s == nil || s.repo == nil || s.storage == nil {
		return result, nil
	}
	now := time.Now()
	if s.now != nil {
		now = s.now()
	}
	cutoff := now.AddDate(0, 0, -PodcastEpisodeRetentionDaysFromEnv())
	candidates, err := s.repo.ListExpiredPodcastPublicCopies(ctx, cutoff, PodcastPublicCleanupBatchLimitFromEnv())
	if err != nil {
		return result, err
	}
	for _, job := range candidates {
		bucket := strings.TrimSpace(job.PodcastPublicBucket)
		key := strings.TrimSpace(ptrString(job.PodcastPublicObjectKey))
		if bucket == "" || key == "" || job.PodcastPublicDeletedAt != nil {
			continue
		}
		// Public copies must never share a bucket with private audio.
		if bucket == NormalizeAudioBriefingStorageBucket(job.R2StorageBucket) || bucket == AudioBriefingStandardBucketFromEnv() || bucket == AudioBriefingIABucketFromEnv() {
			continue
		}
		result.Processed++
		if err := s.storage.DeleteAudioBriefingObjectsInBucket(ctx, bucket, []string{key}); err != nil {
			result.Failed++
			return result, fmt.Errorf("delete expired podcast public object: %w", err)
		}
		if _, err := s.repo.MarkPodcastPublicObjectDeleted(ctx, job.ID); err != nil {
			result.Failed++
			return result, fmt.Errorf("mark expired podcast public object deleted: %w", err)
		}
		result.Deleted++
	}
	return result, nil
}

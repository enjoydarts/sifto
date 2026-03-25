package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

type audioBriefingArchiveRepo interface {
	ListIAMoveCandidates(ctx context.Context, cutoff time.Time, limit int) ([]model.AudioBriefingJob, error)
	ListJobChunks(ctx context.Context, userID, jobID string) ([]model.AudioBriefingScriptChunk, error)
	UpdateStorageBucketForJobAndChunks(ctx context.Context, jobID string, bucket string) error
	MarkPodcastPublicObjectDeleted(ctx context.Context, jobID string) (*model.AudioBriefingJob, error)
}

type audioBriefingArchiveStorage interface {
	CopyAudioBriefingObjects(ctx context.Context, sourceBucket string, targetBucket string, objectKeys []string) error
	DeleteAudioBriefingObjectsInBucket(ctx context.Context, bucket string, objectKeys []string) error
}

type AudioBriefingArchiveResult struct {
	Processed int `json:"processed"`
	Moved     int `json:"moved"`
	Failed    int `json:"failed"`
}

type AudioBriefingArchiveService struct {
	repo           audioBriefingArchiveRepo
	storage        audioBriefingArchiveStorage
	standardBucket string
	iaBucket       string
	publicBucket   string
	moveAfterDays  int
	batchLimit     int
	now            func() time.Time
}

func NewAudioBriefingArchiveService(repo audioBriefingArchiveRepo, storage audioBriefingArchiveStorage) *AudioBriefingArchiveService {
	return &AudioBriefingArchiveService{
		repo:           repo,
		storage:        storage,
		standardBucket: AudioBriefingStandardBucketFromEnv(),
		iaBucket:       AudioBriefingIABucketFromEnv(),
		publicBucket:   AudioBriefingPublicBucketFromEnv(),
		moveAfterDays:  AudioBriefingIAMoveAfterDaysFromEnv(),
		batchLimit:     AudioBriefingIAMoveBatchLimitFromEnv(),
		now:            time.Now,
	}
}

func (s *AudioBriefingArchiveService) MovePublishedToIA(ctx context.Context) (*AudioBriefingArchiveResult, error) {
	result := &AudioBriefingArchiveResult{}
	if s == nil || s.repo == nil || s.storage == nil {
		return result, nil
	}
	standardBucket := strings.TrimSpace(s.standardBucket)
	iaBucket := strings.TrimSpace(s.iaBucket)
	publicBucket := strings.TrimSpace(s.publicBucket)
	if standardBucket == "" || iaBucket == "" || standardBucket == iaBucket {
		return result, nil
	}
	if publicBucket == standardBucket || publicBucket == iaBucket {
		publicBucket = strings.TrimSpace(publicBucket)
	}
	nowFn := s.now
	if nowFn == nil {
		nowFn = time.Now
	}
	moveAfterDays := s.moveAfterDays
	if moveAfterDays <= 0 {
		moveAfterDays = 30
	}
	batchLimit := s.batchLimit
	if batchLimit <= 0 {
		batchLimit = 50
	}
	cutoff := nowFn().AddDate(0, 0, -moveAfterDays)
	candidates, err := s.repo.ListIAMoveCandidates(ctx, cutoff, batchLimit)
	if err != nil {
		return result, err
	}
	for _, job := range candidates {
		sourceBucket := NormalizeAudioBriefingStorageBucket(job.R2StorageBucket)
		if sourceBucket == "" {
			sourceBucket = standardBucket
		}
		if sourceBucket == iaBucket {
			continue
		}
		result.Processed++
		if err := s.moveJobToBucket(ctx, &job, sourceBucket, iaBucket, publicBucket); err != nil {
			result.Failed++
			return result, err
		}
		result.Moved++
	}
	return result, nil
}

func (s *AudioBriefingArchiveService) moveJobToBucket(ctx context.Context, job *model.AudioBriefingJob, sourceBucket string, targetBucket string, publicBucket string) error {
	if job == nil {
		return nil
	}
	chunks, err := s.repo.ListJobChunks(ctx, job.UserID, job.ID)
	if err != nil {
		return err
	}
	refs := CollectAudioBriefingObjectRefs(job, chunks)
	grouped := groupAudioBriefingObjectRefsByBucket(refs)
	for bucket, objectKeys := range grouped {
		if bucket == "" || len(objectKeys) == 0 {
			continue
		}
		if err := s.storage.CopyAudioBriefingObjects(ctx, bucket, targetBucket, objectKeys); err != nil {
			return fmt.Errorf("copy audio briefing objects: %w", err)
		}
	}
	if err := s.repo.UpdateStorageBucketForJobAndChunks(ctx, job.ID, targetBucket); err != nil {
		return err
	}
	if publicKey := strings.TrimSpace(ptrString(job.PodcastPublicObjectKey)); publicBucket != "" && publicKey != "" {
		if err := s.storage.DeleteAudioBriefingObjectsInBucket(ctx, publicBucket, []string{publicKey}); err != nil {
			return fmt.Errorf("delete podcast public object: %w", err)
		}
		if _, err := s.repo.MarkPodcastPublicObjectDeleted(ctx, job.ID); err != nil {
			return fmt.Errorf("mark podcast public object deleted: %w", err)
		}
	}
	for bucket, objectKeys := range grouped {
		if bucket == "" || len(objectKeys) == 0 {
			continue
		}
		if err := s.storage.DeleteAudioBriefingObjectsInBucket(ctx, bucket, objectKeys); err != nil {
			return fmt.Errorf("delete archived source audio briefing objects: %w", err)
		}
	}
	return nil
}

func groupAudioBriefingObjectRefsByBucket(refs []AudioBriefingObjectRef) map[string][]string {
	grouped := make(map[string][]string)
	for _, ref := range refs {
		bucket := strings.TrimSpace(ref.Bucket)
		objectKey := strings.TrimSpace(ref.ObjectKey)
		if bucket == "" || objectKey == "" {
			continue
		}
		grouped[bucket] = append(grouped[bucket], objectKey)
	}
	return grouped
}

var ErrAudioBriefingArchiveDisabled = errors.New("audio briefing archive is disabled")

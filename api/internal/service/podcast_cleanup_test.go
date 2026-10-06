package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

type stubPodcastCleanupRepo struct {
	candidates          []model.AudioBriefingJob
	cutoff              time.Time
	limit               int
	listErr             error
	markedDeletedJobIDs []string
	markErr             error
}

func (s *stubPodcastCleanupRepo) ListExpiredPodcastPublicCopies(_ context.Context, cutoff time.Time, limit int) ([]model.AudioBriefingJob, error) {
	s.cutoff, s.limit = cutoff, limit
	return s.candidates, s.listErr
}

func (s *stubPodcastCleanupRepo) MarkPodcastPublicObjectDeleted(_ context.Context, jobID string) (*model.AudioBriefingJob, error) {
	if s.markErr != nil {
		return nil, s.markErr
	}
	s.markedDeletedJobIDs = append(s.markedDeletedJobIDs, jobID)
	return &model.AudioBriefingJob{ID: jobID}, nil
}

type podcastDeleteCall struct {
	bucket string
	keys   []string
}

type stubPodcastCleanupStorage struct {
	deleteCalls []podcastDeleteCall
	deleteErr   error
}

func (s *stubPodcastCleanupStorage) DeleteAudioBriefingObjectsInBucket(_ context.Context, bucket string, keys []string) error {
	s.deleteCalls = append(s.deleteCalls, podcastDeleteCall{bucket: bucket, keys: append([]string(nil), keys...)})
	return s.deleteErr
}

func TestExpiredPodcastCleanupDoesNotCopyPrivateAudio(t *testing.T) {
	t.Setenv("AUDIO_BRIEFING_R2_STANDARD_BUCKET", "briefings-standard")
	t.Setenv("AUDIO_BRIEFING_R2_IA_BUCKET", "briefings-ia")
	t.Setenv("PODCAST_EPISODE_RETENTION_DAYS", "30")
	t.Setenv("PODCAST_PUBLIC_CLEANUP_BATCH_LIMIT", "20")
	audioKey := "audio-briefings/user-1/job-1/episode.mp3"
	publicKey := "podcasts/user-1/job-1.mp3"
	repo := &stubPodcastCleanupRepo{candidates: []model.AudioBriefingJob{{
		ID: "job-1", R2StorageBucket: "briefings-standard", R2AudioObjectKey: &audioKey,
		PodcastPublicBucket: "briefings-public", PodcastPublicObjectKey: &publicKey,
	}}}
	storage := &stubPodcastCleanupStorage{}
	svc := NewPodcastCleanupService(repo, storage)
	now := time.Date(2026, 10, 6, 3, 17, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	result, err := svc.DeleteExpiredPublicCopies(context.Background())
	if err != nil {
		t.Fatalf("expired podcast cleanup failed: %v", err)
	}
	if result.Processed != 1 || result.Deleted != 1 || result.Failed != 0 {
		t.Fatalf("result = %+v, want processed=1 deleted=1 failed=0", result)
	}
	if !repo.cutoff.Equal(now.AddDate(0, 0, -30)) || repo.limit != 20 {
		t.Fatalf("cutoff=%v limit=%d", repo.cutoff, repo.limit)
	}
	if len(storage.deleteCalls) != 1 || storage.deleteCalls[0].bucket != "briefings-public" || len(storage.deleteCalls[0].keys) != 1 || storage.deleteCalls[0].keys[0] != publicKey {
		t.Fatalf("delete calls = %+v, want only the public podcast copy", storage.deleteCalls)
	}
	if len(repo.markedDeletedJobIDs) != 1 || repo.markedDeletedJobIDs[0] != "job-1" {
		t.Fatalf("marked deleted jobs = %v", repo.markedDeletedJobIDs)
	}
	if repo.candidates[0].R2StorageBucket != "briefings-standard" || *repo.candidates[0].R2AudioObjectKey != audioKey {
		t.Fatal("private audio storage changed")
	}
}

func TestPodcastCleanupSkipsPrivateBucketsAndDeletedCopies(t *testing.T) {
	t.Setenv("AUDIO_BRIEFING_R2_STANDARD_BUCKET", "briefings-standard")
	t.Setenv("AUDIO_BRIEFING_R2_IA_BUCKET", "briefings-ia")
	now := time.Now()
	for _, job := range []model.AudioBriefingJob{
		{R2StorageBucket: "legacy-private", PodcastPublicBucket: "legacy-private", PodcastPublicObjectKey: stringPtr("episode.mp3")},
		{R2StorageBucket: "legacy-private", PodcastPublicBucket: "briefings-standard", PodcastPublicObjectKey: stringPtr("episode.mp3")},
		{R2StorageBucket: "briefings-standard", PodcastPublicBucket: "briefings-ia", PodcastPublicObjectKey: stringPtr("episode.mp3")},
		{PodcastPublicBucket: "briefings-public", PodcastPublicObjectKey: stringPtr("episode.mp3"), PodcastPublicDeletedAt: &now},
		{PodcastPublicBucket: "briefings-public"},
		{PodcastPublicObjectKey: stringPtr("episode.mp3")},
	} {
		repo := &stubPodcastCleanupRepo{candidates: []model.AudioBriefingJob{job}}
		storage := &stubPodcastCleanupStorage{}
		result, err := NewPodcastCleanupService(repo, storage).DeleteExpiredPublicCopies(context.Background())
		if err != nil || result.Processed != 0 || len(storage.deleteCalls) != 0 || len(repo.markedDeletedJobIDs) != 0 {
			t.Fatalf("unsafe candidate %+v: result=%+v err=%v deletes=%v", job, result, err, storage.deleteCalls)
		}
	}
}

func TestPodcastCleanupFailureCanBeRetried(t *testing.T) {
	for _, stage := range []string{"list", "delete", "mark"} {
		t.Run(stage, func(t *testing.T) {
			failure := errors.New("temporary failure")
			repo := &stubPodcastCleanupRepo{candidates: []model.AudioBriefingJob{{
				ID: "job-1", R2StorageBucket: "briefings-standard",
				PodcastPublicBucket: "briefings-public", PodcastPublicObjectKey: stringPtr("episode.mp3"),
			}}}
			storage := &stubPodcastCleanupStorage{}
			switch stage {
			case "list":
				repo.listErr = failure
			case "delete":
				storage.deleteErr = failure
			case "mark":
				repo.markErr = failure
			}
			svc := NewPodcastCleanupService(repo, storage)
			result, err := svc.DeleteExpiredPublicCopies(context.Background())
			if !errors.Is(err, failure) || result.Deleted != 0 || len(repo.markedDeletedJobIDs) != 0 {
				t.Fatalf("result=%+v err=%v marked=%v", result, err, repo.markedDeletedJobIDs)
			}
			if stage == "list" && len(storage.deleteCalls) != 0 {
				t.Fatal("storage called after candidate query failed")
			}
			repo.listErr, repo.markErr, storage.deleteErr = nil, nil, nil
			result, err = svc.DeleteExpiredPublicCopies(context.Background())
			if err != nil || result.Deleted != 1 || len(repo.markedDeletedJobIDs) != 1 {
				t.Fatalf("retry result=%+v err=%v marked=%v", result, err, repo.markedDeletedJobIDs)
			}
		})
	}
}

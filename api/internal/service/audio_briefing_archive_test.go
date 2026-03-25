package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

type stubAudioBriefingArchiveRepo struct {
	candidates         []model.AudioBriefingJob
	listCandidatesAt   time.Time
	listCandidatesN    int
	listErr            error
	chunksByJobID      map[string][]model.AudioBriefingScriptChunk
	chunkErr           error
	updatedJobID       string
	updatedBucket      string
	updateErr          error
	markedDeletedJobID string
	markDeletedErr     error
}

func (s *stubAudioBriefingArchiveRepo) ListIAMoveCandidates(_ context.Context, cutoff time.Time, limit int) ([]model.AudioBriefingJob, error) {
	s.listCandidatesAt = cutoff
	s.listCandidatesN = limit
	return s.candidates, s.listErr
}

func (s *stubAudioBriefingArchiveRepo) ListJobChunks(_ context.Context, _, jobID string) ([]model.AudioBriefingScriptChunk, error) {
	if s.chunkErr != nil {
		return nil, s.chunkErr
	}
	return append([]model.AudioBriefingScriptChunk(nil), s.chunksByJobID[jobID]...), nil
}

func (s *stubAudioBriefingArchiveRepo) UpdateStorageBucketForJobAndChunks(_ context.Context, jobID string, bucket string) error {
	s.updatedJobID = jobID
	s.updatedBucket = bucket
	return s.updateErr
}

func (s *stubAudioBriefingArchiveRepo) MarkPodcastPublicObjectDeleted(_ context.Context, jobID string) (*model.AudioBriefingJob, error) {
	s.markedDeletedJobID = jobID
	if s.markDeletedErr != nil {
		return nil, s.markDeletedErr
	}
	return &model.AudioBriefingJob{ID: jobID}, nil
}

type audioBriefingCopyCall struct {
	sourceBucket string
	targetBucket string
	objectKeys   []string
}

type audioBriefingDeleteCall struct {
	bucket     string
	objectKeys []string
}

type stubAudioBriefingArchiveStorage struct {
	copyCalls   []audioBriefingCopyCall
	copyErr     error
	deleteCalls []audioBriefingDeleteCall
	deleteErr   error
}

func (s *stubAudioBriefingArchiveStorage) CopyAudioBriefingObjects(ctx context.Context, sourceBucket string, targetBucket string, objectKeys []string) error {
	s.copyCalls = append(s.copyCalls, audioBriefingCopyCall{
		sourceBucket: sourceBucket,
		targetBucket: targetBucket,
		objectKeys:   append([]string(nil), objectKeys...),
	})
	return s.copyErr
}

func (s *stubAudioBriefingArchiveStorage) DeleteAudioBriefingObjectsInBucket(ctx context.Context, bucket string, objectKeys []string) error {
	s.deleteCalls = append(s.deleteCalls, audioBriefingDeleteCall{
		bucket:     bucket,
		objectKeys: append([]string(nil), objectKeys...),
	})
	return s.deleteErr
}

func TestAudioBriefingArchiveServiceMovesPublishedFilesToIABucket(t *testing.T) {
	publishedAt := time.Date(2026, 2, 10, 8, 0, 0, 0, time.UTC)
	audioKey := "audio-briefings/user-1/job-1/episode.mp3"
	manifestKey := "audio-briefings/user-1/job-1/manifest.json"
	chunkKey1 := "audio-briefings/user-1/job-1/chunk-01.mp3"
	chunkKey2 := "audio-briefings/user-1/job-1/chunk-02.mp3"
	repo := &stubAudioBriefingArchiveRepo{
		candidates: []model.AudioBriefingJob{
			{
				ID:                     "job-1",
				UserID:                 "user-1",
				Status:                 "published",
				PublishedAt:            &publishedAt,
				R2StorageBucket:        "briefings-standard",
				R2AudioObjectKey:       &audioKey,
				R2ManifestObjectKey:    &manifestKey,
				PodcastPublicBucket:    "briefings-public",
				PodcastPublicObjectKey: &audioKey,
			},
		},
		chunksByJobID: map[string][]model.AudioBriefingScriptChunk{
			"job-1": {
				{ID: "chunk-1", R2AudioObjectKey: &chunkKey1, R2StorageBucket: "briefings-standard"},
				{ID: "chunk-2", R2AudioObjectKey: &chunkKey2, R2StorageBucket: "briefings-standard"},
			},
		},
	}
	storage := &stubAudioBriefingArchiveStorage{}
	svc := &AudioBriefingArchiveService{
		repo:           repo,
		storage:        storage,
		standardBucket: "briefings-standard",
		iaBucket:       "briefings-ia",
		publicBucket:   "briefings-public",
		moveAfterDays:  30,
		batchLimit:     20,
		now: func() time.Time {
			return time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
		},
	}

	result, err := svc.MovePublishedToIA(context.Background())
	if err != nil {
		t.Fatalf("MovePublishedToIA(...) error = %v", err)
	}
	if result.Moved != 1 || result.Processed != 1 || result.Failed != 0 {
		t.Fatalf("result = %+v, want moved=1 processed=1 failed=0", result)
	}
	if len(storage.copyCalls) != 1 {
		t.Fatalf("copy call count = %d, want 1", len(storage.copyCalls))
	}
	if got := storage.copyCalls[0]; got.sourceBucket != "briefings-standard" || got.targetBucket != "briefings-ia" {
		t.Fatalf("copy call = %+v, want standard -> ia", got)
	}
	if repo.updatedJobID != "job-1" || repo.updatedBucket != "briefings-ia" {
		t.Fatalf("updated job = %q bucket = %q, want job-1 / briefings-ia", repo.updatedJobID, repo.updatedBucket)
	}
	if len(storage.deleteCalls) != 2 {
		t.Fatalf("delete call count = %d, want 2", len(storage.deleteCalls))
	}
	if got := storage.deleteCalls[0]; got.bucket != "briefings-public" {
		t.Fatalf("first delete bucket = %q, want briefings-public", got.bucket)
	}
	if got := storage.deleteCalls[1]; got.bucket != "briefings-standard" {
		t.Fatalf("second delete bucket = %q, want briefings-standard", got.bucket)
	}
	if repo.markedDeletedJobID != "job-1" {
		t.Fatalf("markedDeletedJobID = %q, want job-1", repo.markedDeletedJobID)
	}
}

func TestAudioBriefingArchiveServiceSkipsSourceDeleteWhenBucketUpdateFails(t *testing.T) {
	publishedAt := time.Date(2026, 2, 10, 8, 0, 0, 0, time.UTC)
	audioKey := "audio-briefings/user-1/job-1/episode.mp3"
	repo := &stubAudioBriefingArchiveRepo{
		candidates: []model.AudioBriefingJob{
			{
				ID:                     "job-1",
				UserID:                 "user-1",
				Status:                 "published",
				PublishedAt:            &publishedAt,
				R2StorageBucket:        "briefings-standard",
				R2AudioObjectKey:       &audioKey,
				PodcastPublicBucket:    "briefings-public",
				PodcastPublicObjectKey: &audioKey,
			},
		},
		chunksByJobID: map[string][]model.AudioBriefingScriptChunk{"job-1": {}},
		updateErr:     errors.New("update failed"),
	}
	storage := &stubAudioBriefingArchiveStorage{}
	svc := &AudioBriefingArchiveService{
		repo:           repo,
		storage:        storage,
		standardBucket: "briefings-standard",
		iaBucket:       "briefings-ia",
		publicBucket:   "briefings-public",
		moveAfterDays:  30,
		batchLimit:     20,
		now: func() time.Time {
			return time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
		},
	}

	_, err := svc.MovePublishedToIA(context.Background())
	if err == nil {
		t.Fatal("MovePublishedToIA(...) error = nil, want error")
	}
	if len(storage.copyCalls) != 1 {
		t.Fatalf("copy call count = %d, want 1", len(storage.copyCalls))
	}
	if len(storage.deleteCalls) != 0 {
		t.Fatalf("delete call count = %d, want 0", len(storage.deleteCalls))
	}
}

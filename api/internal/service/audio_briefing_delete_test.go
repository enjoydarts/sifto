package service

import (
	"context"
	"errors"
	"testing"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
)

type stubAudioBriefingDeleteRepo struct {
	job         *model.AudioBriefingJob
	jobErr      error
	chunks      []model.AudioBriefingScriptChunk
	chunkErr    error
	deleteErr   error
	deleteCalls int
}

func (s *stubAudioBriefingDeleteRepo) GetJobByID(_ context.Context, _, _ string) (*model.AudioBriefingJob, error) {
	return s.job, s.jobErr
}

func (s *stubAudioBriefingDeleteRepo) ListJobChunks(_ context.Context, _, _ string) ([]model.AudioBriefingScriptChunk, error) {
	return s.chunks, s.chunkErr
}

func (s *stubAudioBriefingDeleteRepo) DeleteJob(_ context.Context, _, _ string) error {
	s.deleteCalls++
	return s.deleteErr
}

type stubAudioBriefingObjectDeleter struct {
	refs  []AudioBriefingObjectRef
	calls int
	err   error
}

func (s *stubAudioBriefingObjectDeleter) DeleteAudioBriefingObjects(_ context.Context, objectRefs []AudioBriefingObjectRef) error {
	s.calls++
	s.refs = append([]AudioBriefingObjectRef(nil), objectRefs...)
	return s.err
}

func TestAudioBriefingDeleteServiceDeletesObjectsThenJob(t *testing.T) {
	audioKey := "audio-briefings/user-1/job-1/episode.mp3"
	manifestKey := "audio-briefings/user-1/job-1/manifest.json"
	chunkKey := "audio-briefings/user-1/job-1/chunk-1.mp3"
	repo := &stubAudioBriefingDeleteRepo{
		job: &model.AudioBriefingJob{
			ID:                  "job-1",
			UserID:              "user-1",
			Status:              "published",
			R2AudioObjectKey:    &audioKey,
			R2ManifestObjectKey: &manifestKey,
			R2StorageBucket:     "briefings-ia",
		},
		chunks: []model.AudioBriefingScriptChunk{
			{R2AudioObjectKey: &chunkKey, R2StorageBucket: "briefings-standard"},
			{R2AudioObjectKey: &chunkKey, R2StorageBucket: "briefings-standard"},
		},
	}
	deleter := &stubAudioBriefingObjectDeleter{}
	service := NewAudioBriefingDeleteService(repo, deleter)

	if err := service.Delete(context.Background(), "user-1", "job-1"); err != nil {
		t.Fatalf("Delete(...) error = %v", err)
	}
	if deleter.calls != 1 {
		t.Fatalf("DeleteAudioBriefingObjects call count = %d, want 1", deleter.calls)
	}
	wantRefs := []AudioBriefingObjectRef{
		{Bucket: "briefings-ia", ObjectKey: audioKey},
		{Bucket: "briefings-ia", ObjectKey: manifestKey},
		{Bucket: "briefings-standard", ObjectKey: chunkKey},
	}
	if len(deleter.refs) != len(wantRefs) {
		t.Fatalf("deleted refs len = %d, want %d (%v)", len(deleter.refs), len(wantRefs), deleter.refs)
	}
	for i, want := range wantRefs {
		if deleter.refs[i] != want {
			t.Fatalf("deleted ref[%d] = %+v, want %+v", i, deleter.refs[i], want)
		}
	}
	if repo.deleteCalls != 1 {
		t.Fatalf("DeleteJob call count = %d, want 1", repo.deleteCalls)
	}
}

func TestAudioBriefingDeleteServiceRejectsActiveJob(t *testing.T) {
	repo := &stubAudioBriefingDeleteRepo{
		job: &model.AudioBriefingJob{
			ID:     "job-1",
			UserID: "user-1",
			Status: "concatenating",
		},
	}
	deleter := &stubAudioBriefingObjectDeleter{}
	service := NewAudioBriefingDeleteService(repo, deleter)

	err := service.Delete(context.Background(), "user-1", "job-1")
	if !errors.Is(err, repository.ErrInvalidState) {
		t.Fatalf("Delete(...) error = %v, want ErrInvalidState", err)
	}
	if deleter.calls != 0 {
		t.Fatalf("DeleteAudioBriefingObjects call count = %d, want 0", deleter.calls)
	}
	if repo.deleteCalls != 0 {
		t.Fatalf("DeleteJob call count = %d, want 0", repo.deleteCalls)
	}
}

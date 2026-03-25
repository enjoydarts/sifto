package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

type stubAudioBriefingStaleVoicingRepo struct {
	jobs               []model.AudioBriefingJob
	jobsErr            error
	chunksByJobID      map[string][]model.AudioBriefingScriptChunk
	chunkErrByJobID    map[string]error
	markedChunkIDs     []string
	markedChunkMessage []string
	failJobIDs         []string
	failMessages       []string
	failErrByJobID     map[string]error
}

func (s *stubAudioBriefingStaleVoicingRepo) ListStaleVoicingJobs(_ context.Context, _ time.Time, _ int) ([]model.AudioBriefingJob, error) {
	if s.jobsErr != nil {
		return nil, s.jobsErr
	}
	return append([]model.AudioBriefingJob(nil), s.jobs...), nil
}

func (s *stubAudioBriefingStaleVoicingRepo) ListJobChunks(_ context.Context, _ string, jobID string) ([]model.AudioBriefingScriptChunk, error) {
	if err := s.chunkErrByJobID[jobID]; err != nil {
		return nil, err
	}
	return append([]model.AudioBriefingScriptChunk(nil), s.chunksByJobID[jobID]...), nil
}

func (s *stubAudioBriefingStaleVoicingRepo) MarkChunkFailed(_ context.Context, chunkID string, errorMessage string) error {
	s.markedChunkIDs = append(s.markedChunkIDs, chunkID)
	s.markedChunkMessage = append(s.markedChunkMessage, errorMessage)
	return nil
}

func (s *stubAudioBriefingStaleVoicingRepo) FailVoicingJob(_ context.Context, jobID string, _ string, errorMessage string) (*model.AudioBriefingJob, error) {
	if err := s.failErrByJobID[jobID]; err != nil {
		return nil, err
	}
	s.failJobIDs = append(s.failJobIDs, jobID)
	s.failMessages = append(s.failMessages, errorMessage)
	return &model.AudioBriefingJob{ID: jobID, Status: "failed"}, nil
}

func TestAudioBriefingStaleVoicingServiceFailsStalledGeneratingJobs(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	repo := &stubAudioBriefingStaleVoicingRepo{
		jobs: []model.AudioBriefingJob{
			{ID: "job-1", UserID: "user-1", Status: "voicing"},
		},
		chunksByJobID: map[string][]model.AudioBriefingScriptChunk{
			"job-1": {
				{ID: "chunk-1", TTSStatus: "generating", UpdatedAt: now.Add(-16 * time.Minute)},
				{ID: "chunk-2", TTSStatus: "pending", UpdatedAt: now},
			},
		},
		chunkErrByJobID: map[string]error{},
		failErrByJobID:  map[string]error{},
	}
	service := NewAudioBriefingStaleVoicingService(repo)
	service.now = func() time.Time { return now }
	service.staleAfter = 15 * time.Minute

	result, err := service.FailStaleJobs(context.Background())
	if err != nil {
		t.Fatalf("FailStaleJobs(...) error = %v", err)
	}
	if result == nil || result.Processed != 1 || result.Failed != 0 {
		t.Fatalf("result = %#v, want processed=1 failed=0", result)
	}
	if len(repo.markedChunkIDs) != 1 || repo.markedChunkIDs[0] != "chunk-1" {
		t.Fatalf("markedChunkIDs = %v, want [chunk-1]", repo.markedChunkIDs)
	}
	if len(repo.failJobIDs) != 1 || repo.failJobIDs[0] != "job-1" {
		t.Fatalf("failJobIDs = %v, want [job-1]", repo.failJobIDs)
	}
}

func TestAudioBriefingStaleVoicingServiceCountsPerJobFailures(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	repo := &stubAudioBriefingStaleVoicingRepo{
		jobs: []model.AudioBriefingJob{
			{ID: "job-1", UserID: "user-1", Status: "voicing"},
		},
		chunksByJobID: map[string][]model.AudioBriefingScriptChunk{
			"job-1": {
				{ID: "chunk-1", TTSStatus: "generating", UpdatedAt: now.Add(-16 * time.Minute)},
			},
		},
		chunkErrByJobID: map[string]error{},
		failErrByJobID: map[string]error{
			"job-1": errors.New("fail voicing"),
		},
	}
	service := NewAudioBriefingStaleVoicingService(repo)
	service.now = func() time.Time { return now }
	service.staleAfter = 15 * time.Minute

	result, err := service.FailStaleJobs(context.Background())
	if err != nil {
		t.Fatalf("FailStaleJobs(...) error = %v", err)
	}
	if result == nil || result.Processed != 0 || result.Failed != 1 {
		t.Fatalf("result = %#v, want processed=0 failed=1", result)
	}
}

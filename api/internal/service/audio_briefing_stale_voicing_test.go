package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

type stubAudioBriefingStaleVoicingRepo struct {
	jobs              []model.AudioBriefingJob
	jobsErr           error
	chunksByJobID     map[string][]model.AudioBriefingScriptChunk
	chunkErrByJobID   map[string]error
	retryWaitChunkIDs []string
	retryWaitMessages []string
	exhaustedChunkIDs []string
	exhaustedMessages []string
	failJobIDs        []string
	failMessages      []string
	failErrByJobID    map[string]error
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

func (s *stubAudioBriefingStaleVoicingRepo) MarkChunkRetryWait(_ context.Context, chunkID string, errorCode string, errorMessage string) error {
	s.retryWaitChunkIDs = append(s.retryWaitChunkIDs, chunkID)
	s.retryWaitMessages = append(s.retryWaitMessages, errorCode+":"+errorMessage)
	return nil
}

func (s *stubAudioBriefingStaleVoicingRepo) MarkChunkExhausted(_ context.Context, chunkID string, errorCode string, errorMessage string) error {
	s.exhaustedChunkIDs = append(s.exhaustedChunkIDs, chunkID)
	s.exhaustedMessages = append(s.exhaustedMessages, errorCode+":"+errorMessage)
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

func TestAudioBriefingStaleVoicingServiceMovesStalledChunkToRetryWaitBeforeExhaustion(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	repo := &stubAudioBriefingStaleVoicingRepo{
		jobs: []model.AudioBriefingJob{
			{ID: "job-1", UserID: "user-1", Status: "voicing"},
		},
		chunksByJobID: map[string][]model.AudioBriefingScriptChunk{
			"job-1": {
				{ID: "chunk-1", TTSStatus: "generating", AttemptCount: 2, LastHeartbeatAt: ptrTime(now.Add(-16 * time.Minute)), UpdatedAt: now.Add(-16 * time.Minute)},
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
	if len(repo.retryWaitChunkIDs) != 1 || repo.retryWaitChunkIDs[0] != "chunk-1" {
		t.Fatalf("retryWaitChunkIDs = %v, want [chunk-1]", repo.retryWaitChunkIDs)
	}
	if len(repo.exhaustedChunkIDs) != 0 {
		t.Fatalf("exhaustedChunkIDs = %v, want none", repo.exhaustedChunkIDs)
	}
	if len(repo.failJobIDs) != 0 {
		t.Fatalf("failJobIDs = %v, want none", repo.failJobIDs)
	}
}

func TestAudioBriefingStaleVoicingServiceExhaustsChunkAfterThreeAttempts(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	repo := &stubAudioBriefingStaleVoicingRepo{
		jobs: []model.AudioBriefingJob{
			{ID: "job-1", UserID: "user-1", Status: "voicing"},
		},
		chunksByJobID: map[string][]model.AudioBriefingScriptChunk{
			"job-1": {
				{ID: "chunk-1", TTSStatus: "generating", AttemptCount: 3, LastHeartbeatAt: ptrTime(now.Add(-16 * time.Minute)), UpdatedAt: now.Add(-16 * time.Minute)},
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
	if len(repo.retryWaitChunkIDs) != 0 {
		t.Fatalf("retryWaitChunkIDs = %v, want none", repo.retryWaitChunkIDs)
	}
	if len(repo.exhaustedChunkIDs) != 1 || repo.exhaustedChunkIDs[0] != "chunk-1" {
		t.Fatalf("exhaustedChunkIDs = %v, want [chunk-1]", repo.exhaustedChunkIDs)
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
				{ID: "chunk-1", TTSStatus: "generating", AttemptCount: 3, LastHeartbeatAt: ptrTime(now.Add(-16 * time.Minute)), UpdatedAt: now.Add(-16 * time.Minute)},
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

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

type stubAudioBriefingConcatRepo struct {
	job                       *model.AudioBriefingJob
	chunks                    []model.AudioBriefingScriptChunk
	updateConcatProviderJobID error
	failConcatLaunchCalls     int
	failConcatLaunchCode      string
	failConcatLaunchMessage   string
}

func (s *stubAudioBriefingConcatRepo) GetJobByID(_ context.Context, _ string, _ string) (*model.AudioBriefingJob, error) {
	return s.job, nil
}

func (s *stubAudioBriefingConcatRepo) ListJobChunks(_ context.Context, _ string, _ string) ([]model.AudioBriefingScriptChunk, error) {
	return s.chunks, nil
}

func (s *stubAudioBriefingConcatRepo) BeginConcatCallback(_ context.Context, _ string, _ string, _ string, _, _ *string, _ time.Time) (*model.AudioBriefingJob, *model.AudioBriefingCallbackToken, error) {
	return s.job, nil, nil
}

func (s *stubAudioBriefingConcatRepo) UpdateConcatProviderJobID(_ context.Context, _ string, _ string) (*model.AudioBriefingJob, error) {
	if s.updateConcatProviderJobID != nil {
		return nil, s.updateConcatProviderJobID
	}
	return s.job, nil
}

func (s *stubAudioBriefingConcatRepo) FailConcatLaunch(_ context.Context, _ string, errorCode string, errorMessage string) (*model.AudioBriefingJob, error) {
	s.failConcatLaunchCalls++
	s.failConcatLaunchCode = errorCode
	s.failConcatLaunchMessage = errorMessage
	return s.job, nil
}

type stubAudioConcatRunner struct {
	resp *AudioConcatRunResponse
	err  error
}

func (s *stubAudioConcatRunner) Enabled() bool { return true }

func (s *stubAudioConcatRunner) RunAudioConcat(_ context.Context, _ AudioConcatRunRequest) (*AudioConcatRunResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.resp, nil
}

func TestAudioBriefingConcatStarterFailsLaunchWhenProviderJobIDUpdateFails(t *testing.T) {
	t.Setenv("APP_BASE_URL", "https://api.example.com")

	repo := &stubAudioBriefingConcatRepo{
		job: &model.AudioBriefingJob{
			ID:     "job-1",
			UserID: "user-1",
			Status: "concatenating",
		},
		chunks: []model.AudioBriefingScriptChunk{
			{R2AudioObjectKey: stringPtr("audio-briefings/user-1/job-1/chunk-1.mp3")},
		},
		updateConcatProviderJobID: errors.New("write failed"),
	}
	starter := &AudioBriefingConcatStarter{
		repo: repo,
		runner: &stubAudioConcatRunner{
			resp: &AudioConcatRunResponse{ExecutionName: "projects/p/locations/nrt/jobs/j/executions/e"},
		},
		mode: audioBriefingConcatModeCloudRun,
	}

	err := starter.Start(context.Background(), "user-1", "job-1")
	if err == nil || err.Error() != "write failed" {
		t.Fatalf("Start(...) error = %v, want write failed", err)
	}
	if repo.failConcatLaunchCalls != 1 {
		t.Fatalf("FailConcatLaunch call count = %d, want 1", repo.failConcatLaunchCalls)
	}
	if repo.failConcatLaunchCode != "concat_launch_failed" {
		t.Fatalf("FailConcatLaunch code = %q, want concat_launch_failed", repo.failConcatLaunchCode)
	}
	if repo.failConcatLaunchMessage != "write failed" {
		t.Fatalf("FailConcatLaunch message = %q, want write failed", repo.failConcatLaunchMessage)
	}
}

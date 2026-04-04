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
	settings                  *model.AudioBriefingSettings
	chunks                    []model.AudioBriefingScriptChunk
	updateConcatProviderJobID error
	failConcatLaunchCalls     int
	failConcatLaunchCode      string
	failConcatLaunchMessage   string
}

func (s *stubAudioBriefingConcatRepo) GetJobByID(_ context.Context, _ string, _ string) (*model.AudioBriefingJob, error) {
	return s.job, nil
}

func (s *stubAudioBriefingConcatRepo) GetSettings(_ context.Context, _ string) (*model.AudioBriefingSettings, error) {
	return s.settings, nil
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
	req  *AudioConcatRunRequest
}

func (s *stubAudioConcatRunner) Enabled() bool { return true }

func (s *stubAudioConcatRunner) RunAudioConcat(_ context.Context, req AudioConcatRunRequest) (*AudioConcatRunResponse, error) {
	s.req = &req
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
		settings: &model.AudioBriefingSettings{},
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

func TestAudioBriefingConcatStarterPassesBGMConfigToRunner(t *testing.T) {
	t.Setenv("APP_BASE_URL", "https://api.example.com")

	repo := &stubAudioBriefingConcatRepo{
		job: &model.AudioBriefingJob{
			ID:     "job-1",
			UserID: "user-1",
			Status: "voiced",
		},
		settings: &model.AudioBriefingSettings{
			BGMEnabled:  true,
			BGMR2Prefix: stringPtr("audio/bgm"),
		},
		chunks: []model.AudioBriefingScriptChunk{
			{R2AudioObjectKey: stringPtr("audio-briefings/user-1/job-1/chunk-1.mp3")},
		},
	}
	runner := &stubAudioConcatRunner{
		resp: &AudioConcatRunResponse{ExecutionName: "projects/p/locations/nrt/jobs/j/executions/e"},
	}
	starter := &AudioBriefingConcatStarter{
		repo:   repo,
		runner: runner,
		mode:   audioBriefingConcatModeCloudRun,
	}

	err := starter.Start(context.Background(), "user-1", "job-1")
	if err != nil {
		t.Fatalf("Start(...) error = %v", err)
	}
	if runner.req == nil {
		t.Fatal("runner request was not captured")
	}
	if !runner.req.BGMEnabled {
		t.Fatalf("runner.req.BGMEnabled = %v, want true", runner.req.BGMEnabled)
	}
	if runner.req.BGMR2Prefix != "audio/bgm" {
		t.Fatalf("runner.req.BGMR2Prefix = %q, want audio/bgm", runner.req.BGMR2Prefix)
	}
}

func TestAudioBriefingConcatStarterDeduplicatesConsecutiveChunkAudioKeys(t *testing.T) {
	t.Setenv("APP_BASE_URL", "https://api.example.com")

	shared := "audio-briefings/user-1/job-1/opening.wav"
	repo := &stubAudioBriefingConcatRepo{
		job: &model.AudioBriefingJob{
			ID:     "job-1",
			UserID: "user-1",
			Status: "voiced",
		},
		settings: &model.AudioBriefingSettings{},
		chunks: []model.AudioBriefingScriptChunk{
			{Seq: 1, R2AudioObjectKey: &shared},
			{Seq: 2, R2AudioObjectKey: &shared},
			{Seq: 3, R2AudioObjectKey: stringPtr("audio-briefings/user-1/job-1/article-1.wav")},
		},
	}
	runner := &stubAudioConcatRunner{
		resp: &AudioConcatRunResponse{ExecutionName: "projects/p/locations/nrt/jobs/j/executions/e"},
	}
	starter := &AudioBriefingConcatStarter{
		repo:   repo,
		runner: runner,
		mode:   audioBriefingConcatModeCloudRun,
	}

	if err := starter.Start(context.Background(), "user-1", "job-1"); err != nil {
		t.Fatalf("Start(...) error = %v", err)
	}
	if runner.req == nil {
		t.Fatal("runner request was not captured")
	}
	if len(runner.req.AudioObjectKeys) != 2 {
		t.Fatalf("len(runner.req.AudioObjectKeys) = %d, want 2", len(runner.req.AudioObjectKeys))
	}
	if runner.req.AudioObjectKeys[0] != shared {
		t.Fatalf("AudioObjectKeys[0] = %q, want shared opening key", runner.req.AudioObjectKeys[0])
	}
}

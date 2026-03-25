package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
)

func TestAudioBriefingManualSlotKeyAtUsesManualPrefix(t *testing.T) {
	now := time.Date(2026, 3, 25, 14, 37, 12, 123456789, timeutil.JST)

	got := AudioBriefingManualSlotKeyAt(now)

	if !strings.HasPrefix(got, "manual-2026-03-25-143712-") {
		t.Fatalf("AudioBriefingManualSlotKeyAt(...) = %q, want manual timestamp prefix", got)
	}
}

func TestAudioBriefingNextPipelineStage(t *testing.T) {
	tests := []struct {
		name   string
		job    model.AudioBriefingJob
		chunks []model.AudioBriefingScriptChunk
		want   audioBriefingPipelineStage
		err    bool
	}{
		{
			name: "pending goes to scripting",
			job:  model.AudioBriefingJob{Status: "pending"},
			want: audioBriefingPipelineStageScript,
		},
		{
			name: "scripted goes to voice",
			job:  model.AudioBriefingJob{Status: "scripted"},
			want: audioBriefingPipelineStageVoice,
		},
		{
			name: "voiced goes to concat",
			job:  model.AudioBriefingJob{Status: "voiced"},
			want: audioBriefingPipelineStageConcat,
		},
		{
			name: "failed with incomplete chunks resumes voice",
			job:  model.AudioBriefingJob{Status: "failed"},
			chunks: []model.AudioBriefingScriptChunk{
				{TTSStatus: "generated"},
				{TTSStatus: "failed"},
			},
			want: audioBriefingPipelineStageVoice,
		},
		{
			name: "failed with fully generated chunks resumes concat",
			job:  model.AudioBriefingJob{Status: "failed"},
			chunks: []model.AudioBriefingScriptChunk{
				{TTSStatus: "generated", R2AudioObjectKey: stringPtr("chunk-1")},
				{TTSStatus: "generated", R2AudioObjectKey: stringPtr("chunk-2")},
			},
			want: audioBriefingPipelineStageConcat,
		},
		{
			name: "published has no next stage",
			job:  model.AudioBriefingJob{Status: "published"},
			err:  true,
		},
		{
			name: "failed without chunks resumes scripting",
			job:  model.AudioBriefingJob{Status: "failed"},
			want: audioBriefingPipelineStageScript,
		},
	}

	for _, tt := range tests {
		got, err := audioBriefingNextPipelineStage(&tt.job, tt.chunks)
		if tt.err {
			if err == nil {
				t.Fatalf("%s: expected error, got nil", tt.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tt.name, err)
		}
		if got != tt.want {
			t.Fatalf("%s: stage = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestAudioBriefingShouldContinue(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{status: "pending", want: true},
		{status: "scripted", want: true},
		{status: "voiced", want: true},
		{status: "failed", want: true},
		{status: "voicing", want: false},
		{status: "published", want: false},
	}

	for _, tt := range tests {
		if got := audioBriefingShouldContinue(tt.status); got != tt.want {
			t.Fatalf("audioBriefingShouldContinue(%q) = %t, want %t", tt.status, got, tt.want)
		}
	}
}

func TestAudioBriefingArticleBatchSize(t *testing.T) {
	tests := []struct {
		itemCount int
		want      int
	}{
		{itemCount: 0, want: 1},
		{itemCount: 1, want: 1},
		{itemCount: 3, want: 3},
		{itemCount: 4, want: 4},
		{itemCount: 12, want: 4},
	}

	for _, tt := range tests {
		if got := audioBriefingArticleBatchSize(tt.itemCount); got != tt.want {
			t.Fatalf("audioBriefingArticleBatchSize(%d) = %d, want %d", tt.itemCount, got, tt.want)
		}
	}
}

func TestAudioBriefingArticleBatchTargetChars(t *testing.T) {
	got := audioBriefingArticleBatchTargetChars(12000, 20, 4)
	if got < 2000 {
		t.Fatalf("audioBriefingArticleBatchTargetChars(...) = %d, want at least 2000", got)
	}
}

func TestAudioBriefingGenerateArticleSegmentsBatchSplitsOnError(t *testing.T) {
	articles := []AudioBriefingScriptArticle{
		{ItemID: "item-1"},
		{ItemID: "item-2"},
		{ItemID: "item-3"},
		{ItemID: "item-4"},
	}
	var seen []int
	call := func(batch []AudioBriefingScriptArticle, _ int, _ bool, _ bool, _ bool, _ bool) (*AudioBriefingScriptResponse, error) {
		seen = append(seen, len(batch))
		if len(batch) > 1 {
			return nil, fmt.Errorf("batch too large")
		}
		return &AudioBriefingScriptResponse{
			ArticleSegments: []AudioBriefingScriptSegment{
				{ItemID: batch[0].ItemID, Headline: "h", Commentary: "c"},
			},
		}, nil
	}

	got, err := audioBriefingGenerateArticleSegmentsBatch(articles, 12000, 4, call)
	if err != nil {
		t.Fatalf("audioBriefingGenerateArticleSegmentsBatch(...) error = %v", err)
	}
	if len(got.Segments) != 4 {
		t.Fatalf("audioBriefingGenerateArticleSegmentsBatch(...) len = %d, want 4", len(got.Segments))
	}
	if len(seen) < 3 {
		t.Fatalf("expected recursive split calls, saw %v", seen)
	}
}

func TestAudioBriefingGenerateArticleSegmentsBatchTracksRecoveredFailures(t *testing.T) {
	articles := []AudioBriefingScriptArticle{
		{ItemID: "item-1"},
		{ItemID: "item-2"},
		{ItemID: "item-3"},
		{ItemID: "item-4"},
	}
	call := func(batch []AudioBriefingScriptArticle, _ int, _ bool, _ bool, _ bool, _ bool) (*AudioBriefingScriptResponse, error) {
		if len(batch) > 1 {
			return nil, fmt.Errorf("batch too large")
		}
		return &AudioBriefingScriptResponse{
			ArticleSegments: []AudioBriefingScriptSegment{
				{ItemID: batch[0].ItemID, Headline: "h", Commentary: "c"},
			},
		}, nil
	}

	result, err := audioBriefingGenerateArticleSegmentsBatch(articles, 12000, 4, call)
	if err != nil {
		t.Fatalf("audioBriefingGenerateArticleSegmentsBatch(...) error = %v", err)
	}
	if len(result.Segments) != 4 {
		t.Fatalf("segments len = %d, want 4", len(result.Segments))
	}
	if len(result.RecoveredFailures) == 0 {
		t.Fatal("expected recovered failures to be recorded")
	}
}

func TestAudioBriefingBuildDraftFailsWhenVoiceIsNotConfigured(t *testing.T) {
	orchestrator := &AudioBriefingOrchestrator{}

	_, err := orchestrator.buildDraft(
		t.Context(),
		"user-1",
		time.Date(2026, 3, 25, 6, 0, 0, 0, timeutil.JST),
		"editor",
		[]model.AudioBriefingJobItem{{ItemID: "item-1"}},
		nil,
		20,
	)
	if err == nil {
		t.Fatal("expected missing voice configuration error")
	}
	if !strings.Contains(err.Error(), "voice is not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecoverAudioBriefingStageErrorReturnsFailedJobWhenAlreadyFailed(t *testing.T) {
	job := &model.AudioBriefingJob{ID: "job-1", Status: "failed"}

	got, err := recoverAudioBriefingStageError(audioBriefingPipelineStageVoice, job, fmt.Errorf("boom"), func(errorCode, errorMessage string) (*model.AudioBriefingJob, error) {
		t.Fatalf("fail callback should not be called")
		return nil, nil
	})
	if err != nil {
		t.Fatalf("recoverAudioBriefingStageError(...) error = %v", err)
	}
	if got != job {
		t.Fatalf("recoverAudioBriefingStageError(...) job = %#v, want original job", got)
	}
}

func TestRecoverAudioBriefingStageErrorFailsActiveScriptingJob(t *testing.T) {
	job := &model.AudioBriefingJob{ID: "job-1", Status: "scripting"}
	var gotCode string
	var gotMessage string

	got, err := recoverAudioBriefingStageError(audioBriefingPipelineStageScript, job, fmt.Errorf("script boom"), func(errorCode, errorMessage string) (*model.AudioBriefingJob, error) {
		gotCode = errorCode
		gotMessage = errorMessage
		return &model.AudioBriefingJob{ID: "job-1", Status: "failed"}, nil
	})
	if err != nil {
		t.Fatalf("recoverAudioBriefingStageError(...) error = %v", err)
	}
	if got == nil || got.Status != "failed" {
		t.Fatalf("recoverAudioBriefingStageError(...) job = %#v, want failed job", got)
	}
	if gotCode != "script_failed" {
		t.Fatalf("errorCode = %q, want script_failed", gotCode)
	}
	if !strings.Contains(gotMessage, "script boom") {
		t.Fatalf("errorMessage = %q, want script boom", gotMessage)
	}
}

func TestRecoverAudioBriefingStageErrorFailsActiveVoicingJob(t *testing.T) {
	job := &model.AudioBriefingJob{ID: "job-1", Status: "voicing"}
	var gotCode string

	got, err := recoverAudioBriefingStageError(audioBriefingPipelineStageVoice, job, fmt.Errorf("tts boom"), func(errorCode, errorMessage string) (*model.AudioBriefingJob, error) {
		gotCode = errorCode
		return &model.AudioBriefingJob{ID: "job-1", Status: "failed"}, nil
	})
	if err != nil {
		t.Fatalf("recoverAudioBriefingStageError(...) error = %v", err)
	}
	if got == nil || got.Status != "failed" {
		t.Fatalf("recoverAudioBriefingStageError(...) job = %#v, want failed job", got)
	}
	if gotCode != "tts_failed" {
		t.Fatalf("errorCode = %q, want tts_failed", gotCode)
	}
}

func TestAudioBriefingFailureContextDetachesFromCanceledParent(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel()

	ctx, release := audioBriefingFailureContext(parent)
	defer release()

	if err := ctx.Err(); err != nil {
		t.Fatalf("failure context should not inherit cancellation: %v", err)
	}
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("failure context should have a deadline")
	}
}

func stringPtr(v string) *string { return &v }

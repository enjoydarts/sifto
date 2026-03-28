package repository

import (
	"errors"
	"testing"
	"time"
)

type stubAudioBriefingScanner struct {
	values []any
}

func (s stubAudioBriefingScanner) Scan(dest ...any) error {
	if len(dest) != len(s.values) {
		return errors.New("scan arg count mismatch")
	}
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = s.values[i].(string)
		case **string:
			if s.values[i] == nil {
				*d = nil
				continue
			}
			value := s.values[i].(string)
			*d = &value
		case *int:
			*d = s.values[i].(int)
		case **int:
			if s.values[i] == nil {
				*d = nil
				continue
			}
			value := s.values[i].(int)
			*d = &value
		case *time.Time:
			*d = s.values[i].(time.Time)
		case **time.Time:
			if s.values[i] == nil {
				*d = nil
				continue
			}
			value := s.values[i].(time.Time)
			*d = &value
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}

func TestAudioBriefingJobScanReadsStorageBucket(t *testing.T) {
	now := time.Date(2026, 3, 25, 11, 30, 0, 0, time.UTC)
	audioKey := "audio-briefings/user-1/job-1/episode.mp3"
	manifestKey := "audio-briefings/user-1/job-1/manifest.json"
	providerJobID := "provider-job-1"
	idempotencyKey := "manual-user-1"

	job, err := scanAudioBriefingJob(stubAudioBriefingScanner{values: []any{
		"job-1",
		"user-1",
		now,
		"2026-03-25-18",
		"editor",
		"published",
		"active",
		6,
		0,
		12000,
		"claude-sonnet-4-20250514",
		1800,
		"夕方の音声ブリーフィング",
		audioKey,
		manifestKey,
		"bgm/track-1.mp3",
		"briefings-ia",
		audioKey,
		"briefings-public",
		now,
		providerJobID,
		idempotencyKey,
		"",
		"",
		now,
		now,
		now,
		now,
	}})
	if err != nil {
		t.Fatalf("scanAudioBriefingJob(...) error = %v", err)
	}
	if job.R2StorageBucket != "briefings-ia" {
		t.Fatalf("job.R2StorageBucket = %q, want briefings-ia", job.R2StorageBucket)
	}
}

func TestAudioBriefingScriptChunkScanReadsStorageBucket(t *testing.T) {
	now := time.Date(2026, 3, 25, 11, 30, 0, 0, time.UTC)
	audioKey := "audio-briefings/user-1/job-1/chunk-01.mp3"
	ttsProvider := "aivis"
	voiceModel := "voice-model"
	voiceStyle := "voice-style"

	chunk, err := scanAudioBriefingScriptChunk(stubAudioBriefingScanner{values: []any{
		"chunk-1",
		"job-1",
		1,
		"opening",
		"本文です。",
		12,
		"generated",
		1,
		"",
		ttsProvider,
		voiceModel,
		voiceStyle,
		audioKey,
		"briefings-standard",
		120,
		"",
		"",
		nil,
		nil,
		nil,
		now,
		now,
	}})
	if err != nil {
		t.Fatalf("scanAudioBriefingScriptChunk(...) error = %v", err)
	}
	if chunk.R2StorageBucket != "briefings-standard" {
		t.Fatalf("chunk.R2StorageBucket = %q, want briefings-standard", chunk.R2StorageBucket)
	}
}

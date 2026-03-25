package service

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

type audioBriefingStaleVoicingRepo interface {
	ListStaleVoicingJobs(ctx context.Context, cutoff time.Time, limit int) ([]model.AudioBriefingJob, error)
	ListJobChunks(ctx context.Context, userID, jobID string) ([]model.AudioBriefingScriptChunk, error)
	MarkChunkFailed(ctx context.Context, chunkID string, errorMessage string) error
	FailVoicingJob(ctx context.Context, jobID string, errorCode string, errorMessage string) (*model.AudioBriefingJob, error)
}

type AudioBriefingStaleVoicingResult struct {
	Processed int
	Failed    int
}

type AudioBriefingStaleVoicingService struct {
	repo       audioBriefingStaleVoicingRepo
	now        func() time.Time
	staleAfter time.Duration
	batchLimit int
}

func NewAudioBriefingStaleVoicingService(repo audioBriefingStaleVoicingRepo) *AudioBriefingStaleVoicingService {
	return &AudioBriefingStaleVoicingService{
		repo:       repo,
		now:        time.Now,
		staleAfter: audioBriefingChunkRetryAfter(),
		batchLimit: audioBriefingStaleVoicingBatchLimit(),
	}
}

func (s *AudioBriefingStaleVoicingService) FailStaleJobs(ctx context.Context) (*AudioBriefingStaleVoicingResult, error) {
	if s == nil || s.repo == nil {
		return &AudioBriefingStaleVoicingResult{}, nil
	}
	result := &AudioBriefingStaleVoicingResult{}
	cutoff := s.currentTime().Add(-s.staleAfter)
	jobs, err := s.repo.ListStaleVoicingJobs(ctx, cutoff, s.batchLimit)
	if err != nil {
		return nil, err
	}
	for _, job := range jobs {
		chunks, err := s.repo.ListJobChunks(ctx, job.UserID, job.ID)
		if err != nil {
			result.Failed++
			continue
		}
		staleChunkIDs := staleGeneratingChunkIDs(chunks, cutoff)
		if len(staleChunkIDs) == 0 {
			continue
		}
		message := fmt.Sprintf("audio chunk generation stalled for over %s", s.staleAfter.Round(time.Second))
		failed := false
		for _, chunkID := range staleChunkIDs {
			if err := s.repo.MarkChunkFailed(ctx, chunkID, message); err != nil {
				result.Failed++
				failed = true
				break
			}
		}
		if failed {
			continue
		}
		if _, err := s.repo.FailVoicingJob(ctx, job.ID, "tts_stalled", message); err != nil {
			result.Failed++
			continue
		}
		result.Processed++
	}
	return result, nil
}

func (s *AudioBriefingStaleVoicingService) currentTime() time.Time {
	if s == nil || s.now == nil {
		return time.Now()
	}
	return s.now()
}

func staleGeneratingChunkIDs(chunks []model.AudioBriefingScriptChunk, cutoff time.Time) []string {
	out := make([]string, 0)
	for _, chunk := range chunks {
		if strings.TrimSpace(chunk.TTSStatus) != "generating" {
			continue
		}
		if chunk.UpdatedAt.IsZero() || chunk.UpdatedAt.After(cutoff) {
			continue
		}
		out = append(out, strings.TrimSpace(chunk.ID))
	}
	return out
}

func audioBriefingStaleVoicingBatchLimit() int {
	raw := strings.TrimSpace(os.Getenv("AUDIO_BRIEFING_STALE_VOICING_BATCH_LIMIT"))
	if raw == "" {
		return 50
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 50
	}
	return value
}

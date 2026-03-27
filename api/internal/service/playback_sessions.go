package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
)

type playbackSessionRepo interface {
	CreateSession(ctx context.Context, session *model.PlaybackSession) error
	InterruptActiveSessions(ctx context.Context, userID string) error
	UpdateSessionProgress(ctx context.Context, session *model.PlaybackSession) error
	CompleteSession(ctx context.Context, session *model.PlaybackSession) error
	InterruptSession(ctx context.Context, session *model.PlaybackSession) error
	LatestSessionByMode(ctx context.Context, userID, mode string) (*model.PlaybackSession, error)
	ListSessions(ctx context.Context, userID string, mode string, status string, limit int) ([]model.PlaybackSession, error)
	CreateEvent(ctx context.Context, event *model.PlaybackEvent) error
	GetSession(ctx context.Context, userID, sessionID string) (*model.PlaybackSession, error)
}

type PlaybackSessionsService struct {
	repo playbackSessionRepo
	now  func() time.Time
}

func NewPlaybackSessionsService(repo playbackSessionRepo) *PlaybackSessionsService {
	return &PlaybackSessionsService{repo: repo, now: time.Now}
}

type StartPlaybackSessionInput struct {
	UserID             string          `json:"user_id"`
	Mode               string          `json:"mode"`
	Title              string          `json:"title"`
	Subtitle           string          `json:"subtitle"`
	CurrentPositionSec int             `json:"current_position_sec"`
	DurationSec        int             `json:"duration_sec"`
	ProgressRatio      *float64        `json:"progress_ratio,omitempty"`
	ResumePayload      json.RawMessage `json:"resume_payload"`
}

type UpdatePlaybackSessionInput struct {
	UserID             string          `json:"user_id"`
	SessionID          string          `json:"session_id"`
	Title              string          `json:"title"`
	Subtitle           string          `json:"subtitle"`
	CurrentPositionSec int             `json:"current_position_sec"`
	DurationSec        int             `json:"duration_sec"`
	ProgressRatio      *float64        `json:"progress_ratio,omitempty"`
	ResumePayload      json.RawMessage `json:"resume_payload"`
}

func (s *PlaybackSessionsService) StartSession(ctx context.Context, input StartPlaybackSessionInput) (*model.PlaybackSession, error) {
	if s == nil || s.repo == nil {
		return nil, repository.ErrInvalidState
	}
	nowFn := s.now
	if nowFn == nil {
		nowFn = time.Now
	}
	if err := s.repo.InterruptActiveSessions(ctx, input.UserID); err != nil {
		return nil, err
	}
	session := &model.PlaybackSession{
		UserID:             input.UserID,
		Mode:               normalizePlaybackMode(input.Mode),
		Status:             model.PlaybackSessionStatusInProgress,
		Title:              strings.TrimSpace(input.Title),
		Subtitle:           strings.TrimSpace(input.Subtitle),
		CurrentPositionSec: input.CurrentPositionSec,
		DurationSec:        input.DurationSec,
		ProgressRatio:      input.ProgressRatio,
		ResumePayload:      normalizePlaybackPayload(input.ResumePayload),
		StartedAt:          nowFn().UTC(),
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *PlaybackSessionsService) UpdateProgress(ctx context.Context, input UpdatePlaybackSessionInput) (*model.PlaybackSession, error) {
	session, err := s.repo.GetSession(ctx, input.UserID, input.SessionID)
	if err != nil {
		return nil, err
	}
	session.Title = strings.TrimSpace(input.Title)
	session.Subtitle = strings.TrimSpace(input.Subtitle)
	session.CurrentPositionSec = input.CurrentPositionSec
	session.DurationSec = input.DurationSec
	session.ProgressRatio = input.ProgressRatio
	session.ResumePayload = normalizePlaybackPayload(input.ResumePayload)
	if err := s.repo.UpdateSessionProgress(ctx, session); err != nil {
		return nil, err
	}
	return s.repo.GetSession(ctx, input.UserID, input.SessionID)
}

func (s *PlaybackSessionsService) CompleteSession(ctx context.Context, input UpdatePlaybackSessionInput) (*model.PlaybackSession, error) {
	session, err := s.repo.GetSession(ctx, input.UserID, input.SessionID)
	if err != nil {
		return nil, err
	}
	session.Title = strings.TrimSpace(input.Title)
	session.Subtitle = strings.TrimSpace(input.Subtitle)
	session.CurrentPositionSec = input.CurrentPositionSec
	session.DurationSec = input.DurationSec
	session.ProgressRatio = input.ProgressRatio
	session.ResumePayload = normalizePlaybackPayload(input.ResumePayload)
	if err := s.repo.CompleteSession(ctx, session); err != nil {
		return nil, err
	}
	return s.repo.GetSession(ctx, input.UserID, input.SessionID)
}

func (s *PlaybackSessionsService) InterruptSession(ctx context.Context, input UpdatePlaybackSessionInput) (*model.PlaybackSession, error) {
	session, err := s.repo.GetSession(ctx, input.UserID, input.SessionID)
	if err != nil {
		return nil, err
	}
	session.Title = strings.TrimSpace(input.Title)
	session.Subtitle = strings.TrimSpace(input.Subtitle)
	session.CurrentPositionSec = input.CurrentPositionSec
	session.DurationSec = input.DurationSec
	session.ProgressRatio = input.ProgressRatio
	session.ResumePayload = normalizePlaybackPayload(input.ResumePayload)
	if err := s.repo.InterruptSession(ctx, session); err != nil {
		return nil, err
	}
	return s.repo.GetSession(ctx, input.UserID, input.SessionID)
}

func (s *PlaybackSessionsService) LatestSessions(ctx context.Context, userID string) (map[string]*model.PlaybackSession, error) {
	out := map[string]*model.PlaybackSession{
		model.PlaybackModeSummaryQueue:  nil,
		model.PlaybackModeAudioBriefing: nil,
	}
	summary, err := s.repo.LatestSessionByMode(ctx, userID, model.PlaybackModeSummaryQueue)
	if err != nil && err != repository.ErrNotFound {
		return nil, err
	}
	briefing, err := s.repo.LatestSessionByMode(ctx, userID, model.PlaybackModeAudioBriefing)
	if err != nil && err != repository.ErrNotFound {
		return nil, err
	}
	out[model.PlaybackModeSummaryQueue] = summary
	out[model.PlaybackModeAudioBriefing] = briefing
	return out, nil
}

func (s *PlaybackSessionsService) ListHistory(ctx context.Context, userID, mode, status string, limit int) ([]model.PlaybackSession, error) {
	return s.repo.ListSessions(ctx, userID, normalizePlaybackMode(mode), strings.TrimSpace(status), limit)
}

func normalizePlaybackMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case model.PlaybackModeAudioBriefing:
		return model.PlaybackModeAudioBriefing
	case model.PlaybackModeSummaryQueue:
		return model.PlaybackModeSummaryQueue
	default:
		return strings.TrimSpace(mode)
	}
}

func normalizePlaybackPayload(payload json.RawMessage) json.RawMessage {
	if len(payload) == 0 {
		return json.RawMessage(`{}`)
	}
	return payload
}

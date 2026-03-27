package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
)

type stubPlaybackSessionRepo struct {
	sessions            map[string]*model.PlaybackSession
	interruptActiveUser string
}

func (s *stubPlaybackSessionRepo) CreateSession(_ context.Context, session *model.PlaybackSession) error {
	if s.sessions == nil {
		s.sessions = map[string]*model.PlaybackSession{}
	}
	if session.ID == "" {
		session.ID = "session-1"
	}
	copied := *session
	s.sessions[session.ID] = &copied
	return nil
}

func (s *stubPlaybackSessionRepo) InterruptActiveSessions(_ context.Context, userID string) error {
	s.interruptActiveUser = userID
	for _, session := range s.sessions {
		if session.UserID == userID && session.Status == model.PlaybackSessionStatusInProgress {
			session.Status = model.PlaybackSessionStatusInterrupted
		}
	}
	return nil
}

func (s *stubPlaybackSessionRepo) UpdateSessionProgress(_ context.Context, session *model.PlaybackSession) error {
	s.sessions[session.ID] = clonePlaybackSession(session)
	return nil
}

func (s *stubPlaybackSessionRepo) CompleteSession(_ context.Context, session *model.PlaybackSession) error {
	session.Status = model.PlaybackSessionStatusCompleted
	now := time.Now().UTC()
	session.CompletedAt = &now
	s.sessions[session.ID] = clonePlaybackSession(session)
	return nil
}

func (s *stubPlaybackSessionRepo) InterruptSession(_ context.Context, session *model.PlaybackSession) error {
	session.Status = model.PlaybackSessionStatusInterrupted
	s.sessions[session.ID] = clonePlaybackSession(session)
	return nil
}

func (s *stubPlaybackSessionRepo) LatestSessionByMode(_ context.Context, userID, mode string) (*model.PlaybackSession, error) {
	var latest *model.PlaybackSession
	for _, session := range s.sessions {
		if session.UserID != userID || session.Mode != mode {
			continue
		}
		if latest == nil || session.UpdatedAt.After(latest.UpdatedAt) {
			latest = clonePlaybackSession(session)
		}
	}
	if latest == nil {
		return nil, repository.ErrNotFound
	}
	return latest, nil
}

func (s *stubPlaybackSessionRepo) ListSessions(_ context.Context, userID string, mode string, status string, limit int) ([]model.PlaybackSession, error) {
	out := make([]model.PlaybackSession, 0, limit)
	for _, session := range s.sessions {
		if session.UserID != userID {
			continue
		}
		if mode != "" && session.Mode != mode {
			continue
		}
		if status != "" && session.Status != status {
			continue
		}
		out = append(out, *clonePlaybackSession(session))
	}
	return out, nil
}

func (s *stubPlaybackSessionRepo) CreateEvent(_ context.Context, event *model.PlaybackEvent) error {
	return nil
}

func (s *stubPlaybackSessionRepo) GetSession(_ context.Context, userID, sessionID string) (*model.PlaybackSession, error) {
	session, ok := s.sessions[sessionID]
	if !ok || session.UserID != userID {
		return nil, repository.ErrNotFound
	}
	return clonePlaybackSession(session), nil
}

func clonePlaybackSession(session *model.PlaybackSession) *model.PlaybackSession {
	if session == nil {
		return nil
	}
	copied := *session
	if session.ResumePayload != nil {
		copied.ResumePayload = append(json.RawMessage(nil), session.ResumePayload...)
	}
	return &copied
}

func TestPlaybackSessionsServiceStartSessionInterruptsActivePlayback(t *testing.T) {
	repo := &stubPlaybackSessionRepo{
		sessions: map[string]*model.PlaybackSession{
			"old": {
				ID:        "old",
				UserID:    "user-1",
				Mode:      model.PlaybackModeSummaryQueue,
				Status:    model.PlaybackSessionStatusInProgress,
				UpdatedAt: time.Date(2026, 3, 27, 9, 0, 0, 0, time.UTC),
			},
		},
	}
	svc := &PlaybackSessionsService{repo: repo, now: func() time.Time {
		return time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)
	}}

	session, err := svc.StartSession(context.Background(), StartPlaybackSessionInput{
		UserID:        "user-1",
		Mode:          model.PlaybackModeAudioBriefing,
		Title:         "Morning Briefing",
		ResumePayload: json.RawMessage(`{"briefing_id":"b1"}`),
	})
	if err != nil {
		t.Fatalf("StartSession(...) error = %v", err)
	}
	if repo.interruptActiveUser != "user-1" {
		t.Fatalf("interrupt user = %q, want user-1", repo.interruptActiveUser)
	}
	if repo.sessions["old"].Status != model.PlaybackSessionStatusInterrupted {
		t.Fatalf("old session status = %q, want interrupted", repo.sessions["old"].Status)
	}
	if session.Status != model.PlaybackSessionStatusInProgress {
		t.Fatalf("new session status = %q, want in_progress", session.Status)
	}
}

func TestPlaybackSessionsServiceCompleteSessionMarksCompleted(t *testing.T) {
	repo := &stubPlaybackSessionRepo{
		sessions: map[string]*model.PlaybackSession{
			"session-1": {
				ID:        "session-1",
				UserID:    "user-1",
				Mode:      model.PlaybackModeAudioBriefing,
				Status:    model.PlaybackSessionStatusInProgress,
				UpdatedAt: time.Date(2026, 3, 27, 9, 0, 0, 0, time.UTC),
			},
		},
	}
	svc := NewPlaybackSessionsService(repo)

	session, err := svc.CompleteSession(context.Background(), UpdatePlaybackSessionInput{
		UserID:             "user-1",
		SessionID:          "session-1",
		CurrentPositionSec: 120,
		DurationSec:        120,
		ResumePayload:      json.RawMessage(`{"briefing_id":"b1","current_offset_sec":120}`),
	})
	if err != nil {
		t.Fatalf("CompleteSession(...) error = %v", err)
	}
	if session.Status != model.PlaybackSessionStatusCompleted {
		t.Fatalf("session status = %q, want completed", session.Status)
	}
	if session.CompletedAt == nil {
		t.Fatal("session completed_at = nil, want timestamp")
	}
}

package repository

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PlaybackSessionRepo struct{ db *pgxpool.Pool }

func NewPlaybackSessionRepo(db *pgxpool.Pool) *PlaybackSessionRepo {
	return &PlaybackSessionRepo{db: db}
}

func listPlaybackSessionsQuery() string {
	return `
		SELECT id, user_id, mode, status, title, subtitle, current_position_sec, duration_sec,
		       progress_ratio, COALESCE(resume_payload, '{}'::jsonb), started_at, updated_at, completed_at
		FROM playback_sessions
		WHERE user_id = $1
		  AND ($2 = '' OR mode = $2)
		  AND ($3 = '' OR status = $3)
		ORDER BY updated_at DESC
		LIMIT $4`
}

func latestPlaybackSessionByModeQuery() string {
	return `
		SELECT id, user_id, mode, status, title, subtitle, current_position_sec, duration_sec,
		       progress_ratio, COALESCE(resume_payload, '{}'::jsonb), started_at, updated_at, completed_at
		FROM playback_sessions
		WHERE user_id = $1
		  AND mode = $2
		ORDER BY updated_at DESC
		LIMIT 1`
}

func scanPlaybackSession(row interface{ Scan(dest ...any) error }) (*model.PlaybackSession, error) {
	var session model.PlaybackSession
	var payload []byte
	if err := row.Scan(
		&session.ID,
		&session.UserID,
		&session.Mode,
		&session.Status,
		&session.Title,
		&session.Subtitle,
		&session.CurrentPositionSec,
		&session.DurationSec,
		&session.ProgressRatio,
		&payload,
		&session.StartedAt,
		&session.UpdatedAt,
		&session.CompletedAt,
	); err != nil {
		return nil, err
	}
	session.ResumePayload = json.RawMessage(payload)
	if len(session.ResumePayload) == 0 {
		session.ResumePayload = json.RawMessage(`{}`)
	}
	return &session, nil
}

func (r *PlaybackSessionRepo) CreateSession(ctx context.Context, session *model.PlaybackSession) error {
	if session == nil {
		return nil
	}
	if strings.TrimSpace(session.ID) == "" {
		session.ID = uuid.NewString()
	}
	if len(session.ResumePayload) == 0 {
		session.ResumePayload = json.RawMessage(`{}`)
	}
	now := time.Now().UTC()
	if session.StartedAt.IsZero() {
		session.StartedAt = now
	}
	session.UpdatedAt = now
	_, err := r.db.Exec(ctx, `
		INSERT INTO playback_sessions (
			id, user_id, mode, status, title, subtitle, current_position_sec, duration_sec,
			progress_ratio, resume_payload, started_at, updated_at, completed_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10::jsonb, $11, $12, $13
		)`,
		session.ID,
		session.UserID,
		session.Mode,
		session.Status,
		session.Title,
		session.Subtitle,
		session.CurrentPositionSec,
		session.DurationSec,
		session.ProgressRatio,
		[]byte(session.ResumePayload),
		session.StartedAt,
		session.UpdatedAt,
		session.CompletedAt,
	)
	return mapDBError(err)
}

func (r *PlaybackSessionRepo) InterruptActiveSessions(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE playback_sessions
		SET status = $2,
		    updated_at = NOW()
		WHERE user_id = $1
		  AND status = $3`,
		userID,
		model.PlaybackSessionStatusInterrupted,
		model.PlaybackSessionStatusInProgress,
	)
	return mapDBError(err)
}

func (r *PlaybackSessionRepo) UpdateSessionProgress(ctx context.Context, session *model.PlaybackSession) error {
	if session == nil {
		return nil
	}
	if len(session.ResumePayload) == 0 {
		session.ResumePayload = json.RawMessage(`{}`)
	}
	tag, err := r.db.Exec(ctx, `
		UPDATE playback_sessions
		SET title = $3,
		    subtitle = $4,
		    current_position_sec = $5,
		    duration_sec = $6,
		    progress_ratio = $7,
		    resume_payload = $8::jsonb,
		    updated_at = NOW()
		WHERE id = $1
		  AND user_id = $2`,
		session.ID,
		session.UserID,
		session.Title,
		session.Subtitle,
		session.CurrentPositionSec,
		session.DurationSec,
		session.ProgressRatio,
		[]byte(session.ResumePayload),
	)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PlaybackSessionRepo) CompleteSession(ctx context.Context, session *model.PlaybackSession) error {
	return r.updateTerminalSession(ctx, session, model.PlaybackSessionStatusCompleted, true)
}

func (r *PlaybackSessionRepo) InterruptSession(ctx context.Context, session *model.PlaybackSession) error {
	return r.updateTerminalSession(ctx, session, model.PlaybackSessionStatusInterrupted, false)
}

func (r *PlaybackSessionRepo) updateTerminalSession(ctx context.Context, session *model.PlaybackSession, status string, markCompleted bool) error {
	if session == nil {
		return nil
	}
	if len(session.ResumePayload) == 0 {
		session.ResumePayload = json.RawMessage(`{}`)
	}
	var completedAt any
	if markCompleted {
		now := time.Now().UTC()
		session.CompletedAt = &now
		completedAt = now
	}
	tag, err := r.db.Exec(ctx, `
		UPDATE playback_sessions
		SET status = $3,
		    title = $4,
		    subtitle = $5,
		    current_position_sec = $6,
		    duration_sec = $7,
		    progress_ratio = $8,
		    resume_payload = $9::jsonb,
		    updated_at = NOW(),
		    completed_at = CASE WHEN $10::timestamptz IS NULL THEN completed_at ELSE $10::timestamptz END
		WHERE id = $1
		  AND user_id = $2`,
		session.ID,
		session.UserID,
		status,
		session.Title,
		session.Subtitle,
		session.CurrentPositionSec,
		session.DurationSec,
		session.ProgressRatio,
		[]byte(session.ResumePayload),
		completedAt,
	)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PlaybackSessionRepo) LatestSessionByMode(ctx context.Context, userID, mode string) (*model.PlaybackSession, error) {
	session, err := scanPlaybackSession(r.db.QueryRow(ctx, latestPlaybackSessionByModeQuery(), userID, mode))
	if err != nil {
		return nil, mapDBError(err)
	}
	return session, nil
}

func (r *PlaybackSessionRepo) ListSessions(ctx context.Context, userID string, mode string, status string, limit int) ([]model.PlaybackSession, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.Query(ctx, listPlaybackSessionsQuery(), userID, strings.TrimSpace(mode), strings.TrimSpace(status), limit)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	out := make([]model.PlaybackSession, 0, limit)
	for rows.Next() {
		session, err := scanPlaybackSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *session)
	}
	return out, rows.Err()
}

func (r *PlaybackSessionRepo) CreateEvent(ctx context.Context, event *model.PlaybackEvent) error {
	if event == nil {
		return nil
	}
	if strings.TrimSpace(event.ID) == "" {
		event.ID = uuid.NewString()
	}
	if len(event.Payload) == 0 {
		event.Payload = json.RawMessage(`{}`)
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO playback_events (
			id, session_id, user_id, mode, event_type, position_sec, payload, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7::jsonb, $8
		)`,
		event.ID,
		event.SessionID,
		event.UserID,
		event.Mode,
		event.EventType,
		event.PositionSec,
		[]byte(event.Payload),
		event.CreatedAt,
	)
	return mapDBError(err)
}

func (r *PlaybackSessionRepo) GetSession(ctx context.Context, userID, sessionID string) (*model.PlaybackSession, error) {
	session, err := scanPlaybackSession(r.db.QueryRow(ctx, `
		SELECT id, user_id, mode, status, title, subtitle, current_position_sec, duration_sec,
		       progress_ratio, COALESCE(resume_payload, '{}'::jsonb), started_at, updated_at, completed_at
		FROM playback_sessions
		WHERE id = $1
		  AND user_id = $2`,
		sessionID, userID))
	if err != nil {
		return nil, mapDBError(err)
	}
	return session, nil
}

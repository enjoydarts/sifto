package model

import (
	"encoding/json"
	"time"
)

const (
	PlaybackModeSummaryQueue  = "summary_queue"
	PlaybackModeAudioBriefing = "audio_briefing"
)

const (
	PlaybackSessionStatusInProgress  = "in_progress"
	PlaybackSessionStatusCompleted   = "completed"
	PlaybackSessionStatusInterrupted = "interrupted"
)

const (
	PlaybackEventStarted    = "started"
	PlaybackEventPaused     = "paused"
	PlaybackEventResumed    = "resumed"
	PlaybackEventStopped    = "stopped"
	PlaybackEventCompleted  = "completed"
	PlaybackEventReplaced   = "replaced"
	PlaybackEventProgressed = "progressed"
)

type PlaybackSession struct {
	ID                 string          `json:"id"`
	UserID             string          `json:"user_id"`
	Mode               string          `json:"mode"`
	Status             string          `json:"status"`
	Title              string          `json:"title"`
	Subtitle           string          `json:"subtitle"`
	CurrentPositionSec int             `json:"current_position_sec"`
	DurationSec        int             `json:"duration_sec"`
	ProgressRatio      *float64        `json:"progress_ratio,omitempty"`
	ResumePayload      json.RawMessage `json:"resume_payload"`
	StartedAt          time.Time       `json:"started_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	CompletedAt        *time.Time      `json:"completed_at,omitempty"`
}

type PlaybackEvent struct {
	ID          string          `json:"id"`
	SessionID   string          `json:"session_id"`
	UserID      string          `json:"user_id"`
	Mode        string          `json:"mode"`
	EventType   string          `json:"event_type"`
	PositionSec int             `json:"position_sec"`
	Payload     json.RawMessage `json:"payload"`
	CreatedAt   time.Time       `json:"created_at"`
}

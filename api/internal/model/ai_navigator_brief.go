package model

import "time"

const (
	AINavigatorBriefSlotMorning = "morning"
	AINavigatorBriefSlotNoon    = "noon"
	AINavigatorBriefSlotEvening = "evening"
)

const (
	AINavigatorBriefStatusQueued    = "queued"
	AINavigatorBriefStatusGenerated = "generated"
	AINavigatorBriefStatusFailed    = "failed"
	AINavigatorBriefStatusNotified  = "notified"
)

type AINavigatorBrief struct {
	ID                 string                 `json:"id"`
	UserID             string                 `json:"user_id"`
	Slot               string                 `json:"slot"`
	Status             string                 `json:"status"`
	Title              string                 `json:"title"`
	Intro              string                 `json:"intro"`
	Summary            string                 `json:"summary"`
	Persona            string                 `json:"persona"`
	Model              string                 `json:"model"`
	SourceWindowStart  *time.Time             `json:"source_window_start,omitempty"`
	SourceWindowEnd    *time.Time             `json:"source_window_end,omitempty"`
	GeneratedAt        *time.Time             `json:"generated_at,omitempty"`
	NotificationSentAt *time.Time             `json:"notification_sent_at,omitempty"`
	ErrorMessage       string                 `json:"error_message,omitempty"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
	Items              []AINavigatorBriefItem `json:"items,omitempty"`
}

type AINavigatorBriefItem struct {
	ID                      string    `json:"id"`
	BriefID                 string    `json:"brief_id"`
	Rank                    int       `json:"rank"`
	ItemID                  string    `json:"item_id"`
	TitleSnapshot           string    `json:"title_snapshot"`
	TranslatedTitleSnapshot string    `json:"translated_title_snapshot"`
	SourceTitleSnapshot     string    `json:"source_title_snapshot"`
	Comment                 string    `json:"comment"`
	CreatedAt               time.Time `json:"created_at"`
}

type AINavigatorBriefListResponse struct {
	Items []AINavigatorBrief `json:"items"`
}

type AINavigatorBriefDetailResponse struct {
	Brief *AINavigatorBrief `json:"brief,omitempty"`
}

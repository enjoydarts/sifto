package service

import (
	"context"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

type audioBriefingSingleDraftStrategy struct {
	orchestrator *AudioBriefingOrchestrator
}

func (s audioBriefingSingleDraftStrategy) BuildDraft(
	ctx context.Context,
	userID string,
	slotStartedAt time.Time,
	persona string,
	items []model.AudioBriefingJobItem,
	voice *model.AudioBriefingPersonaVoice,
	targetDurationMinutes int,
) (AudioBriefingDraft, error) {
	return s.orchestrator.buildSingleDraft(ctx, userID, slotStartedAt, persona, items, voice, targetDurationMinutes)
}

type audioBriefingDuoDraftStrategy struct {
	orchestrator *AudioBriefingOrchestrator
}

func (s audioBriefingDuoDraftStrategy) BuildDraft(
	ctx context.Context,
	userID string,
	slotStartedAt time.Time,
	persona string,
	items []model.AudioBriefingJobItem,
	voice *model.AudioBriefingPersonaVoice,
	targetDurationMinutes int,
) (AudioBriefingDraft, error) {
	// Duo runtime is implemented incrementally; keep single behavior available until turn-based script/TTS lands.
	return s.orchestrator.buildSingleDraft(ctx, userID, slotStartedAt, persona, items, voice, targetDurationMinutes)
}

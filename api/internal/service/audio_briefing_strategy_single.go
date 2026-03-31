package service

import (
	"context"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
)

type audioBriefingSingleDraftStrategy struct {
	orchestrator *AudioBriefingOrchestrator
}

func (s audioBriefingSingleDraftStrategy) BuildDraft(
	ctx context.Context,
	job *model.AudioBriefingJob,
	items []model.AudioBriefingJobItem,
	voice *model.AudioBriefingPersonaVoice,
	targetDurationMinutes int,
) (AudioBriefingDraft, error) {
	if job == nil {
		return AudioBriefingDraft{}, repository.ErrNotFound
	}
	return s.orchestrator.buildSingleDraft(ctx, job.UserID, job.SlotStartedAtJST, job.Persona, items, voice, targetDurationMinutes)
}

type audioBriefingDuoDraftStrategy struct {
	orchestrator *AudioBriefingOrchestrator
}

func (s audioBriefingDuoDraftStrategy) BuildDraft(
	ctx context.Context,
	job *model.AudioBriefingJob,
	items []model.AudioBriefingJobItem,
	voice *model.AudioBriefingPersonaVoice,
	targetDurationMinutes int,
) (AudioBriefingDraft, error) {
	if job == nil {
		return AudioBriefingDraft{}, repository.ErrNotFound
	}
	return s.orchestrator.buildDuoDraft(ctx, job, items, voice, targetDurationMinutes)
}

package service

import (
	"context"

	"github.com/enjoydarts/sifto/api/internal/model"
)

type audioBriefingDraftStrategy interface {
	BuildDraft(
		ctx context.Context,
		job *model.AudioBriefingJob,
		items []model.AudioBriefingJobItem,
		voice *model.AudioBriefingPersonaVoice,
		targetDurationMinutes int,
	) (AudioBriefingDraft, error)
}

func normalizeAudioBriefingConversationModeValue(mode string) string {
	if mode == "duo" {
		return "duo"
	}
	return "single"
}

func audioBriefingInitialPipelineStageForMode(mode string) string {
	switch normalizeAudioBriefingConversationModeValue(mode) {
	case "duo":
		return "duo_script"
	default:
		return "single_script"
	}
}

func (o *AudioBriefingOrchestrator) draftStrategy(mode string) audioBriefingDraftStrategy {
	switch normalizeAudioBriefingConversationModeValue(mode) {
	case "duo":
		return audioBriefingDuoDraftStrategy{orchestrator: o}
	default:
		return audioBriefingSingleDraftStrategy{orchestrator: o}
	}
}

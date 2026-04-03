package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

type OpenAITTSVoiceCatalogService struct{}

func NewOpenAITTSVoiceCatalogService() *OpenAITTSVoiceCatalogService {
	return &OpenAITTSVoiceCatalogService{}
}

type openAIBuiltinVoice struct {
	VoiceID string
	Name    string
}

var openAIBuiltinVoices = []openAIBuiltinVoice{
	{VoiceID: "alloy", Name: "Alloy"},
	{VoiceID: "ash", Name: "Ash"},
	{VoiceID: "ballad", Name: "Ballad"},
	{VoiceID: "coral", Name: "Coral"},
	{VoiceID: "echo", Name: "Echo"},
	{VoiceID: "fable", Name: "Fable"},
	{VoiceID: "onyx", Name: "Onyx"},
	{VoiceID: "nova", Name: "Nova"},
	{VoiceID: "sage", Name: "Sage"},
	{VoiceID: "shimmer", Name: "Shimmer"},
	{VoiceID: "verse", Name: "Verse"},
	{VoiceID: "marin", Name: "Marin"},
	{VoiceID: "cedar", Name: "Cedar"},
}

func (s *OpenAITTSVoiceCatalogService) FetchVoices(ctx context.Context) ([]repository.OpenAITTSVoiceSnapshot, error) {
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	fetchedAt := time.Now().UTC()
	out := make([]repository.OpenAITTSVoiceSnapshot, 0, len(openAIBuiltinVoices))
	for _, voice := range openAIBuiltinVoices {
		metadata, _ := json.Marshal(map[string]any{
			"catalog_source": "openai-docs",
			"voice_kind":     "builtin",
			"supported_models": []string{
				"tts-1",
				"tts-1-hd",
				"gpt-4o-mini-tts",
				"gpt-4o-mini-tts-2025-12-15",
			},
		})
		out = append(out, repository.OpenAITTSVoiceSnapshot{
			VoiceID:      strings.TrimSpace(voice.VoiceID),
			Name:         strings.TrimSpace(voice.Name),
			Description:  "OpenAI built-in voice",
			Language:     "multilingual",
			PreviewURL:   "",
			MetadataJSON: metadata,
			FetchedAt:    fetchedAt,
		})
	}
	return out, nil
}

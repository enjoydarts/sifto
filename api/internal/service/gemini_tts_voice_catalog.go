package service

import (
	"context"
	"encoding/json"
	"strings"
)

type GeminiTTSVoiceCatalog struct {
	CatalogName string                 `json:"catalog_name"`
	Provider    string                 `json:"provider"`
	Source      string                 `json:"source"`
	Voices      []GeminiTTSVoiceRecord `json:"voices"`
}

type GeminiTTSVoiceRecord struct {
	VoiceName       string `json:"voice_name"`
	Label           string `json:"label"`
	Tone            string `json:"tone"`
	Description     string `json:"description"`
	SampleAudioPath string `json:"sample_audio_path"`
}

type GeminiTTSVoiceCatalogService struct{}

func NewGeminiTTSVoiceCatalogService() *GeminiTTSVoiceCatalogService {
	return &GeminiTTSVoiceCatalogService{}
}

func (s *GeminiTTSVoiceCatalogService) LoadCatalog(ctx context.Context) (*GeminiTTSVoiceCatalog, error) {
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	body, err := readSharedAsset("gemini_tts_voices.json")
	if err != nil {
		return nil, err
	}
	var catalog GeminiTTSVoiceCatalog
	if err := json.Unmarshal(body, &catalog); err != nil {
		return nil, err
	}
	catalog.CatalogName = strings.TrimSpace(catalog.CatalogName)
	catalog.Provider = strings.TrimSpace(catalog.Provider)
	catalog.Source = strings.TrimSpace(catalog.Source)
	voices := make([]GeminiTTSVoiceRecord, 0, len(catalog.Voices))
	for _, voice := range catalog.Voices {
		voices = append(voices, GeminiTTSVoiceRecord{
			VoiceName:       strings.TrimSpace(voice.VoiceName),
			Label:           strings.TrimSpace(voice.Label),
			Tone:            strings.TrimSpace(voice.Tone),
			Description:     strings.TrimSpace(voice.Description),
			SampleAudioPath: strings.TrimSpace(voice.SampleAudioPath),
		})
	}
	catalog.Voices = voices
	return &catalog, nil
}

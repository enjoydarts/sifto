package service

import "strings"

type TTSProviderMetadata struct {
	Capabilities                           TTSProviderCapabilities
	SummaryRequiresTTSModel                bool
	SummaryPreprocessPromptKey             string
	AudioBriefingSinglePreprocessPromptKey string
	AudioBriefingDuoPreprocessPromptKey    string
	PreprocessUsagePurpose                 string
}

var ttsProviderMetadataByProvider = map[string]TTSProviderMetadata{
	"aivis": {
		Capabilities: TTSProviderCapabilities{
			RequiresVoiceStyle:    true,
			SupportsCatalogPicker: true,
			SupportsSpeechTuning:  true,
			RequiresUserAPIKey:    true,
		},
		SummaryPreprocessPromptKey:             fishSummaryPreprocessPromptKey,
		AudioBriefingSinglePreprocessPromptKey: fishAudioBriefingSinglePreprocessPromptKey,
		AudioBriefingDuoPreprocessPromptKey:    fishAudioBriefingDuoPreprocessPromptKey,
		PreprocessUsagePurpose:                 fishPreprocessPurpose,
	},
	"xai": {
		Capabilities: TTSProviderCapabilities{
			SupportsCatalogPicker: true,
			RequiresUserAPIKey:    true,
		},
		SummaryPreprocessPromptKey:             xaiSummaryPreprocessPromptKey,
		AudioBriefingSinglePreprocessPromptKey: xaiAudioBriefingSinglePreprocessPromptKey,
		AudioBriefingDuoPreprocessPromptKey:    xaiAudioBriefingDuoPreprocessPromptKey,
		PreprocessUsagePurpose:                 xaiTTSPreprocessPurpose,
	},
	"openai": {
		Capabilities: TTSProviderCapabilities{
			SupportsCatalogPicker:    true,
			SupportsSeparateTTSModel: true,
			RequiresUserAPIKey:       true,
		},
		SummaryRequiresTTSModel:                true,
		SummaryPreprocessPromptKey:             fishSummaryPreprocessPromptKey,
		AudioBriefingSinglePreprocessPromptKey: fishAudioBriefingSinglePreprocessPromptKey,
		AudioBriefingDuoPreprocessPromptKey:    fishAudioBriefingDuoPreprocessPromptKey,
		PreprocessUsagePurpose:                 fishPreprocessPurpose,
	},
	"fish": {
		Capabilities: TTSProviderCapabilities{
			SupportsCatalogPicker:    true,
			SupportsSeparateTTSModel: true,
			RequiresUserAPIKey:       true,
		},
		SummaryRequiresTTSModel:                true,
		SummaryPreprocessPromptKey:             fishSummaryPreprocessPromptKey,
		AudioBriefingSinglePreprocessPromptKey: fishAudioBriefingSinglePreprocessPromptKey,
		AudioBriefingDuoPreprocessPromptKey:    fishAudioBriefingDuoPreprocessPromptKey,
		PreprocessUsagePurpose:                 fishPreprocessPurpose,
	},
	"elevenlabs": {
		Capabilities: TTSProviderCapabilities{
			SupportsCatalogPicker:    true,
			SupportsSeparateTTSModel: true,
			RequiresUserAPIKey:       true,
		},
		SummaryRequiresTTSModel:                true,
		SummaryPreprocessPromptKey:             elevenLabsSummaryPreprocessPromptKey,
		AudioBriefingSinglePreprocessPromptKey: elevenLabsAudioBriefingSinglePreprocessPromptKey,
		AudioBriefingDuoPreprocessPromptKey:    elevenLabsAudioBriefingDuoPreprocessPromptKey,
		PreprocessUsagePurpose:                 elevenLabsTTSPreprocessPurpose,
	},
	"gemini_tts": {
		Capabilities: TTSProviderCapabilities{
			SupportsCatalogPicker:    true,
			SupportsSeparateTTSModel: true,
			RequiresUserAPIKey:       false,
		},
		SummaryRequiresTTSModel:                true,
		SummaryPreprocessPromptKey:             geminiSummaryPreprocessPromptKey,
		AudioBriefingSinglePreprocessPromptKey: geminiAudioBriefingSinglePreprocessPromptKey,
		AudioBriefingDuoPreprocessPromptKey:    geminiAudioBriefingDuoPreprocessPromptKey,
		PreprocessUsagePurpose:                 geminiTTSPreprocessPurpose,
	},
	"mock": {},
}

func LookupTTSProviderMetadata(provider string) TTSProviderMetadata {
	normalized := strings.TrimSpace(strings.ToLower(provider))
	metadata, ok := ttsProviderMetadataByProvider[normalized]
	if !ok {
		return TTSProviderMetadata{}
	}
	return metadata
}

func preprocessPurposeForKnownPromptKey(promptKey string) (string, bool) {
	normalized := strings.TrimSpace(promptKey)
	for _, metadata := range ttsProviderMetadataByProvider {
		if normalized == metadata.SummaryPreprocessPromptKey ||
			normalized == metadata.AudioBriefingSinglePreprocessPromptKey ||
			normalized == metadata.AudioBriefingDuoPreprocessPromptKey {
			purpose := strings.TrimSpace(metadata.PreprocessUsagePurpose)
			if purpose == "" {
				return "", false
			}
			return purpose, true
		}
	}
	return "", false
}

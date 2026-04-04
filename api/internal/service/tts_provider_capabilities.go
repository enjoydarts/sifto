package service

import "strings"

type TTSProviderCapabilities struct {
	RequiresVoiceStyle       bool
	SupportsCatalogPicker    bool
	SupportsSeparateTTSModel bool
	SupportsSpeechTuning     bool
	RequiresUserAPIKey       bool
}

func LookupTTSProviderCapabilities(provider string) TTSProviderCapabilities {
	switch strings.TrimSpace(strings.ToLower(provider)) {
	case "aivis":
		return TTSProviderCapabilities{
			RequiresVoiceStyle:    true,
			SupportsCatalogPicker: true,
			SupportsSpeechTuning:  true,
			RequiresUserAPIKey:    true,
		}
	case "xai":
		return TTSProviderCapabilities{
			SupportsCatalogPicker: true,
			RequiresUserAPIKey:    true,
		}
	case "openai":
		return TTSProviderCapabilities{
			SupportsCatalogPicker:    true,
			SupportsSeparateTTSModel: true,
			RequiresUserAPIKey:       true,
		}
	case "gemini_tts":
		return TTSProviderCapabilities{
			SupportsCatalogPicker:    true,
			SupportsSeparateTTSModel: true,
			RequiresUserAPIKey:       false,
		}
	case "mock":
		return TTSProviderCapabilities{}
	default:
		return TTSProviderCapabilities{}
	}
}

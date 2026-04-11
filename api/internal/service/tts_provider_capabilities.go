package service

type TTSProviderCapabilities struct {
	RequiresVoiceStyle       bool
	SupportsCatalogPicker    bool
	SupportsSeparateTTSModel bool
	SupportsSpeechTuning     bool
	RequiresUserAPIKey       bool
	RequiresRegion           bool
}

func LookupTTSProviderCapabilities(provider string) TTSProviderCapabilities {
	return LookupTTSProviderMetadata(provider).Capabilities
}

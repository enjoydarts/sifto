package service

import "testing"

func TestLookupTTSProviderMetadataIncludesXAIAndGemini(t *testing.T) {
	xai := LookupTTSProviderMetadata("xai")
	if xai.SummaryRequiresTTSModel {
		t.Fatalf("xai SummaryRequiresTTSModel = true, want false")
	}
	if xai.SummaryPreprocessPromptKey != xaiSummaryPreprocessPromptKey {
		t.Fatalf("xai SummaryPreprocessPromptKey = %q", xai.SummaryPreprocessPromptKey)
	}
	if xai.PreprocessUsagePurpose != xaiTTSPreprocessPurpose {
		t.Fatalf("xai PreprocessUsagePurpose = %q", xai.PreprocessUsagePurpose)
	}

	gemini := LookupTTSProviderMetadata("gemini_tts")
	if !gemini.SummaryRequiresTTSModel {
		t.Fatalf("gemini SummaryRequiresTTSModel = false, want true")
	}
	if gemini.SummaryPreprocessPromptKey != geminiSummaryPreprocessPromptKey {
		t.Fatalf("gemini SummaryPreprocessPromptKey = %q", gemini.SummaryPreprocessPromptKey)
	}
	if gemini.PreprocessUsagePurpose != geminiTTSPreprocessPurpose {
		t.Fatalf("gemini PreprocessUsagePurpose = %q", gemini.PreprocessUsagePurpose)
	}
}

func TestLookupTTSProviderMetadataUnknownProviderReturnsEmptyMetadata(t *testing.T) {
	unknown := LookupTTSProviderMetadata("custom")
	if unknown.SummaryPreprocessPromptKey != "" {
		t.Fatalf("unknown SummaryPreprocessPromptKey = %q", unknown.SummaryPreprocessPromptKey)
	}
	if unknown.AudioBriefingSinglePreprocessPromptKey != "" {
		t.Fatalf("unknown AudioBriefingSinglePreprocessPromptKey = %q", unknown.AudioBriefingSinglePreprocessPromptKey)
	}
	if unknown.AudioBriefingDuoPreprocessPromptKey != "" {
		t.Fatalf("unknown AudioBriefingDuoPreprocessPromptKey = %q", unknown.AudioBriefingDuoPreprocessPromptKey)
	}
}

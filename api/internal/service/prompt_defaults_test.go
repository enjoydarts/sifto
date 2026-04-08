package service

import (
	"strings"
	"testing"
)

func TestLookupPromptTemplateDefault(t *testing.T) {
	t.Parallel()

	cases := []string{
		"summary.default",
		"facts.default",
		"digest.default",
		"audio_briefing_script.single",
		"audio_briefing_script.duo",
		"fish.summary_preprocess",
		"fish.audio_briefing_single_preprocess",
		"fish.audio_briefing_duo_preprocess",
		"gemini.summary_preprocess",
		"gemini.audio_briefing_single_preprocess",
		"gemini.audio_briefing_duo_preprocess",
	}

	for _, key := range cases {
		key := key
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			out, err := LookupPromptTemplateDefault(key)
			if err != nil {
				t.Fatalf("LookupPromptTemplateDefault(%s) error = %v", key, err)
			}
			if out.PromptText == "" {
				t.Fatalf("prompt text is empty for %s", key)
			}
			if out.Notes == "" {
				t.Fatalf("notes are empty for %s", key)
			}
			if len(out.PreviewVariables) == 0 {
				t.Fatalf("preview variables are empty for %s", key)
			}
		})
	}
}

func TestLookupPromptTemplateDefaultUnknownKeyReturnsError(t *testing.T) {
	t.Parallel()

	if _, err := LookupPromptTemplateDefault("unknown.prompt"); err == nil {
		t.Fatal("expected error for unknown prompt key")
	}
}

func TestAudioBriefingPromptDefaultsIncludeProgramNameVariable(t *testing.T) {
	t.Parallel()

	for _, key := range []string{"audio_briefing_script.single", "audio_briefing_script.duo"} {
		key := key
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			out, err := LookupPromptTemplateDefault(key)
			if err != nil {
				t.Fatalf("LookupPromptTemplateDefault(%s) error = %v", key, err)
			}
			if !strings.Contains(string(out.VariablesSchema), `"program_name"`) {
				t.Fatalf("variables schema for %s must include program_name: %s", key, string(out.VariablesSchema))
			}
			if !strings.Contains(string(out.PreviewVariables), `"program_name"`) {
				t.Fatalf("preview variables for %s must include program_name: %s", key, string(out.PreviewVariables))
			}
			if !strings.Contains(out.SystemInstruction, "{{program_name}}") {
				t.Fatalf("system instruction for %s must reference program_name", key)
			}
			if !strings.Contains(out.PromptText, "{{program_name}}") {
				t.Fatalf("prompt text for %s must reference program_name", key)
			}
		})
	}
}

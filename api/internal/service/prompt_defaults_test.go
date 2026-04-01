package service

import "testing"

func TestLookupPromptTemplateDefault(t *testing.T) {
	t.Parallel()

	cases := []string{
		"summary.default",
		"facts.default",
		"digest.default",
		"audio_briefing_script.single",
		"audio_briefing_script.duo",
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

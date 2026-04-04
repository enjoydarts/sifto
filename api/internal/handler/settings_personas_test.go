package handler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveNavigatorPersonasPathUsesNavigatorEnv(t *testing.T) {
	t.Setenv("NAVIGATOR_PERSONAS_PATH", "/tmp/personas.json")
	t.Setenv("LLM_CATALOG_PATH", "/shared/llm_catalog.json")

	got, err := resolveNavigatorPersonasPath()
	if err != nil {
		t.Fatalf("resolve path: %v", err)
	}
	if got != "/tmp/personas.json" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveNavigatorPersonasPathFallsBackToLLMCatalogDir(t *testing.T) {
	t.Setenv("NAVIGATOR_PERSONAS_PATH", "")
	t.Setenv("LLM_CATALOG_PATH", "/app/shared/llm_catalog.json")

	got, err := resolveNavigatorPersonasPath()
	if err != nil {
		t.Fatalf("resolve path: %v", err)
	}
	want := filepath.Join("/app/shared", "ai_navigator_personas.json")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNavigatorPersonaDefinitionSupportsSamplingProfile(t *testing.T) {
	body := []byte(`{
		"editor": {
			"name": "編集長 水城",
			"gender": "男性",
			"age_vibe": "40代後半",
			"first_person": "わたくし",
			"speech_style": "静か",
			"occupation": "ニュースレター編集長",
			"experience": "経験豊富",
			"personality": "静か",
			"values": "重要度",
			"interests": "政策",
			"dislikes": "ノイズ",
			"voice": "落ち着いた編集者",
			"sampling_profile": {
				"temperature_hint": "low",
				"top_p_hint": "narrow",
				"verbosity_hint": "balanced"
			},
			"briefing": {
				"comment_range": "55〜95字",
				"intro_range": "80〜140字",
				"intro_style": "端正"
			},
			"item": {
				"style": "論点を整理"
			}
		}
	}`)

	var payload map[string]navigatorPersonaDefinition
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	got := payload["editor"].SamplingProfile
	if got.TemperatureHint != "low" {
		t.Fatalf("temperature hint = %q", got.TemperatureHint)
	}
	if got.TopPHint != "narrow" {
		t.Fatalf("top_p hint = %q", got.TopPHint)
	}
	if got.VerbosityHint != "balanced" {
		t.Fatalf("verbosity hint = %q", got.VerbosityHint)
	}
}

func TestNavigatorPersonaDefinitionsIncludeAudioBriefingPrompts(t *testing.T) {
	body := mustReadSharedAsset(t, "ai_navigator_personas.json")

	type audioBriefingDefinition struct {
		TonePrompt            string `json:"tone_prompt"`
		SpeakingStylePrompt   string `json:"speaking_style_prompt"`
		DuoConversationPrompt string `json:"duo_conversation_prompt"`
	}
	type navigatorPersonaWithAudioBriefing struct {
		AudioBriefing audioBriefingDefinition `json:"audio_briefing"`
	}

	var payload map[string]navigatorPersonaWithAudioBriefing
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal shared personas: %v", err)
	}
	if len(payload) == 0 {
		t.Fatal("shared personas payload is empty")
	}
	for personaKey, persona := range payload {
		if strings.TrimSpace(persona.AudioBriefing.TonePrompt) == "" {
			t.Fatalf("%s audio_briefing.tone_prompt is empty", personaKey)
		}
		if strings.TrimSpace(persona.AudioBriefing.SpeakingStylePrompt) == "" {
			t.Fatalf("%s audio_briefing.speaking_style_prompt is empty", personaKey)
		}
		if strings.TrimSpace(persona.AudioBriefing.DuoConversationPrompt) == "" {
			t.Fatalf("%s audio_briefing.duo_conversation_prompt is empty", personaKey)
		}
	}
}

func TestGeminiTTSVoiceCatalogIncludesCuratedVoicePickerEntries(t *testing.T) {
	body := mustReadSharedAsset(t, "gemini_tts_voices.json")

	type geminiTTSVoiceDefinition struct {
		VoiceName   string `json:"voice_name"`
		Label       string `json:"label"`
		Tone        string `json:"tone"`
		Description string `json:"description"`
	}
	type geminiTTSVoiceCatalog struct {
		CatalogName string                     `json:"catalog_name"`
		Provider    string                     `json:"provider"`
		Source      string                     `json:"source"`
		Voices      []geminiTTSVoiceDefinition `json:"voices"`
	}

	var payload geminiTTSVoiceCatalog
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal gemini tts voices: %v", err)
	}
	if payload.Provider != "gemini" {
		t.Fatalf("provider = %q, want gemini", payload.Provider)
	}
	if len(payload.Voices) != 30 {
		t.Fatalf("len(voices) = %d, want 30", len(payload.Voices))
	}
	if payload.Voices[0].VoiceName != "Zephyr" || payload.Voices[len(payload.Voices)-1].VoiceName != "Sulafat" {
		t.Fatalf("voices = %#v, want curated Gemini voice order", payload.Voices)
	}
	seen := map[string]struct{}{}
	for _, voice := range payload.Voices {
		if strings.TrimSpace(voice.VoiceName) == "" {
			t.Fatal("found voice with empty voice_name")
		}
		if strings.TrimSpace(voice.Label) == "" {
			t.Fatalf("voice %q has empty label", voice.VoiceName)
		}
		if strings.TrimSpace(voice.Tone) == "" {
			t.Fatalf("voice %q has empty tone", voice.VoiceName)
		}
		if strings.TrimSpace(voice.Description) == "" {
			t.Fatalf("voice %q has empty description", voice.VoiceName)
		}
		if _, ok := seen[voice.VoiceName]; ok {
			t.Fatalf("duplicate voice_name %q", voice.VoiceName)
		}
		seen[voice.VoiceName] = struct{}{}
	}
}

func mustReadSharedAsset(t *testing.T, filename string) []byte {
	t.Helper()

	body, err := os.ReadFile(filepath.Join("..", "..", "..", "shared", filename))
	if err != nil {
		t.Fatalf("read shared asset %s: %v", filename, err)
	}
	return body
}

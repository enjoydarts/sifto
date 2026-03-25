package handler

import (
	"encoding/json"
	"path/filepath"
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

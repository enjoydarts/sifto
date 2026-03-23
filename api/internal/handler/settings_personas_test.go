package handler

import (
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

package service

import "testing"

func TestIsFireworksTextModel(t *testing.T) {
	t.Run("excludes obvious non text models", func(t *testing.T) {
		item := fireworksModelListItem{
			Name:        "fireworks/whisper-v3",
			DisplayName: "Whisper",
			Description: "Speech to text model",
		}
		if isFireworksTextModel(item) {
			t.Fatal("expected whisper model to be excluded")
		}
	})

	t.Run("keeps instruct text models", func(t *testing.T) {
		item := fireworksModelListItem{
			Name:        "fireworks/glm-5",
			DisplayName: "GLM-5 Instruct",
			Description: "LLM text model",
		}
		if !isFireworksTextModel(item) {
			t.Fatal("expected glm-5 instruct model to be treated as text")
		}
	})
}

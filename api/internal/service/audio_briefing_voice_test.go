package service

import (
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func TestNextAudioBriefingVoicingChunkWaitsForFreshGeneratingChunk(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	chunks := []model.AudioBriefingScriptChunk{
		{ID: "chunk-1", Seq: 1, TTSStatus: "generating", UpdatedAt: now.Add(-5 * time.Minute)},
		{ID: "chunk-2", Seq: 2, TTSStatus: "pending", UpdatedAt: now},
	}

	selection, chunk, resetGenerating := nextAudioBriefingVoicingChunk(chunks, now)
	if selection != audioBriefingVoicingChunkSelectionWaiting {
		t.Fatalf("selection = %q, want waiting", selection)
	}
	if chunk != nil {
		t.Fatalf("chunk = %#v, want nil", chunk)
	}
	if resetGenerating {
		t.Fatal("resetGenerating = true, want false")
	}
}

func TestNextAudioBriefingVoicingChunkRetriesStaleGeneratingChunk(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	chunks := []model.AudioBriefingScriptChunk{
		{ID: "chunk-1", Seq: 1, TTSStatus: "generating", UpdatedAt: now.Add(-20 * time.Minute)},
	}

	selection, chunk, resetGenerating := nextAudioBriefingVoicingChunk(chunks, now)
	if selection != audioBriefingVoicingChunkSelectionProcess {
		t.Fatalf("selection = %q, want process", selection)
	}
	if chunk == nil || chunk.ID != "chunk-1" {
		t.Fatalf("chunk = %#v, want chunk-1", chunk)
	}
	if !resetGenerating {
		t.Fatal("resetGenerating = false, want true")
	}
}

func TestNextAudioBriefingVoicingChunkCompletesWhenAllChunksGenerated(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	key := "chunk-1.mp3"
	chunks := []model.AudioBriefingScriptChunk{
		{ID: "chunk-1", Seq: 1, TTSStatus: "generated", R2AudioObjectKey: &key, UpdatedAt: now},
	}

	selection, chunk, resetGenerating := nextAudioBriefingVoicingChunk(chunks, now)
	if selection != audioBriefingVoicingChunkSelectionComplete {
		t.Fatalf("selection = %q, want complete", selection)
	}
	if chunk != nil {
		t.Fatalf("chunk = %#v, want nil", chunk)
	}
	if resetGenerating {
		t.Fatal("resetGenerating = true, want false")
	}
}

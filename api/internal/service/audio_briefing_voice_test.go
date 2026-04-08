package service

import (
	"strings"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func TestNextAudioBriefingVoicingChunkWaitsForFreshGeneratingChunk(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	chunks := []model.AudioBriefingScriptChunk{
		{ID: "chunk-1", Seq: 1, TTSStatus: "generating", LastHeartbeatAt: ptrTime(now.Add(-5 * time.Minute)), UpdatedAt: now.Add(-5 * time.Minute)},
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
		{ID: "chunk-1", Seq: 1, TTSStatus: "generating", AttemptCount: 2, LastHeartbeatAt: ptrTime(now.Add(-20 * time.Minute)), UpdatedAt: now.Add(-20 * time.Minute)},
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

func TestNextAudioBriefingVoicingChunkProcessesRetryWaitChunk(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	chunks := []model.AudioBriefingScriptChunk{
		{ID: "chunk-1", Seq: 1, TTSStatus: "retry_wait", AttemptCount: 1, UpdatedAt: now.Add(-2 * time.Minute)},
	}

	selection, chunk, resetGenerating := nextAudioBriefingVoicingChunk(chunks, now)
	if selection != audioBriefingVoicingChunkSelectionProcess {
		t.Fatalf("selection = %q, want process", selection)
	}
	if chunk == nil || chunk.ID != "chunk-1" {
		t.Fatalf("chunk = %#v, want chunk-1", chunk)
	}
	if resetGenerating {
		t.Fatal("resetGenerating = true, want false")
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

func TestAudioBriefingSpeechParamsForChunkUsesPartnerVoiceForPartnerChunk(t *testing.T) {
	hostVoice := &model.AudioBriefingPersonaVoice{
		SpeechRate:              1.1,
		EmotionalIntensity:      1.2,
		TempoDynamics:           1.3,
		LineBreakSilenceSeconds: 0.4,
		Pitch:                   0.1,
		VolumeGain:              0.2,
	}
	partnerVoice := &model.AudioBriefingPersonaVoice{
		SpeechRate:              0.9,
		EmotionalIntensity:      0.8,
		TempoDynamics:           0.7,
		LineBreakSilenceSeconds: 0.6,
		Pitch:                   -0.1,
		VolumeGain:              -0.2,
	}
	settings := &model.AudioBriefingSettings{ChunkTrailingSilenceSeconds: 1.0}
	chunk := &model.AudioBriefingScriptChunk{Speaker: stringPtr("partner")}

	got := audioBriefingSpeechParamsForChunk(chunk, hostVoice, partnerVoice, settings)

	if got.SpeechRate != partnerVoice.SpeechRate {
		t.Fatalf("SpeechRate = %v, want %v", got.SpeechRate, partnerVoice.SpeechRate)
	}
	if got.EmotionalIntensity != partnerVoice.EmotionalIntensity {
		t.Fatalf("EmotionalIntensity = %v, want %v", got.EmotionalIntensity, partnerVoice.EmotionalIntensity)
	}
	if got.TempoDynamics != partnerVoice.TempoDynamics {
		t.Fatalf("TempoDynamics = %v, want %v", got.TempoDynamics, partnerVoice.TempoDynamics)
	}
	if got.LineBreakSilenceSeconds != partnerVoice.LineBreakSilenceSeconds {
		t.Fatalf("LineBreakSilenceSeconds = %v, want %v", got.LineBreakSilenceSeconds, partnerVoice.LineBreakSilenceSeconds)
	}
	if got.Pitch != partnerVoice.Pitch {
		t.Fatalf("Pitch = %v, want %v", got.Pitch, partnerVoice.Pitch)
	}
	if got.VolumeGain != partnerVoice.VolumeGain {
		t.Fatalf("VolumeGain = %v, want %v", got.VolumeGain, partnerVoice.VolumeGain)
	}
	if got.ChunkTrailingSilenceSecond != settings.ChunkTrailingSilenceSeconds {
		t.Fatalf("ChunkTrailingSilenceSecond = %v, want %v", got.ChunkTrailingSilenceSecond, settings.ChunkTrailingSilenceSeconds)
	}
}

func TestAudioBriefingVoiceConfigCompleteAllowsXAIWithoutVoiceStyle(t *testing.T) {
	if !audioBriefingVoiceConfigComplete("xai", "voice-1", "") {
		t.Fatal("audioBriefingVoiceConfigComplete(xai) = false, want true")
	}
}

func TestAudioBriefingVoiceConfigCompleteRequiresVoiceStyleForAivis(t *testing.T) {
	if audioBriefingVoiceConfigComplete("aivis", "voice-1", "") {
		t.Fatal("audioBriefingVoiceConfigComplete(aivis) = true, want false")
	}
}

func TestAudioBriefingVoiceConfigCompleteAllowsOpenAIWithoutVoiceStyle(t *testing.T) {
	if !audioBriefingVoiceConfigComplete("openai", "alloy", "") {
		t.Fatal("audioBriefingVoiceConfigComplete(openai) = false, want true")
	}
}

func TestAudioBriefingVoiceConfigCompleteAllowsGeminiWithoutVoiceStyle(t *testing.T) {
	if !audioBriefingVoiceConfigComplete("gemini_tts", "Kore", "") {
		t.Fatal("audioBriefingVoiceConfigComplete(gemini_tts) = false, want true")
	}
}

func TestAudioBriefingVoiceConfigCompleteAllowsElevenLabsWithoutVoiceStyle(t *testing.T) {
	if !audioBriefingVoiceConfigComplete("elevenlabs", "voice-1", "") {
		t.Fatal("audioBriefingVoiceConfigComplete(elevenlabs) = false, want true")
	}
}

func TestAudioBriefingElevenLabsDuoReadyRequiresElevenV3AndDistinctVoices(t *testing.T) {
	host := &model.AudioBriefingPersonaVoice{
		TTSProvider: "elevenlabs",
		TTSModel:    "eleven_v3",
		VoiceModel:  "voice-1",
	}
	partner := &model.AudioBriefingPersonaVoice{
		TTSProvider: "elevenlabs",
		TTSModel:    "eleven_v3",
		VoiceModel:  "voice-2",
	}
	if !audioBriefingElevenLabsDuoReady(host, partner) {
		t.Fatal("audioBriefingElevenLabsDuoReady() = false, want true")
	}
	partner.TTSModel = "eleven_multilingual_v2"
	if audioBriefingElevenLabsDuoReady(host, partner) {
		t.Fatal("audioBriefingElevenLabsDuoReady() = true, want false for non-v3")
	}
}

func TestAudioBriefingChunkGroupForSelectionUsesArticleItemID(t *testing.T) {
	chunks := []model.AudioBriefingScriptChunk{
		{ID: "chunk-1", Seq: 1, PartType: "article", ItemID: stringPtr("item-1"), Speaker: stringPtr("host"), Text: "h1"},
		{ID: "chunk-2", Seq: 2, PartType: "article", ItemID: stringPtr("item-1"), Speaker: stringPtr("partner"), Text: "p1"},
		{ID: "chunk-3", Seq: 3, PartType: "article", ItemID: stringPtr("item-2"), Speaker: stringPtr("host"), Text: "h2"},
	}

	group := audioBriefingChunkGroupForSelection(chunks, &chunks[1])

	if group.ItemID != "item-1" {
		t.Fatalf("group.ItemID = %q, want item-1", group.ItemID)
	}
	if len(group.Chunks) != 2 {
		t.Fatalf("len(group.Chunks) = %d, want 2", len(group.Chunks))
	}
}

func TestAudioBriefingGeminiDuoTurnsUsesGroupedChunkSpeakers(t *testing.T) {
	group := audioBriefingChunkGroup{
		PartType: "article",
		ItemID:   "item-1",
		Chunks: []*model.AudioBriefingScriptChunk{
			{Speaker: stringPtr("host"), Text: "最初の発話"},
			{Speaker: stringPtr("partner"), Text: "次の発話"},
		},
	}

	turns := audioBriefingGeminiDuoTurns(group)

	if len(turns) != 2 {
		t.Fatalf("len(turns) = %d, want 2", len(turns))
	}
	if turns[0].Speaker != "host" || turns[1].Speaker != "partner" {
		t.Fatalf("turn speakers = %#v, want host/partner", turns)
	}
}

func TestAudioBriefingGeminiDuoTurnsFromTextParsesSpeakerTaggedText(t *testing.T) {
	group := audioBriefingChunkGroup{
		PartType: "article",
		ItemID:   "item-1",
		Chunks: []*model.AudioBriefingScriptChunk{
			{Speaker: stringPtr("host"), Text: "最初の発話"},
			{Speaker: stringPtr("partner"), Text: "次の発話"},
		},
	}

	turns := audioBriefingGeminiDuoTurnsFromText(group, "<|speaker:0|>[short pause] 最初の発話<|speaker:1|>[laughing] 次の発話")

	if len(turns) != 2 {
		t.Fatalf("len(turns) = %d, want 2", len(turns))
	}
	if turns[0].Speaker != "host" || turns[1].Speaker != "partner" {
		t.Fatalf("turn speakers = %#v, want host/partner", turns)
	}
	if !strings.Contains(turns[0].Text, "[short pause]") {
		t.Fatalf("turns[0].Text = %q, want preserved markup", turns[0].Text)
	}
	if !strings.Contains(turns[1].Text, "[laughing]") {
		t.Fatalf("turns[1].Text = %q, want preserved markup", turns[1].Text)
	}
}

func TestAudioBriefingChunkGroupForSelectionSortsBySeq(t *testing.T) {
	chunks := []model.AudioBriefingScriptChunk{
		{ID: "chunk-2", Seq: 2, PartType: "article", ItemID: stringPtr("item-1"), Speaker: stringPtr("partner"), Text: "p1"},
		{ID: "chunk-1", Seq: 1, PartType: "article", ItemID: stringPtr("item-1"), Speaker: stringPtr("host"), Text: "h1"},
	}

	group := audioBriefingChunkGroupForSelection(chunks, &chunks[0])

	if len(group.Chunks) != 2 {
		t.Fatalf("len(group.Chunks) = %d, want 2", len(group.Chunks))
	}
	if group.Chunks[0] == nil || group.Chunks[0].Seq != 1 {
		t.Fatalf("group.Chunks[0].Seq = %#v, want 1", group.Chunks[0])
	}
	if group.Chunks[1] == nil || group.Chunks[1].Seq != 2 {
		t.Fatalf("group.Chunks[1].Seq = %#v, want 2", group.Chunks[1])
	}

	turns := audioBriefingGeminiDuoTurns(group)
	if len(turns) != 2 {
		t.Fatalf("len(turns) = %d, want 2", len(turns))
	}
	if turns[0].Speaker != "host" || turns[1].Speaker != "partner" {
		t.Fatalf("turn speakers = %#v, want host/partner in seq order", turns)
	}
}

func TestAudioBriefingChunkGroupForSelectionSkipsGeneratedChunks(t *testing.T) {
	objectKey := "audio-briefings/user/job/chunk-001.mp3"
	chunks := []model.AudioBriefingScriptChunk{
		{ID: "chunk-1", Seq: 1, PartType: "article", ItemID: stringPtr("item-1"), TTSStatus: "generated", R2AudioObjectKey: &objectKey, Text: "done"},
		{ID: "chunk-2", Seq: 2, PartType: "article", ItemID: stringPtr("item-1"), TTSStatus: "pending", Text: "pending"},
		{ID: "chunk-3", Seq: 3, PartType: "article", ItemID: stringPtr("item-1"), TTSStatus: "pending", Text: "pending2"},
	}

	group := audioBriefingChunkGroupForSelection(chunks, &chunks[1])

	if len(group.Chunks) != 2 {
		t.Fatalf("len(group.Chunks) = %d, want 2", len(group.Chunks))
	}
	if group.Chunks[0].ID != "chunk-2" || group.Chunks[1].ID != "chunk-3" {
		t.Fatalf("group chunk ids = %s, %s; want chunk-2, chunk-3", group.Chunks[0].ID, group.Chunks[1].ID)
	}
}

func TestAudioBriefingGeminiDuoSplitGroupsKeepsSmallArticleAsSingleGroup(t *testing.T) {
	group := audioBriefingChunkGroup{
		PartType: "article",
		ItemID:   "item-1",
		Chunks: []*model.AudioBriefingScriptChunk{
			{ID: "chunk-1", Seq: 1, Speaker: stringPtr("host"), Text: "短い本文1"},
			{ID: "chunk-2", Seq: 2, Speaker: stringPtr("partner"), Text: "短い本文2"},
		},
	}

	split := audioBriefingGeminiDuoSplitGroups(group, 4096)

	if len(split) != 1 {
		t.Fatalf("len(split) = %d, want 1", len(split))
	}
	if len(split[0].Chunks) != 2 {
		t.Fatalf("len(split[0].Chunks) = %d, want 2", len(split[0].Chunks))
	}
}

func TestAudioBriefingGeminiDuoSplitGroupsSplitsLargeArticleByTurnBoundary(t *testing.T) {
	longText := strings.Repeat("あ", 700)
	group := audioBriefingChunkGroup{
		PartType: "article",
		ItemID:   "item-1",
		Chunks: []*model.AudioBriefingScriptChunk{
			{ID: "chunk-1", Seq: 1, Speaker: stringPtr("host"), Text: longText},
			{ID: "chunk-2", Seq: 2, Speaker: stringPtr("partner"), Text: longText},
			{ID: "chunk-3", Seq: 3, Speaker: stringPtr("host"), Text: "最後の短い発話"},
		},
	}

	split := audioBriefingGeminiDuoSplitGroups(group, 3200)

	if len(split) != 2 {
		t.Fatalf("len(split) = %d, want 2", len(split))
	}
	if len(split[0].Chunks) != 1 || split[0].Chunks[0].Seq != 1 {
		t.Fatalf("split[0] = %#v, want only seq 1", split[0].Chunks)
	}
	if len(split[1].Chunks) != 2 {
		t.Fatalf("len(split[1].Chunks) = %d, want 2", len(split[1].Chunks))
	}
	if split[1].Chunks[0].Seq != 2 || split[1].Chunks[1].Seq != 3 {
		t.Fatalf("split[1] seqs = %d,%d want 2,3", split[1].Chunks[0].Seq, split[1].Chunks[1].Seq)
	}
}

func TestAudioBriefingGeminiDuoSplitGroupsIsolatesOversizedSingleTurn(t *testing.T) {
	longText := strings.Repeat("あ", 1800)
	group := audioBriefingChunkGroup{
		PartType: "article",
		ItemID:   "item-1",
		Chunks: []*model.AudioBriefingScriptChunk{
			{ID: "chunk-1", Seq: 1, Speaker: stringPtr("host"), Text: longText},
			{ID: "chunk-2", Seq: 2, Speaker: stringPtr("partner"), Text: "後続の短い発話"},
		},
	}

	split := audioBriefingGeminiDuoSplitGroups(group, 3200)

	if len(split) != 2 {
		t.Fatalf("len(split) = %d, want 2", len(split))
	}
	if len(split[0].Chunks) != 1 || split[0].Chunks[0].Seq != 1 {
		t.Fatalf("split[0] = %#v, want only oversized seq 1", split[0].Chunks)
	}
	if len(split[1].Chunks) != 1 || split[1].Chunks[0].Seq != 2 {
		t.Fatalf("split[1] = %#v, want only seq 2", split[1].Chunks)
	}
}

func TestAudioBriefingFishDuoPreprocessTextUsesSpeakerTagsInSeqOrder(t *testing.T) {
	group := audioBriefingChunkGroup{
		PartType: "article",
		ItemID:   "item-1",
		Chunks: []*model.AudioBriefingScriptChunk{
			{Seq: 2, Speaker: stringPtr("partner"), Text: "補足します。"},
			{Seq: 1, Speaker: stringPtr("host"), Text: "最初の話題です。"},
		},
	}

	got := audioBriefingFishDuoPreprocessText(group)

	if got != "<|speaker:0|>最初の話題です。<|speaker:1|>補足します。" {
		t.Fatalf("audioBriefingFishDuoPreprocessText() = %q", got)
	}
}

func TestAudioBriefingFishDuoPreprocessTextDefaultsBlankSpeakerToHost(t *testing.T) {
	group := audioBriefingChunkGroup{
		PartType: "article",
		ItemID:   "item-1",
		Chunks: []*model.AudioBriefingScriptChunk{
			{Seq: 1, Text: "話者未設定です。"},
			{Seq: 2, Speaker: stringPtr("partner"), Text: "補足します。"},
		},
	}

	got := audioBriefingFishDuoPreprocessText(group)

	if got != "<|speaker:0|>話者未設定です。<|speaker:1|>補足します。" {
		t.Fatalf("audioBriefingFishDuoPreprocessText() = %q", got)
	}
}

func TestAudioBriefingValidateFishDuoPreprocessedTextRejectsMissingSpeakerTags(t *testing.T) {
	if err := audioBriefingValidateFishDuoPreprocessedText("[自然に] テキスト"); err == nil {
		t.Fatal("audioBriefingValidateFishDuoPreprocessedText() error = nil, want error")
	}
}

func TestAudioBriefingValidateFishDuoPreprocessedTextAcceptsBothSpeakerTags(t *testing.T) {
	if err := audioBriefingValidateFishDuoPreprocessedText("<|speaker:0|>[自然に] 冒頭<|speaker:1|>[少し柔らかく] 補足"); err != nil {
		t.Fatalf("audioBriefingValidateFishDuoPreprocessedText() error = %v", err)
	}
}

func ptrTime(v time.Time) *time.Time {
	return &v
}

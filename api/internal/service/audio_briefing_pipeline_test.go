package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
)

func TestAudioBriefingManualSlotKeyAtUsesManualPrefix(t *testing.T) {
	now := time.Date(2026, 3, 25, 14, 37, 12, 123456789, timeutil.JST)

	got := AudioBriefingManualSlotKeyAt(now)

	if !strings.HasPrefix(got, "manual-2026-03-25-143712-") {
		t.Fatalf("AudioBriefingManualSlotKeyAt(...) = %q, want manual timestamp prefix", got)
	}
}

func TestAudioBriefingNextPipelineStage(t *testing.T) {
	tests := []struct {
		name   string
		job    model.AudioBriefingJob
		chunks []model.AudioBriefingScriptChunk
		want   audioBriefingPipelineStage
		err    bool
	}{
		{
			name: "pending goes to scripting",
			job:  model.AudioBriefingJob{Status: "pending"},
			want: audioBriefingPipelineStageScript,
		},
		{
			name: "scripted goes to voice",
			job:  model.AudioBriefingJob{Status: "scripted"},
			want: audioBriefingPipelineStageVoice,
		},
		{
			name: "scripting resumes scripting stage",
			job:  model.AudioBriefingJob{Status: "scripting"},
			want: audioBriefingPipelineStageScript,
		},
		{
			name: "voicing resumes voice stage",
			job:  model.AudioBriefingJob{Status: "voicing"},
			want: audioBriefingPipelineStageVoice,
		},
		{
			name: "voiced goes to concat",
			job:  model.AudioBriefingJob{Status: "voiced"},
			want: audioBriefingPipelineStageConcat,
		},
		{
			name: "concatenating resumes concat stage",
			job:  model.AudioBriefingJob{Status: "concatenating"},
			want: audioBriefingPipelineStageConcat,
		},
		{
			name: "failed with incomplete chunks resumes voice",
			job:  model.AudioBriefingJob{Status: "failed"},
			chunks: []model.AudioBriefingScriptChunk{
				{TTSStatus: "generated"},
				{TTSStatus: "failed"},
			},
			want: audioBriefingPipelineStageVoice,
		},
		{
			name: "failed with fully generated chunks resumes concat",
			job:  model.AudioBriefingJob{Status: "failed"},
			chunks: []model.AudioBriefingScriptChunk{
				{TTSStatus: "generated", R2AudioObjectKey: stringPtr("chunk-1")},
				{TTSStatus: "generated", R2AudioObjectKey: stringPtr("chunk-2")},
			},
			want: audioBriefingPipelineStageConcat,
		},
		{
			name: "published has no next stage",
			job:  model.AudioBriefingJob{Status: "published"},
			err:  true,
		},
		{
			name: "failed without chunks resumes scripting",
			job:  model.AudioBriefingJob{Status: "failed"},
			want: audioBriefingPipelineStageScript,
		},
	}

	for _, tt := range tests {
		got, err := audioBriefingNextPipelineStage(&tt.job, tt.chunks)
		if tt.err {
			if err == nil {
				t.Fatalf("%s: expected error, got nil", tt.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tt.name, err)
		}
		if got != tt.want {
			t.Fatalf("%s: stage = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestAudioBriefingShouldContinue(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{status: "pending", want: true},
		{status: "scripting", want: true},
		{status: "scripted", want: true},
		{status: "voicing", want: true},
		{status: "voiced", want: true},
		{status: "concatenating", want: true},
		{status: "failed", want: true},
		{status: "published", want: false},
	}

	for _, tt := range tests {
		if got := audioBriefingShouldContinue(tt.status); got != tt.want {
			t.Fatalf("audioBriefingShouldContinue(%q) = %t, want %t", tt.status, got, tt.want)
		}
	}
}

func TestResolveAudioBriefingPartnerVoiceRequiresExplicitPartnerPersona(t *testing.T) {
	orchestrator := &AudioBriefingOrchestrator{}
	job := &model.AudioBriefingJob{
		ID:               "job-1",
		UserID:           "user-1",
		Persona:          "editor",
		ConversationMode: "duo",
	}

	partnerPersona, partnerVoice, err := orchestrator.resolveAudioBriefingPartnerVoice(context.Background(), job, &model.AudioBriefingPersonaVoice{
		TTSProvider: "xai",
		VoiceModel:  "voice-host",
	}, false)
	if err == nil {
		t.Fatal("resolveAudioBriefingPartnerVoice(...) error = nil, want explicit configuration error")
	}
	if !strings.Contains(err.Error(), "must be explicitly configured") {
		t.Fatalf("error = %v, want explicit configuration message", err)
	}
	if partnerPersona != "" {
		t.Fatalf("partnerPersona = %q, want empty", partnerPersona)
	}
	if partnerVoice != nil {
		t.Fatalf("partnerVoice = %#v, want nil", partnerVoice)
	}
}

func TestResolveAudioBriefingPartnerVoiceRequiresExplicitPartnerPersonaForGemini(t *testing.T) {
	orchestrator := &AudioBriefingOrchestrator{}
	job := &model.AudioBriefingJob{
		ID:               "job-1",
		UserID:           "user-1",
		Persona:          "editor",
		ConversationMode: "duo",
	}

	_, _, err := orchestrator.resolveAudioBriefingPartnerVoice(context.Background(), job, &model.AudioBriefingPersonaVoice{
		TTSProvider: "gemini_tts",
		TTSModel:    "gemini-2.5-flash-tts",
		VoiceModel:  "Kore",
	}, false)
	if err == nil {
		t.Fatal("resolveAudioBriefingPartnerVoice(...) error = nil, want explicit gemini configuration error")
	}
	if !strings.Contains(err.Error(), "must be explicitly configured for gemini_tts") {
		t.Fatalf("error = %v, want gemini explicit configuration message", err)
	}
}

func TestResolveRandomAudioBriefingPartnerCandidate(t *testing.T) {
	voices := []model.AudioBriefingPersonaVoice{
		{Persona: "editor", TTSProvider: "xai", VoiceModel: "host"},
		{Persona: "hype", TTSProvider: "xai", VoiceModel: "partner-1"},
		{Persona: "analyst", TTSProvider: "xai", VoiceModel: "partner-2"},
	}

	gotPersona, gotVoice, ok := resolveRandomAudioBriefingPartnerCandidate("editor", false, &model.AudioBriefingPersonaVoice{
		TTSProvider: "xai",
		VoiceModel:  "host",
	}, voices, func(candidates []string) (string, bool) {
		if len(candidates) != 2 {
			t.Fatalf("candidate count = %d, want 2", len(candidates))
		}
		return "analyst", true
	})
	if !ok {
		t.Fatal("resolveRandomAudioBriefingPartnerCandidate(...) ok = false, want true")
	}
	if gotPersona != "analyst" {
		t.Fatalf("partner persona = %q, want analyst", gotPersona)
	}
	if gotVoice == nil || gotVoice.VoiceModel != "partner-2" {
		t.Fatalf("partner voice = %#v, want analyst voice", gotVoice)
	}
}

func TestResolveRandomAudioBriefingPartnerCandidateSkipsHostAndGeminiMismatch(t *testing.T) {
	voices := []model.AudioBriefingPersonaVoice{
		{Persona: "editor", TTSProvider: "gemini_tts", TTSModel: "gemini-2.5-flash-tts", VoiceModel: "Kore"},
		{Persona: "hype", TTSProvider: "xai", VoiceModel: "Different"},
		{Persona: "analyst", TTSProvider: "gemini_tts", TTSModel: "gemini-2.5-pro-tts", VoiceModel: "Mismatched"},
	}

	gotPersona, gotVoice, ok := resolveRandomAudioBriefingPartnerCandidate("editor", true, &model.AudioBriefingPersonaVoice{
		TTSProvider: "gemini_tts",
		TTSModel:    "gemini-2.5-flash-tts",
		VoiceModel:  "Kore",
	}, voices, func(candidates []string) (string, bool) {
		t.Fatalf("picker should not be called when no valid candidates, got %#v", candidates)
		return "", false
	})
	if ok {
		t.Fatal("resolveRandomAudioBriefingPartnerCandidate(...) ok = true, want false")
	}
	if gotPersona != "" {
		t.Fatalf("partner persona = %q, want empty", gotPersona)
	}
	if gotVoice != nil {
		t.Fatalf("partner voice = %#v, want nil", gotVoice)
	}
}

func TestNormalizeAudioBriefingConversationModeValue(t *testing.T) {
	if got := normalizeAudioBriefingConversationModeValue("duo"); got != "duo" {
		t.Fatalf("normalizeAudioBriefingConversationModeValue(duo) = %q, want duo", got)
	}
	if got := normalizeAudioBriefingConversationModeValue(" Duo "); got != "duo" {
		t.Fatalf("normalizeAudioBriefingConversationModeValue(trimmed duo) = %q, want duo", got)
	}
	if got := normalizeAudioBriefingConversationModeValue("something-else"); got != "single" {
		t.Fatalf("normalizeAudioBriefingConversationModeValue(invalid) = %q, want single", got)
	}
}

func TestAudioBriefingInitialPipelineStageForMode(t *testing.T) {
	if got := audioBriefingInitialPipelineStageForMode("single"); got != "single_script" {
		t.Fatalf("audioBriefingInitialPipelineStageForMode(single) = %q, want single_script", got)
	}
	if got := audioBriefingInitialPipelineStageForMode("duo"); got != "duo_script" {
		t.Fatalf("audioBriefingInitialPipelineStageForMode(duo) = %q, want duo_script", got)
	}
}

func TestAudioBriefingJobCanBeResumedAt(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	staleAfter := 30 * time.Minute

	if !audioBriefingJobCanBeResumedAt(&model.AudioBriefingJob{Status: "pending", UpdatedAt: now}, now, staleAfter) {
		t.Fatal("pending job should be resumable")
	}
	if !audioBriefingJobCanBeResumedAt(&model.AudioBriefingJob{Status: "failed", UpdatedAt: now}, now, staleAfter) {
		t.Fatal("failed job should be resumable")
	}
	if !audioBriefingJobCanBeResumedAt(&model.AudioBriefingJob{Status: "voicing", UpdatedAt: now.Add(-31 * time.Minute)}, now, staleAfter) {
		t.Fatal("stale voicing job should be resumable")
	}
	if audioBriefingJobCanBeResumedAt(&model.AudioBriefingJob{Status: "voicing", UpdatedAt: now.Add(-10 * time.Minute)}, now, staleAfter) {
		t.Fatal("fresh voicing job should not be resumable")
	}
	if audioBriefingJobCanBeResumedAt(&model.AudioBriefingJob{Status: "published", UpdatedAt: now}, now, staleAfter) {
		t.Fatal("published job should not be resumable")
	}
}

func TestAudioBriefingArticleBatchSize(t *testing.T) {
	tests := []struct {
		itemCount int
		want      int
	}{
		{itemCount: 0, want: 1},
		{itemCount: 1, want: 1},
		{itemCount: 3, want: 3},
		{itemCount: 4, want: 3},
		{itemCount: 12, want: 3},
	}

	for _, tt := range tests {
		if got := audioBriefingArticleBatchSize(tt.itemCount); got != tt.want {
			t.Fatalf("audioBriefingArticleBatchSize(%d) = %d, want %d", tt.itemCount, got, tt.want)
		}
	}
}

func TestAudioBriefingDuoArticleBatchSize(t *testing.T) {
	tests := []struct {
		itemCount int
		want      int
	}{
		{itemCount: 0, want: 1},
		{itemCount: 1, want: 1},
		{itemCount: 2, want: 2},
		{itemCount: 3, want: 3},
		{itemCount: 4, want: 3},
		{itemCount: 12, want: 3},
	}

	for _, tt := range tests {
		if got := audioBriefingDuoArticleBatchSize(tt.itemCount); got != tt.want {
			t.Fatalf("audioBriefingDuoArticleBatchSize(%d) = %d, want %d", tt.itemCount, got, tt.want)
		}
	}
}

func TestAudioBriefingTurnSectionIntroContext(t *testing.T) {
	base := map[string]any{"time_of_day": "morning"}

	got := audioBriefingTurnSectionIntroContext(base, "overall_summary")

	if got["time_of_day"] != "morning" {
		t.Fatalf("time_of_day = %#v, want morning", got["time_of_day"])
	}
	if got["audio_briefing_generation_section"] != "overall_summary" {
		t.Fatalf("generation_section = %#v, want overall_summary", got["audio_briefing_generation_section"])
	}
	if got["audio_briefing_program_position"] != "after_opening_before_articles" {
		t.Fatalf("program_position = %#v, want after_opening_before_articles", got["audio_briefing_program_position"])
	}
}

func TestBuildAudioBriefingIntroContextIncludesProgramName(t *testing.T) {
	got := buildAudioBriefingIntroContext(time.Date(2026, 4, 2, 7, 0, 0, 0, timeutil.JST), strptr("Morning Sifto"))

	if got["program_name"] != "Morning Sifto" {
		t.Fatalf("program_name = %#v, want Morning Sifto", got["program_name"])
	}
}

func TestAudioBriefingTurnArticleIntroContext(t *testing.T) {
	base := map[string]any{"time_of_day": "morning"}

	got := audioBriefingTurnArticleIntroContext(base, 3, 6, 20)

	if got["time_of_day"] != "morning" {
		t.Fatalf("time_of_day = %#v, want morning", got["time_of_day"])
	}
	if got["audio_briefing_generation_section"] != "article_segments" {
		t.Fatalf("generation_section = %#v, want article_segments", got["audio_briefing_generation_section"])
	}
	if got["audio_briefing_program_position"] != "article_midstream" {
		t.Fatalf("program_position = %#v, want article_midstream", got["audio_briefing_program_position"])
	}
	if got["audio_briefing_article_batch_start_index"] != 4 {
		t.Fatalf("batch_start = %#v, want 4", got["audio_briefing_article_batch_start_index"])
	}
	if got["audio_briefing_article_batch_end_index"] != 6 {
		t.Fatalf("batch_end = %#v, want 6", got["audio_briefing_article_batch_end_index"])
	}
	if got["audio_briefing_total_articles"] != 20 {
		t.Fatalf("total_articles = %#v, want 20", got["audio_briefing_total_articles"])
	}
}

func TestAudioBriefingArticleBatchTargetChars(t *testing.T) {
	got := audioBriefingArticleBatchTargetChars(12000, 20, 4)
	if got != 2132 {
		t.Fatalf("audioBriefingArticleBatchTargetChars(...) = %d, want %d", got, 2132)
	}
}

func TestAudioBriefingGenerateArticleSegmentsBatchSplitsOnError(t *testing.T) {
	articles := []AudioBriefingScriptArticle{
		{ItemID: "item-1"},
		{ItemID: "item-2"},
		{ItemID: "item-3"},
		{ItemID: "item-4"},
	}
	var seen []int
	call := func(batch []AudioBriefingScriptArticle, _ map[string]any, _ int, _ bool, _ bool, _ bool, _ bool) (*AudioBriefingScriptResponse, error) {
		seen = append(seen, len(batch))
		if len(batch) > 1 {
			return nil, fmt.Errorf("batch too large")
		}
		return &AudioBriefingScriptResponse{
			ArticleSegments: []AudioBriefingScriptSegment{
				{ItemID: batch[0].ItemID, Headline: "h", SummaryIntro: "s", Commentary: "c"},
			},
		}, nil
	}

	got, err := audioBriefingGenerateArticleSegmentsBatch(articles, 12000, 4, call, map[string]any{"time_of_day": "morning"})
	if err != nil {
		t.Fatalf("audioBriefingGenerateArticleSegmentsBatch(...) error = %v", err)
	}
	if len(got.Segments) != 4 {
		t.Fatalf("audioBriefingGenerateArticleSegmentsBatch(...) len = %d, want 4", len(got.Segments))
	}
	if len(seen) < 3 {
		t.Fatalf("expected recursive split calls, saw %v", seen)
	}
}

func TestAudioBriefingGenerateArticleSegmentsBatchTracksRecoveredFailures(t *testing.T) {
	articles := []AudioBriefingScriptArticle{
		{ItemID: "item-1"},
		{ItemID: "item-2"},
		{ItemID: "item-3"},
		{ItemID: "item-4"},
	}
	call := func(batch []AudioBriefingScriptArticle, _ map[string]any, _ int, _ bool, _ bool, _ bool, _ bool) (*AudioBriefingScriptResponse, error) {
		if len(batch) > 1 {
			return nil, fmt.Errorf("batch too large")
		}
		return &AudioBriefingScriptResponse{
			ArticleSegments: []AudioBriefingScriptSegment{
				{ItemID: batch[0].ItemID, Headline: "h", SummaryIntro: "s", Commentary: "c"},
			},
		}, nil
	}

	result, err := audioBriefingGenerateArticleSegmentsBatch(articles, 12000, 4, call, map[string]any{"time_of_day": "morning"})
	if err != nil {
		t.Fatalf("audioBriefingGenerateArticleSegmentsBatch(...) error = %v", err)
	}
	if len(result.Segments) != 4 {
		t.Fatalf("segments len = %d, want 4", len(result.Segments))
	}
	if len(result.RecoveredFailures) == 0 {
		t.Fatal("expected recovered failures to be recorded")
	}
}

func TestAudioBriefingGenerateTurnSectionFailsOnEmptyTurns(t *testing.T) {
	call := func(batch []AudioBriefingScriptArticle, _ map[string]any, _ int, _ bool, _ bool, _ bool, _ bool) (*AudioBriefingScriptResponse, error) {
		return &AudioBriefingScriptResponse{
			Turns: nil,
			LLM:   &LLMUsage{Provider: "anthropic", ResolvedModel: "claude-sonnet-4-6"},
		}, nil
	}

	_, _, err := audioBriefingGenerateTurnSection(
		[]AudioBriefingScriptArticle{{ItemID: "item-1"}},
		map[string]any{"time_of_day": "morning"},
		12000,
		"opening",
		call,
	)
	if err == nil {
		t.Fatal("expected empty turns to fail")
	}
	if !strings.Contains(err.Error(), "returned no turns") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAudioBriefingGenerateTurnArticleBatchSplitsOnEmptyTurns(t *testing.T) {
	articles := []AudioBriefingScriptArticle{
		{ItemID: "item-1"},
		{ItemID: "item-2"},
		{ItemID: "item-3"},
		{ItemID: "item-4"},
	}
	var seen []int
	call := func(batch []AudioBriefingScriptArticle, _ map[string]any, _ int, _ bool, _ bool, _ bool, _ bool) (*AudioBriefingScriptResponse, error) {
		seen = append(seen, len(batch))
		if len(batch) > 1 {
			return &AudioBriefingScriptResponse{
				Turns: nil,
				LLM:   &LLMUsage{Provider: "openai", ResolvedModel: "gpt-5.4"},
			}, nil
		}
		return &AudioBriefingScriptResponse{
			Turns: []AudioBriefingScriptTurn{{Speaker: "host", Section: "article", ItemID: stringPtr(batch[0].ItemID), Text: "本文です。"}},
			LLM:   &LLMUsage{Provider: "openai", ResolvedModel: "gpt-5.4"},
		}, nil
	}

	got, err := audioBriefingGenerateTurnArticleBatch(articles, 12000, 4, call, map[string]any{"time_of_day": "morning"})
	if err != nil {
		t.Fatalf("audioBriefingGenerateTurnArticleBatch(...) error = %v", err)
	}
	if len(got.Turns) != 4 {
		t.Fatalf("turns len = %d, want 4", len(got.Turns))
	}
	if len(got.RecoveredFailures) == 0 {
		t.Fatal("expected recovered failures to be recorded")
	}
	if len(seen) < 3 {
		t.Fatalf("expected recursive split calls, saw %v", seen)
	}
}

func TestAudioBriefingFrameSectionNeedsSupplement(t *testing.T) {
	if !audioBriefingFrameSectionNeedsSupplement("opening", 12000, "おはようございます。") {
		t.Fatal("short opening should require supplement")
	}
	if audioBriefingFrameSectionNeedsSupplement("opening", 12000, strings.Repeat("朝の空気が少し軽いですね。\n", 40)) {
		t.Fatal("longer opening should not require supplement")
	}
}

func TestAudioBriefingMergeSectionTextDropsRepeatedLeadingSentences(t *testing.T) {
	base := "おはようございます、編集長 水城です。\n火曜の朝は、空気が少し軽く、通勤や支度の足取りも変わりますね。\n今朝は、働き方や暮らし方、技術の行き先を静かに見比べる回です。"
	supplement := "おはようございます、編集長 水城です。\n火曜の朝は、空気が少し軽く、通勤や支度の足取りも変わりますね。\n春の陽気に誘われて、カバンの中身も入れ替えたくなる頃ですが、わたくしは相変わらず無骨なトートで押し通しております。"

	got := audioBriefingMergeSectionText(base, supplement)

	if strings.Count(got, "おはようございます、編集長 水城です。") != 1 {
		t.Fatalf("merged text duplicated greeting: %q", got)
	}
	if !strings.Contains(got, "春の陽気に誘われて") {
		t.Fatalf("merged text missing new supplement sentence: %q", got)
	}
}

func TestAudioBriefingSupplementIntroContextIncludesExistingText(t *testing.T) {
	base := map[string]any{"time_of_day": "morning"}
	got := audioBriefingSupplementIntroContext(base, "ending", "ここまでです。")

	if got["time_of_day"] != "morning" {
		t.Fatalf("time_of_day = %#v, want morning", got["time_of_day"])
	}
	if got["audio_briefing_generation_mode"] != "supplement" {
		t.Fatalf("generation_mode = %#v, want supplement", got["audio_briefing_generation_mode"])
	}
	if got["audio_briefing_generation_section"] != "ending" {
		t.Fatalf("generation_section = %#v, want ending", got["audio_briefing_generation_section"])
	}
	if got["audio_briefing_existing_section_text"] != "ここまでです。" {
		t.Fatalf("existing_section_text = %#v, want original text", got["audio_briefing_existing_section_text"])
	}
}

func TestAudioBriefingArticleSupplementIntroContextIncludesExistingSegments(t *testing.T) {
	base := map[string]any{"time_of_day": "morning"}
	segments := []AudioBriefingScriptSegment{{ItemID: "item-1", Headline: "見出し", SummaryIntro: "要約", Commentary: "感想"}}

	got := audioBriefingArticleSupplementIntroContext(base, segments)

	if got["time_of_day"] != "morning" {
		t.Fatalf("time_of_day = %#v, want morning", got["time_of_day"])
	}
	if got["audio_briefing_generation_mode"] != "supplement" {
		t.Fatalf("generation_mode = %#v, want supplement", got["audio_briefing_generation_mode"])
	}
	if got["audio_briefing_generation_section"] != "article_segments" {
		t.Fatalf("generation_section = %#v, want article_segments", got["audio_briefing_generation_section"])
	}
	gotSegments, ok := got["audio_briefing_existing_article_segments"].([]AudioBriefingScriptSegment)
	if !ok || len(gotSegments) != 1 || gotSegments[0].ItemID != "item-1" {
		t.Fatalf("existing_article_segments = %#v", got["audio_briefing_existing_article_segments"])
	}
}

func TestAudioBriefingGenerateArticleSegmentsBatchSupplementsShortSegments(t *testing.T) {
	articles := []AudioBriefingScriptArticle{{ItemID: "item-1"}}
	callCount := 0
	call := func(batch []AudioBriefingScriptArticle, intro map[string]any, _ int, _ bool, _ bool, _ bool, _ bool) (*AudioBriefingScriptResponse, error) {
		callCount++
		if callCount == 1 {
			return &AudioBriefingScriptResponse{
				ArticleSegments: []AudioBriefingScriptSegment{
					{ItemID: batch[0].ItemID, Headline: "短い見出し", SummaryIntro: "短い要約", Commentary: "短い感想"},
				},
				LLM: &LLMUsage{Provider: "openai", ResolvedModel: "gpt-5.4"},
			}, nil
		}
		if intro["audio_briefing_generation_section"] != "article_segments" {
			t.Fatalf("generation_section = %#v, want article_segments", intro["audio_briefing_generation_section"])
		}
		return &AudioBriefingScriptResponse{
			ArticleSegments: []AudioBriefingScriptSegment{
				{ItemID: batch[0].ItemID, Headline: "", SummaryIntro: "補足の要約です。追加の焦点も入れます。", Commentary: "補足の感想です。理由も比較も今後の見方も足します。"},
			},
			LLM: &LLMUsage{Provider: "google", ResolvedModel: "gemini-2.5-pro"},
		}, nil
	}

	got, err := audioBriefingGenerateArticleSegmentsBatch(articles, 12000, 20, call, map[string]any{"time_of_day": "morning"})
	if err != nil {
		t.Fatalf("audioBriefingGenerateArticleSegmentsBatch(...) error = %v", err)
	}
	if callCount != 2 {
		t.Fatalf("callCount = %d, want 2", callCount)
	}
	if len(got.Segments) != 1 {
		t.Fatalf("segments len = %d, want 1", len(got.Segments))
	}
	if !strings.Contains(got.Segments[0].SummaryIntro, "補足の要約です。") {
		t.Fatalf("summary_intro = %q", got.Segments[0].SummaryIntro)
	}
	if !strings.Contains(got.Segments[0].Commentary, "補足の感想です。") {
		t.Fatalf("commentary = %q", got.Segments[0].Commentary)
	}
	if len(got.ScriptLLMModels) != 2 {
		t.Fatalf("script models len = %d, want 2", len(got.ScriptLLMModels))
	}
}

func TestAudioBriefingGenerateArticleSegmentsBatchRetriesInvalidInitialSegments(t *testing.T) {
	articles := []AudioBriefingScriptArticle{
		{ItemID: "item-1"},
		{ItemID: "item-2"},
	}
	var seen []int
	call := func(batch []AudioBriefingScriptArticle, _ map[string]any, _ int, _ bool, _ bool, _ bool, _ bool) (*AudioBriefingScriptResponse, error) {
		seen = append(seen, len(batch))
		if len(batch) > 1 {
			return &AudioBriefingScriptResponse{
				ArticleSegments: []AudioBriefingScriptSegment{
					{ItemID: batch[0].ItemID, Headline: "見出し1", SummaryIntro: "", Commentary: "感想1"},
					{ItemID: batch[1].ItemID, Headline: "見出し2", SummaryIntro: "要約2", Commentary: "感想2"},
				},
				LLM: &LLMUsage{Provider: "openrouter", ResolvedModel: "moonshot/kimi-k2"},
			}, nil
		}
		return &AudioBriefingScriptResponse{
			ArticleSegments: []AudioBriefingScriptSegment{
				{ItemID: batch[0].ItemID, Headline: "見出し", SummaryIntro: "要約", Commentary: "感想"},
			},
		}, nil
	}

	got, err := audioBriefingGenerateArticleSegmentsBatch(articles, 12000, 20, call, map[string]any{"time_of_day": "morning"})
	if err != nil {
		t.Fatalf("audioBriefingGenerateArticleSegmentsBatch(...) error = %v", err)
	}
	if len(got.Segments) != 2 {
		t.Fatalf("segments len = %d, want 2", len(got.Segments))
	}
	if len(seen) < 3 {
		t.Fatalf("expected retry via recursive split, saw %v", seen)
	}
	if len(got.RecoveredFailures) == 0 {
		t.Fatal("expected invalid initial response to be recorded as recovered failure")
	}
}

func TestResolveAudioBriefingScriptModelsPrefersPrimaryThenFallback(t *testing.T) {
	settings := &model.UserSettings{
		AudioBriefingScriptModel:         stringPtr("openrouter::openai/gpt-oss-120b"),
		AudioBriefingScriptFallbackModel: stringPtr("gemini-2.5-flash"),
		HasOpenRouterAPIKey:              true,
		HasGoogleAPIKey:                  true,
	}

	got := resolveAudioBriefingScriptModels(settings)

	if len(got) != 2 {
		t.Fatalf("len(models) = %d, want 2 (%v)", len(got), got)
	}
	if got[0] != "openrouter::openai/gpt-oss-120b" {
		t.Fatalf("models[0] = %q, want primary", got[0])
	}
	if got[1] != "gemini-2.5-flash" {
		t.Fatalf("models[1] = %q, want fallback", got[1])
	}
}

func TestIsRetryableAudioBriefingScriptWorkerError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "worker 502", err: fmt.Errorf("worker /audio-briefing-script: status 502"), want: true},
		{name: "worker 422", err: fmt.Errorf("worker /audio-briefing-script: status 422"), want: false},
		{name: "timeout", err: fmt.Errorf("Post \"x\": net/http: request canceled (Client.Timeout exceeded while awaiting headers)"), want: true},
	}
	for _, tt := range tests {
		if got := isRetryableAudioBriefingScriptWorkerError(tt.err); got != tt.want {
			t.Fatalf("%s: got %t want %t", tt.name, got, tt.want)
		}
	}
}

func TestAppendAudioBriefingScriptModelPrefersResolvedAndDedupes(t *testing.T) {
	var got []string

	got = appendAudioBriefingScriptModel(got, &LLMUsage{
		Provider:       "anthropic",
		Model:          "claude-3-7-sonnet",
		ResolvedModel:  "claude-sonnet-4-20250514",
		RequestedModel: "claude-sonnet-4",
	})
	got = appendAudioBriefingScriptModel(got, &LLMUsage{
		Provider:      "anthropic",
		Model:         "claude-3-7-sonnet",
		ResolvedModel: "claude-sonnet-4-20250514",
	})
	got = appendAudioBriefingScriptModel(got, &LLMUsage{
		Provider: "google",
		Model:    "gemini-2.5-pro",
	})

	if len(got) != 2 {
		t.Fatalf("len(models) = %d, want 2 (%v)", len(got), got)
	}
	if got[0] != "Anthropic / claude-sonnet-4-20250514" {
		t.Fatalf("models[0] = %q, want provider-prefixed resolved model", got[0])
	}
	if got[1] != "Google / gemini-2.5-pro" {
		t.Fatalf("models[1] = %q, want provider-prefixed model", got[1])
	}
}

func TestAudioBriefingBuildDraftFailsWhenVoiceIsNotConfigured(t *testing.T) {
	orchestrator := &AudioBriefingOrchestrator{}

	_, err := orchestrator.buildDraft(
		t.Context(),
		"user-1",
		time.Date(2026, 3, 25, 6, 0, 0, 0, timeutil.JST),
		"editor",
		[]model.AudioBriefingJobItem{{ItemID: "item-1"}},
		nil,
		20,
	)
	if err == nil {
		t.Fatal("expected missing voice configuration error")
	}
	if !strings.Contains(err.Error(), "voice is not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecoverAudioBriefingStageErrorReturnsFailedJobWhenAlreadyFailed(t *testing.T) {
	job := &model.AudioBriefingJob{ID: "job-1", Status: "failed"}

	got, err := recoverAudioBriefingStageError(audioBriefingPipelineStageVoice, job, fmt.Errorf("boom"), func(errorCode, errorMessage string) (*model.AudioBriefingJob, error) {
		t.Fatalf("fail callback should not be called")
		return nil, nil
	})
	if err != nil {
		t.Fatalf("recoverAudioBriefingStageError(...) error = %v", err)
	}
	if got != job {
		t.Fatalf("recoverAudioBriefingStageError(...) job = %#v, want original job", got)
	}
}

func TestRecoverAudioBriefingStageErrorFailsActiveScriptingJob(t *testing.T) {
	job := &model.AudioBriefingJob{ID: "job-1", Status: "scripting"}
	var gotCode string
	var gotMessage string

	got, err := recoverAudioBriefingStageError(audioBriefingPipelineStageScript, job, fmt.Errorf("script boom"), func(errorCode, errorMessage string) (*model.AudioBriefingJob, error) {
		gotCode = errorCode
		gotMessage = errorMessage
		return &model.AudioBriefingJob{ID: "job-1", Status: "failed"}, nil
	})
	if err != nil {
		t.Fatalf("recoverAudioBriefingStageError(...) error = %v", err)
	}
	if got == nil || got.Status != "failed" {
		t.Fatalf("recoverAudioBriefingStageError(...) job = %#v, want failed job", got)
	}
	if gotCode != "script_failed" {
		t.Fatalf("errorCode = %q, want script_failed", gotCode)
	}
	if !strings.Contains(gotMessage, "script boom") {
		t.Fatalf("errorMessage = %q, want script boom", gotMessage)
	}
}

func TestRecoverAudioBriefingStageErrorFailsActiveVoicingJob(t *testing.T) {
	job := &model.AudioBriefingJob{ID: "job-1", Status: "voicing"}
	var gotCode string

	got, err := recoverAudioBriefingStageError(audioBriefingPipelineStageVoice, job, fmt.Errorf("tts boom"), func(errorCode, errorMessage string) (*model.AudioBriefingJob, error) {
		gotCode = errorCode
		return &model.AudioBriefingJob{ID: "job-1", Status: "failed"}, nil
	})
	if err != nil {
		t.Fatalf("recoverAudioBriefingStageError(...) error = %v", err)
	}
	if got == nil || got.Status != "failed" {
		t.Fatalf("recoverAudioBriefingStageError(...) job = %#v, want failed job", got)
	}
	if gotCode != "tts_failed" {
		t.Fatalf("errorCode = %q, want tts_failed", gotCode)
	}
}

func TestAudioBriefingFailureContextDetachesFromCanceledParent(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel()

	ctx, release := audioBriefingFailureContext(parent)
	defer release()

	if err := ctx.Err(); err != nil {
		t.Fatalf("failure context should not inherit cancellation: %v", err)
	}
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("failure context should have a deadline")
	}
}

func stringPtr(v string) *string { return &v }

package service

import (
	"strings"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
)

func TestBuildAudioBriefingDraftSkippedWhenNoItems(t *testing.T) {
	draft := BuildAudioBriefingDraft(time.Date(2026, 3, 24, 6, 0, 0, 0, timeutil.JST), "editor", nil, nil, 0)
	if draft.Status != "skipped" {
		t.Fatalf("draft.Status = %q, want skipped", draft.Status)
	}
	if len(draft.Chunks) != 0 {
		t.Fatalf("len(draft.Chunks) = %d, want 0", len(draft.Chunks))
	}
}

func TestBuildAudioBriefingDraftBuildsChunks(t *testing.T) {
	title := "原題"
	translated := "翻訳題"
	source := "Tech"
	summary := "要約本文です。ここでは大事なポイントをまとめます。"
	voice := &model.AudioBriefingPersonaVoice{
		Persona:     "editor",
		TTSProvider: "aivis",
		VoiceModel:  "speaker-a",
		VoiceStyle:  "calm",
	}
	draft := BuildAudioBriefingDraft(
		time.Date(2026, 3, 24, 6, 0, 0, 0, timeutil.JST),
		"editor",
		[]model.AudioBriefingJobItem{{
			ItemID:          "item-1",
			Rank:            1,
			Title:           &title,
			TranslatedTitle: &translated,
			SourceTitle:     &source,
			SummarySnapshot: &summary,
		}},
		voice,
		0,
	)
	if draft.Status != "scripted" {
		t.Fatalf("draft.Status = %q, want scripted", draft.Status)
	}
	if len(draft.Chunks) != 4 {
		t.Fatalf("len(draft.Chunks) = %d, want 4", len(draft.Chunks))
	}
	if draft.Chunks[2].PartType != "article" {
		t.Fatalf("draft.Chunks[2].PartType = %q, want article", draft.Chunks[2].PartType)
	}
	if draft.Chunks[2].TTSProvider == nil || *draft.Chunks[2].TTSProvider != "aivis" {
		t.Fatalf("draft.Chunks[2].TTSProvider = %v, want aivis", draft.Chunks[2].TTSProvider)
	}
}

func TestBuildAudioBriefingDraftFromNarrationUsesNarration(t *testing.T) {
	title := "原題"
	translated := "翻訳題"
	source := "Tech"
	summary := "要約本文です。ここでは大事なポイントをまとめます。"
	voice := &model.AudioBriefingPersonaVoice{
		Persona:     "editor",
		TTSProvider: "aivis",
		VoiceModel:  "speaker-a",
		VoiceStyle:  "calm",
	}
	draft := BuildAudioBriefingDraftFromNarration(
		time.Date(2026, 3, 24, 6, 0, 0, 0, timeutil.JST),
		"editor",
		[]model.AudioBriefingJobItem{{
			ItemID:          "item-1",
			Rank:            1,
			Title:           &title,
			TranslatedTitle: &translated,
			SourceTitle:     &source,
			SummarySnapshot: &summary,
		}},
		voice,
		AudioBriefingNarration{
			Opening:        "編集長 水城です。今朝の流れを素早く見ていきましょう。",
			OverallSummary: "まず全体として、AIとプロダクトの境目がまた一段近づいています。",
			Articles: map[string]AudioBriefingNarrationArticle{
				"item-1": {
					Headline:   "LLMで見た翻訳題",
					Commentary: "ここは背景と含意を押さえておく価値があります。",
				},
			},
			Ending: "続きはSiftoで確認してください。",
		},
		0,
	)
	if draft.Status != "scripted" {
		t.Fatalf("draft.Status = %q, want scripted", draft.Status)
	}
	if len(draft.Chunks) != 4 {
		t.Fatalf("len(draft.Chunks) = %d, want 4", len(draft.Chunks))
	}
	if got := draft.Chunks[0].Text; got != "編集長 水城です。今朝の流れを素早く見ていきましょう。\n\n" {
		t.Fatalf("opening = %q", got)
	}
	if got := draft.Chunks[1].PartType; got != "summary" {
		t.Fatalf("summary part type = %q", got)
	}
	if got := draft.Chunks[1].Text; got != "まず全体として、AIとプロダクトの境目がまた一段近づいています。\n\n" {
		t.Fatalf("summary = %q", got)
	}
	if got := draft.Chunks[2].Text; got != "LLMで見た翻訳題です。ここは背景と含意を押さえておく価値があります。\n\n" {
		t.Fatalf("article = %q", got)
	}
	if got := draft.Chunks[3].Text; got != "続きはSiftoで確認してください。" {
		t.Fatalf("ending = %q", got)
	}
}

func TestBuildAudioBriefingDraftAddsBlankLineBetweenSections(t *testing.T) {
	title := "原題"
	translated := "翻訳題"
	summary := "要約本文です。"

	draft := BuildAudioBriefingDraft(
		time.Date(2026, 3, 24, 6, 0, 0, 0, timeutil.JST),
		"editor",
		[]model.AudioBriefingJobItem{{
			ItemID:          "item-1",
			Rank:            1,
			Title:           &title,
			TranslatedTitle: &translated,
			SummarySnapshot: &summary,
		}},
		nil,
		0,
	)

	if !strings.HasSuffix(draft.Chunks[0].Text, "\n\n") {
		t.Fatalf("opening should end with section break: %q", draft.Chunks[0].Text)
	}
	if !strings.HasSuffix(draft.Chunks[1].Text, "\n\n") {
		t.Fatalf("summary should end with section break: %q", draft.Chunks[1].Text)
	}
	if !strings.HasSuffix(draft.Chunks[2].Text, "\n\n") {
		t.Fatalf("article should end with section break: %q", draft.Chunks[2].Text)
	}
	if strings.HasSuffix(draft.Chunks[3].Text, "\n\n") {
		t.Fatalf("ending should not end with section break: %q", draft.Chunks[3].Text)
	}
}

func TestBuildAudioBriefingDraftUsesClosingEndingFallback(t *testing.T) {
	title := "原題"
	translated := "翻訳題"
	summary := "要約本文です。ここでは大事なポイントをまとめます。"

	draft := BuildAudioBriefingDraft(
		time.Date(2026, 3, 24, 6, 0, 0, 0, timeutil.JST),
		"editor",
		[]model.AudioBriefingJobItem{{
			ItemID:          "item-1",
			Rank:            1,
			Title:           &title,
			TranslatedTitle: &translated,
			SummarySnapshot: &summary,
		}},
		nil,
		0,
	)

	got := draft.Chunks[len(draft.Chunks)-1].Text
	if strings.Contains(got, "気になった記事") {
		t.Fatalf("ending should not summarize articles: %q", got)
	}
	if !strings.Contains(got, "ありがとうございました") {
		t.Fatalf("ending should close the program: %q", got)
	}
}

func TestAudioBriefingTargetCharsUsesSevenHundredCharsPerMinute(t *testing.T) {
	if got := AudioBriefingTargetChars(20); got != 14000 {
		t.Fatalf("AudioBriefingTargetChars(20) = %d, want 14000", got)
	}
}

func TestAudioBriefingSectionBudgetsFavorLongerFrameSections(t *testing.T) {
	if got := audioBriefingOpeningBudget(14000); got < 2000 {
		t.Fatalf("opening budget = %d, want >= 2000", got)
	}
	if got := audioBriefingSummaryBudget(14000); got < 3900 {
		t.Fatalf("summary budget = %d, want >= 3900", got)
	}
	if got := audioBriefingEndingBudget(14000); got < 1800 {
		t.Fatalf("ending budget = %d, want >= 1800", got)
	}
	if got := audioBriefingCommentaryBudget(14000, 5); got < 1200 {
		t.Fatalf("commentary budget = %d, want >= 1200", got)
	}
}

func TestBuildAudioBriefingDraftFromNarrationTrimsToTargetBudgets(t *testing.T) {
	title := "原題"
	translated := "翻訳題"
	summary := "要約本文です。"
	longSentence := "これはかなり長めの説明文です。"
	longOpening := strings.Repeat(longSentence, 20)
	longSummary := strings.Repeat(longSentence, 24)
	longCommentary := strings.Repeat(longSentence, 18)
	longEnding := strings.Repeat(longSentence, 10)

	draft := BuildAudioBriefingDraftFromNarration(
		time.Date(2026, 3, 24, 6, 0, 0, 0, timeutil.JST),
		"editor",
		[]model.AudioBriefingJobItem{{
			ItemID:          "item-1",
			Rank:            1,
			Title:           &title,
			TranslatedTitle: &translated,
			SummarySnapshot: &summary,
		}},
		nil,
		AudioBriefingNarration{
			Opening:        longOpening,
			OverallSummary: longSummary,
			Articles: map[string]AudioBriefingNarrationArticle{
				"item-1": {Headline: "見出し", Commentary: longCommentary},
			},
			Ending: longEnding,
		},
		1200,
	)

	if got := draft.Chunks[0].CharCount; got > audioBriefingOpeningBudget(1200) {
		t.Fatalf("opening char count = %d, want <= %d", got, audioBriefingOpeningBudget(1200))
	}
	if got := draft.Chunks[1].CharCount; got > audioBriefingSummaryBudget(1200) {
		t.Fatalf("summary char count = %d, want <= %d", got, audioBriefingSummaryBudget(1200))
	}
	if got := draft.Chunks[2].CharCount; got > charCount("見出しです。")+audioBriefingCommentaryBudget(1200, 1)+charCount("\n\n") {
		t.Fatalf("article char count = %d, want commentary to be trimmed", got)
	}
	if got := draft.Chunks[3].CharCount; got > audioBriefingEndingBudget(1200) {
		t.Fatalf("ending char count = %d, want <= %d", got, audioBriefingEndingBudget(1200))
	}
}

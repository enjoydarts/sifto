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
	if got := draft.Chunks[0].Text; got != "編集長 水城です。今朝の流れを素早く見ていきましょう。" {
		t.Fatalf("opening = %q", got)
	}
	if got := draft.Chunks[1].PartType; got != "summary" {
		t.Fatalf("summary part type = %q", got)
	}
	if got := draft.Chunks[1].Text; got != "まず全体として、AIとプロダクトの境目がまた一段近づいています。" {
		t.Fatalf("summary = %q", got)
	}
	if got := draft.Chunks[2].Text; got != "LLMで見た翻訳題です。 ここは背景と含意を押さえておく価値があります。" {
		t.Fatalf("article = %q", got)
	}
	if got := draft.Chunks[3].Text; got != "続きはSiftoで確認してください。" {
		t.Fatalf("ending = %q", got)
	}
}

func TestAudioBriefingArticleTextKeepsHeadlineSentenceEnding(t *testing.T) {
	got := audioBriefingArticleText("これは競争環境が変わる記事です。", "ここは温度感が出ます。")
	want := "これは競争環境が変わる記事です。 ここは温度感が出ます。"
	if got != want {
		t.Fatalf("audioBriefingArticleText(...) = %q, want %q", got, want)
	}
}

func TestBuildAudioBriefingDraftDoesNotAddBlankLineBetweenSections(t *testing.T) {
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

	if strings.Contains(draft.Chunks[0].Text, "\n\n") {
		t.Fatalf("opening should not include section break: %q", draft.Chunks[0].Text)
	}
	if strings.Contains(draft.Chunks[1].Text, "\n\n") {
		t.Fatalf("summary should not include section break: %q", draft.Chunks[1].Text)
	}
	if strings.Contains(draft.Chunks[2].Text, "\n\n") {
		t.Fatalf("article should not include section break: %q", draft.Chunks[2].Text)
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

func TestAudioBriefingTargetCharsUsesSixHundredFiftyCharsPerMinute(t *testing.T) {
	want := 20 * audioBriefingCharsPerMinute
	if got := AudioBriefingTargetChars(20); got != want {
		t.Fatalf("AudioBriefingTargetChars(20) = %d, want %d", got, want)
	}
}

func TestAudioBriefingSectionBudgetsFavorLongerFrameSections(t *testing.T) {
	if got := audioBriefingOpeningBudget(14000); got != 700 {
		t.Fatalf("opening budget = %d, want %d", got, 700)
	}
	if got := audioBriefingSummaryBudget(14000); got != 980 {
		t.Fatalf("summary budget = %d, want %d", got, 980)
	}
	if got := audioBriefingEndingBudget(14000); got != 700 {
		t.Fatalf("ending budget = %d, want %d", got, 700)
	}
	if got := audioBriefingCommentaryBudget(14000, 5); got != 2304 {
		t.Fatalf("commentary budget = %d, want %d", got, 2304)
	}
}

func TestBuildAudioBriefingDraftFromNarrationPreservesGeneratedSectionText(t *testing.T) {
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

	if got := draft.Chunks[0].Text; got != longOpening {
		t.Fatalf("opening text was trimmed: %q", got)
	}
	if got := draft.Chunks[1].Text; got != longSummary {
		t.Fatalf("summary text was trimmed: %q", got)
	}
	if got := draft.Chunks[2].CharCount; got <= charCount("見出しです。") {
		t.Fatalf("article char count = %d, want article text to include commentary", got)
	}
	if got := draft.Chunks[3].Text; got != longEnding {
		t.Fatalf("ending text was trimmed: %q", got)
	}
}

func TestBuildAudioBriefingDraftFromNarrationDoesNotSplitSingleArticleIntoMultipleChunks(t *testing.T) {
	title := "原題"
	translated := "翻訳題"
	summary := "要約本文です。"
	longSentence := "これはかなり長めの説明文です。"
	longCommentary := strings.Repeat(longSentence, 120)

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
			Opening:        "導入です。",
			OverallSummary: "総括です。",
			Articles: map[string]AudioBriefingNarrationArticle{
				"item-1": {Headline: "見出し", Commentary: longCommentary},
			},
			Ending: "締めです。",
		},
		1200,
	)

	articleChunks := 0
	for _, chunk := range draft.Chunks {
		if chunk.PartType == "article" {
			articleChunks++
		}
	}
	if articleChunks != 1 {
		t.Fatalf("article chunk count = %d, want 1", articleChunks)
	}
}

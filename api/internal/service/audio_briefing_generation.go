package service

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
)

type AudioBriefingDraft struct {
	Title           string
	Status          string
	ScriptCharCount int
	ScriptLLMModels []string
	Items           []model.AudioBriefingJobItem
	Chunks          []model.AudioBriefingScriptChunk
}

type AudioBriefingNarration struct {
	Opening        string
	OverallSummary string
	Articles       map[string]AudioBriefingNarrationArticle
	Ending         string
}

type AudioBriefingNarrationArticle struct {
	Headline   string
	Commentary string
}

const audioBriefingCharsPerMinute = 650

func BuildAudioBriefingDraft(
	slotStartedAt time.Time,
	persona string,
	items []model.AudioBriefingJobItem,
	voice *model.AudioBriefingPersonaVoice,
	targetChars int,
) AudioBriefingDraft {
	slotStartedAt = slotStartedAt.In(timeutil.JST)
	if len(items) == 0 {
		title := fmt.Sprintf("%02d:%02d便の音声ブリーフィング", slotStartedAt.Hour(), slotStartedAt.Minute())
		return AudioBriefingDraft{
			Title:  title,
			Status: "skipped",
		}
	}

	slotLabel := fmt.Sprintf("%02d:%02d便", slotStartedAt.Hour(), slotStartedAt.Minute())
	title := fmt.Sprintf("%sの音声ブリーフィング", slotLabel)
	chunks := make([]model.AudioBriefingScriptChunk, 0, len(items)+3)
	ttsProvider, voiceModel, voiceStyle := audioBriefingVoiceRefs(voice)
	commentaryBudget := audioBriefingCommentaryBudget(targetChars, len(items))

	opening := fmt.Sprintf("%sです。%sのSifto音声ブリーフィングをお届けします。今回は直近の注目記事を%d本まとめて見ていきます。", audioBriefingSpeakerName(persona), slotLabel, len(items))
	for _, part := range audioBriefingSectionParts(opening, 1200, true) {
		chunks = append(chunks, newAudioBriefingChunk(len(chunks)+1, "opening", part, ttsProvider, voiceModel, voiceStyle))
	}

	summary := fmt.Sprintf("今日は%sを中心に、重要度と新しさを優先してピックアップしました。少し重複を含むことがありますが、流れがつかみやすい順で並べています。", audioBriefingSourceDigest(items))
	for _, part := range audioBriefingSectionParts(summary, 1200, true) {
		chunks = append(chunks, newAudioBriefingChunk(len(chunks)+1, "summary", part, ttsProvider, voiceModel, voiceStyle))
	}

	totalChars := 0
	for _, chunk := range chunks {
		totalChars += chunk.CharCount
	}
	for _, item := range items {
		headline := strings.TrimSpace(coalesceString(item.TranslatedTitle, item.Title))
		if headline == "" {
			headline = fmt.Sprintf("記事%d", item.Rank)
		}
		body := strings.TrimSpace(coalesceString(item.SummarySnapshot, item.SegmentTitle))
		body = fitAudioBriefingTextToChars(body, commentaryBudget)
		text := audioBriefingArticleText(headline, body)
		for _, part := range audioBriefingSectionParts(text, 1200, true) {
			chunks = append(chunks, newAudioBriefingChunk(len(chunks)+1, "article", part, ttsProvider, voiceModel, voiceStyle))
			totalChars += charCount(part)
		}
	}

	ending := "この時間はここまでです。最後まで聞いていただき、ありがとうございました。"
	chunks = append(chunks, newAudioBriefingChunk(len(chunks)+1, "ending", ending, ttsProvider, voiceModel, voiceStyle))
	totalChars += charCount(ending)

	return AudioBriefingDraft{
		Title:           title,
		Status:          "scripted",
		ScriptCharCount: totalChars,
		Items:           items,
		Chunks:          chunks,
	}
}

func BuildAudioBriefingDraftFromNarration(
	slotStartedAt time.Time,
	persona string,
	items []model.AudioBriefingJobItem,
	voice *model.AudioBriefingPersonaVoice,
	narration AudioBriefingNarration,
	targetChars int,
) AudioBriefingDraft {
	slotStartedAt = slotStartedAt.In(timeutil.JST)
	if len(items) == 0 {
		title := fmt.Sprintf("%02d:%02d便の音声ブリーフィング", slotStartedAt.Hour(), slotStartedAt.Minute())
		return AudioBriefingDraft{
			Title:  title,
			Status: "skipped",
		}
	}

	slotLabel := fmt.Sprintf("%02d:%02d便", slotStartedAt.Hour(), slotStartedAt.Minute())
	title := fmt.Sprintf("%sの音声ブリーフィング", slotLabel)
	chunks := make([]model.AudioBriefingScriptChunk, 0, len(items)+3)
	ttsProvider, voiceModel, voiceStyle := audioBriefingVoiceRefs(voice)
	commentaryBudget := audioBriefingCommentaryBudget(targetChars, len(items))
	openingBudget := audioBriefingOpeningBudget(targetChars)
	summaryBudget := audioBriefingSummaryBudget(targetChars)
	endingBudget := audioBriefingEndingBudget(targetChars)

	opening := strings.TrimSpace(narration.Opening)
	if opening == "" {
		opening = fmt.Sprintf("%sです。%sのSifto音声ブリーフィングをお届けします。今回は直近の注目記事を%d本まとめて見ていきます。", audioBriefingSpeakerName(persona), slotLabel, len(items))
	}
	opening = fitAudioBriefingTextToChars(opening, openingBudget)
	for _, part := range audioBriefingSectionParts(opening, 1200, true) {
		chunks = append(chunks, newAudioBriefingChunk(len(chunks)+1, "opening", part, ttsProvider, voiceModel, voiceStyle))
	}

	totalChars := 0
	for _, chunk := range chunks {
		totalChars += chunk.CharCount
	}

	overallSummary := strings.TrimSpace(narration.OverallSummary)
	if overallSummary != "" {
		overallSummary = fitAudioBriefingTextToChars(overallSummary, summaryBudget)
		for _, part := range audioBriefingSectionParts(overallSummary, 1200, true) {
			chunks = append(chunks, newAudioBriefingChunk(len(chunks)+1, "summary", part, ttsProvider, voiceModel, voiceStyle))
			totalChars += charCount(part)
		}
	}

	for _, item := range items {
		headline := strings.TrimSpace(coalesceString(item.TranslatedTitle, item.Title))
		if headline == "" {
			headline = fmt.Sprintf("記事%d", item.Rank)
		}
		commentary := strings.TrimSpace(coalesceString(item.SummarySnapshot, item.SegmentTitle))
		if article, ok := narration.Articles[item.ItemID]; ok {
			if candidate := strings.TrimSpace(article.Headline); candidate != "" {
				headline = candidate
			}
			if candidate := strings.TrimSpace(article.Commentary); candidate != "" {
				commentary = candidate
			}
		}
		commentary = fitAudioBriefingTextToChars(commentary, commentaryBudget)
		text := audioBriefingArticleText(headline, commentary)
		for _, part := range audioBriefingSectionParts(text, 1200, true) {
			chunks = append(chunks, newAudioBriefingChunk(len(chunks)+1, "article", part, ttsProvider, voiceModel, voiceStyle))
			totalChars += charCount(part)
		}
	}

	ending := strings.TrimSpace(narration.Ending)
	if ending == "" {
		ending = "この時間はここまでです。最後まで聞いていただき、ありがとうございました。"
	}
	ending = fitAudioBriefingTextToChars(ending, endingBudget)
	chunks = append(chunks, newAudioBriefingChunk(len(chunks)+1, "ending", ending, ttsProvider, voiceModel, voiceStyle))
	totalChars += charCount(ending)

	return AudioBriefingDraft{
		Title:           title,
		Status:          "scripted",
		ScriptCharCount: totalChars,
		Items:           items,
		Chunks:          chunks,
	}
}

func splitAudioBriefingText(text string, maxChars int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if charCount(text) <= maxChars {
		return []string{text}
	}

	parts := make([]string, 0, 4)
	remaining := text
	for charCount(remaining) > maxChars {
		cut := maxChars
		runes := []rune(remaining)
		if len(runes) < cut {
			cut = len(runes)
		}
		for i := cut; i > maxChars/2; i-- {
			switch runes[i-1] {
			case '。', '、', '！', '？':
				cut = i
				i = 0
			}
		}
		parts = append(parts, strings.TrimSpace(string(runes[:cut])))
		remaining = strings.TrimSpace(string(runes[cut:]))
	}
	if remaining != "" {
		parts = append(parts, remaining)
	}
	return parts
}

func audioBriefingSectionParts(text string, maxChars int, addTrailingBreak bool) []string {
	parts := splitAudioBriefingText(text, maxChars)
	return parts
}

func newAudioBriefingChunk(seq int, partType, text string, ttsProvider, voiceModel, voiceStyle *string) model.AudioBriefingScriptChunk {
	return model.AudioBriefingScriptChunk{
		Seq:         seq,
		PartType:    partType,
		Text:        text,
		CharCount:   charCount(text),
		TTSStatus:   "pending",
		TTSProvider: ttsProvider,
		VoiceModel:  voiceModel,
		VoiceStyle:  voiceStyle,
	}
}

func audioBriefingSpeakerName(persona string) string {
	switch strings.TrimSpace(persona) {
	case "snark":
		return "ナビゲーター"
	case "analyst":
		return "アナリスト"
	case "concierge":
		return "コンシェルジュ"
	case "hype":
		return "ホスト"
	case "native":
		return "ナビゲーター"
	default:
		return "エディター"
	}
}

func audioBriefingSourceDigest(items []model.AudioBriefingJobItem) string {
	sourceSet := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		source := strings.TrimSpace(coalesceString(item.SourceTitle, nil))
		if source == "" {
			continue
		}
		if _, ok := seen[source]; ok {
			continue
		}
		seen[source] = struct{}{}
		sourceSet = append(sourceSet, source)
		if len(sourceSet) == 3 {
			break
		}
	}
	if len(sourceSet) == 0 {
		return "複数ソース"
	}
	return strings.Join(sourceSet, "、")
}

func audioBriefingVoiceRefs(voice *model.AudioBriefingPersonaVoice) (*string, *string, *string) {
	if voice == nil {
		return nil, nil, nil
	}
	return &voice.TTSProvider, &voice.VoiceModel, &voice.VoiceStyle
}

func charCount(v string) int {
	return utf8.RuneCountInString(v)
}

func audioBriefingArticleText(headline, commentary string) string {
	headline = strings.TrimSpace(headline)
	commentary = strings.TrimSpace(commentary)
	switch {
	case headline == "":
		return commentary
	case commentary == "":
		return headline
	default:
		return fmt.Sprintf("%sです。%s", headline, commentary)
	}
}

func AudioBriefingTargetChars(targetDurationMinutes int) int {
	if targetDurationMinutes <= 0 {
		targetDurationMinutes = 20
	}
	return targetDurationMinutes * audioBriefingCharsPerMinute
}

func audioBriefingCommentaryBudget(targetChars, itemCount int) int {
	if targetChars <= 0 || itemCount <= 0 {
		return 0
	}
	usable := targetChars - audioBriefingOpeningBudget(targetChars) - audioBriefingSummaryBudget(targetChars) - audioBriefingEndingBudget(targetChars) - 100
	minimum := 360
	if usable <= 0 {
		return minimum
	}
	perArticle := usable / itemCount
	if perArticle < minimum {
		return minimum
	}
	return perArticle
}

func audioBriefingOpeningBudget(targetChars int) int {
	if targetChars <= 0 {
		return 840
	}
	budget := int(math.Round(float64(targetChars) * 0.15))
	if budget < 420 {
		return 420
	}
	if budget > 2200 {
		return 2200
	}
	return budget
}

func audioBriefingSummaryBudget(targetChars int) int {
	if targetChars <= 0 {
		return 2400
	}
	budget := int(math.Round(float64(targetChars) * 0.28))
	if budget < 1500 {
		return 1500
	}
	if budget > 4600 {
		return 4600
	}
	return budget
}

func audioBriefingEndingBudget(targetChars int) int {
	if targetChars <= 0 {
		return 840
	}
	budget := int(math.Round(float64(targetChars) * 0.13))
	if budget < 380 {
		return 380
	}
	if budget > 1800 {
		return 1800
	}
	return budget
}

func fitAudioBriefingTextToChars(text string, maxChars int) string {
	text = strings.TrimSpace(text)
	if text == "" || maxChars <= 0 || charCount(text) <= maxChars {
		return text
	}
	runes := []rune(text)
	if len(runes) <= maxChars {
		return text
	}
	cut := maxChars
	for i := maxChars; i > maxChars/2; i-- {
		switch runes[i-1] {
		case '。', '！', '？':
			cut = i
			i = 0
		}
	}
	return strings.TrimSpace(string(runes[:cut]))
}

func coalesceString(values ...*string) string {
	for _, value := range values {
		if value == nil {
			continue
		}
		if trimmed := strings.TrimSpace(*value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

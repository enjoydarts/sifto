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
	Headline     string
	SummaryIntro string
	Commentary   string
}

type audioBriefingSpeakerVoices struct {
	Host    *model.AudioBriefingPersonaVoice
	Partner *model.AudioBriefingPersonaVoice
}

const audioBriefingCharsPerMinute = 400

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
	opening := fmt.Sprintf("%sです。%sのSifto音声ブリーフィングをお届けします。今回は直近の注目記事を%d本まとめて見ていきます。", audioBriefingSpeakerName(persona), slotLabel, len(items))
	for _, part := range audioBriefingSectionParts(opening, 1200, true) {
		chunks = append(chunks, newAudioBriefingChunk(len(chunks)+1, "opening", nil, part, ttsProvider, voiceModel, voiceStyle))
	}

	summary := fmt.Sprintf("今日は%sを中心に、重要度と新しさを優先してピックアップしました。少し重複を含むことがありますが、流れがつかみやすい順で並べています。", audioBriefingSourceDigest(items))
	for _, part := range audioBriefingSectionParts(summary, 1200, true) {
		chunks = append(chunks, newAudioBriefingChunk(len(chunks)+1, "summary", nil, part, ttsProvider, voiceModel, voiceStyle))
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
		text := audioBriefingArticleText(headline, "", "")
		chunks = append(chunks, newAudioBriefingChunk(len(chunks)+1, "article", nil, text, ttsProvider, voiceModel, voiceStyle))
		totalChars += charCount(text)
	}

	ending := "この時間はここまでです。最後まで聞いていただき、ありがとうございました。"
	chunks = append(chunks, newAudioBriefingChunk(len(chunks)+1, "ending", nil, ending, ttsProvider, voiceModel, voiceStyle))
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

	opening := strings.TrimSpace(narration.Opening)
	if opening == "" {
		opening = fmt.Sprintf("%sです。%sのSifto音声ブリーフィングをお届けします。今回は直近の注目記事を%d本まとめて見ていきます。", audioBriefingSpeakerName(persona), slotLabel, len(items))
	}
	for _, part := range audioBriefingSectionParts(opening, 1200, true) {
		chunks = append(chunks, newAudioBriefingChunk(len(chunks)+1, "opening", nil, part, ttsProvider, voiceModel, voiceStyle))
	}

	totalChars := 0
	for _, chunk := range chunks {
		totalChars += chunk.CharCount
	}

	overallSummary := strings.TrimSpace(narration.OverallSummary)
	if overallSummary != "" {
		for _, part := range audioBriefingSectionParts(overallSummary, 1200, true) {
			chunks = append(chunks, newAudioBriefingChunk(len(chunks)+1, "summary", nil, part, ttsProvider, voiceModel, voiceStyle))
			totalChars += charCount(part)
		}
	}

	for _, item := range items {
		headline := strings.TrimSpace(coalesceString(item.TranslatedTitle, item.Title))
		if headline == "" {
			headline = fmt.Sprintf("記事%d", item.Rank)
		}
		commentary := ""
		summaryIntro := ""
		text := ""
		if article, ok := narration.Articles[item.ItemID]; ok {
			if candidate := strings.TrimSpace(article.Headline); candidate != "" {
				headline = candidate
			}
			if candidate := strings.TrimSpace(article.SummaryIntro); candidate != "" {
				summaryIntro = candidate
			}
			if candidate := strings.TrimSpace(article.Commentary); candidate != "" {
				commentary = candidate
			}
			text = audioBriefingArticleText(headline, summaryIntro, commentary)
		} else {
			text = audioBriefingArticleText(headline, "", commentary)
		}
		chunks = append(chunks, newAudioBriefingChunk(len(chunks)+1, "article", nil, text, ttsProvider, voiceModel, voiceStyle))
		totalChars += charCount(text)
	}

	ending := strings.TrimSpace(narration.Ending)
	if ending == "" {
		ending = "この時間はここまでです。最後まで聞いていただき、ありがとうございました。"
	}
	chunks = append(chunks, newAudioBriefingChunk(len(chunks)+1, "ending", nil, ending, ttsProvider, voiceModel, voiceStyle))
	totalChars += charCount(ending)

	return AudioBriefingDraft{
		Title:           title,
		Status:          "scripted",
		ScriptCharCount: totalChars,
		Items:           items,
		Chunks:          chunks,
	}
}

func BuildAudioBriefingDraftFromTurns(
	slotStartedAt time.Time,
	hostPersona string,
	partnerPersona string,
	items []model.AudioBriefingJobItem,
	hostVoice *model.AudioBriefingPersonaVoice,
	partnerVoice *model.AudioBriefingPersonaVoice,
	turns []AudioBriefingScriptTurn,
	targetChars int,
) AudioBriefingDraft {
	slotStartedAt = slotStartedAt.In(timeutil.JST)
	_ = partnerPersona
	if len(items) == 0 {
		title := fmt.Sprintf("%02d:%02d便の音声ブリーフィング", slotStartedAt.Hour(), slotStartedAt.Minute())
		return AudioBriefingDraft{
			Title:  title,
			Status: "skipped",
		}
	}

	slotLabel := fmt.Sprintf("%02d:%02d便", slotStartedAt.Hour(), slotStartedAt.Minute())
	title := fmt.Sprintf("%sの音声ブリーフィング", slotLabel)
	if len(turns) == 0 {
		return BuildAudioBriefingDraft(slotStartedAt, hostPersona, items, hostVoice, targetChars)
	}

	voices := audioBriefingSpeakerVoices{Host: hostVoice, Partner: partnerVoice}
	chunks := make([]model.AudioBriefingScriptChunk, 0, len(turns))
	totalChars := 0
	for _, turn := range turns {
		text := strings.TrimSpace(turn.Text)
		if text == "" {
			continue
		}
		partType := audioBriefingTurnPartType(turn.Section)
		speaker := audioBriefingSpeakerPtr(turn.Speaker)
		ttsProvider, voiceModel, voiceStyle := audioBriefingVoiceRefsForSpeaker(voices, turn.Speaker)
		chunks = append(chunks, newAudioBriefingChunk(len(chunks)+1, partType, speaker, text, ttsProvider, voiceModel, voiceStyle))
		totalChars += charCount(text)
	}
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

func audioBriefingTurnPartType(section string) string {
	switch strings.TrimSpace(section) {
	case "opening":
		return "opening"
	case "overall_summary":
		return "summary"
	case "ending":
		return "ending"
	default:
		return "article"
	}
}

func newAudioBriefingChunk(seq int, partType string, speaker *string, text string, ttsProvider, voiceModel, voiceStyle *string) model.AudioBriefingScriptChunk {
	return model.AudioBriefingScriptChunk{
		Seq:         seq,
		PartType:    partType,
		Speaker:     speaker,
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
	case "junior":
		return "ナビゲーター"
	case "urban":
		return "ナビゲーター"
	default:
		return "エディター"
	}
}

func audioBriefingVoiceRefsForSpeaker(voices audioBriefingSpeakerVoices, speaker string) (*string, *string, *string) {
	switch strings.TrimSpace(speaker) {
	case "partner":
		return audioBriefingVoiceRefs(voices.Partner)
	default:
		return audioBriefingVoiceRefs(voices.Host)
	}
}

func audioBriefingSpeakerPtr(speaker string) *string {
	switch strings.TrimSpace(speaker) {
	case "host", "partner":
		value := strings.TrimSpace(speaker)
		return &value
	default:
		return nil
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

func audioBriefingArticleText(headline, summaryIntro, commentary string) string {
	headline = strings.TrimSpace(headline)
	summaryIntro = strings.TrimSpace(summaryIntro)
	commentary = strings.TrimSpace(commentary)
	parts := make([]string, 0, 3)
	if headline != "" {
		if strings.HasSuffix(headline, "。") || strings.HasSuffix(headline, "！") || strings.HasSuffix(headline, "？") {
			parts = append(parts, headline)
		} else {
			parts = append(parts, fmt.Sprintf("%sです。", headline))
		}
	}
	if summaryIntro != "" {
		parts = append(parts, summaryIntro)
	}
	if commentary != "" {
		parts = append(parts, commentary)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
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
	if usable <= 0 {
		return 0
	}
	perArticle := usable / itemCount
	if perArticle < 0 {
		return 0
	}
	return perArticle
}

func audioBriefingSummaryIntroBudget(commentaryBudget int) int {
	if commentaryBudget <= 0 {
		return 0
	}
	budget := commentaryBudget / 3
	if budget > 520 {
		return 520
	}
	return budget
}

func audioBriefingOpeningBudget(targetChars int) int {
	if targetChars <= 0 {
		return 300
	}
	budget := int(math.Round(float64(targetChars) * 0.05))
	if budget < 180 {
		return 180
	}
	if budget > 1000 {
		return 1000
	}
	return budget
}

func audioBriefingSummaryBudget(targetChars int) int {
	if targetChars <= 0 {
		return 560
	}
	budget := int(math.Round(float64(targetChars) * 0.07))
	if budget < 300 {
		return 300
	}
	if budget > 1600 {
		return 1600
	}
	return budget
}

func audioBriefingEndingBudget(targetChars int) int {
	if targetChars <= 0 {
		return 300
	}
	budget := int(math.Round(float64(targetChars) * 0.05))
	if budget < 180 {
		return 180
	}
	if budget > 1000 {
		return 1000
	}
	return budget
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

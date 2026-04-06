package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type PromptTemplateDefault struct {
	Label              string          `json:"label"`
	SystemInstruction  string          `json:"system_instruction"`
	PromptText         string          `json:"prompt_text"`
	FallbackPromptText string          `json:"fallback_prompt_text"`
	VariablesSchema    json.RawMessage `json:"variables_schema,omitempty"`
	PreviewVariables   json.RawMessage `json:"preview_variables,omitempty"`
	Notes              string          `json:"notes"`
}

func LookupPromptTemplateDefault(promptKey string) (PromptTemplateDefault, error) {
	systemInstruction, err := readPromptTemplate(promptKey, "system")
	if err != nil {
		return PromptTemplateDefault{}, err
	}
	promptText, err := readPromptTemplate(promptKey, "prompt")
	if err != nil {
		return PromptTemplateDefault{}, err
	}
	return PromptTemplateDefault{
		SystemInstruction: systemInstruction,
		PromptText:        promptText,
		VariablesSchema:   defaultPromptVariablesSchema(promptKey),
		PreviewVariables:  defaultPromptPreviewVariables(promptKey),
		Notes:             defaultPromptNotes(promptKey),
	}, nil
}

func readPromptTemplate(promptKey string, part string) (string, error) {
	baseDir, err := resolvePromptTemplatesDir()
	if err != nil {
		return "", fmt.Errorf("resolve prompt templates dir: %w", err)
	}
	raw, err := os.ReadFile(filepath.Join(baseDir, promptKey+"."+part+".txt"))
	if err != nil {
		return "", fmt.Errorf("read prompt template %s/%s: %w", promptKey, part, err)
	}
	return string(raw), nil
}

func resolvePromptTemplatesDir() (string, error) {
	candidates := []string{
		"/app/shared/prompt_templates",
		"/shared/prompt_templates",
		filepath.Join("shared", "prompt_templates"),
		filepath.Join("..", "shared", "prompt_templates"),
		filepath.Join("..", "..", "shared", "prompt_templates"),
		filepath.Join("..", "..", "..", "shared", "prompt_templates"),
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
	}
	return "", os.ErrNotExist
}

func defaultPromptVariablesSchema(promptKey string) json.RawMessage {
	switch promptKey {
	case "summary.default":
		return mustRawJSON(`{
  "title": {"type": "string"},
  "facts_text": {"type": "string"},
  "target_chars": {"type": "integer"},
  "min_chars": {"type": "integer"},
  "max_chars": {"type": "integer"}
}`)
	case "facts.default":
		return mustRawJSON(`{
  "title": {"type": "string"},
  "content": {"type": "string"},
  "fact_range": {"type": "string"},
  "output_rule": {"type": "string"}
}`)
	case "digest.default":
		return mustRawJSON(`{
  "digest_date": {"type": "string"},
  "items_count": {"type": "integer"},
  "input_mode": {"type": "string"},
  "digest_input": {"type": "string"}
}`)
	case "audio_briefing_script.single":
		return mustRawJSON(`{
  "persona_key": {"type": "string"},
  "display_name": {"type": "string"},
  "gender": {"type": "string"},
  "age_vibe": {"type": "string"},
  "first_person": {"type": "string"},
  "speech_style": {"type": "string"},
  "occupation": {"type": "string"},
  "experience": {"type": "string"},
  "personality": {"type": "string"},
  "values": {"type": "string"},
  "interests": {"type": "string"},
  "dislikes": {"type": "string"},
  "voice": {"type": "string"},
  "intro_style": {"type": "string"},
  "comment_range": {"type": "string"},
  "item_style": {"type": "string"},
  "include_opening": {"type": "string"},
  "include_overall_summary": {"type": "string"},
  "include_article_segments": {"type": "string"},
  "include_ending": {"type": "string"},
  "target_duration_minutes": {"type": "integer"},
  "chars_per_minute": {"type": "integer"},
  "target_chars": {"type": "integer"},
  "target_chars_min": {"type": "integer"},
  "target_chars_max": {"type": "integer"},
  "sentence_length_spec": {"type": "string"},
  "opening_sentence_range": {"type": "string"},
  "opening_char_range": {"type": "string"},
  "summary_sentence_range": {"type": "string"},
  "summary_char_range": {"type": "string"},
  "ending_sentence_range": {"type": "string"},
  "ending_char_range": {"type": "string"},
  "article_char_range": {"type": "string"},
  "headline_sentence_range": {"type": "string"},
  "headline_char_range": {"type": "string"},
  "summary_intro_sentence_range": {"type": "string"},
  "summary_intro_char_range": {"type": "string"},
  "commentary_sentence_range": {"type": "string"},
  "commentary_char_range": {"type": "string"},
  "generation_mode": {"type": "string"},
  "generation_section": {"type": "string"},
  "program_position": {"type": "string"},
  "program_name": {"type": "string"},
  "total_articles": {"type": "integer"},
  "article_batch_range": {"type": "string"},
  "supplement_note": {"type": "string"},
  "now_jst": {"type": "string"},
  "date_jst": {"type": "string"},
  "weekday_jst": {"type": "string"},
  "time_of_day": {"type": "string"},
  "season_hint": {"type": "string"},
  "response_example": {"type": "string"},
  "articles_json": {"type": "string"},
  "existing_context": {"type": "string"}
}`)
	case "audio_briefing_script.duo":
		return mustRawJSON(`{
  "host_persona_key": {"type": "string"},
  "host_display_name": {"type": "string"},
  "host_first_person": {"type": "string"},
  "host_speech_style": {"type": "string"},
  "host_personality": {"type": "string"},
  "host_values": {"type": "string"},
  "host_voice": {"type": "string"},
  "partner_persona_key": {"type": "string"},
  "partner_display_name": {"type": "string"},
  "partner_first_person": {"type": "string"},
  "partner_speech_style": {"type": "string"},
  "partner_personality": {"type": "string"},
  "partner_values": {"type": "string"},
  "partner_voice": {"type": "string"},
  "include_opening": {"type": "string"},
  "include_overall_summary": {"type": "string"},
  "include_article_segments": {"type": "string"},
  "include_ending": {"type": "string"},
  "active_section": {"type": "string"},
  "allowed_sections": {"type": "string"},
  "target_duration_minutes": {"type": "integer"},
  "chars_per_minute": {"type": "integer"},
  "target_chars": {"type": "integer"},
  "target_chars_min": {"type": "integer"},
  "target_chars_max": {"type": "integer"},
  "article_char_range": {"type": "string"},
  "duo_article_turn_phrase": {"type": "string"},
  "duo_article_turn_sequence": {"type": "string"},
  "duo_article_turn_flow": {"type": "string"},
  "generation_mode": {"type": "string"},
  "generation_section": {"type": "string"},
  "program_position": {"type": "string"},
  "program_name": {"type": "string"},
  "total_articles": {"type": "integer"},
  "article_batch_range": {"type": "string"},
  "call_scope_note": {"type": "string"},
  "supplement_note": {"type": "string"},
  "now_jst": {"type": "string"},
  "date_jst": {"type": "string"},
  "weekday_jst": {"type": "string"},
  "time_of_day": {"type": "string"},
  "season_hint": {"type": "string"},
  "response_example": {"type": "string"},
  "articles_json": {"type": "string"},
  "existing_context": {"type": "string"}
}`)
	case "fish.summary_preprocess":
		return mustRawJSON(`{
  "text": {"type": "string"}
}`)
	default:
		return nil
	}
}

func defaultPromptNotes(promptKey string) string {
	switch promptKey {
	case "summary.default", "facts.default", "digest.default":
		return "現行コード既定と同じテンプレートを編集用にそのまま表示します。"
	case "audio_briefing_script.single", "audio_briefing_script.duo":
		return "現行コード既定と同等の完成promptを直接編集できます。固定ルールは本文に見える形で持ち、persona や記事データなどの runtime 変数だけを差し込みます。"
	case "fish.summary_preprocess":
		return "Fish Audio の読み上げ前テキストに自然言語タグを挿入する前処理 prompt です。現在は Summary Audio の Fish 経路でのみ使います。"
	default:
		return ""
	}
}

func defaultPromptPreviewVariables(promptKey string) json.RawMessage {
	switch promptKey {
	case "summary.default":
		return mustRawJSON(`{
  "title": "OpenAI updates model lineup",
  "facts_text": "- OpenAI announced a new lineup.\n- The company said the release targets enterprise use.\n- Pricing will change next month.",
  "target_chars": 420,
  "min_chars": 336,
  "max_chars": 504
}`)
	case "facts.default":
		return mustRawJSON(`{
  "title": "OpenAI updates model lineup",
  "content": "OpenAI announced a new lineup for enterprise customers. The company said pricing will change next month and positioned the update for practical deployment.",
  "fact_range": "8〜18個",
  "output_rule": "- 出力は必ず {\"facts\": [\"...\", \"...\"]} のJSONオブジェクト1つのみにしてください。"
}`)
	case "digest.default":
		return mustRawJSON(`{
  "digest_date": "2026-04-01",
  "items_count": 2,
  "input_mode": "items",
  "digest_input": "- item=1 rank=1 | title=OpenAI updates model lineup | topics=AI | score=0.9 | summary=Summary\n- item=2 rank=2 | title=Google announces change | topics=Search | score=0.8 | summary=Summary"
}`)
	case "audio_briefing_script.single":
		return mustRawJSON(`{
  "persona_key": "editor",
  "display_name": "エディター",
  "gender": "不問",
  "age_vibe": "30代後半",
  "first_person": "私",
  "speech_style": "落ち着いた会話調",
  "occupation": "編集者",
  "experience": "テックニュース編集を長く担当",
  "personality": "冷静で整理がうまい",
  "values": "正確さと文脈を重視",
  "interests": "AI、プロダクト、業界動向",
  "dislikes": "大げさな断定",
  "voice": "穏やかで知的",
  "intro_style": "すっと本題に入る",
  "comment_range": "中程度",
  "item_style": "ニュースレター編集者のように整理する",
  "include_opening": "true",
  "include_overall_summary": "true",
  "include_article_segments": "true",
  "include_ending": "true",
  "target_duration_minutes": 10,
  "chars_per_minute": 700,
  "target_chars": 4000,
  "target_chars_min": 3600,
  "target_chars_max": 4400,
  "sentence_length_spec": "40〜80文字",
  "opening_sentence_range": "4〜6文",
  "opening_char_range": "320〜520文字",
  "summary_sentence_range": "5〜8文",
  "summary_char_range": "520〜820文字",
  "ending_sentence_range": "4〜6文",
  "ending_char_range": "260〜440文字",
  "article_char_range": "800〜1200文字",
  "headline_sentence_range": "1〜2文",
  "headline_char_range": "70〜150文字",
  "summary_intro_sentence_range": "2〜4文",
  "summary_intro_char_range": "180〜340文字",
  "commentary_sentence_range": "4〜7文",
  "commentary_char_range": "420〜760文字",
  "generation_mode": "full",
  "generation_section": "all",
  "program_position": "full_episode",
  "program_name": "Morning Sifto",
  "total_articles": 1,
  "article_batch_range": "1 - 1",
  "supplement_note": "通常生成です。既存台本への追記ではありません。",
  "now_jst": "2026-04-01T19:30:00+09:00",
  "date_jst": "2026-04-01",
  "weekday_jst": "Wednesday",
  "time_of_day": "evening",
  "season_hint": "spring",
  "response_example": "{\n  \"opening\": \"導入\",\n  \"overall_summary\": \"全体サマリー\",\n  \"article_segments\": [],\n  \"ending\": \"締め\"\n}",
  "articles_json": "[{\"item_id\":\"item-1\",\"title\":\"Example title\",\"translated_title\":\"翻訳タイトル\",\"source_title\":\"Source\",\"summary\":\"Summary text\",\"published_at\":\"2026-04-01T08:00:00Z\"}]",
  "existing_context": "なし"
}`)
	case "audio_briefing_script.duo":
		return mustRawJSON(`{
  "host_persona_key": "editor",
  "host_display_name": "エディター",
  "host_first_person": "私",
  "host_speech_style": "落ち着いた会話調",
  "host_personality": "冷静で整理がうまい",
  "host_values": "正確さと文脈を重視",
  "host_voice": "穏やかで知的",
  "partner_persona_key": "analyst",
  "partner_display_name": "アナリスト",
  "partner_first_person": "僕",
  "partner_speech_style": "ややくだけた会話調",
  "partner_personality": "観察が細かい",
  "partner_values": "論点の背景を見る",
  "partner_voice": "軽快で知的",
  "include_opening": "false",
  "include_overall_summary": "false",
  "include_article_segments": "true",
  "include_ending": "false",
  "active_section": "article",
  "allowed_sections": "article",
  "target_duration_minutes": 10,
  "chars_per_minute": 700,
  "target_chars": 4000,
  "target_chars_min": 3600,
  "target_chars_max": 4400,
  "article_char_range": "700〜1000文字",
  "duo_article_turn_phrase": "5手",
  "duo_article_turn_sequence": "host -> partner -> host -> partner -> host",
  "duo_article_turn_flow": "host(setup) -> partner(reaction) -> host(deepen) -> partner(contrast) -> host(close)",
  "generation_mode": "full",
  "generation_section": "article_segments",
  "program_position": "middle",
  "program_name": "Morning Sifto",
  "total_articles": 2,
  "article_batch_range": "1 - 2",
  "call_scope_note": "番組途中の article パートだけを担当します。opening や ending に戻らず、各記事の会話だけを書いてください。",
  "supplement_note": "通常生成です。既存台本への追記ではありません。",
  "now_jst": "2026-04-01T19:30:00+09:00",
  "date_jst": "2026-04-01",
  "weekday_jst": "Wednesday",
  "time_of_day": "evening",
  "season_hint": "spring",
  "response_example": "{\n  \"turns\": [\n    {\"speaker\": \"host\", \"section\": \"article\", \"item_id\": \"item-1\", \"text\": \"host が記事を導入する\"},\n    {\"speaker\": \"partner\", \"section\": \"article\", \"item_id\": \"item-1\", \"text\": \"partner が反応を返す\"}\n  ]\n}",
  "articles_json": "[{\"item_id\":\"item-1\",\"title\":\"Example title\",\"translated_title\":\"翻訳タイトル\",\"source_title\":\"Source\",\"summary\":\"Summary text\",\"published_at\":\"2026-04-01T08:00:00Z\"}]",
  "existing_context": "なし"
}`)
	case "fish.summary_preprocess":
		return mustRawJSON(`{
  "text": "新しいAIモデルが公開されました。APIの料金は23.5パーセント下がります。"
}`)
	default:
		return nil
	}
}

func mustRawJSON(value string) json.RawMessage {
	return json.RawMessage(value)
}

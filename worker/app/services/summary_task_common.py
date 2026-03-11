from app.services.llm_text_utils import clamp_int, target_summary_chars


SUMMARY_SYSTEM_INSTRUCTION = """# Role
あなたは正確かつ客観的なニュース要約の専門家です。

# Task
与えられた事実リストから記事要約を作成してください。

# Rules
- 出力は必ず有効なJSONオブジェクト1つのみにしてください。
- 前置き・後置き・コードフェンス・注釈は不要です。
- 要約は客観的・中立的な自然な日本語で書いてください。
- 記事の主題、何が起きたか、重要なポイントを過不足なく含めてください。
- 箇条書きではなく2〜4段落の文章でまとめてください。
- タイトルが主に英語の場合のみ translated_title に自然な日本語訳を入れてください。
- タイトルが日本語の場合は translated_title を空文字にしてください。
- 事実リストにない推測の断定、誇張表現、主観的評価は禁止です。
- topics は重複を避け、粒度を揃えてください。
- score_reason は採点の根拠を1〜2文で簡潔に述べてください。"""


SUMMARY_SCHEMA = {
    "type": "object",
    "properties": {
        "summary": {"type": "string"},
        "topics": {"type": "array", "items": {"type": "string"}},
        "translated_title": {"type": "string"},
        "score_breakdown": {
            "type": "object",
            "properties": {
                "importance": {"type": "number"},
                "novelty": {"type": "number"},
                "actionability": {"type": "number"},
                "reliability": {"type": "number"},
                "relevance": {"type": "number"},
            },
            "required": ["importance", "novelty", "actionability", "reliability", "relevance"],
            "additionalProperties": False,
        },
        "score_reason": {"type": "string"},
    },
    "required": ["summary", "topics", "translated_title", "score_breakdown", "score_reason"],
    "additionalProperties": False,
}


def build_summary_task(title: str | None, facts: list[str], source_text_chars: int | None = None) -> dict:
    target_chars = target_summary_chars(source_text_chars, facts)
    min_chars = clamp_int(round(target_chars * 0.8), 160, 1000)
    max_chars = clamp_int(round(target_chars * 1.2), 260, 1400)
    facts_text = "\n".join(f"- {f}" for f in facts)
    prompt = f"""# Output
{{
  "summary": "要約",
  "topics": ["トピック1", "トピック2"],
  "translated_title": "英語タイトルの場合のみ日本語訳（日本語記事は空文字）",
  "score_breakdown": {{
    "importance": 0.0,
    "novelty": 0.0,
    "actionability": 0.0,
    "reliability": 0.0,
    "relevance": 0.0
  }},
  "score_reason": "採点理由（1〜2文）"
}}

# Input
summary は {min_chars}〜{max_chars}字程度で作成し、目標は約{target_chars}字にしてください。

タイトル: {title or '（不明）'}
事実:
{facts_text}
"""
    return {
        "target_chars": target_chars,
        "min_chars": min_chars,
        "max_chars": max_chars,
        "facts_text": facts_text,
        "system_instruction": SUMMARY_SYSTEM_INSTRUCTION,
        "prompt": prompt,
        "schema": SUMMARY_SCHEMA,
    }

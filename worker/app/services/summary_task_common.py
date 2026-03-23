from app.services.llm_text_utils import clamp_int, target_summary_chars
from app.services.langfuse_client import get_prompt_text


SUMMARY_SYSTEM_INSTRUCTION = """# Role
あなたは正確かつ客観的なニュース要約の専門家です。
同時に、ニュースレター編集者のように、読者が一息で理解できる自然な前文を書く役割も担います。

# Task
与えられた事実リストから記事要約を作成してください。

# Rules
- 出力は必ず有効なJSONオブジェクト1つのみにしてください。
- 前置き・後置き・コードフェンス・注釈は不要です。
- 要約は客観的・中立的な自然な日本語で書いてください。
- 記事の主題、何が起きたか、重要なポイントを過不足なく含めてください。
- 箇条書きではなく文章でまとめてください。
- 各 fact を1文ずつ順番に言い換えるのではなく、関連する facts を統合して流れのある説明文に再構成してください。
- 段落ごとに適切に改行し、段落間には空行を1つ入れてください。
- 1段落目で記事の要点をまとめ、2段落目で背景・影響・補足を自然につないでください。
- 1段落が長くなりすぎる場合は、話題の切れ目で改行して読みやすくしてください。
- 同じ文末表現を3文以上連続させないでください。特に「〜である。」の連続を避けてください。
- 接続語や言い換えを使い、短文の羅列ではなく自然な段落として読める文章にしてください。
- ニュースレター編集者が書く前文のように、冒頭から読者が文脈をつかめる自然な導入を作ってください。
- 短文を切って並べるのではなく、因果関係・対比・背景説明がつながるように文と文を接続してください。
- 必要に応じて主語や関係を補い、名詞句の断片を並べるだけの要約にしないでください。
- タイトルが主に英語の場合のみ translated_title に自然な日本語訳を入れてください。
- タイトルが日本語の場合は translated_title を空文字にしてください。
- 事実リストにない推測の断定、誇張表現、主観的評価は禁止です。
- topics は重複を避け、粒度を揃えてください。
- score_reason は採点の根拠を1〜2文で簡潔に述べてください。
- score_breakdown は各項目を0.0〜1.0で採点してください。
- score_breakdown は記事内容に応じて差を付けてください。
- score_breakdown の全項目を同じ値にしないでください。
- 根拠なく全項目を0.0にしないでください。"""


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
    prompt_fallback = f"""# Output
{{
  "summary": "要約",
  "topics": ["トピック1", "トピック2"],
  "translated_title": "英語タイトルの場合のみ日本語訳（日本語記事は空文字）",
  "score_breakdown": {{
    "importance": 0.78,
    "novelty": 0.54,
    "actionability": 0.61,
    "reliability": 0.83,
    "relevance": 0.57
  }},
  "score_reason": "採点理由（1〜2文）"
}}

# Input
summary は {min_chars}〜{max_chars}字程度で作成し、目標は約{target_chars}字にしてください。
summary は文章で作成し、段落ごとに改行し、段落間には空行を1つ入れてください。
ニュースレターの編集者が書く前文のように、冒頭から読者が自然に理解できる説明文にしてください。
各 fact を1文ずつ順番に言い換えるのではなく、関連する facts をまとめて流れのある要約文に再構成してください。
1段落目で記事の要点をまとめ、2段落目で背景・影響・補足を自然につないでください。
同じ文末表現を3文以上連続させないでください。特に「〜である。」の連続を避けてください。
短文を切って並べるのではなく、接続語や言い換えを使って文と文を自然につないでください。
必要に応じて主語や関係を補い、読み手が文脈を追いやすい自然な文章にしてください。
1段落が長くなりすぎる場合は、話題の切れ目で自然に改行してください。

タイトル: {title or '（不明）'}
事実:
{facts_text}
"""
    prompt = get_prompt_text(
        "summary.primary",
        prompt_fallback,
        variables={
            "title": title or "（不明）",
            "facts_text": facts_text,
            "target_chars": target_chars,
            "min_chars": min_chars,
            "max_chars": max_chars,
        },
    )
    return {
        "target_chars": target_chars,
        "min_chars": min_chars,
        "max_chars": max_chars,
        "facts_text": facts_text,
        "system_instruction": SUMMARY_SYSTEM_INSTRUCTION,
        "prompt": prompt,
        "schema": SUMMARY_SCHEMA,
    }

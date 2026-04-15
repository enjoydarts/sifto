from app.services.llm_text_utils import clamp_int, target_summary_chars
from app.services.langfuse_client import get_prompt_text
from app.services.prompt_template_defaults import get_default_prompt_template
from app.services.runtime_prompt_overrides import apply_prompt_override

_SUMMARY_GENRE_PATTERN = r"^[ぁ-ゖァ-ヺー一-龥々〆ヵヶ]{1,12}$"
_SUMMARY_GENRE_GUIDANCE = (
    "genre は必須です。短い日本語の単一ラベルを1つだけ返してください。"
    "複数ラベル、説明文、列挙、スラッシュ区切り、英語だけのラベルは不可です。"
    "判定が難しい場合は最も近い単一ラベルを返し、どうしても無理な場合のみ空文字にしてください。"
    "空文字は上流で null 扱いになります。"
)


def _append_summary_genre_guidance(text: str) -> str:
    rendered = str(text or "").strip()
    if not rendered:
        return ""
    if _SUMMARY_GENRE_GUIDANCE in rendered:
        return rendered
    return f"{rendered}\n\n# Genre\n{_SUMMARY_GENRE_GUIDANCE}"


SUMMARY_SYSTEM_INSTRUCTION = _append_summary_genre_guidance(
    str(get_default_prompt_template("summary.default").get("system_instruction") or "")
)


SUMMARY_SCHEMA = {
    "type": "object",
    "properties": {
        "summary": {"type": "string"},
        "topics": {"type": "array", "items": {"type": "string"}},
        "translated_title": {"type": "string"},
        "genre": {
            "anyOf": [
                {
                    "type": "string",
                    "minLength": 1,
                    "maxLength": 12,
                    "pattern": _SUMMARY_GENRE_PATTERN,
                },
                {
                    "type": "string",
                    "maxLength": 0,
                },
            ]
        },
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
    "required": ["summary", "topics", "translated_title", "score_breakdown", "score_reason", "genre"],
    "additionalProperties": False,
}


def build_summary_task(title: str | None, facts: list[str], source_text_chars: int | None = None) -> dict:
    default_template = get_default_prompt_template("summary.default")
    target_chars = target_summary_chars(source_text_chars, facts)
    min_chars = clamp_int(round(target_chars * 0.8), 160, 1000)
    max_chars = clamp_int(round(target_chars * 1.2), 260, 1400)
    facts_text = "\n".join(f"- {f}" for f in facts)
    variables = {
        "title": title or "（不明）",
        "facts_text": facts_text,
        "target_chars": target_chars,
        "min_chars": min_chars,
        "max_chars": max_chars,
    }
    prompt = get_prompt_text(
        "summary.primary",
        str(default_template.get("prompt_text") or ""),
        variables=variables,
    )
    system_instruction, prompt = apply_prompt_override("summary.default", SUMMARY_SYSTEM_INSTRUCTION, prompt, variables)
    system_instruction = _append_summary_genre_guidance(system_instruction)
    prompt = _append_summary_genre_guidance(prompt)
    return {
        "target_chars": target_chars,
        "min_chars": min_chars,
        "max_chars": max_chars,
        "facts_text": facts_text,
        "system_instruction": system_instruction,
        "prompt": prompt,
        "schema": SUMMARY_SCHEMA,
    }

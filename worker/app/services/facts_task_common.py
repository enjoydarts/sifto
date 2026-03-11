import re
from app.services.langfuse_client import get_prompt_text

from app.services.llm_text_utils import (
    decode_json_string_fragment,
    extract_first_json_object,
    parse_json_string_array,
    strip_code_fence,
)


FACTS_SCHEMA = {
    "type": "object",
    "properties": {
        "facts": {
            "type": "array",
            "items": {"type": "string"},
        }
    },
    "required": ["facts"],
    "additionalProperties": False,
}


def build_facts_task(title: str | None, content: str, *, output_mode: str = "object", fact_range: str = "8〜18個") -> dict:
    output_rule = (
        '- 出力は必ず {"facts": ["...", "..."]} のJSONオブジェクト1つのみにしてください。'
        if output_mode == "object"
        else '- 出力は必ず ["事実1", "事実2", ...] のJSON形式の配列のみとしてください。'
    )
    system_instruction_fallback = f"""# Role
あなたは正確かつ客観的なニュース要約の専門家です。

# Task
提供される記事から重要な事実を{fact_range}の箇条書きで抽出してください。

# Rules
{output_rule}
- 余計な挨拶や解説は一切不要です。
- 事実は客観的かつ具体的に記述してください。
- 記事が英語の場合も、出力は自然な日本語にしてください。
- 固有名詞は原文を尊重し、適宜英字を維持してください。"""
    prompt_fallback = f"""# Input
タイトル: {title or '（不明）'}

本文:
{content}
"""
    system_instruction = get_prompt_text(
        "facts.system",
        system_instruction_fallback,
        variables={"fact_range": fact_range, "output_rule": output_rule},
    )
    prompt = get_prompt_text(
        "facts.primary",
        prompt_fallback,
        variables={"title": title or "（不明）", "content": content, "fact_range": fact_range},
    )
    return {
        "system_instruction": system_instruction,
        "prompt": prompt,
        "schema": FACTS_SCHEMA,
    }


def extract_bulletish_lines(text: str) -> list[str]:
    out: list[str] = []
    for raw in strip_code_fence(text or "").splitlines():
        s = re.sub(r"^\s*(?:[-*•]|\d+[.)]|[\-\*\u2022\d\.\)\s]+)\s*", "", raw).strip()
        if len(s) >= 6:
            out.append(s)
    return dedupe_facts(out)


def is_placeholder_fact(text: str) -> bool:
    s = str(text or "").strip()
    if not s:
        return True
    return re.fullmatch(r"事実\s*\d+[\.\:]?", s) is not None


def dedupe_facts(raw: list[str], max_items: int = 18) -> list[str]:
    facts = [str(v).strip() for v in raw if str(v).strip() and not is_placeholder_fact(v)]
    seen: set[str] = set()
    out: list[str] = []
    for fact in facts:
        key = " ".join(fact.lower().split())
        if key in seen:
            continue
        seen.add(key)
        out.append(fact)
        if len(out) >= max_items:
            break
    return out


def parse_facts_result(text: str, *, max_items: int = 18) -> list[str]:
    obj = extract_first_json_object(text) or {}
    raw = obj.get("facts")
    facts = dedupe_facts(raw if isinstance(raw, list) else [], max_items=max_items)
    if facts:
        return facts
    facts = dedupe_facts(parse_json_string_array(text), max_items=max_items)
    if facts:
        return facts
    matches = re.findall(r'"((?:\\.|[^"\\])*)"', strip_code_fence(text), re.S)
    facts = dedupe_facts([decode_json_string_fragment(m).strip() for m in matches if decode_json_string_fragment(m).strip()], max_items=max_items)
    if facts:
        return facts
    return dedupe_facts(extract_bulletish_lines(text), max_items=max_items)

import re
from app.services.langfuse_client import get_prompt_text
from app.services.prompt_template_defaults import get_default_prompt_template
from app.services.runtime_prompt_overrides import apply_prompt_override

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


def build_facts_localization_task(title: str | None, facts: list[str]) -> dict:
    facts_text = "\n".join(f"- {str(f).strip()}" for f in (facts or []) if str(f).strip())
    system_instruction = """# Role
あなたはニュース記事の事実リストを日本語へ整える編集者です。

# Task
与えられた facts を、意味を変えずに自然な日本語へ統一してください。

# Rules
- 出力は必ず {"facts": ["...", "..."]} のJSONオブジェクト1つのみ
- 各 facts は必ず日本語の文字を1文字以上含める
- 固有名詞・企業名・製品名・人名・数値・記号は原文を尊重してよい
- 英文の文構造を残さず、日本語の箇条書きとして自然に言い換える
- 事実の追加・削除・推測は禁止"""
    prompt = f"""# Output
{{
  "facts": ["日本語化した事実1", "日本語化した事実2"]
}}

# Input
タイトル: {title or '（不明）'}

facts:
{facts_text}
"""
    return {
        "system_instruction": system_instruction,
        "prompt": prompt,
        "schema": FACTS_SCHEMA,
    }


def build_facts_task(title: str | None, content: str, *, output_mode: str = "object", fact_range: str = "8〜18個") -> dict:
    default_template = get_default_prompt_template("facts.default")
    output_rule = (
        '- 出力は必ず {"facts": ["...", "..."]} のJSONオブジェクト1つのみにしてください。'
        if output_mode == "object"
        else '- 出力は必ず ["事実1", "事実2", ...] のJSON形式の配列のみとしてください。'
    )
    variables = {
        "title": title or "（不明）",
        "content": content,
        "fact_range": fact_range,
        "output_rule": output_rule,
    }
    system_instruction = get_prompt_text(
        "facts.system",
        str(default_template.get("system_instruction") or ""),
        variables=variables,
    )
    prompt = get_prompt_text(
        "facts.primary",
        str(default_template.get("prompt_text") or ""),
        variables=variables,
    )
    system_instruction, prompt = apply_prompt_override("facts.default", system_instruction, prompt, variables)
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

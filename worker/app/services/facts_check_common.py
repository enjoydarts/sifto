import json
import re


FACTS_CHECK_SCHEMA = {
    "type": "object",
    "properties": {
        "verdict": {"type": "string"},
        "short_comment": {"type": "string"},
    },
    "required": ["verdict", "short_comment"],
    "additionalProperties": False,
}


def facts_check_system_instruction() -> str:
    return """# Role
あなたはニュース記事から抽出された facts の妥当性を検査するレビュアーです。

# Task
facts が元記事本文に忠実かを判定してください。

# Rules
- facts に書かれた内容が article で裏付けられているかだけを見る
- 推測、誇張、因果の飛躍、本文にない断定があれば厳しく評価する
- 軽微な言い換えや自然な圧縮は許容する
- placeholder や雛形のような抽象的箇条書きは厳しく評価する
- 出力は必ず JSON オブジェクト 1 つのみ
- verdict は pass / warn / fail のいずれか
- short_comment は日本語で 1〜2 文、120 文字以内
- short_comment は今回の article と facts を踏まえた具体的な寸評を書く
- 汎用的な定型文だけで済ませない"""


def facts_check_prompt(title: str | None, content: str, facts: list[str]) -> str:
    facts_text = "\n".join(f"- {f}" for f in facts)
    return f"""# Output
{{
  "verdict": "pass",
  "short_comment": "本文で裏付けられた事実抽出です。"
}}

# 判定基準
- pass: 主要 facts が article に忠実で、明確な unsupported fact がない
- warn: おおむね妥当だが、抽象的な箇所や軽い取りこぼしがある
- fail: article にない断定、矛盾、placeholder、重大な欠落がある

# Input
タイトル: {title or "（不明）"}

article:
{content}

facts:
{facts_text}
"""


def extract_first_json_object(text: str) -> dict | None:
    s = (text or "").strip().lstrip("\ufeff")
    if s.startswith("```"):
        s = re.sub(r"^```[a-zA-Z0-9_-]*\n?", "", s)
        s = re.sub(r"\n?```$", "", s).strip()
    decoder = json.JSONDecoder()
    idx = s.find("{")
    while idx >= 0:
        try:
            obj, _ = decoder.raw_decode(s[idx:])
            if isinstance(obj, dict):
                return obj
        except Exception:
            pass
        idx = s.find("{", idx + 1)
    return None


def normalize_facts_check_result(data: dict | None) -> dict:
    payload = data or {}
    verdict = str(payload.get("verdict") or "").strip().lower()
    if verdict not in {"pass", "warn", "fail"}:
        verdict = "warn"
    short_comment = str(payload.get("short_comment") or "").strip()
    return {
        "verdict": verdict,
        "short_comment": short_comment[:240],
    }


def require_facts_check_comment(result: dict, raw_text: str) -> dict:
    if str(result.get("short_comment") or "").strip():
        return result
    snippet = (raw_text or "").strip().replace("\n", " ")
    raise RuntimeError(f"facts check short_comment missing: response_snippet={snippet[:500]}")

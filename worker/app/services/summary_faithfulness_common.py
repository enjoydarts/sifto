import json
import re


SUMMARY_FAITHFULNESS_SCHEMA = {
    "type": "object",
    "properties": {
        "verdict": {"type": "string"},
        "short_comment": {"type": "string"},
    },
    "required": ["verdict", "short_comment"],
    "additionalProperties": False,
}


def summary_faithfulness_system_instruction() -> str:
    return """# Role
あなたはニュース要約の faithfuleness を検査するレビュアーです。

# Task
summary が facts に忠実かを判定してください。

# Rules
- summary に書かれた内容が facts で裏付けられているかだけを見る
- 推測、誇張、因果の飛躍、事実にない断定があれば厳しく評価する
- 軽微な言い換えや自然な圧縮は許容する
- 出力は必ず JSON オブジェクト 1 つのみ
- verdict は pass / warn / fail のいずれか
- short_comment は日本語で 1〜2 文、120 文字以内"""


def summary_faithfulness_prompt(title: str | None, facts: list[str], summary: str) -> str:
    facts_text = "\n".join(f"- {f}" for f in facts)
    return f"""# Output
{{
  "verdict": "pass",
  "short_comment": "facts で裏付けられた自然な要約です。"
}}

# 判定基準
- pass: 主要内容が facts に忠実で、明確な unsupported claim がない
- warn: おおむね忠実だが、やや強い表現や重要事実の軽い欠落がある
- fail: facts にない断定、矛盾、重大な欠落がある

# Input
タイトル: {title or "（不明）"}

facts:
{facts_text}

summary:
{summary}
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


def normalize_summary_faithfulness_result(data: dict | None) -> dict:
    payload = data or {}
    verdict = str(payload.get("verdict") or "").strip().lower()
    if verdict not in {"pass", "warn", "fail"}:
        verdict = "warn"
    short_comment = str(payload.get("short_comment") or "").strip()
    if not short_comment:
        short_comment = {
            "pass": "facts に沿った要約です。",
            "warn": "概ね要点は合っていますが、表現の強さや欠落に注意が必要です。",
            "fail": "facts にない断定または重要な欠落があり、再生成が必要です。",
        }[verdict]
    return {
        "verdict": verdict,
        "short_comment": short_comment[:240],
    }

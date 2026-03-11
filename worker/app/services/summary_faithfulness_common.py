import json
import re
from app.services.langfuse_client import get_prompt_text


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
    fallback = """# Role
あなたはニュース要約の faithfulness を検査するレビュアーです。

# Task
summary が facts に忠実かを判定してください。

# Rules
- summary に書かれた内容が facts で裏付けられているかだけを見る
- 推測、誇張、因果の飛躍、事実にない断定があれば厳しく評価する
- 軽微な言い換えや自然な圧縮は許容する
- 出力は必ず JSON オブジェクト 1 つのみ
- verdict は pass / warn / fail のいずれか
- short_comment は日本語で 1〜2 文、120 文字以内
- short_comment は今回の summary と facts を踏まえた具体的な寸評を書く
- 汎用的な定型文だけで済ませない"""
    return get_prompt_text("faithfulness_check.system", fallback)


def summary_faithfulness_prompt(title: str | None, facts: list[str], summary: str) -> str:
    facts_text = "\n".join(f"- {f}" for f in facts)
    fallback = f"""# Output
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
    return get_prompt_text(
        "faithfulness_check.primary",
        fallback,
        variables={"title": title or "（不明）", "facts_text": facts_text, "summary": summary},
    )


def summary_faithfulness_retry_prompt(title: str | None, facts: list[str], summary: str) -> str:
    facts_text = "\n".join(f"- {f}" for f in facts)
    fallback = f"""1行のみで返してください。
形式は verdict のみです。

条件:
- verdict は pass / warn / fail のいずれか
- 前置き、後置き、コードフェンス禁止
- 例: pass

タイトル: {title or "（不明）"}

facts:
{facts_text}

summary:
{summary}
"""
    return get_prompt_text(
        "faithfulness_check.retry",
        fallback,
        variables={"title": title or "（不明）", "facts_text": facts_text, "summary": summary},
    )


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
    return {
        "verdict": verdict,
        "short_comment": short_comment[:240],
    }


def summary_faithfulness_comment_for_verdict(verdict: str) -> str:
    mapping = {
        "pass": "facts に忠実な要約です。",
        "warn": "おおむね忠実ですが一部に確認したい表現があります。",
        "fail": "facts にない断定または重要な欠落があります。",
    }
    return mapping.get(str(verdict or "").strip().lower(), "要約の忠実性判定結果を確認してください。")


def parse_summary_faithfulness_line(text: str) -> dict | None:
    s = (text or "").strip()
    if not s:
        return None
    if s.startswith("```"):
        s = re.sub(r"^```[a-zA-Z0-9_-]*\n?", "", s)
        s = re.sub(r"\n?```$", "", s).strip()
    first = s.splitlines()[0].strip()
    if not first:
        return None
    verdict = first.strip().lower()
    m = re.search(r"\b(pass|warn|fail)\b", verdict)
    if m:
        verdict = m.group(1)
    if verdict not in {"pass", "warn", "fail"}:
        return None
    return {"verdict": verdict, "short_comment": summary_faithfulness_comment_for_verdict(verdict)}


def require_summary_faithfulness_comment(result: dict, raw_text: str) -> dict:
    if str(result.get("short_comment") or "").strip():
        return result
    snippet = (raw_text or "").strip().replace("\n", " ")
    raise RuntimeError(f"faithfulness short_comment missing: response_snippet={snippet[:500]}")

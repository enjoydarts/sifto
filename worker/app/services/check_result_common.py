import json
import re

from app.services.langfuse_client import score_current


CHECK_RESULT_SCHEMA = {
    "type": "object",
    "properties": {
        "verdict": {"type": "string"},
        "short_comment": {"type": "string"},
    },
    "required": ["verdict", "short_comment"],
    "additionalProperties": False,
}


def append_check_output_contract(text: str, *, comment_rule: str) -> str:
    base = (text or "").strip()
    contract = f"""# JSON 出力契約
- JSON オブジェクト 1 つのみを返す
- 必須キーは verdict と short_comment の 2 つ
- verdict は pass / warn / fail のいずれか
- short_comment は必ず空でない文字列にする
- short_comment は {comment_rule}
- verdict だけの JSON、short_comment 欠落、空文字、null は禁止
- 前置き、後置き、Markdown コードフェンスは禁止

必ず次の形で返す:
{{
  "verdict": "pass",
  "short_comment": "本文で裏付けられた判定理由です。"
}}"""
    if not base:
        return contract
    return f"{base}\n\n{contract}"


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
    repaired = _repair_missing_opening_brace_json(s)
    if repaired is not None:
        try:
            obj = json.loads(repaired)
            if isinstance(obj, dict):
                return obj
        except Exception:
            pass
    return None


def _repair_missing_opening_brace_json(text: str) -> str | None:
    s = (text or "").strip()
    if s.startswith("{") or "{" in s:
        return None
    if '"verdict"' not in s or '"short_comment"' not in s:
        return None
    end = s.rfind("}")
    if end < 0:
        return None
    return "{" + s[:end + 1]


def normalize_check_result(data: dict | None, *, max_comment_len: int = 240) -> dict:
    payload = data or {}
    verdict = str(payload.get("verdict") or "").strip().lower()
    if verdict not in {"pass", "warn", "fail"}:
        verdict = "warn"
    short_comment = str(payload.get("short_comment") or "").strip()
    return {
        "verdict": verdict,
        "short_comment": short_comment[:max_comment_len],
    }


def parse_check_line(text: str, *, comment_for_verdict) -> dict | None:
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
    return {"verdict": verdict, "short_comment": comment_for_verdict(verdict)}


def require_check_comment(result: dict, raw_text: str, *, error_prefix: str, validator=None) -> dict:
    short_comment = str(result.get("short_comment") or "").strip()
    if validator is None:
        if short_comment:
            return result
    elif validator(short_comment):
        return result
    snippet = (raw_text or "").strip().replace("\n", " ")
    if not snippet:
        snippet = "(empty)"
    raise RuntimeError(f"{error_prefix} short_comment missing: response_snippet={snippet[:500]}")


def record_check_score(score_name: str, result: dict) -> None:
    score_current(
        score_name,
        result.get("verdict"),
        comment=str(result.get("short_comment") or ""),
        data_type="CATEGORICAL",
    )

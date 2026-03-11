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

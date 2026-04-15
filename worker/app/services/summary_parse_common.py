from collections.abc import Callable

from app.services.llm_text_utils import extract_json_string_value_loose, summary_composite_score
from app.services.summary_result_common import finalize_translated_title, normalize_score_breakdown

DEFAULT_SCORE_REASON = "総合的な重要度・新規性・実用性を基に採点。"


def finalize_summary_result(
    *,
    title: str | None,
    summary_text: str,
    topics: list[str] | None,
    genre: str | None,
    raw_score_breakdown: dict | None,
    score_reason: str,
    translated_title: str,
    translate_func: Callable[[str], str],
    llm: dict,
    error_prefix: str,
    response_text: str,
) -> dict:
    summary = str(summary_text or "").strip()
    if not summary:
        summary = extract_json_string_value_loose(response_text, "summary")
    if not summary:
        raise RuntimeError(f"{error_prefix}: response_snippet={response_text[:500]}")
    normalized_genre = str(genre or "").strip()
    if not normalized_genre:
        normalized_genre = extract_json_string_value_loose(response_text, "genre")
    score_breakdown = normalize_score_breakdown(raw_score_breakdown)
    return {
        "summary": summary,
        "topics": [str(t).strip() for t in (topics or []) if str(t).strip()],
        "genre": str(normalized_genre or "").strip(),
        "translated_title": finalize_translated_title(
            title,
            str(translated_title or "").strip(),
            translate_func=translate_func,
        ),
        "score": summary_composite_score(score_breakdown),
        "score_breakdown": score_breakdown,
        "score_reason": (str(score_reason or "").strip() or DEFAULT_SCORE_REASON)[:400],
        "score_policy_version": "v4",
        "llm": llm,
    }

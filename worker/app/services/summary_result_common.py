from collections.abc import Callable

from app.services.llm_text_utils import contains_japanese, clamp01, needs_title_translation


def normalize_score_breakdown(raw_breakdown: dict | None) -> dict:
    breakdown = raw_breakdown if isinstance(raw_breakdown, dict) else {}
    return {
        "importance": clamp01(breakdown.get("importance", 0.5)),
        "novelty": clamp01(breakdown.get("novelty", 0.5)),
        "actionability": clamp01(breakdown.get("actionability", 0.5)),
        "reliability": clamp01(breakdown.get("reliability", 0.5)),
        "relevance": clamp01(breakdown.get("relevance", 0.5)),
    }


def finalize_translated_title(
    title: str | None,
    translated_title: str,
    *,
    translate_func: Callable[[str], str],
) -> str:
    candidate = str(translated_title or "").strip()
    if contains_japanese(title or ""):
        return ""
    if needs_title_translation(title, "") and not contains_japanese(candidate):
        candidate = ""
    if needs_title_translation(title, candidate):
        candidate = translate_func(title or "")
    return candidate[:300]

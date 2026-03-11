import re
from collections.abc import Callable

from app.services.llm_text_utils import contains_japanese, needs_title_translation, strip_code_fence


TITLE_TRANSLATION_SCHEMA = {
    "type": "object",
    "properties": {
        "translated_title": {"type": "string"},
    },
    "required": ["translated_title"],
    "additionalProperties": False,
}


def normalize_title_candidate(value: str) -> str:
    text = strip_code_fence(str(value or "")).strip().strip('"').strip("'")
    text = re.sub(r"\s+", " ", text)
    return text.strip()


def is_untranslated_title(source_title: str, candidate: str) -> bool:
    normalized = normalize_title_candidate(candidate)
    if not normalized:
        return True
    if contains_japanese(normalized):
        return False
    source_normalized = normalize_title_candidate(source_title)
    if normalized.casefold() == source_normalized.casefold():
        return True
    return re.search(r"[A-Za-z]", normalized) is not None


def run_title_translation(
    title: str,
    *,
    structured_call: Callable[[], str],
    plain_retry_call: Callable[[], str] | None = None,
    final_retry_call: Callable[[], str] | None = None,
) -> str:
    src = (title or "").strip()
    if not needs_title_translation(src, ""):
        return ""

    structured = normalize_title_candidate(structured_call())
    if not is_untranslated_title(src, structured):
        return structured[:300]

    if plain_retry_call is not None:
        plain = normalize_title_candidate(plain_retry_call())
        if not is_untranslated_title(src, plain):
            return plain[:300]

    if final_retry_call is not None:
        retried = normalize_title_candidate(final_retry_call())
        if not is_untranslated_title(src, retried):
            return retried[:300]

    return ""

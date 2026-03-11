from collections.abc import Callable

from app.services.facts_check_common import (
    extract_first_json_object,
    normalize_facts_check_result,
    parse_facts_check_line,
    require_facts_check_comment,
)


def _parse_facts_check_response(text: str) -> dict:
    line_result = parse_facts_check_line(text)
    if line_result is not None:
        return line_result
    return require_facts_check_comment(
        normalize_facts_check_result(extract_first_json_object(text)),
        text,
    )


def run_facts_check(
    primary_call: Callable[[], tuple[str, dict | None]],
    *,
    retry_call: Callable[[], tuple[str, dict | None]] | None = None,
    retry_attempts: int = 2,
) -> dict:
    text, llm = primary_call()
    try:
        result = _parse_facts_check_response(text)
    except RuntimeError as exc:
        if retry_call is None:
            raise
        last_exc = exc
        result = None
        for _ in range(max(1, int(retry_attempts or 1))):
            retry_text, retry_llm = retry_call()
            llm = retry_llm
            try:
                result = _parse_facts_check_response(retry_text)
                break
            except RuntimeError as retry_exc:
                last_exc = retry_exc
        if result is None:
            raise last_exc
    result["llm"] = llm
    return result

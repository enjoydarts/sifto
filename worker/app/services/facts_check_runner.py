from collections.abc import Callable

from app.services.facts_check_common import (
    extract_first_json_object,
    normalize_facts_check_result,
    require_facts_check_comment,
)
from app.services.check_result_common import record_check_score


def _parse_facts_check_response(text: str) -> dict:
    return require_facts_check_comment(
        normalize_facts_check_result(extract_first_json_object(text)),
        text,
    )


def _sum_check_usage(first: dict | None, second: dict | None) -> dict | None:
    if not first:
        return second
    if not second:
        return first
    result = dict(second)
    for key in ("input_tokens", "output_tokens", "cache_creation_input_tokens", "cache_read_input_tokens", "estimated_cost_usd"):
        if key in first or key in second:
            result[key] = first.get(key, 0) + second.get(key, 0)
    return result


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
        else:
            last_exc = exc
            result = None
            for _ in range(max(1, int(retry_attempts or 1))):
                retry_text, retry_llm = retry_call()
                llm = _sum_check_usage(llm, retry_llm)
                try:
                    result = _parse_facts_check_response(retry_text)
                    break
                except RuntimeError as retry_exc:
                    last_exc = retry_exc
            if result is None:
                raise last_exc
    result["llm"] = llm
    record_check_score("facts_check_verdict", result)
    return result

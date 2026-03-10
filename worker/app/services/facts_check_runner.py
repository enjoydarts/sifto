from collections.abc import Callable

from app.services.facts_check_common import (
    extract_first_json_object,
    normalize_facts_check_result,
    parse_facts_check_line,
    require_facts_check_comment,
)


def run_facts_check(
    primary_call: Callable[[], tuple[str, dict | None]],
    *,
    retry_call: Callable[[], tuple[str, dict | None]] | None = None,
) -> dict:
    text, llm = primary_call()
    try:
        result = require_facts_check_comment(
            normalize_facts_check_result(extract_first_json_object(text)),
            text,
        )
    except RuntimeError:
        if retry_call is None:
            raise
        retry_text, retry_llm = retry_call()
        result = require_facts_check_comment(
            normalize_facts_check_result(parse_facts_check_line(retry_text) or extract_first_json_object(retry_text)),
            retry_text,
        )
        llm = retry_llm
    result["llm"] = llm
    return result

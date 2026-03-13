from collections.abc import Callable

from app.services.facts_check_common import (
    extract_first_json_object,
    normalize_facts_check_result,
    parse_facts_check_line,
    require_facts_check_comment,
)
from app.services.check_result_common import record_check_score


def _parse_facts_check_response(text: str) -> dict:
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
    fallback_line = None
    try:
        result = _parse_facts_check_response(text)
    except RuntimeError as exc:
        fallback_line = parse_facts_check_line(text)
        if retry_call is None:
            if fallback_line is not None:
                result = fallback_line
            else:
                raise
        else:
            last_exc = exc
            result = None
            for _ in range(max(1, int(retry_attempts or 1))):
                retry_text, retry_llm = retry_call()
                llm = retry_llm
                fallback_line = parse_facts_check_line(retry_text)
                try:
                    result = _parse_facts_check_response(retry_text)
                    break
                except RuntimeError as retry_exc:
                    last_exc = retry_exc
            if result is None:
                if fallback_line is not None:
                    result = fallback_line
                else:
                    result = {
                        "verdict": "warn",
                        "short_comment": "事実抽出チェックの判定応答を取得できなかったため未確認です。再試行してください。",
                    }
    result["llm"] = llm
    record_check_score("facts_check_verdict", result)
    return result

from collections.abc import Callable

from app.services.summary_faithfulness_common import (
    extract_first_json_object,
    normalize_summary_faithfulness_result,
    parse_summary_faithfulness_line,
    require_summary_faithfulness_comment,
)
from app.services.check_result_common import record_check_score


def _parse_summary_faithfulness_response(text: str) -> dict:
    line_result = parse_summary_faithfulness_line(text)
    if line_result is not None:
        return line_result
    return require_summary_faithfulness_comment(
        normalize_summary_faithfulness_result(extract_first_json_object(text)),
        text,
    )


def run_summary_faithfulness_check(
    primary_call: Callable[[], tuple[str, dict | None]],
    *,
    retry_call: Callable[[], tuple[str, dict | None]] | None = None,
    retry_attempts: int = 2,
) -> dict:
    text, llm = primary_call()
    try:
        result = _parse_summary_faithfulness_response(text)
    except RuntimeError as exc:
        if retry_call is None:
            raise
        last_exc = exc
        result = None
        for _ in range(max(1, int(retry_attempts or 1))):
            retry_text, retry_llm = retry_call()
            llm = retry_llm
            try:
                result = _parse_summary_faithfulness_response(retry_text)
                break
            except RuntimeError as retry_exc:
                last_exc = retry_exc
        if result is None:
            result = {
                "verdict": "warn",
                "short_comment": "忠実性チェックの判定応答を取得できなかったため未確認です。再試行してください。",
            }
    result["llm"] = llm
    record_check_score("faithfulness_verdict", result)
    return result

from contextlib import contextmanager

from app.services.langfuse_client import bind_span, update_current


def observe_request_input(metadata: dict | None = None, input_payload: dict | None = None) -> None:
    update_current(metadata=metadata or {}, input=input_payload or {})


def observe_request_output(output_payload: dict | None = None) -> None:
    update_current(output=output_payload or {})


def llm_usage_summary(result: dict | None) -> dict:
    llm = (result or {}).get("llm") or {}
    if not isinstance(llm, dict):
        return {}
    return {
        "llm_provider": llm.get("provider") or "",
        "llm_model": llm.get("model") or "",
        "estimated_cost_usd": llm.get("estimated_cost_usd") or 0,
        "input_tokens": llm.get("input_tokens") or 0,
        "output_tokens": llm.get("output_tokens") or 0,
    }


@contextmanager
def bind_request_span(request) -> None:
    with bind_span(getattr(getattr(request, "state", None), "langfuse_span", None)):
        yield

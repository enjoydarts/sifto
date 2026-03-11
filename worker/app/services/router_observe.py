from contextlib import contextmanager

from app.services.langfuse_client import bind_span, update_current


def observe_request_input(metadata: dict | None = None, input_payload: dict | None = None) -> None:
    update_current(metadata=metadata or {}, input=input_payload or {})


def _langfuse_llm_update(result: dict | None) -> dict:
    llm = (result or {}).get("llm") or {}
    if not isinstance(llm, dict):
        return {}
    model = str(llm.get("model") or "").strip()
    provider = str(llm.get("provider") or "").strip()
    usage_details = {
        "input": int(llm.get("input_tokens") or 0),
        "output": int(llm.get("output_tokens") or 0),
        "cache_read_input": int(llm.get("cache_read_input_tokens") or 0),
        "cache_creation_input": int(llm.get("cache_creation_input_tokens") or 0),
    }
    cost = float(llm.get("estimated_cost_usd") or 0.0)
    model_parameters = {"provider": provider} if provider else None
    cost_details = {"total_usd": cost} if cost > 0 else None
    return {
        "model": model or None,
        "model_parameters": model_parameters,
        "usage_details": usage_details if any(usage_details.values()) else None,
        "cost_details": cost_details,
    }


def observe_request_output(output_payload: dict | None = None, *, llm_result: dict | None = None) -> None:
    update_current(output=output_payload or {}, **_langfuse_llm_update(llm_result))


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

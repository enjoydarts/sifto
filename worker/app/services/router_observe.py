from contextlib import contextmanager

from app.services.langfuse_client import bind_span, update_current


def observe_request_input(metadata: dict | None = None, input_payload: dict | None = None) -> None:
    update_current(metadata=metadata or {}, input=input_payload or {})


def _langfuse_llm_update(result: dict | None) -> dict:
    llm = (result or {}).get("llm") or {}
    if not isinstance(llm, dict):
        return {}
    input_tokens = int(llm.get("input_tokens") or 0)
    output_tokens = int(llm.get("output_tokens") or 0)
    cache_read_input_tokens = int(llm.get("cache_read_input_tokens") or 0)
    cache_creation_input_tokens = int(llm.get("cache_creation_input_tokens") or 0)
    model = str(llm.get("model") or "").strip()
    provider = str(llm.get("provider") or "").strip()
    usage_details = {
        "input": input_tokens,
        "output": output_tokens,
        "prompt_tokens": input_tokens,
        "completion_tokens": output_tokens,
        "cache_read_input": cache_read_input_tokens,
        "cache_creation_input": cache_creation_input_tokens,
        "cache_read_input_tokens": cache_read_input_tokens,
        "cache_creation_input_tokens": cache_creation_input_tokens,
        "total": input_tokens + output_tokens,
    }
    cost = float(llm.get("estimated_cost_usd") or 0.0)
    model_parameters = {"provider": provider} if provider else None
    cost_details = {"total": cost, "total_cost": cost} if cost > 0 else None
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


def observe_result(
    result,
    *,
    output_builder=None,
    include_llm_result: bool = True,
):
    output_payload = output_builder(result) if callable(output_builder) else (output_builder or {})
    llm_result = result if include_llm_result and isinstance(result, dict) else None
    observe_request_output(output_payload, llm_result=llm_result)
    return result


def run_observed_request(
    request,
    *,
    metadata: dict | None = None,
    input_payload: dict | None = None,
    call,
    output_builder=None,
    include_llm_result: bool = True,
):
    with bind_request_span(request):
        observe_request_input(metadata=metadata, input_payload=input_payload)
        result = call()
        return observe_result(
            result,
            output_builder=output_builder,
            include_llm_result=include_llm_result,
        )


@contextmanager
def bind_request_span(request) -> None:
    with bind_span(getattr(getattr(request, "state", None), "langfuse_span", None)):
        yield

def wrap_usage_transport(call, llm_meta_builder):
    text, usage = call()
    return text, llm_meta_builder(usage)


def empty_llm_meta(provider: str, model: str, pricing_source: str = "default") -> dict:
    return {
        "provider": provider,
        "model": model,
        "pricing_model_family": model,
        "pricing_source": pricing_source,
        "input_tokens": 0,
        "output_tokens": 0,
        "cache_creation_input_tokens": 0,
        "cache_read_input_tokens": 0,
        "estimated_cost_usd": 0.0,
    }


def wrap_message_transport(message, llm_meta_builder, empty_llm: dict) -> tuple[str, dict]:
    if message is None:
        return "", empty_llm
    return message.content[0].text.strip(), llm_meta_builder(message)


def wrap_message_fallback_transport(result, llm_meta_builder, provider: str, fallback_model: str, pricing_source: str) -> tuple[str, dict]:
    message = None
    resolved_model = None
    if isinstance(result, tuple):
        if len(result) >= 1:
            message = result[0]
        if len(result) >= 2:
            resolved_model = result[1]
    return wrap_message_transport(
        message,
        lambda msg: llm_meta_builder(msg, resolved_model or fallback_model),
        empty_llm_meta(provider, resolved_model or fallback_model, pricing_source),
    )


def wrap_json_transport(call, llm_meta_builder):
    return wrap_usage_transport(call, llm_meta_builder)


def wrap_anthropic_message(message, llm_meta_builder, empty_llm: dict) -> tuple[str, dict]:
    return wrap_message_transport(message, llm_meta_builder, empty_llm)


def wrap_anthropic_result(result, llm_meta_builder, provider: str, fallback_model: str, pricing_source: str) -> tuple[str, dict]:
    return wrap_message_fallback_transport(result, llm_meta_builder, provider, fallback_model, pricing_source)

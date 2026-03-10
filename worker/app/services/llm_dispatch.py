from collections.abc import Callable

from fastapi import Request

from app.services.llm_catalog import provider_api_key_header, provider_for_model


def dispatch_by_model(
    request: Request,
    model: str | None,
    *,
    handlers: dict[str, Callable[[str | None], dict]],
    default_provider: str = "anthropic",
) -> dict:
    provider = provider_for_model(model) or default_provider
    handler = handlers.get(provider) or handlers.get(default_provider)
    if handler is None:
        raise RuntimeError(f"no handler registered for provider={provider}")
    api_key_header = provider_api_key_header(provider) or provider_api_key_header(default_provider)
    api_key = request.headers.get(api_key_header) if api_key_header else None
    return handler(api_key or None)

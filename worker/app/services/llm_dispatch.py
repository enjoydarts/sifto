import logging
from collections.abc import Callable
from typing import Literal

from fastapi import Request

from app.services.llm_catalog import provider_api_key_header, provider_for_model

logger = logging.getLogger(__name__)

Provider = Literal[
    "openai", "anthropic", "google", "groq", "deepseek",
    "openrouter", "together", "mistral", "xai", "alibaba",
    "poe", "siliconflow", "fireworks", "zai", "moonshot",
]


def dispatch_by_model(
    request: Request,
    model: str | None,
    *,
    handlers: dict[str, Callable[[str | None], dict]],
    default_provider: str = "anthropic",
) -> dict:
    provider = str(request.headers.get("X-Sifto-LLM-Provider") or "").strip().lower() or provider_for_model(model) or default_provider
    handler = handlers.get(provider)
    if handler is None:
        logger.warning("unknown provider '%s' for model '%s', falling back to %s", provider, model, default_provider)
        handler = handlers.get(default_provider)
    if handler is None:
        raise RuntimeError(f"no handler registered for provider={provider}")
    api_key_header = provider_api_key_header(provider) or provider_api_key_header(default_provider)
    api_key = request.headers.get(api_key_header) if api_key_header else None
    return handler(api_key or None)


async def dispatch_by_model_async(
    request: Request,
    model: str | None,
    *,
    handlers: dict[str, Callable[[str | None], dict]],
    default_provider: str = "anthropic",
) -> dict:
    provider = str(request.headers.get("X-Sifto-LLM-Provider") or "").strip().lower() or provider_for_model(model) or default_provider
    handler = handlers.get(provider)
    if handler is None:
        logger.warning("unknown provider '%s' for model '%s', falling back to %s", provider, model, default_provider)
        handler = handlers.get(default_provider)
    if handler is None:
        raise RuntimeError(f"no handler registered for provider={provider}")
    api_key_header = provider_api_key_header(provider) or provider_api_key_header(default_provider)
    api_key = request.headers.get(api_key_header) if api_key_header else None
    return await handler(api_key or None)

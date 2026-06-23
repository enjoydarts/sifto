import logging
from collections.abc import Callable

from fastapi import Request

from app.services.llm_catalog import provider_api_key_header, provider_for_model, get_llm_providers
from app.services.openai_compat_transport import provider_request_context

logger = logging.getLogger(__name__)

# Provider IDs are obtained exclusively at runtime from shared/llm_catalog.json via get_llm_providers().
# The static list below is kept only for reference / legacy type hints; do not extend it when adding providers.
Provider = str  # was Literal[...] hard-coded list; now catalog-driven (see get_llm_providers)


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
    with provider_request_context(request.headers.get("X-Sifto-User-Id")):
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
    with provider_request_context(request.headers.get("X-Sifto-User-Id")):
        return await handler(api_key or None)

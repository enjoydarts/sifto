import importlib
from collections.abc import Callable

from app.services.llm_catalog import get_llm_providers, provider_requires_anthropic_args, provider_service_module

# NOTE: provider list and module resolution now exclusively from shared/llm_catalog.json via get_llm_providers + provider_service_module
# (reduces surface for adding new catalog-listed providers)


def build_handler_map(
    task_name: str,
    args_fn: Callable,
    anthropic_args_fn: Callable | None = None,
    providers: list[str] | None = None,
) -> dict[str, Callable]:
    if providers is None:
        providers = get_llm_providers()
    if anthropic_args_fn is None:
        anthropic_args_fn = args_fn
    handlers: dict[str, Callable] = {}
    for provider_name in providers:
        module_name = provider_service_module(provider_name)
        service_module = importlib.import_module(f"app.services.{module_name}")
        task_func = getattr(service_module, task_name)
        # special args driven by catalog metadata (requires_anthropic_args), not module string literal
        if provider_requires_anthropic_args(provider_name):
            handlers[provider_name] = lambda api_key, tf=task_func, af=anthropic_args_fn: af(tf, api_key)
        else:
            handlers[provider_name] = lambda api_key, tf=task_func, a=args_fn: a(tf, api_key)
    return handlers


def build_handler_map_async(
    task_name: str,
    args_fn: Callable,
    anthropic_args_fn: Callable | None = None,
    providers: list[str] | None = None,
) -> dict[str, Callable]:
    if providers is None:
        providers = get_llm_providers()
    if anthropic_args_fn is None:
        anthropic_args_fn = args_fn
    handlers: dict[str, Callable] = {}
    async_task_name = task_name + "_async"
    for provider_name in providers:
        module_name = provider_service_module(provider_name)
        service_module = importlib.import_module(f"app.services.{module_name}")
        task_func = getattr(service_module, async_task_name)
        if provider_requires_anthropic_args(provider_name):
            handlers[provider_name] = lambda api_key, tf=task_func, af=anthropic_args_fn: af(tf, api_key)
        else:
            handlers[provider_name] = lambda api_key, tf=task_func, a=args_fn: a(tf, api_key)
    return handlers

import importlib
from collections.abc import Callable

_PROVIDER_MODULE_MAP = {
    "anthropic": "claude_service",
    "google": "gemini_service",
}

_DEFAULT_PROVIDERS = [
    "anthropic", "google", "fireworks", "groq", "deepseek", "alibaba",
    "mistral", "together", "moonshot", "minimax", "xai", "zai", "openrouter",
    "poe", "siliconflow", "openai",
]


def build_handler_map(
    task_name: str,
    args_fn: Callable,
    anthropic_args_fn: Callable | None = None,
    providers: list[str] | None = None,
) -> dict[str, Callable]:
    if providers is None:
        providers = _DEFAULT_PROVIDERS
    if anthropic_args_fn is None:
        anthropic_args_fn = args_fn
    handlers: dict[str, Callable] = {}
    for provider_name in providers:
        module_name = _PROVIDER_MODULE_MAP.get(provider_name, f"{provider_name}_service")
        service_module = importlib.import_module(f"app.services.{module_name}")
        task_func = getattr(service_module, task_name)
        if provider_name == "anthropic":
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
        providers = _DEFAULT_PROVIDERS
    if anthropic_args_fn is None:
        anthropic_args_fn = args_fn
    handlers: dict[str, Callable] = {}
    async_task_name = task_name + "_async"
    for provider_name in providers:
        module_name = _PROVIDER_MODULE_MAP.get(provider_name, f"{provider_name}_service")
        service_module = importlib.import_module(f"app.services.{module_name}")
        task_func = getattr(service_module, async_task_name)
        if provider_name == "anthropic":
            handlers[provider_name] = lambda api_key, tf=task_func, af=anthropic_args_fn: af(tf, api_key)
        else:
            handlers[provider_name] = lambda api_key, tf=task_func, a=args_fn: a(tf, api_key)
    return handlers

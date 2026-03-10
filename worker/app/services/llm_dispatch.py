from collections.abc import Callable

from fastapi import Request

from app.services.model_router import is_deepseek_model, is_gemini_model, is_groq_model, is_openai_model


def dispatch_by_model(
    request: Request,
    model: str | None,
    *,
    anthropic: Callable[[str | None], dict],
    google: Callable[[str], dict] | None = None,
    groq: Callable[[str], dict] | None = None,
    deepseek: Callable[[str], dict] | None = None,
    openai: Callable[[str], dict] | None = None,
) -> dict:
    if is_gemini_model(model) and google is not None:
        return google(request.headers.get("x-google-api-key") or "")
    if is_deepseek_model(model) and deepseek is not None:
        return deepseek(request.headers.get("x-deepseek-api-key") or "")
    if is_groq_model(model) and groq is not None:
        return groq(request.headers.get("x-groq-api-key") or "")
    if is_openai_model(model) and openai is not None:
        return openai(request.headers.get("x-openai-api-key") or "")
    return anthropic(request.headers.get("x-anthropic-api-key") or None)

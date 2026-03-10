from app.services.llm_catalog import provider_for_model


def is_gemini_model(model: str | None) -> bool:
    return provider_for_model(model) == "google"


def is_groq_model(model: str | None) -> bool:
    return provider_for_model(model) == "groq"


def is_deepseek_model(model: str | None) -> bool:
    return provider_for_model(model) == "deepseek"


def is_openai_model(model: str | None) -> bool:
    return provider_for_model(model) == "openai"

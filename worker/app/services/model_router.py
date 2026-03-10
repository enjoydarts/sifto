def is_gemini_model(model: str | None) -> bool:
    m = str(model or "").strip().lower()
    if not m:
        return False
    return m.startswith("gemini-") or "/models/gemini-" in m


def is_groq_model(model: str | None) -> bool:
    m = str(model or "").strip().lower()
    if not m:
        return False
    return (
        m.startswith("openai/gpt-oss-")
        or m.startswith("qwen/")
        or m.startswith("meta-llama/")
        or m.startswith("llama-")
    )


def is_deepseek_model(model: str | None) -> bool:
    m = str(model or "").strip().lower()
    if not m:
        return False
    return m in ("deepseek-chat", "deepseek-reasoner")

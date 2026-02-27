def is_gemini_model(model: str | None) -> bool:
    m = str(model or "").strip().lower()
    if not m:
        return False
    return m.startswith("gemini-") or "/models/gemini-" in m

import json
import os
from functools import lru_cache
from pathlib import Path

OPENROUTER_ALIAS_PREFIX = "openrouter::"
POE_ALIAS_PREFIX = "poe::"


def resolve_model_id(model: str | None) -> str:
    m = str(model or "").strip()
    if m.startswith(OPENROUTER_ALIAS_PREFIX):
        return m[len(OPENROUTER_ALIAS_PREFIX) :]
    if m.startswith(POE_ALIAS_PREFIX):
        return m[len(POE_ALIAS_PREFIX) :]
    return m


def _catalog_path() -> Path:
    env_path = str(os.getenv("LLM_CATALOG_PATH") or "").strip()
    if env_path:
        return Path(env_path)
    here = Path(__file__).resolve()
    candidates = [
        Path("/app/shared/llm_catalog.json"),
        Path("/shared/llm_catalog.json"),
        Path.cwd() / "shared" / "llm_catalog.json",
        here.parents[3] / "shared" / "llm_catalog.json",
    ]
    for path in candidates:
        if path.exists():
            return path
    candidate_list = ", ".join(str(path) for path in candidates)
    raise FileNotFoundError(f"llm_catalog.json not found; tried: {candidate_list}")


@lru_cache(maxsize=1)
def load_llm_catalog() -> dict:
    path = _catalog_path()
    with path.open("r", encoding="utf-8") as f:
        return json.load(f)


def provider_for_model(model: str | None) -> str:
    m = str(model or "").strip()
    if not m:
        return ""
    if m.startswith(OPENROUTER_ALIAS_PREFIX):
        return "openrouter"
    if m.startswith(POE_ALIAS_PREFIX):
        return "poe"
    catalog = load_llm_catalog()
    for group in ("chat_models", "embedding_models"):
        for item in catalog.get(group, []):
            if str(item.get("id") or "").strip() == m:
                return str(item.get("provider") or "").strip()
    lower = m.lower()
    for provider in catalog.get("providers", []):
        for exact in provider.get("match_exact", []):
            if lower == str(exact or "").strip().lower():
                return str(provider.get("id") or "").strip()
        for prefix in provider.get("match_prefixes", []):
            p = str(prefix or "").strip().lower()
            if p and lower.startswith(p):
                return str(provider.get("id") or "").strip()
    return ""


def provider_config(provider_id: str | None) -> dict | None:
    pid = str(provider_id or "").strip()
    if not pid:
        return None
    catalog = load_llm_catalog()
    for provider in catalog.get("providers", []):
        if str(provider.get("id") or "").strip() == pid:
            return dict(provider)
    return None


def model_pricing(model: str | None) -> dict | None:
    m = str(model or "").strip()
    if not m:
        return None
    resolved = resolve_model_id(m)
    catalog = load_llm_catalog()
    for group in ("chat_models", "embedding_models"):
        for item in catalog.get(group, []):
            if str(item.get("id") or "").strip() == m:
                pricing = item.get("pricing")
                return dict(pricing) if isinstance(pricing, dict) else None
            if resolved != m and str(item.get("id") or "").strip() == resolved:
                pricing = item.get("pricing")
                return dict(pricing) if isinstance(pricing, dict) else None
    return None


def model_capabilities(model: str | None) -> dict | None:
    m = str(model or "").strip()
    if not m:
        return None
    resolved = resolve_model_id(m)
    catalog = load_llm_catalog()
    for group in ("chat_models", "embedding_models"):
        for item in catalog.get(group, []):
            if str(item.get("id") or "").strip() == m:
                capabilities = item.get("capabilities")
                return dict(capabilities) if isinstance(capabilities, dict) else None
            if resolved != m and str(item.get("id") or "").strip() == resolved:
                capabilities = item.get("capabilities")
                return dict(capabilities) if isinstance(capabilities, dict) else None
    return None


def model_supports(model: str | None, capability: str) -> bool:
    capabilities = model_capabilities(model) or {}
    return bool(capabilities.get(capability))


def provider_api_key_header(provider_id: str | None) -> str:
    provider = provider_config(provider_id) or {}
    return str(provider.get("api_key_header") or "").strip()

import os
from collections.abc import Callable

from app.services.llm_catalog import model_pricing, resolve_model_id


def env_optional_float(name: str) -> float | None:
    raw = os.getenv(name)
    if raw is None or raw == "":
        return None
    try:
        return float(raw)
    except Exception:
        return None


def normalize_model_name(model: str, *, use_resolve_model_id: bool = False) -> str:
    normalized = str(model or "").strip()
    if use_resolve_model_id:
        return resolve_model_id(normalized)
    return normalized


def normalize_model_family(
    model: str,
    *,
    legacy_model_pricing: dict | None = None,
    model_families: list[str] | None = None,
    use_resolve_model_id: bool = False,
) -> str:
    normalized = normalize_model_name(model, use_resolve_model_id=use_resolve_model_id)
    if legacy_model_pricing:
        if model_pricing(normalized) is not None:
            return normalized
        for family in sorted(legacy_model_pricing.keys(), key=len, reverse=True):
            if normalized == family or normalized.startswith(family + "-"):
                return family
        return normalized
    if model_families:
        if model_pricing(normalized) is not None:
            return normalized
        for family in model_families:
            if normalized == family or normalized.startswith(family + "-"):
                return family
    return normalized


def pricing_for_model(
    model: str,
    purpose: str,
    *,
    env_prefix: str,
    pricing_source_version: str,
    legacy_model_pricing: dict | None = None,
    normalize_model_family_func: Callable[[str], str],
) -> dict:
    family = normalize_model_family_func(model)
    catalog_pricing = model_pricing(family) or model_pricing(model)
    if legacy_model_pricing:
        base = dict(catalog_pricing or legacy_model_pricing.get(family, {"input_per_mtok_usd": 0.0, "output_per_mtok_usd": 0.0, "cache_read_per_mtok_usd": 0.0}))
    else:
        base = dict(catalog_pricing or {"input_per_mtok_usd": 0.0, "output_per_mtok_usd": 0.0, "cache_read_per_mtok_usd": 0.0})
    source = str(base.get("pricing_source") or pricing_source_version)
    prefix = f"{env_prefix}_{purpose.upper()}_"
    override_map = {
        "input_per_mtok_usd": env_optional_float(prefix + "INPUT_PER_MTOK_USD"),
        "output_per_mtok_usd": env_optional_float(prefix + "OUTPUT_PER_MTOK_USD"),
        "cache_read_per_mtok_usd": env_optional_float(prefix + "CACHE_READ_PER_MTOK_USD"),
    }
    for key, value in override_map.items():
        if value is not None:
            base[key] = value
            source = "env_override"
    base["pricing_source"] = source
    base["pricing_model_family"] = family
    return base


def estimate_cost_usd(
    model: str,
    purpose: str,
    usage: dict,
    *,
    pricing_for_model_func: Callable[[str, str], dict],
) -> float:
    pricing = pricing_for_model_func(model, purpose)
    non_cached_input_tokens = max(0, int(usage.get("input_tokens", 0) or 0) - int(usage.get("cache_read_input_tokens", 0) or 0))
    total = 0.0
    total += non_cached_input_tokens / 1_000_000 * pricing["input_per_mtok_usd"]
    total += int(usage.get("output_tokens", 0) or 0) / 1_000_000 * pricing["output_per_mtok_usd"]
    total += int(usage.get("cache_read_input_tokens", 0) or 0) / 1_000_000 * pricing.get("cache_read_per_mtok_usd", 0.0)
    return round(total, 8)

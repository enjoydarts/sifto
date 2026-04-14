import os
import time

import anthropic


def env_timeout_seconds(name: str, default: float) -> float:
    raw = os.getenv(name)
    if raw is None or raw == "":
        return default
    try:
        v = float(raw)
        return v if v > 0 else default
    except Exception:
        return default


def client_for_api_key(
    api_key: str | None,
    *,
    base_url: str | None = None,
    default_headers: dict[str, str] | None = None,
):
    if api_key:
        kwargs = {"api_key": api_key}
        if base_url:
            kwargs["base_url"] = base_url
        if default_headers:
            kwargs["default_headers"] = default_headers
        return anthropic.Anthropic(**kwargs)
    return None


def async_client_for_api_key(
    api_key: str | None,
    *,
    base_url: str | None = None,
    default_headers: dict[str, str] | None = None,
):
    if api_key:
        kwargs = {"api_key": api_key}
        if base_url:
            kwargs["base_url"] = base_url
        if default_headers:
            kwargs["default_headers"] = default_headers
        return anthropic.AsyncAnthropic(**kwargs)
    return None


def message_usage(message) -> dict:
    usage = getattr(message, "usage", None)
    if usage is None:
        return {
            "input_tokens": 0,
            "output_tokens": 0,
            "cache_creation_input_tokens": 0,
            "cache_read_input_tokens": 0,
        }
    return {
        "input_tokens": int(getattr(usage, "input_tokens", 0) or 0),
        "output_tokens": int(getattr(usage, "output_tokens", 0) or 0),
        "cache_creation_input_tokens": int(getattr(usage, "cache_creation_input_tokens", 0) or 0),
        "cache_read_input_tokens": int(getattr(usage, "cache_read_input_tokens", 0) or 0),
    }


def messages_create(
    prompt: str,
    model: str,
    max_tokens: int = 1024,
    api_key: str | None = None,
    timeout_sec: float | None = None,
    system_prompt: str | None = None,
    user_prompt: str | None = None,
    enable_prompt_cache: bool = False,
    temperature: float | None = None,
    top_p: float | None = None,
    base_url: str | None = None,
    default_headers: dict[str, str] | None = None,
):
    client = client_for_api_key(api_key, base_url=base_url, default_headers=default_headers)
    if client is None:
        return None
    req_timeout = timeout_sec if timeout_sec and timeout_sec > 0 else env_timeout_seconds("ANTHROPIC_TIMEOUT_SEC", 300.0)
    kwargs = {
        "model": model,
        "max_tokens": max_tokens,
        "timeout": req_timeout,
    }
    if temperature is not None:
        kwargs["temperature"] = temperature
    if top_p is not None:
        kwargs["top_p"] = top_p
    if system_prompt is not None:
        system_block: dict = {"type": "text", "text": system_prompt}
        if enable_prompt_cache:
            system_block["cache_control"] = {"type": "ephemeral"}
            kwargs["extra_headers"] = {"anthropic-beta": "prompt-caching-2024-07-31"}
        kwargs["system"] = [system_block]
        kwargs["messages"] = [{"role": "user", "content": user_prompt or prompt}]
    else:
        kwargs["messages"] = [{"role": "user", "content": prompt}]
    return client.messages.create(**kwargs)


def is_rate_limit_error(err: Exception) -> bool:
    s = str(err).lower()
    return "429" in s or "rate_limit" in s


def call_with_retries(
    prompt: str,
    model: str,
    max_tokens: int,
    retries: int = 2,
    api_key: str | None = None,
    timeout_sec: float | None = None,
    system_prompt: str | None = None,
    user_prompt: str | None = None,
    enable_prompt_cache: bool = False,
    temperature: float | None = None,
    top_p: float | None = None,
    base_url: str | None = None,
    default_headers: dict[str, str] | None = None,
    provider_label: str = "anthropic",
    logger=None,
):
    last_err = None
    for attempt in range(retries + 1):
        try:
            return messages_create(
                prompt,
                model,
                max_tokens=max_tokens,
                api_key=api_key,
                timeout_sec=timeout_sec,
                system_prompt=system_prompt,
                user_prompt=user_prompt,
                enable_prompt_cache=enable_prompt_cache,
                temperature=temperature,
                top_p=top_p,
                base_url=base_url,
                default_headers=default_headers,
            )
        except Exception as e:
            last_err = e
            if attempt >= retries or not is_rate_limit_error(e):
                raise
            sleep_sec = 1.0 * (2**attempt)
            if logger is not None:
                logger.warning(
                    "%s rate-limited model=%s retry_in=%.1fs attempt=%d/%d",
                    provider_label,
                    model,
                    sleep_sec,
                    attempt + 1,
                    retries + 1,
                )
            time.sleep(sleep_sec)
    if last_err is not None:
        raise last_err
    return None


def call_with_model_fallback(
    prompt: str,
    primary_model: str,
    fallback_model: str | None,
    max_tokens: int = 1024,
    api_key: str | None = None,
    timeout_sec: float | None = None,
    system_prompt: str | None = None,
    user_prompt: str | None = None,
    enable_prompt_cache: bool = False,
    temperature: float | None = None,
    top_p: float | None = None,
    base_url: str | None = None,
    default_headers: dict[str, str] | None = None,
    provider_label: str = "anthropic",
    logger=None,
):
    if client_for_api_key(api_key, base_url=base_url, default_headers=default_headers) is None:
        return None, None, []
    failures = []
    try:
        return (
            call_with_retries(
                prompt,
                primary_model,
                max_tokens=max_tokens,
                api_key=api_key,
                timeout_sec=timeout_sec,
                system_prompt=system_prompt,
                user_prompt=user_prompt,
                enable_prompt_cache=enable_prompt_cache,
                temperature=temperature,
                top_p=top_p,
                base_url=base_url,
                default_headers=default_headers,
                provider_label=provider_label,
                logger=logger,
            ),
            primary_model,
            failures,
        )
    except Exception as e:
        failures.append({"model": primary_model, "reason": str(e)})
        if logger is not None:
            logger.warning("%s call failed model=%s err=%s", provider_label, primary_model, e)
        if fallback_model and fallback_model != primary_model:
            try:
                return (
                    call_with_retries(
                        prompt,
                        fallback_model,
                        max_tokens=max_tokens,
                        api_key=api_key,
                        timeout_sec=timeout_sec,
                        system_prompt=system_prompt,
                        user_prompt=user_prompt,
                        enable_prompt_cache=enable_prompt_cache,
                        temperature=temperature,
                        top_p=top_p,
                        base_url=base_url,
                        default_headers=default_headers,
                        provider_label=provider_label,
                        logger=logger,
                    ),
                    fallback_model,
                    failures,
                )
            except Exception as e2:
                failures.append({"model": fallback_model, "reason": str(e2)})
                if logger is not None:
                    logger.warning("%s fallback failed model=%s err=%s", provider_label, fallback_model, e2)
        return None, None, failures


async def messages_create_async(
    prompt: str,
    model: str,
    max_tokens: int = 1024,
    api_key: str | None = None,
    timeout_sec: float | None = None,
    system_prompt: str | None = None,
    user_prompt: str | None = None,
    enable_prompt_cache: bool = False,
    temperature: float | None = None,
    top_p: float | None = None,
    base_url: str | None = None,
    default_headers: dict[str, str] | None = None,
):
    client = async_client_for_api_key(api_key, base_url=base_url, default_headers=default_headers)
    if client is None:
        return None
    req_timeout = timeout_sec if timeout_sec and timeout_sec > 0 else env_timeout_seconds("ANTHROPIC_TIMEOUT_SEC", 300.0)
    kwargs = {
        "model": model,
        "max_tokens": max_tokens,
        "timeout": req_timeout,
    }
    if temperature is not None:
        kwargs["temperature"] = temperature
    if top_p is not None:
        kwargs["top_p"] = top_p
    if system_prompt is not None:
        system_block: dict = {"type": "text", "text": system_prompt}
        if enable_prompt_cache:
            system_block["cache_control"] = {"type": "ephemeral"}
            kwargs["extra_headers"] = {"anthropic-beta": "prompt-caching-2024-07-31"}
        kwargs["system"] = [system_block]
        kwargs["messages"] = [{"role": "user", "content": user_prompt or prompt}]
    else:
        kwargs["messages"] = [{"role": "user", "content": prompt}]
    return await client.messages.create(**kwargs)


async def call_with_retries_async(
    prompt: str,
    model: str,
    max_tokens: int,
    retries: int = 2,
    api_key: str | None = None,
    timeout_sec: float | None = None,
    system_prompt: str | None = None,
    user_prompt: str | None = None,
    enable_prompt_cache: bool = False,
    temperature: float | None = None,
    top_p: float | None = None,
    base_url: str | None = None,
    default_headers: dict[str, str] | None = None,
    provider_label: str = "anthropic",
    logger=None,
):
    import asyncio
    last_err = None
    for attempt in range(retries + 1):
        try:
            return await messages_create_async(
                prompt,
                model,
                max_tokens=max_tokens,
                api_key=api_key,
                timeout_sec=timeout_sec,
                system_prompt=system_prompt,
                user_prompt=user_prompt,
                enable_prompt_cache=enable_prompt_cache,
                temperature=temperature,
                top_p=top_p,
                base_url=base_url,
                default_headers=default_headers,
            )
        except Exception as e:
            last_err = e
            if attempt >= retries or not is_rate_limit_error(e):
                raise
            sleep_sec = 1.0 * (2**attempt)
            if logger is not None:
                logger.warning(
                    "%s rate-limited model=%s retry_in=%.1fs attempt=%d/%d",
                    provider_label,
                    model,
                    sleep_sec,
                    attempt + 1,
                    retries + 1,
                )
            await asyncio.sleep(sleep_sec)
    if last_err is not None:
        raise last_err
    return None


async def call_with_model_fallback_async(
    prompt: str,
    primary_model: str,
    fallback_model: str | None,
    max_tokens: int = 1024,
    api_key: str | None = None,
    timeout_sec: float | None = None,
    system_prompt: str | None = None,
    user_prompt: str | None = None,
    enable_prompt_cache: bool = False,
    temperature: float | None = None,
    top_p: float | None = None,
    base_url: str | None = None,
    default_headers: dict[str, str] | None = None,
    provider_label: str = "anthropic",
    logger=None,
):
    if async_client_for_api_key(api_key, base_url=base_url, default_headers=default_headers) is None:
        return None, None, []
    failures = []
    try:
        return (
            await call_with_retries_async(
                prompt,
                primary_model,
                max_tokens=max_tokens,
                api_key=api_key,
                timeout_sec=timeout_sec,
                system_prompt=system_prompt,
                user_prompt=user_prompt,
                enable_prompt_cache=enable_prompt_cache,
                temperature=temperature,
                top_p=top_p,
                base_url=base_url,
                default_headers=default_headers,
                provider_label=provider_label,
                logger=logger,
            ),
            primary_model,
            failures,
        )
    except Exception as e:
        failures.append({"model": primary_model, "reason": str(e)})
        if logger is not None:
            logger.warning("%s call failed model=%s err=%s", provider_label, primary_model, e)
        if fallback_model and fallback_model != primary_model:
            try:
                return (
                    await call_with_retries_async(
                        prompt,
                        fallback_model,
                        max_tokens=max_tokens,
                        api_key=api_key,
                        timeout_sec=timeout_sec,
                        system_prompt=system_prompt,
                        user_prompt=user_prompt,
                        enable_prompt_cache=enable_prompt_cache,
                        temperature=temperature,
                        top_p=top_p,
                        base_url=base_url,
                        default_headers=default_headers,
                        provider_label=provider_label,
                        logger=logger,
                    ),
                    fallback_model,
                    failures,
                )
            except Exception as e2:
                failures.append({"model": fallback_model, "reason": str(e2)})
                if logger is not None:
                    logger.warning("%s fallback failed model=%s err=%s", provider_label, fallback_model, e2)
        return None, None, failures

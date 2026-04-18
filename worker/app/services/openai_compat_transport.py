import asyncio
import httpx
import time
import json
import os
import threading
from contextlib import asynccontextmanager, contextmanager


_PROVIDER_CONCURRENCY_LOCK = threading.Lock()
_PROVIDER_CONCURRENCY_SEMAPHORES: dict[tuple[str, int], threading.Semaphore] = {}


def _provider_max_concurrency(provider_name: str) -> int | None:
    provider = str(provider_name or "").strip().lower()
    if provider != "zai":
        return None
    raw = str(os.getenv("ZAI_MAX_CONCURRENCY", "1") or "1").strip()
    try:
        value = int(raw)
    except Exception:
        return 1
    return value if value > 0 else None


@contextmanager
def _provider_concurrency_guard(provider_name: str):
    limit = _provider_max_concurrency(provider_name)
    if limit is None:
        yield
        return
    key = (str(provider_name or "").strip().lower(), limit)
    with _PROVIDER_CONCURRENCY_LOCK:
        semaphore = _PROVIDER_CONCURRENCY_SEMAPHORES.get(key)
        if semaphore is None:
            semaphore = threading.Semaphore(limit)
            _PROVIDER_CONCURRENCY_SEMAPHORES[key] = semaphore
    semaphore.acquire()
    try:
        yield
    finally:
        semaphore.release()


def _append_execution_failure(usage: dict, model: str, reason: str) -> None:
    model_name = str(model or "").strip()
    failure_reason = str(reason or "").strip()
    if not model_name or not failure_reason:
        return
    failures = usage.setdefault("execution_failures", [])
    if isinstance(failures, list):
        failures.append({"model": model_name, "reason": failure_reason})


def _safe_sorted_keys(value) -> list[str]:
    if not isinstance(value, dict):
        return []
    return sorted(str(k) for k in value.keys())


def _content_shape(content) -> tuple[str, int]:
    if isinstance(content, list):
        text_parts = [str(part.get("text") or "") for part in content if isinstance(part, dict)]
        return "list", len("\n".join(text_parts).strip())
    if content is None:
        return "none", 0
    text = str(content)
    return type(content).__name__, len(text.strip())


def _log_empty_message_content(logger, provider_name: str, model: str, body: dict, data: dict, choice: dict, message: dict) -> None:
    if logger is None or not hasattr(logger, "warning"):
        return
    content_type, content_length = _content_shape(message.get("content"))
    tool_calls = message.get("tool_calls")
    refusal = str(message.get("refusal") or "").strip()
    logger.warning(
        "%s chat.completions returned empty message content model=%s response_format=%s content_type=%s content_length=%d finish_reason=%s tool_calls_count=%d refusal_present=%s provider_field=%s top_level_keys=%s choice_keys=%s message_keys=%s",
        provider_name,
        model,
        str((body.get("response_format") or {}).get("type") or ""),
        content_type,
        content_length,
        str(choice.get("finish_reason") or ""),
        len(tool_calls) if isinstance(tool_calls, list) else 0,
        "true" if refusal else "false",
        str(data.get("provider") or ""),
        _safe_sorted_keys(data),
        _safe_sorted_keys(choice),
        _safe_sorted_keys(message),
    )


def _should_retry_empty_json(body: dict, choice: dict, text: str) -> bool:
    if str(text or "").strip() != "":
        return False
    response_format = body.get("response_format") or {}
    if str(response_format.get("type") or "").strip() != "json_object":
        return False
    return True


def usage_from_chat_response(data: dict) -> dict:
    usage = data.get("usage") or {}
    prompt_details = usage.get("prompt_tokens_details") or {}
    cached_tokens = (
        usage.get("prompt_cache_hit_tokens")
        or usage.get("cache_read_input_tokens")
        or prompt_details.get("cached_tokens")
        or usage.get("cached_tokens")
        or 0
    )
    billed_cost = usage.get("cost")
    usage_payload = {
        "input_tokens": int(usage.get("prompt_tokens", 0) or 0),
        "output_tokens": int(usage.get("completion_tokens", 0) or 0),
        "cache_creation_input_tokens": 0,
        "cache_read_input_tokens": int(cached_tokens or 0),
        "resolved_model": str(data.get("model") or "").strip() or None,
    }
    if billed_cost is not None:
        try:
            usage_payload["billed_cost_usd"] = float(billed_cost)
        except Exception:
            pass
    generation_id = str(data.get("id") or "").strip()
    if generation_id:
        usage_payload["generation_id"] = generation_id
    return usage_payload


def run_chat_json(
    prompt: str,
    model: str,
    api_key: str,
    *,
    url: str,
    normalize_model_name,
    supports_strict_schema,
    timeout_sec: float,
    attempts: int,
    base_sleep_sec: float,
    provider_name: str,
    logger,
    system_instruction: str | None = None,
    max_output_tokens: int = 1200,
    response_schema: dict | None = None,
    schema_name: str = "response",
    include_temperature: bool = True,
    temperature: float | None = None,
    top_p: float | None = None,
    auth_header_name: str = "Authorization",
    auth_scheme: str = "Bearer",
) -> tuple[str, dict]:
    body: dict = {
        "model": normalize_model_name(model),
        "messages": [],
        "max_tokens": max_output_tokens,
    }
    if include_temperature:
        body["temperature"] = temperature if temperature is not None else 0.2
    if top_p is not None:
        body["top_p"] = top_p
    if system_instruction:
        body["messages"].append({"role": "system", "content": system_instruction})
    body["messages"].append({"role": "user", "content": prompt})

    use_strict_schema = response_schema is not None and supports_strict_schema(model)
    if response_schema is not None:
        if use_strict_schema:
            body["response_format"] = {
                "type": "json_schema",
                "json_schema": {
                    "name": schema_name,
                    "strict": True,
                    "schema": response_schema,
                },
            }
        else:
            body["response_format"] = {"type": "json_object"}
    if provider_name in {"zai", "moonshot"}:
        # Some OpenAI-compatible providers enable thinking by default, which can
        # exhaust output tokens into reasoning_content and leave message.content empty.
        body["thinking"] = {"type": "disabled"}

    auth_value = f"{auth_scheme} {api_key}".strip() if auth_scheme else api_key
    headers = {
        auth_header_name: auth_value,
        "Content-Type": "application/json",
    }
    retryable_status = {408, 409, 429, 500, 502, 503, 504}
    resp: httpx.Response | None = None
    last_error: Exception | None = None
    retry_usage: dict = {}
    requested_model = str(model or "").strip() or None

    def is_json_validation_error(response: httpx.Response) -> bool:
        return response.status_code == 400 and "json_validate_failed" in (response.text or "")

    with _provider_concurrency_guard(provider_name):
        for i in range(attempts):
            try:
                with httpx.Client(timeout=timeout_sec) as client:
                    resp = client.post(url, headers=headers, json=body)
                    if is_json_validation_error(resp) and response_schema is not None:
                        if use_strict_schema:
                            fallback_body = dict(body)
                            fallback_body["response_format"] = {"type": "json_object"}
                            resp = client.post(url, headers=headers, json=fallback_body)
                        if is_json_validation_error(resp):
                            fallback_body = dict(body)
                            fallback_body.pop("response_format", None)
                            resp = client.post(url, headers=headers, json=fallback_body)
            except Exception as e:
                last_error = e
                if i < attempts - 1:
                    _append_execution_failure(retry_usage, requested_model, f"request failed: {e}")
                    sleep_sec = base_sleep_sec * (2**i)
                    logger.warning(
                        "%s chat.completions request failed model=%s retry_in=%.1fs attempt=%d/%d err=%s",
                        provider_name,
                        normalize_model_name(model),
                        sleep_sec,
                        i + 1,
                        attempts,
                        e,
                    )
                    time.sleep(sleep_sec)
                    continue
                raise RuntimeError(f"{provider_name} chat.completions request failed: {e}") from e

            if resp.status_code < 400:
                data = resp.json() if resp.content else {}
                choices = data.get("choices") or []
                if not choices:
                    snippet = ""
                    try:
                        snippet = json.dumps(data, ensure_ascii=False)[:1000]
                    except Exception:
                        snippet = (resp.text or "")[:1000]
                    if snippet:
                        raise RuntimeError(f"{provider_name} chat.completions failed: empty choices body={snippet}")
                    raise RuntimeError(f"{provider_name} chat.completions failed: empty choices")
                message = choices[0].get("message") or {}
                content = message.get("content")
                if isinstance(content, list):
                    text = "\n".join(str(part.get("text") or "") for part in content if isinstance(part, dict))
                else:
                    text = str(content or "")
                text = text.strip()
                if text == "":
                    _log_empty_message_content(logger, provider_name, normalize_model_name(model), body, data, choices[0], message)
                    if i < attempts - 1 and _should_retry_empty_json(body, choices[0], text):
                        finish_reason = str(choices[0].get("finish_reason") or "").strip() or "unknown"
                        _append_execution_failure(
                            retry_usage,
                            requested_model,
                            f"empty_json_content finish_reason={finish_reason} provider={str(data.get('provider') or '').strip() or 'unknown'}",
                        )
                        sleep_sec = base_sleep_sec * (2**i)
                        logger.warning(
                            "%s chat.completions retrying model=%s reason=empty_json_content finish_reason=%s retry_in=%.1fs attempt=%d/%d",
                            provider_name,
                            normalize_model_name(model),
                            finish_reason,
                            sleep_sec,
                            i + 1,
                            attempts,
                        )
                        time.sleep(sleep_sec)
                        continue
                usage = usage_from_chat_response(data)
                usage["requested_model"] = requested_model
                if retry_usage.get("execution_failures"):
                    usage["execution_failures"] = list(retry_usage["execution_failures"])
                return text, usage
            if resp.status_code in retryable_status and i < attempts - 1:
                _append_execution_failure(retry_usage, requested_model, f"status={resp.status_code} body={resp.text[:1000]}")
                sleep_sec = base_sleep_sec * (2**i)
                logger.warning(
                    "%s chat.completions retrying model=%s status=%d retry_in=%.1fs attempt=%d/%d",
                    provider_name,
                    normalize_model_name(model),
                    resp.status_code,
                    sleep_sec,
                    i + 1,
                    attempts,
                )
                time.sleep(sleep_sec)
                continue
            break

    if resp is None:
        if last_error is not None:
            raise RuntimeError(f"{provider_name} chat.completions request failed: {last_error}") from last_error
        raise RuntimeError(f"{provider_name} chat.completions failed: no response")
    if resp.status_code >= 400:
        raise RuntimeError(f"{provider_name} chat.completions failed status={resp.status_code} body={resp.text[:1000]}")
    raise RuntimeError(f"{provider_name} chat.completions failed: unexpected retry exit")


_ASYNC_PROVIDER_CONCURRENCY_LOCK = asyncio.Lock()
_ASYNC_PROVIDER_CONCURRENCY_SEMAPHORES: dict[tuple[str, int], asyncio.Semaphore] = {}


@asynccontextmanager
async def _provider_concurrency_guard_async(provider_name: str):
    limit = _provider_max_concurrency(provider_name)
    if limit is None:
        yield
        return
    key = (str(provider_name or "").strip().lower(), limit)
    async with _ASYNC_PROVIDER_CONCURRENCY_LOCK:
        semaphore = _ASYNC_PROVIDER_CONCURRENCY_SEMAPHORES.get(key)
        if semaphore is None:
            semaphore = asyncio.Semaphore(limit)
            _ASYNC_PROVIDER_CONCURRENCY_SEMAPHORES[key] = semaphore
    await semaphore.acquire()
    try:
        yield
    finally:
        semaphore.release()


async def run_chat_json_async(
    prompt: str,
    model: str,
    api_key: str,
    *,
    url: str,
    normalize_model_name,
    supports_strict_schema,
    timeout_sec: float,
    attempts: int,
    base_sleep_sec: float,
    provider_name: str,
    logger,
    system_instruction: str | None = None,
    max_output_tokens: int = 1200,
    response_schema: dict | None = None,
    schema_name: str = "response",
    include_temperature: bool = True,
    temperature: float | None = None,
    top_p: float | None = None,
    auth_header_name: str = "Authorization",
    auth_scheme: str = "Bearer",
) -> tuple[str, dict]:
    body: dict = {
        "model": normalize_model_name(model),
        "messages": [],
        "max_tokens": max_output_tokens,
    }
    if include_temperature:
        body["temperature"] = temperature if temperature is not None else 0.2
    if top_p is not None:
        body["top_p"] = top_p
    if system_instruction:
        body["messages"].append({"role": "system", "content": system_instruction})
    body["messages"].append({"role": "user", "content": prompt})

    use_strict_schema = response_schema is not None and supports_strict_schema(model)
    if response_schema is not None:
        if use_strict_schema:
            body["response_format"] = {
                "type": "json_schema",
                "json_schema": {
                    "name": schema_name,
                    "strict": True,
                    "schema": response_schema,
                },
            }
        else:
            body["response_format"] = {"type": "json_object"}
    if provider_name in {"zai", "moonshot"}:
        body["thinking"] = {"type": "disabled"}

    auth_value = f"{auth_scheme} {api_key}".strip() if auth_scheme else api_key
    headers = {
        auth_header_name: auth_value,
        "Content-Type": "application/json",
    }
    retryable_status = {408, 409, 429, 500, 502, 503, 504}
    resp: httpx.Response | None = None
    last_error: Exception | None = None
    retry_usage: dict = {}
    requested_model = str(model or "").strip() or None

    def is_json_validation_error(response: httpx.Response) -> bool:
        return response.status_code == 400 and "json_validate_failed" in (response.text or "")

    async with _provider_concurrency_guard_async(provider_name):
        for i in range(attempts):
            try:
                # Per-request client is intentional: timeout_sec varies across
                # callers and httpx does not support per-request timeout
                # overrides when the client has a default timeout set.
                async with httpx.AsyncClient(timeout=timeout_sec) as client:
                    resp = await client.post(url, headers=headers, json=body)
                    if is_json_validation_error(resp) and response_schema is not None:
                        if use_strict_schema:
                            fallback_body = dict(body)
                            fallback_body["response_format"] = {"type": "json_object"}
                            resp = await client.post(url, headers=headers, json=fallback_body)
                        if is_json_validation_error(resp):
                            fallback_body = dict(body)
                            fallback_body.pop("response_format", None)
                            resp = await client.post(url, headers=headers, json=fallback_body)
            except Exception as e:
                last_error = e
                if i < attempts - 1:
                    _append_execution_failure(retry_usage, requested_model, f"request failed: {e}")
                    sleep_sec = base_sleep_sec * (2**i)
                    logger.warning(
                        "%s chat.completions request failed model=%s retry_in=%.1fs attempt=%d/%d err=%s",
                        provider_name,
                        normalize_model_name(model),
                        sleep_sec,
                        i + 1,
                        attempts,
                        e,
                    )
                    await asyncio.sleep(sleep_sec)
                    continue
                raise RuntimeError(f"{provider_name} chat.completions request failed: {e}") from e

            if resp.status_code < 400:
                data = resp.json() if resp.content else {}
                choices = data.get("choices") or []
                if not choices:
                    snippet = ""
                    try:
                        snippet = json.dumps(data, ensure_ascii=False)[:1000]
                    except Exception:
                        snippet = (resp.text or "")[:1000]
                    if snippet:
                        raise RuntimeError(f"{provider_name} chat.completions failed: empty choices body={snippet}")
                    raise RuntimeError(f"{provider_name} chat.completions failed: empty choices")
                message = choices[0].get("message") or {}
                content = message.get("content")
                if isinstance(content, list):
                    text = "\n".join(str(part.get("text") or "") for part in content if isinstance(part, dict))
                else:
                    text = str(content or "")
                text = text.strip()
                if text == "":
                    _log_empty_message_content(logger, provider_name, normalize_model_name(model), body, data, choices[0], message)
                    if i < attempts - 1 and _should_retry_empty_json(body, choices[0], text):
                        finish_reason = str(choices[0].get("finish_reason") or "").strip() or "unknown"
                        _append_execution_failure(
                            retry_usage,
                            requested_model,
                            f"empty_json_content finish_reason={finish_reason} provider={str(data.get('provider') or '').strip() or 'unknown'}",
                        )
                        sleep_sec = base_sleep_sec * (2**i)
                        logger.warning(
                            "%s chat.completions retrying model=%s reason=empty_json_content finish_reason=%s retry_in=%.1fs attempt=%d/%d",
                            provider_name,
                            normalize_model_name(model),
                            finish_reason,
                            sleep_sec,
                            i + 1,
                            attempts,
                        )
                        await asyncio.sleep(sleep_sec)
                        continue
                usage = usage_from_chat_response(data)
                usage["requested_model"] = requested_model
                if retry_usage.get("execution_failures"):
                    usage["execution_failures"] = list(retry_usage["execution_failures"])
                return text, usage
            if resp.status_code in retryable_status and i < attempts - 1:
                _append_execution_failure(retry_usage, requested_model, f"status={resp.status_code} body={resp.text[:1000]}")
                sleep_sec = base_sleep_sec * (2**i)
                logger.warning(
                    "%s chat.completions retrying model=%s status=%d retry_in=%.1fs attempt=%d/%d",
                    provider_name,
                    normalize_model_name(model),
                    resp.status_code,
                    sleep_sec,
                    i + 1,
                    attempts,
                )
                await asyncio.sleep(sleep_sec)
                continue
            break

    if resp is None:
        if last_error is not None:
            raise RuntimeError(f"{provider_name} chat.completions request failed: {last_error}") from last_error
        raise RuntimeError(f"{provider_name} chat.completions failed: no response")
    if resp.status_code >= 400:
        raise RuntimeError(f"{provider_name} chat.completions failed status={resp.status_code} body={resp.text[:1000]}")
    raise RuntimeError(f"{provider_name} chat.completions failed: unexpected retry exit")

import asyncio
import contextvars
import httpx
import time
import json
import os
import threading
from contextlib import asynccontextmanager, contextmanager

try:
    import redis
except Exception:  # pragma: no cover
    redis = None


_PROVIDER_CONCURRENCY_LOCK = threading.Lock()
_PROVIDER_CONCURRENCY_SEMAPHORES: dict[tuple[str, int], threading.Semaphore] = {}
_PROVIDER_REQUEST_USER_ID = contextvars.ContextVar("provider_request_user_id", default="")
_REDIS_CLIENT = None
_REDIS_CLIENT_LOCK = threading.Lock()


@contextmanager
def provider_request_context(user_id: str | None = None):
    token = _PROVIDER_REQUEST_USER_ID.set(str(user_id or "").strip())
    try:
        yield
    finally:
        _PROVIDER_REQUEST_USER_ID.reset(token)


def _is_qwen_model(model: str) -> bool:
    return "qwen" in str(model or "").strip().lower()


def _is_moonshot_model(model: str) -> bool:
    normalized = str(model or "").strip().lower()
    return normalized.startswith("moonshotai/") or normalized.startswith("kimi-")


def _is_kimi_k2x_model(model: str) -> bool:
    normalized = str(model or "").strip().lower()
    return normalized in {
        "moonshotai/kimi-k2.5",
        "kimi-k2.5",
        "moonshotai/kimi-k2.6",
        "kimi-k2.6",
    }


def _is_glm_model(model: str) -> bool:
    normalized = str(model or "").strip().lower()
    return normalized.startswith("zai-org/glm-") or normalized.startswith("glm-")


def _is_deepseek_v4_model(model: str) -> bool:
    normalized = str(model or "").strip().lower()
    return normalized in {"deepseek-v4-flash", "deepseek-v4-pro"}


def _is_gpt_oss_model(model: str) -> bool:
    return str(model or "").strip().lower() == "gpt-oss-120b"


def _apply_openai_compat_request_overrides(provider_name: str, normalized_model: str, body: dict) -> None:
    if provider_name == "cerebras":
        if _is_gpt_oss_model(normalized_model):
            # GPT-OSS can spend the whole output budget on reasoning and return
            # empty JSON content. Keep reasoning minimal for structured tasks.
            body["reasoning_effort"] = "low"
            return
        if normalized_model == "zai-glm-4.7":
            body["reasoning_effort"] = "none"
            return
    if provider_name in {"zai", "moonshot"}:
        # Some OpenAI-compatible providers enable thinking by default, which can
        # exhaust output tokens into reasoning_content and leave message.content empty.
        body["thinking"] = {"type": "disabled"}
        return
    if provider_name == "deepseek" and _is_deepseek_v4_model(normalized_model):
        body["thinking"] = {"type": "disabled"}
        return
    if provider_name == "deepinfra":
        if _is_kimi_k2x_model(normalized_model) or _is_glm_model(normalized_model):
            body["thinking"] = {"type": "disabled"}
            return
        if _is_qwen_model(normalized_model):
            body["chat_template_kwargs"] = {"enable_thinking": False}
            return
    if provider_name != "featherless":
        return
    if _is_kimi_k2x_model(normalized_model):
        body["thinking"] = {"type": "disabled"}
        body["reasoning"] = {"enabled": False}
        body["chat_template_kwargs"] = {"enable_thinking": False}
        return
    if _is_glm_model(normalized_model):
        body["thinking"] = {"type": "disabled"}
        return
    if _is_moonshot_model(normalized_model) or _is_qwen_model(normalized_model):
        # OpenAI SDK users would pass this via extra_body; on the raw HTTP body
        # it must be included as a top-level field.
        body["chat_template_kwargs"] = {"enable_thinking": False}


def _provider_max_concurrency(provider_name: str) -> int | None:
    provider = str(provider_name or "").strip().lower()
    defaults = {
        "zai": 1,
        "featherless": 1,
    }
    maximums = {
        "featherless": 1,
    }
    default = defaults.get(provider)
    if default is None:
        return None
    env_name = f"{provider.upper()}_MAX_CONCURRENCY"
    raw = str(os.getenv(env_name, str(default)) or str(default)).strip()
    try:
        value = int(raw)
    except Exception:
        value = default
    if value <= 0:
        return None
    maximum = maximums.get(provider)
    if maximum is not None and value > maximum:
        return maximum
    return value


def _env_positive_int(name: str, default: int) -> int:
    raw = str(os.getenv(name, str(default)) or str(default)).strip()
    try:
        value = int(raw)
    except Exception:
        value = default
    return value if value > 0 else default


def _provider_user_max_concurrency(provider_name: str) -> int | None:
    provider = str(provider_name or "").strip().lower()
    if provider != "featherless":
        return None
    raw = str(os.getenv("FEATHERLESS_USER_MAX_CONCURRENCY", "1") or "1").strip()
    try:
        value = int(raw)
    except Exception:
        value = 1
    if value <= 0:
        return None
    return min(value, 1)


def _provider_user_concurrency_wait_sec() -> int:
    return _env_positive_int("FEATHERLESS_USER_CONCURRENCY_WAIT_SEC", 5)


def _redis_client(logger):
    global _REDIS_CLIENT
    if redis is None:
        return None
    with _REDIS_CLIENT_LOCK:
        if _REDIS_CLIENT is not None:
            return _REDIS_CLIENT
        redis_url = os.getenv("REDIS_URL") or os.getenv("UPSTASH_REDIS_URL") or ""
        if not redis_url:
            return None
        try:
            _REDIS_CLIENT = redis.Redis.from_url(redis_url, decode_responses=True)
        except Exception as exc:
            if logger is not None and hasattr(logger, "warning"):
                logger.warning("provider concurrency redis init failed: %s", exc)
            _REDIS_CLIENT = None
        return _REDIS_CLIENT


_REDIS_ACQUIRE_SCRIPT = """
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
local count = redis.call('ZCARD', KEYS[1])
if count < tonumber(ARGV[2]) then
  redis.call('ZADD', KEYS[1], ARGV[3], ARGV[4])
  redis.call('PEXPIRE', KEYS[1], ARGV[5])
  return 1
end
return 0
"""


class _RedisProviderLease:
    def __init__(self, client, key: str, member: str, ttl_ms: int):
        self.client = client
        self.key = key
        self.member = member
        self.ttl_ms = ttl_ms

    def refresh(self) -> None:
        now_ms = int(time.time() * 1000)
        try:
            self.client.zadd(self.key, {self.member: now_ms + self.ttl_ms})
            self.client.pexpire(self.key, self.ttl_ms)
        except Exception:
            pass

    def release(self) -> None:
        try:
            self.client.zrem(self.key, self.member)
        except Exception:
            pass


def _acquire_redis_provider_lease(provider_name: str, logger) -> _RedisProviderLease | None:
    limit = _provider_user_max_concurrency(provider_name)
    user_id = str(_PROVIDER_REQUEST_USER_ID.get() or "").strip()
    if limit is None or not user_id:
        return None
    client = _redis_client(logger)
    if client is None:
        return None
    ttl_sec = _env_positive_int("FEATHERLESS_USER_CONCURRENCY_TTL_SEC", 900)
    wait_sec = _provider_user_concurrency_wait_sec()
    poll_sec = max(float(os.getenv("FEATHERLESS_USER_CONCURRENCY_POLL_SEC", "0.25") or "0.25"), 0.05)
    ttl_ms = ttl_sec * 1000
    key = f"sifto:llm-concurrency:{provider_name}:user:{user_id}"
    member = f"{os.getpid()}:{threading.get_ident()}:{time.time_ns()}"
    deadline = time.time() + wait_sec
    while True:
        now_ms = int(time.time() * 1000)
        try:
            acquired = int(client.eval(_REDIS_ACQUIRE_SCRIPT, 1, key, now_ms, limit, now_ms + ttl_ms, member, ttl_ms) or 0)
        except Exception as exc:
            if logger is not None and hasattr(logger, "warning"):
                logger.warning("%s redis concurrency acquire failed user_id=%s: %s", provider_name, user_id, exc)
            return None
        if acquired == 1:
            return _RedisProviderLease(client, key, member, ttl_ms)
        if time.time() >= deadline:
            raise RuntimeError(f"{provider_name} user concurrency wait timeout user_id={user_id} limit={limit}")
        time.sleep(poll_sec)


@contextmanager
def _provider_user_concurrency_guard(provider_name: str, logger):
    lease = _acquire_redis_provider_lease(provider_name, logger)
    if lease is None:
        yield
        return
    try:
        yield
    finally:
        lease.release()


@asynccontextmanager
async def _provider_user_concurrency_guard_async(provider_name: str, logger):
    lease = await asyncio.to_thread(_acquire_redis_provider_lease, provider_name, logger)
    if lease is None:
        yield
        return
    try:
        yield
    finally:
        await asyncio.to_thread(lease.release)


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
    normalized_model = str(normalize_model_name(model) or "")
    _apply_openai_compat_request_overrides(provider_name, normalized_model, body)

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

    with _provider_user_concurrency_guard(provider_name, logger), _provider_concurrency_guard(provider_name):
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
    normalized_model = str(normalize_model_name(model) or "")
    _apply_openai_compat_request_overrides(provider_name, normalized_model, body)

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

    async with _provider_user_concurrency_guard_async(provider_name, logger), _provider_concurrency_guard_async(provider_name):
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

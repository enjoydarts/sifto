import os
import time

import httpx


def _env_timeout_seconds(name: str, default: float) -> float:
    raw = os.getenv(name)
    if raw is None or raw == "":
        return default
    try:
        v = float(raw)
        return v if v > 0 else default
    except Exception:
        return default


def usage_from_responses(data: dict) -> dict:
    usage = data.get("usage") or {}
    input_details = usage.get("input_tokens_details") or {}
    return {
        "input_tokens": int(usage.get("input_tokens", 0) or 0),
        "output_tokens": int(usage.get("output_tokens", 0) or 0),
        "cache_creation_input_tokens": 0,
        "cache_read_input_tokens": int(input_details.get("cached_tokens", 0) or 0),
    }


def extract_text_from_responses(data: dict) -> str:
    output_text = str(data.get("output_text") or "").strip()
    if output_text:
        return output_text
    out: list[str] = []
    for item in data.get("output") or []:
        if not isinstance(item, dict):
            continue
        for content in item.get("content") or []:
            if not isinstance(content, dict):
                continue
            text = str(content.get("text") or content.get("output_text") or "").strip()
            if text:
                out.append(text)
            if str(content.get("type") or "").strip() == "refusal":
                refusal_text = str(content.get("refusal") or "").strip()
                if refusal_text:
                    out.append(refusal_text)
        refusal = str(item.get("refusal") or "").strip()
        if refusal:
            out.append(refusal)
    return "\n".join(v for v in out if v).strip()


def run_responses_json(
    prompt: str,
    model: str,
    api_key: str,
    *,
    normalize_model_name,
    responses_reasoning,
    supports_strict_schema,
    system_instruction: str | None = None,
    max_output_tokens: int = 1200,
    response_schema: dict | None = None,
    schema_name: str = "response",
    timeout_sec: float | None = None,
    temperature: float | None = None,
    top_p: float | None = None,
) -> tuple[str, dict]:
    body: dict = {
        "model": normalize_model_name(model),
        "input": prompt,
        "max_output_tokens": max_output_tokens,
    }
    if system_instruction:
        body["instructions"] = system_instruction
    if temperature is not None:
        body["temperature"] = temperature
    if top_p is not None:
        body["top_p"] = top_p
    reasoning = responses_reasoning(model)
    if reasoning is not None:
        body["reasoning"] = reasoning
    use_strict_schema = response_schema is not None and supports_strict_schema(model)
    if response_schema is not None:
        if use_strict_schema:
            body["text"] = {
                "format": {
                    "type": "json_schema",
                    "name": schema_name,
                    "schema": response_schema,
                    "strict": True,
                }
            }
        else:
            body["text"] = {"format": {"type": "json_object"}}
    url = "https://api.openai.com/v1/responses"
    headers = {
        "Authorization": f"Bearer {api_key}",
        "Content-Type": "application/json",
    }
    req_timeout = timeout_sec if timeout_sec and timeout_sec > 0 else _env_timeout_seconds("OPENAI_TIMEOUT_SEC", 90.0)
    attempts = max(1, int(os.getenv("OPENAI_RETRY_ATTEMPTS", "3") or "3"))
    base_sleep_sec = _env_timeout_seconds("OPENAI_RETRY_BASE_SEC", 0.5)
    retryable_status = {408, 409, 429, 500, 502, 503, 504}
    resp: httpx.Response | None = None
    last_error: Exception | None = None
    with httpx.Client(timeout=req_timeout) as client:
        for i in range(attempts):
            try:
                resp = client.post(url, headers=headers, json=body)
                if resp.status_code == 400 and response_schema is not None and "format" in (resp.text or ""):
                    fallback_body = dict(body)
                    fallback_body.pop("text", None)
                    resp = client.post(url, headers=headers, json=fallback_body)
            except Exception as e:
                last_error = e
                if i < attempts - 1:
                    time.sleep(base_sleep_sec * (2**i))
                    continue
                raise RuntimeError(f"openai responses request failed: {e}") from e
            if resp.status_code < 400:
                break
            if resp.status_code in retryable_status and i < attempts - 1:
                time.sleep(base_sleep_sec * (2**i))
                continue
            break
    if resp is None:
        if last_error is not None:
            raise RuntimeError(f"openai responses request failed: {last_error}") from last_error
        raise RuntimeError("openai responses failed: no response")
    if resp.status_code >= 400:
        raise RuntimeError(f"openai responses failed status={resp.status_code} body={resp.text[:1000]}")
    data = resp.json() if resp.content else {}
    text = extract_text_from_responses(data)
    if not text and response_schema is not None:
        fallback_body = dict(body)
        fallback_body.pop("text", None)
        with httpx.Client(timeout=req_timeout) as client:
            resp = client.post(url, headers=headers, json=fallback_body)
        if resp.status_code >= 400:
            raise RuntimeError(f"openai responses fallback failed status={resp.status_code} body={resp.text[:1000]}")
        data = resp.json() if resp.content else {}
        text = extract_text_from_responses(data)
    if not text:
        status = str(data.get("status") or "")
        incomplete = data.get("incomplete_details")
        raise RuntimeError(f"openai responses returned empty output status={status} incomplete={incomplete}")
    return text, usage_from_responses(data)

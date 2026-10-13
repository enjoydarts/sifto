import contextvars
import logging
import os
import sys
from contextlib import contextmanager, nullcontext


_log = logging.getLogger(__name__)
_prompt_refs_var = contextvars.ContextVar("langfuse_prompt_refs", default=())
_current_span_var = contextvars.ContextVar("langfuse_current_span", default=None)

try:  # pragma: no cover - optional dependency
    from langfuse import get_client as _langfuse_get_client
except Exception:  # pragma: no cover
    _langfuse_get_client = None

try:  # pragma: no cover - optional dependency
    from langfuse import propagate_attributes as _langfuse_propagate_attributes
except Exception:  # pragma: no cover
    _langfuse_propagate_attributes = None


def enabled() -> bool:
    return _env_flag("LANGFUSE_ENABLED", default="0")


def prompt_override_enabled() -> bool:
    return _env_flag("LANGFUSE_PROMPT_OVERRIDE_ENABLED", default="0")


def _env_flag(name: str, *, default: str = "0") -> bool:
    value = os.getenv(name, default).strip()
    value = value.strip('"').strip("'").strip().lower()
    return value not in ("", "0", "false", "no", "off")


def _normalize_langfuse_env() -> None:
    base_url = os.getenv("LANGFUSE_BASE_URL", "").strip()
    legacy_host = os.getenv("LANGFUSE_HOST", "").strip()
    if not base_url and legacy_host:
        os.environ["LANGFUSE_BASE_URL"] = legacy_host


def runtime_status() -> dict[str, object]:
    _normalize_langfuse_env()
    base_url = os.getenv("LANGFUSE_BASE_URL", "").strip()
    public_key = os.getenv("LANGFUSE_PUBLIC_KEY", "").strip()
    secret_key = os.getenv("LANGFUSE_SECRET_KEY", "").strip()
    is_enabled = enabled()
    prompt_override = prompt_override_enabled()
    sdk_imported = _langfuse_get_client is not None
    keys_configured = bool(public_key and secret_key)
    base_url_configured = bool(base_url)
    ready = is_enabled and sdk_imported and keys_configured and base_url_configured
    return {
        "enabled": is_enabled,
        "prompt_override_enabled": prompt_override,
        "sdk_imported": sdk_imported,
        "base_url_configured": base_url_configured,
        "keys_configured": keys_configured,
        "ready": ready,
        "base_url": base_url,
    }


def log_runtime_status() -> None:
    status = runtime_status()
    if not status["enabled"]:
        _log.info("langfuse disabled")
        return
    if not status["sdk_imported"]:
        _log.warning("langfuse enabled but SDK is not installed in worker image")
        return
    if not status["base_url_configured"]:
        _log.warning("langfuse enabled but LANGFUSE_BASE_URL is missing")
        return
    if not status["keys_configured"]:
        _log.warning("langfuse enabled but LANGFUSE_PUBLIC_KEY or LANGFUSE_SECRET_KEY is missing")
        return
    _log.info(
        "langfuse enabled base_url=%s prompt_override=%s",
        status["base_url"],
        status["prompt_override_enabled"],
    )


def _client():
    if not enabled() or _langfuse_get_client is None:
        return None
    _normalize_langfuse_env()
    try:
        return _langfuse_get_client()
    except Exception as e:  # pragma: no cover
        _log.warning("langfuse client unavailable: %s", e)
        return None


def _append_prompt_ref(ref: dict) -> None:
    current = list(_prompt_refs_var.get(()))
    current.append(ref)
    _prompt_refs_var.set(tuple(current))


@contextmanager
def bind_span(current_span):
    token = _current_span_var.set(current_span)
    try:
        yield
    finally:
        _current_span_var.reset(token)


def update_current(
    *,
    input=None,
    output=None,
    metadata=None,
    level=None,
    status_message=None,
    model: str | None = None,
    model_parameters: dict | None = None,
    usage_details: dict | None = None,
    cost_details: dict | None = None,
) -> None:
    kwargs = {}
    if input is not None:
        kwargs["input"] = input
    if output is not None:
        kwargs["output"] = output
    if metadata is not None:
        kwargs["metadata"] = metadata
    if level is not None:
        kwargs["level"] = level
    if status_message is not None:
        kwargs["status_message"] = status_message
    if model:
        kwargs["model"] = model
    if model_parameters:
        kwargs["model_parameters"] = model_parameters
    if usage_details:
        kwargs["usage_details"] = usage_details
    if cost_details:
        kwargs["cost_details"] = cost_details
    if not kwargs:
        return
    current_span = _current_span_var.get()
    if current_span is not None:
        try:  # pragma: no cover
            current_span.update(**kwargs)
            return
        except Exception as e:
            _log.warning("langfuse span.update failed: %s", e)
    client = _client()
    if client is None:
        return
    try:  # pragma: no cover
        client.update_current_span(**kwargs)
    except Exception as e:
        _log.warning("langfuse update_current_span failed: %s", e)


def score_current(name: str, value, *, comment: str | None = None, data_type: str | None = None) -> None:
    kwargs = {"name": name, "value": value}
    if comment:
        kwargs["comment"] = comment
    if data_type:
        kwargs["data_type"] = data_type
    current_span = _current_span_var.get()
    if current_span is not None:
        try:  # pragma: no cover
            current_span.score(**kwargs)
            return
        except Exception as e:
            _log.warning("langfuse span.score failed: %s", e)
    client = _client()
    if client is None:
        return
    try:  # pragma: no cover
        client.score_current_span(**kwargs)
    except Exception as e:
        _log.warning("langfuse score_current_span failed: %s", e)


def _update_legacy_trace(client, attributes: dict) -> None:
    if not attributes:
        return
    try:
        # SDK 3 keeps these trace-level fields; SDK 4 uses observation propagation.
        update_trace = getattr(client, "update_current_trace", None)
        if callable(update_trace):
            update_trace(**attributes)
    except Exception as e:
        _log.warning("langfuse update_current_trace failed: %s", e)


def _extract_prompt_text(prompt_obj) -> str:
    if prompt_obj is None:
        return ""
    if isinstance(prompt_obj, str):
        return prompt_obj
    for attr in ("prompt", "text", "content"):
        value = getattr(prompt_obj, attr, None)
        if isinstance(value, str) and value.strip():
            return value.strip()
    nested = getattr(prompt_obj, "prompt", None)
    if nested is not None and nested is not prompt_obj:
        return _extract_prompt_text(nested)
    return ""


def _render_prompt(prompt_obj, fallback: str, variables: dict[str, object] | None) -> str:
    vars_dict = variables or {}
    if hasattr(prompt_obj, "compile"):
        try:
            rendered = prompt_obj.compile(**vars_dict)
            if isinstance(rendered, str) and rendered.strip():
                return rendered.strip()
        except Exception:
            pass
    text = _extract_prompt_text(prompt_obj) or fallback
    for key, value in vars_dict.items():
        rendered_value = str(value)
        text = text.replace("{{" + key + "}}", rendered_value)
        text = text.replace("{" + key + "}", rendered_value)
    return text


def get_prompt_text(name: str, fallback: str, *, variables: dict[str, object] | None = None, prompt_type: str = "text") -> str:
    if not prompt_override_enabled():
        return fallback
    client = _client()
    if client is None:
        return fallback
    label = os.getenv("LANGFUSE_PROMPT_LABEL", "production").strip() or "production"
    try:  # pragma: no cover
        prompt_obj = client.get_prompt(name, type=prompt_type, label=label)
        rendered = _render_prompt(prompt_obj, fallback, variables)
        version = str(getattr(prompt_obj, "version", "") or getattr(prompt_obj, "commit", "") or "")
        _append_prompt_ref(
            {
                "name": name,
                "label": label,
                "version": version,
                "source": "langfuse",
            }
        )
        update_current(metadata={"langfuse_prompts": list(_prompt_refs_var.get(()))})
        return rendered or fallback
    except Exception as e:
        _log.warning("langfuse get_prompt failed name=%s err=%s", name, e)
        _append_prompt_ref({"name": name, "label": label, "version": "", "source": "code"})
        update_current(metadata={"langfuse_prompts": list(_prompt_refs_var.get(()))})
        return fallback


@contextmanager
def _sdk_context(factory, name: str):
    """Fail open for SDK failures without suppressing application exceptions."""
    try:
        sdk_cm = factory()
        value = sdk_cm.__enter__()
    except Exception as e:
        _log.warning("langfuse context unavailable name=%s err=%s", name, e)
        yield None
        return
    try:
        yield value
    except BaseException:
        try:
            sdk_cm.__exit__(*sys.exc_info())
        except Exception:
            _log.warning("langfuse context exit failed name=%s", name)
        raise
    else:
        try:
            sdk_cm.__exit__(None, None, None)
        except Exception:
            _log.warning("langfuse context exit failed name=%s", name)


@contextmanager
def span(
    name: str, *, input=None, metadata=None, tags=None, as_type: str = "span",
    user_id: str | None = None, session_id: str | None = None,
):
    client = _client()
    token = _prompt_refs_var.set(())
    span_token = _current_span_var.set(None)
    try:
        if client is None:
            yield None
            return

        merged_metadata = dict(metadata or {})
        attributes = {}
        if user_id:
            attributes["user_id"] = user_id
        if session_id:
            attributes["session_id"] = session_id
        if tags:
            attributes["tags"] = list(tags)
            merged_metadata["tags"] = list(tags)
        kwargs = {"name": name, "as_type": as_type}
        if input is not None:
            kwargs["input"] = input
        if merged_metadata:
            kwargs["metadata"] = merged_metadata

        with _sdk_context(lambda: client.start_as_current_observation(**kwargs), name) as current_span:
            _current_span_var.set(current_span)
            if current_span is None:
                yield None
                return
            # Enter after the root exists: propagation sets its attributes as well
            # as those of children, without changing an enclosing request's span.
            attribute_scope = (
                _sdk_context(lambda: _langfuse_propagate_attributes(**attributes), name)
                if attributes and callable(_langfuse_propagate_attributes)
                else nullcontext()
            )
            with attribute_scope:
                _update_legacy_trace(client, attributes)
                yield current_span
    finally:
        _prompt_refs_var.reset(token)
        _current_span_var.reset(span_token)


def flush() -> None:
    client = _client()
    if client is None:
        return
    try:  # pragma: no cover
        client.flush()
    except Exception as e:
        _log.warning("langfuse flush failed: %s", e)

import contextvars
import logging
import os
from contextlib import contextmanager


_log = logging.getLogger(__name__)
_prompt_refs_var = contextvars.ContextVar("langfuse_prompt_refs", default=())

try:  # pragma: no cover - optional dependency
    from langfuse import get_client as _langfuse_get_client
except Exception:  # pragma: no cover
    _langfuse_get_client = None


def enabled() -> bool:
    return os.getenv("LANGFUSE_ENABLED", "0").strip() not in ("", "0", "false", "False")


def prompt_override_enabled() -> bool:
    return os.getenv("LANGFUSE_PROMPT_OVERRIDE_ENABLED", "0").strip() not in ("", "0", "false", "False")


def _normalize_langfuse_env() -> None:
    base_url = os.getenv("LANGFUSE_BASE_URL", "").strip()
    legacy_host = os.getenv("LANGFUSE_HOST", "").strip()
    if not base_url and legacy_host:
        os.environ["LANGFUSE_BASE_URL"] = legacy_host


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


def update_current(*, input=None, output=None, metadata=None, level=None, status_message=None) -> None:
    client = _client()
    if client is None:
        return
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
    if not kwargs:
        return
    try:  # pragma: no cover
        client.update_current_span(**kwargs)
    except Exception as e:
        _log.warning("langfuse update_current_span failed: %s", e)


def score_current(name: str, value, *, comment: str | None = None) -> None:
    client = _client()
    if client is None:
        return
    kwargs = {"name": name, "value": value}
    if comment:
        kwargs["comment"] = comment
    try:  # pragma: no cover
        client.score_current_span(**kwargs)
    except Exception as e:
        _log.warning("langfuse score_current_span failed: %s", e)


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
def span(name: str, *, input=None, metadata=None, tags=None):
    client = _client()
    token = _prompt_refs_var.set(())
    if client is None:
        try:
            yield None
        finally:
            _prompt_refs_var.reset(token)
        return
    kwargs = {"name": name}
    if input is not None:
        kwargs["input"] = input
    if metadata is not None:
        kwargs["metadata"] = metadata
    if tags is not None:
        kwargs["tags"] = tags
    try:  # pragma: no cover
        with client.start_as_current_span(**kwargs) as current_span:
            yield current_span
    except Exception as e:
        _log.warning("langfuse span failed name=%s err=%s", name, e)
        yield None
    finally:
        _prompt_refs_var.reset(token)


def flush() -> None:
    client = _client()
    if client is None:
        return
    try:  # pragma: no cover
        client.flush()
    except Exception as e:
        _log.warning("langfuse flush failed: %s", e)

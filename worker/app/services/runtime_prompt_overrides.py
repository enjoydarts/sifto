import contextvars
from contextlib import contextmanager

from app.services.prompt_template_defaults import render_prompt_template

_prompt_override_var = contextvars.ContextVar("runtime_prompt_override", default=None)


@contextmanager
def bind_prompt_override(prompt_key: str | None, prompt_text: str | None, system_instruction: str | None):
    payload = {
        "prompt_key": str(prompt_key or "").strip(),
        "prompt_text": str(prompt_text or "").strip(),
        "system_instruction": str(system_instruction or "").strip(),
    }
    token = _prompt_override_var.set(payload)
    try:
        yield
    finally:
        _prompt_override_var.reset(token)


class PromptStrategy:
    def render(self, default_system_instruction: str, default_prompt_text: str, variables: dict[str, object] | None = None) -> tuple[str, str]:
        raise NotImplementedError


class DefaultPromptStrategy(PromptStrategy):
    def render(self, default_system_instruction: str, default_prompt_text: str, variables: dict[str, object] | None = None) -> tuple[str, str]:
        return (
            render_prompt_template(default_system_instruction, variables).strip(),
            render_prompt_template(default_prompt_text, variables).strip(),
        )


class OverridePromptStrategy(PromptStrategy):
    def __init__(self, payload: dict):
        self.payload = payload

    def render(self, default_system_instruction: str, default_prompt_text: str, variables: dict[str, object] | None = None) -> tuple[str, str]:
        next_system_template = str(self.payload.get("system_instruction") or "").strip() or default_system_instruction
        next_prompt_template = str(self.payload.get("prompt_text") or "").strip() or default_prompt_text
        return (
            render_prompt_template(next_system_template, variables).strip(),
            render_prompt_template(next_prompt_template, variables).strip(),
        )


def resolve_prompt_strategy(prompt_key: str) -> PromptStrategy:
    current = _prompt_override_var.get()
    if not current:
        return DefaultPromptStrategy()
    current_key = str(current.get("prompt_key") or "").strip()
    if current_key and current_key != prompt_key:
        return DefaultPromptStrategy()
    return OverridePromptStrategy(current)


def apply_prompt_override(prompt_key: str, system_instruction: str, prompt_text: str, variables: dict[str, object] | None = None) -> tuple[str, str]:
    return resolve_prompt_strategy(prompt_key).render(system_instruction, prompt_text, variables)

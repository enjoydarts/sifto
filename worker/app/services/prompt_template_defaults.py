from __future__ import annotations

from functools import lru_cache
from pathlib import Path


def _prompt_templates_dir() -> Path:
    here = Path(__file__).resolve()
    candidates = [
        Path("/app/shared/prompt_templates"),
        Path("/shared/prompt_templates"),
        Path.cwd() / "shared" / "prompt_templates",
        here.parents[3] / "shared" / "prompt_templates",
    ]
    for candidate in candidates:
        if candidate.exists():
            return candidate
    candidate_list = ", ".join(str(path) for path in candidates)
    raise FileNotFoundError(f"prompt_templates directory not found; tried: {candidate_list}")


def render_prompt_template(text: str, variables: dict[str, object] | None = None) -> str:
    rendered = str(text or "")
    for key, value in (variables or {}).items():
        rendered_value = str(value)
        rendered = rendered.replace("{{" + key + "}}", rendered_value)
        rendered = rendered.replace("{" + key + "}", rendered_value)
    return rendered


@lru_cache(maxsize=None)
def get_default_prompt_template(prompt_key: str) -> dict[str, object]:
    base = _prompt_templates_dir()
    return {
        "prompt_key": prompt_key,
        "system_instruction": (base / f"{prompt_key}.system.txt").read_text(encoding="utf-8").strip(),
        "prompt_text": (base / f"{prompt_key}.prompt.txt").read_text(encoding="utf-8").strip(),
    }

from __future__ import annotations

from functools import lru_cache
from pathlib import Path
import re


_PLACEHOLDER_RE = re.compile(r"\{\{([a-zA-Z0-9_]+)\}\}|\{([a-zA-Z0-9_]+)\}")


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
    values = {key: str(value) for key, value in (variables or {}).items()}

    def replace(match: re.Match[str]) -> str:
        key = match.group(1) or match.group(2) or ""
        return values.get(key, match.group(0))

    return _PLACEHOLDER_RE.sub(replace, rendered)


@lru_cache(maxsize=None)
def get_default_prompt_template(prompt_key: str) -> dict[str, object]:
    base = _prompt_templates_dir()
    return {
        "prompt_key": prompt_key,
        "system_instruction": (base / f"{prompt_key}.system.txt").read_text(encoding="utf-8").strip(),
        "prompt_text": (base / f"{prompt_key}.prompt.txt").read_text(encoding="utf-8").strip(),
    }

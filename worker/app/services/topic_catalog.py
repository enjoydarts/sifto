"""The shared, bounded vocabulary used by article summary topics."""

from functools import lru_cache
import json
from pathlib import Path


@lru_cache(maxsize=1)
def load_topic_catalog() -> dict:
    here = Path(__file__).resolve()
    candidates = (
        Path("/app/shared/topic_catalog.json"),
        Path("/shared/topic_catalog.json"),
        Path.cwd() / "shared" / "topic_catalog.json",
        here.parents[3] / "shared" / "topic_catalog.json",
    )
    for path in candidates:
        if path.is_file():
            catalog = json.loads(path.read_text(encoding="utf-8"))
            if catalog.get("version") != 1:
                raise ValueError("unsupported topic catalog version")
            if catalog.get("max_topics_per_article") != 3:
                raise ValueError("unsupported maximum topics per article")
            topics = catalog["topics"]
            if len(topics) != len(set(topics)):
                raise ValueError("topic catalog contains duplicate topics")
            if any(value not in topics for value in catalog["aliases"].values()):
                raise ValueError("topic alias points outside catalog")
            if any(value not in topics for value in catalog["genre_fallback"].values()):
                raise ValueError("genre fallback points outside catalog")
            return catalog
    raise FileNotFoundError("shared/topic_catalog.json not found")


MAX_TOPICS_PER_ARTICLE = load_topic_catalog()["max_topics_per_article"]


def normalize_topics(topics: list | None, genre: str | None) -> list[str]:
    catalog = load_topic_catalog()
    allowed = {topic.casefold(): topic for topic in catalog["topics"]}
    aliases = {alias.casefold(): canonical for alias, canonical in catalog["aliases"].items()}
    normalized = []
    for raw in topics or []:
        if not isinstance(raw, str):
            continue
        value = " ".join(raw.split())
        canonical = allowed.get(value.casefold()) or aliases.get(value.casefold())
        if canonical and canonical not in normalized:
            normalized.append(canonical)
        if len(normalized) == MAX_TOPICS_PER_ARTICLE:
            break
    if not normalized:
        fallback = catalog["genre_fallback"].get(str(genre or "").strip().lower())
        if fallback:
            normalized.append(fallback)
    return normalized

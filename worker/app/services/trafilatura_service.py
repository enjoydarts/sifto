import trafilatura
from trafilatura.settings import use_config


def extract_body(url: str) -> dict | None:
    config = use_config()
    config.set("DEFAULT", "EXTRACTION_TIMEOUT", "30")

    downloaded = trafilatura.fetch_url(url)
    if downloaded is None:
        return None

    result = trafilatura.extract(
        downloaded,
        include_comments=False,
        include_tables=False,
        with_metadata=True,
        output_format="python",
        config=config,
    )
    if result is None:
        return None

    return {
        "title": result.get("title"),
        "content": result.get("text", ""),
        "published_at": result.get("date"),
    }

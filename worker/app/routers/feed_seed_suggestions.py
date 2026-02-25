from fastapi import APIRouter, Request
from pydantic import BaseModel

from app.services.claude_service import suggest_feed_seed_sites

router = APIRouter()


class ExistingSource(BaseModel):
    title: str | None = None
    url: str


class FeedSeedSuggestionRequest(BaseModel):
    existing_sources: list[ExistingSource]
    preferred_topics: list[str] = []
    model: str | None = None


class FeedSeedSuggestionItem(BaseModel):
    url: str
    reason: str


class FeedSeedSuggestionResponse(BaseModel):
    items: list[FeedSeedSuggestionItem]
    llm: dict | None = None


@router.post("/suggest-feed-seed-sites", response_model=FeedSeedSuggestionResponse)
def suggest_feed_seed_sites_endpoint(req: FeedSeedSuggestionRequest, request: Request):
    api_key = request.headers.get("x-anthropic-api-key") or None
    result = suggest_feed_seed_sites(
        existing_sources=[{"title": s.title, "url": s.url} for s in req.existing_sources],
        preferred_topics=req.preferred_topics,
        api_key=api_key,
        model=req.model,
    )
    return FeedSeedSuggestionResponse(**result)

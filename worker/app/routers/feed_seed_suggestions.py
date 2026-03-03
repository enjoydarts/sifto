from fastapi import APIRouter, Request
from pydantic import BaseModel

from app.services.claude_service import suggest_feed_seed_sites
from app.services.gemini_service import suggest_feed_seed_sites as suggest_feed_seed_sites_gemini
from app.services.model_router import is_gemini_model

router = APIRouter()


class ExistingSource(BaseModel):
    title: str | None = None
    url: str


class FeedExample(BaseModel):
    url: str
    title: str | None = None
    reason: str | None = None


class FeedSeedSuggestionRequest(BaseModel):
    existing_sources: list[ExistingSource]
    preferred_topics: list[str] = []
    positive_examples: list[FeedExample] = []
    negative_examples: list[FeedExample] = []
    model: str | None = None


class FeedSeedSuggestionItem(BaseModel):
    url: str
    reason: str


class FeedSeedSuggestionResponse(BaseModel):
    items: list[FeedSeedSuggestionItem]
    llm: dict | None = None


@router.post("/suggest-feed-seed-sites", response_model=FeedSeedSuggestionResponse)
def suggest_feed_seed_sites_endpoint(req: FeedSeedSuggestionRequest, request: Request):
    existing_sources = [{"title": s.title, "url": s.url} for s in req.existing_sources]
    if is_gemini_model(req.model):
        google_api_key = request.headers.get("x-google-api-key") or ""
        result = suggest_feed_seed_sites_gemini(
            existing_sources=existing_sources,
            preferred_topics=req.preferred_topics,
            positive_examples=[{"url": e.url, "title": e.title, "reason": e.reason} for e in req.positive_examples],
            negative_examples=[{"url": e.url, "title": e.title, "reason": e.reason} for e in req.negative_examples],
            model=str(req.model),
            api_key=google_api_key,
        )
    else:
        api_key = request.headers.get("x-anthropic-api-key") or None
        result = suggest_feed_seed_sites(
            existing_sources=existing_sources,
            preferred_topics=req.preferred_topics,
            positive_examples=[{"url": e.url, "title": e.title, "reason": e.reason} for e in req.positive_examples],
            negative_examples=[{"url": e.url, "title": e.title, "reason": e.reason} for e in req.negative_examples],
            api_key=api_key,
            model=req.model,
        )
    return FeedSeedSuggestionResponse(**result)

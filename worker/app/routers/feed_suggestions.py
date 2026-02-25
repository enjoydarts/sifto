from fastapi import APIRouter, Request
from pydantic import BaseModel

from app.services.claude_service import rank_feed_suggestions

router = APIRouter()


class ExistingSource(BaseModel):
    title: str | None = None
    url: str


class CandidateFeed(BaseModel):
    url: str
    title: str | None = None
    reasons: list[str] = []
    matched_topics: list[str] = []


class FeedSuggestionRankRequest(BaseModel):
    existing_sources: list[ExistingSource]
    preferred_topics: list[str] = []
    candidates: list[CandidateFeed]
    model: str | None = None


class FeedSuggestionRankItem(BaseModel):
    url: str
    reason: str
    confidence: float


class FeedSuggestionRankResponse(BaseModel):
    items: list[FeedSuggestionRankItem]
    llm: dict | None = None


@router.post("/rank-feed-suggestions", response_model=FeedSuggestionRankResponse)
def rank_feed_suggestions_endpoint(req: FeedSuggestionRankRequest, request: Request):
    api_key = request.headers.get("x-anthropic-api-key") or None
    result = rank_feed_suggestions(
        existing_sources=[{"title": s.title, "url": s.url} for s in req.existing_sources],
        preferred_topics=req.preferred_topics,
        candidates=[
            {
                "url": c.url,
                "title": c.title,
                "reasons": c.reasons,
                "matched_topics": c.matched_topics,
            }
            for c in req.candidates
        ],
        api_key=api_key,
        model=req.model,
    )
    return FeedSuggestionRankResponse(**result)

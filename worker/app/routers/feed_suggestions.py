from fastapi import APIRouter, Request
from pydantic import BaseModel

from app.services.claude_service import rank_feed_suggestions
from app.services.deepseek_service import rank_feed_suggestions as rank_feed_suggestions_deepseek
from app.services.gemini_service import rank_feed_suggestions as rank_feed_suggestions_gemini
from app.services.groq_service import rank_feed_suggestions as rank_feed_suggestions_groq
from app.services.llm_dispatch import dispatch_by_model
from app.services.openai_service import rank_feed_suggestions as rank_feed_suggestions_openai

router = APIRouter()


class ExistingSource(BaseModel):
    title: str | None = None
    url: str


class FeedExample(BaseModel):
    url: str
    title: str | None = None
    reason: str | None = None


class CandidateFeed(BaseModel):
    id: str | None = None
    url: str
    title: str | None = None
    reasons: list[str] = []
    matched_topics: list[str] = []


class FeedSuggestionRankRequest(BaseModel):
    existing_sources: list[ExistingSource]
    preferred_topics: list[str] = []
    candidates: list[CandidateFeed]
    positive_examples: list[FeedExample] = []
    negative_examples: list[FeedExample] = []
    model: str | None = None


class FeedSuggestionRankItem(BaseModel):
    id: str | None = None
    url: str
    reason: str
    confidence: float


class FeedSuggestionRankResponse(BaseModel):
    items: list[FeedSuggestionRankItem]
    llm: dict | None = None


@router.post("/rank-feed-suggestions", response_model=FeedSuggestionRankResponse)
def rank_feed_suggestions_endpoint(req: FeedSuggestionRankRequest, request: Request):
    existing_sources = [{"title": s.title, "url": s.url} for s in req.existing_sources]
    candidates = [
        {
            "id": c.id,
            "url": c.url,
            "title": c.title,
            "reasons": c.reasons,
            "matched_topics": c.matched_topics,
        }
        for c in req.candidates
    ]
    positive_examples = [{"url": e.url, "title": e.title, "reason": e.reason} for e in req.positive_examples]
    negative_examples = [{"url": e.url, "title": e.title, "reason": e.reason} for e in req.negative_examples]
    result = dispatch_by_model(
        request,
        req.model,
        handlers={
            "anthropic": lambda api_key: rank_feed_suggestions(
                existing_sources=existing_sources,
                preferred_topics=req.preferred_topics,
                candidates=candidates,
                positive_examples=positive_examples,
                negative_examples=negative_examples,
                api_key=api_key,
                model=req.model,
            ),
            "google": lambda api_key: rank_feed_suggestions_gemini(
                existing_sources=existing_sources,
                preferred_topics=req.preferred_topics,
                candidates=candidates,
                positive_examples=positive_examples,
                negative_examples=negative_examples,
                model=str(req.model),
                api_key=api_key or "",
            ),
            "groq": lambda api_key: rank_feed_suggestions_groq(
                existing_sources=existing_sources,
                preferred_topics=req.preferred_topics,
                candidates=candidates,
                positive_examples=positive_examples,
                negative_examples=negative_examples,
                model=str(req.model),
                api_key=api_key or "",
            ),
            "deepseek": lambda api_key: rank_feed_suggestions_deepseek(
                existing_sources=existing_sources,
                preferred_topics=req.preferred_topics,
                candidates=candidates,
                positive_examples=positive_examples,
                negative_examples=negative_examples,
                model=str(req.model),
                api_key=api_key or "",
            ),
            "openai": lambda api_key: rank_feed_suggestions_openai(
                existing_sources=existing_sources,
                preferred_topics=req.preferred_topics,
                candidates=candidates,
                positive_examples=positive_examples,
                negative_examples=negative_examples,
                model=str(req.model),
                api_key=api_key or "",
            ),
        },
    )
    return FeedSuggestionRankResponse(**result)

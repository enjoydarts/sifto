from fastapi import APIRouter, Request
from pydantic import BaseModel

from app.services.claude_service import rank_feed_suggestions
from app.services.llm_dispatch import dispatch_by_model_async
from app.services.router_observe import llm_usage_summary, run_observed_request_async
from app.auto_dispatch import build_handler_map_async

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
async def rank_feed_suggestions_endpoint(req: FeedSuggestionRankRequest, request: Request):
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
    result = await run_observed_request_async(
        request,
        metadata={"model": req.model or "", "existing_sources_count": len(existing_sources), "candidates_count": len(candidates)},
        input_payload={"preferred_topics": req.preferred_topics, "candidates_count": len(candidates), "model": req.model},
        call=lambda: dispatch_by_model_async(
            request,
            req.model,
            handlers=build_handler_map_async(
                "rank_feed_suggestions",
                args_fn=lambda func, api_key: func(existing_sources=existing_sources, preferred_topics=req.preferred_topics, candidates=candidates, positive_examples=positive_examples, negative_examples=negative_examples, model=str(req.model), api_key=api_key or ""),
                anthropic_args_fn=lambda func, api_key: func(existing_sources=existing_sources, preferred_topics=req.preferred_topics, candidates=candidates, positive_examples=positive_examples, negative_examples=negative_examples, api_key=api_key, model=req.model),
            ),
        ),
        output_builder=lambda result: {"items_count": len(result.get("items") or []), **llm_usage_summary(result)},
    )
    return FeedSuggestionRankResponse(**result)

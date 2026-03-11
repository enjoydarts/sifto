from fastapi import APIRouter, Request
from pydantic import BaseModel

from app.services.claude_service import suggest_feed_seed_sites
from app.services.deepseek_service import suggest_feed_seed_sites as suggest_feed_seed_sites_deepseek
from app.services.gemini_service import suggest_feed_seed_sites as suggest_feed_seed_sites_gemini
from app.services.groq_service import suggest_feed_seed_sites as suggest_feed_seed_sites_groq
from app.services.llm_dispatch import dispatch_by_model
from app.services.openai_service import suggest_feed_seed_sites as suggest_feed_seed_sites_openai
from app.services.router_observe import observe_request_input, observe_request_output

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
    positive_examples = [{"url": e.url, "title": e.title, "reason": e.reason} for e in req.positive_examples]
    negative_examples = [{"url": e.url, "title": e.title, "reason": e.reason} for e in req.negative_examples]
    observe_request_input(
        metadata={"model": req.model or "", "existing_sources_count": len(existing_sources), "preferred_topics_count": len(req.preferred_topics or [])},
        input_payload={"preferred_topics": req.preferred_topics, "model": req.model},
    )
    result = dispatch_by_model(
        request,
        req.model,
        handlers={
            "anthropic": lambda api_key: suggest_feed_seed_sites(
                existing_sources=existing_sources,
                preferred_topics=req.preferred_topics,
                positive_examples=positive_examples,
                negative_examples=negative_examples,
                api_key=api_key,
                model=req.model,
            ),
            "google": lambda api_key: suggest_feed_seed_sites_gemini(
                existing_sources=existing_sources,
                preferred_topics=req.preferred_topics,
                positive_examples=positive_examples,
                negative_examples=negative_examples,
                model=str(req.model),
                api_key=api_key or "",
            ),
            "groq": lambda api_key: suggest_feed_seed_sites_groq(
                existing_sources=existing_sources,
                preferred_topics=req.preferred_topics,
                positive_examples=positive_examples,
                negative_examples=negative_examples,
                model=str(req.model),
                api_key=api_key or "",
            ),
            "deepseek": lambda api_key: suggest_feed_seed_sites_deepseek(
                existing_sources=existing_sources,
                preferred_topics=req.preferred_topics,
                positive_examples=positive_examples,
                negative_examples=negative_examples,
                model=str(req.model),
                api_key=api_key or "",
            ),
            "openai": lambda api_key: suggest_feed_seed_sites_openai(
                existing_sources=existing_sources,
                preferred_topics=req.preferred_topics,
                positive_examples=positive_examples,
                negative_examples=negative_examples,
                model=str(req.model),
                api_key=api_key or "",
            ),
        },
    )
    observe_request_output({"items_count": len(result.get("items") or []), "llm_model": ((result.get("llm") or {}).get("model") or "")})
    return FeedSeedSuggestionResponse(**result)

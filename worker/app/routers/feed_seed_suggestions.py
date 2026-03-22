from fastapi import APIRouter, Request
from pydantic import BaseModel

from app.services.alibaba_service import suggest_feed_seed_sites as suggest_feed_seed_sites_alibaba
from app.services.claude_service import suggest_feed_seed_sites
from app.services.deepseek_service import suggest_feed_seed_sites as suggest_feed_seed_sites_deepseek
from app.services.fireworks_service import suggest_feed_seed_sites as suggest_feed_seed_sites_fireworks
from app.services.gemini_service import suggest_feed_seed_sites as suggest_feed_seed_sites_gemini
from app.services.groq_service import suggest_feed_seed_sites as suggest_feed_seed_sites_groq
from app.services.llm_dispatch import dispatch_by_model
from app.services.mistral_service import suggest_feed_seed_sites as suggest_feed_seed_sites_mistral
from app.services.openai_service import suggest_feed_seed_sites as suggest_feed_seed_sites_openai
from app.services.openrouter_service import suggest_feed_seed_sites as suggest_feed_seed_sites_openrouter
from app.services.poe_service import suggest_feed_seed_sites as suggest_feed_seed_sites_poe
from app.services.xai_service import suggest_feed_seed_sites as suggest_feed_seed_sites_xai
from app.services.zai_service import suggest_feed_seed_sites as suggest_feed_seed_sites_zai
from app.services.router_observe import llm_usage_summary, run_observed_request

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
    title: str | None = None
    reason: str


class FeedSeedSuggestionResponse(BaseModel):
    items: list[FeedSeedSuggestionItem]
    llm: dict | None = None


@router.post("/suggest-feed-seed-sites", response_model=FeedSeedSuggestionResponse)
def suggest_feed_seed_sites_endpoint(req: FeedSeedSuggestionRequest, request: Request):
    existing_sources = [{"title": s.title, "url": s.url} for s in req.existing_sources]
    positive_examples = [{"url": e.url, "title": e.title, "reason": e.reason} for e in req.positive_examples]
    negative_examples = [{"url": e.url, "title": e.title, "reason": e.reason} for e in req.negative_examples]
    result = run_observed_request(
        request,
        metadata={"model": req.model or "", "existing_sources_count": len(existing_sources), "preferred_topics_count": len(req.preferred_topics or [])},
        input_payload={"preferred_topics": req.preferred_topics, "model": req.model},
        call=lambda: dispatch_by_model(
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
                "fireworks": lambda api_key: suggest_feed_seed_sites_fireworks(
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
                "alibaba": lambda api_key: suggest_feed_seed_sites_alibaba(
                    existing_sources=existing_sources,
                    preferred_topics=req.preferred_topics,
                    positive_examples=positive_examples,
                    negative_examples=negative_examples,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "mistral": lambda api_key: suggest_feed_seed_sites_mistral(
                    existing_sources=existing_sources,
                    preferred_topics=req.preferred_topics,
                    positive_examples=positive_examples,
                    negative_examples=negative_examples,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "xai": lambda api_key: suggest_feed_seed_sites_xai(
                    existing_sources=existing_sources,
                    preferred_topics=req.preferred_topics,
                    positive_examples=positive_examples,
                    negative_examples=negative_examples,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "zai": lambda api_key: suggest_feed_seed_sites_zai(
                    existing_sources=existing_sources,
                    preferred_topics=req.preferred_topics,
                    positive_examples=positive_examples,
                    negative_examples=negative_examples,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "openrouter": lambda api_key: suggest_feed_seed_sites_openrouter(
                    existing_sources=existing_sources,
                    preferred_topics=req.preferred_topics,
                    positive_examples=positive_examples,
                    negative_examples=negative_examples,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "poe": lambda api_key: suggest_feed_seed_sites_poe(
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
        ),
        output_builder=lambda result: {"items_count": len(result.get("items") or []), **llm_usage_summary(result)},
    )
    return FeedSeedSuggestionResponse(**result)

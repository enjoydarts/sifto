from fastapi import APIRouter, Request
from pydantic import BaseModel

from app.services.alibaba_service import generate_briefing_navigator as generate_briefing_navigator_alibaba
from app.services.claude_service import generate_briefing_navigator
from app.services.deepseek_service import generate_briefing_navigator as generate_briefing_navigator_deepseek
from app.services.fireworks_service import generate_briefing_navigator as generate_briefing_navigator_fireworks
from app.services.gemini_service import generate_briefing_navigator as generate_briefing_navigator_gemini
from app.services.groq_service import generate_briefing_navigator as generate_briefing_navigator_groq
from app.services.llm_dispatch import dispatch_by_model
from app.services.mistral_service import generate_briefing_navigator as generate_briefing_navigator_mistral
from app.services.moonshot_service import generate_briefing_navigator as generate_briefing_navigator_moonshot
from app.services.openai_service import generate_briefing_navigator as generate_briefing_navigator_openai
from app.services.openrouter_service import generate_briefing_navigator as generate_briefing_navigator_openrouter
from app.services.poe_service import generate_briefing_navigator as generate_briefing_navigator_poe
from app.services.router_observe import llm_usage_summary, run_observed_request
from app.services.xai_service import generate_briefing_navigator as generate_briefing_navigator_xai
from app.services.zai_service import generate_briefing_navigator as generate_briefing_navigator_zai

router = APIRouter()


class BriefingNavigatorCandidate(BaseModel):
    item_id: str
    title: str | None = None
    translated_title: str | None = None
    source_title: str | None = None
    summary: str
    topics: list[str] = []
    published_at: str | None = None
    score: float | None = None


class BriefingNavigatorRequest(BaseModel):
    persona: str = "editor"
    candidates: list[BriefingNavigatorCandidate]
    intro_context: dict
    model: str | None = None


class BriefingNavigatorPick(BaseModel):
    item_id: str
    comment: str
    reason_tags: list[str] = []


class BriefingNavigatorResponse(BaseModel):
    intro: str
    picks: list[BriefingNavigatorPick]
    llm: dict | None = None


@router.post("/briefing-navigator", response_model=BriefingNavigatorResponse)
def generate_briefing_navigator_endpoint(req: BriefingNavigatorRequest, request: Request):
    candidates = [
        {
            "item_id": c.item_id,
            "title": c.title,
            "translated_title": c.translated_title,
            "source_title": c.source_title,
            "summary": c.summary,
            "topics": c.topics,
            "published_at": c.published_at,
            "score": c.score,
        }
        for c in req.candidates
    ]
    result = run_observed_request(
        request,
        metadata={"model": req.model or "", "persona": req.persona, "candidates_count": len(candidates)},
        input_payload={"persona": req.persona, "candidates_count": len(candidates), "model": req.model},
        call=lambda: dispatch_by_model(
            request,
            req.model,
            handlers={
                "anthropic": lambda api_key: generate_briefing_navigator(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    api_key=api_key,
                    model=req.model,
                ),
                "google": lambda api_key: generate_briefing_navigator_gemini(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "fireworks": lambda api_key: generate_briefing_navigator_fireworks(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "groq": lambda api_key: generate_briefing_navigator_groq(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "deepseek": lambda api_key: generate_briefing_navigator_deepseek(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "alibaba": lambda api_key: generate_briefing_navigator_alibaba(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "mistral": lambda api_key: generate_briefing_navigator_mistral(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "moonshot": lambda api_key: generate_briefing_navigator_moonshot(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "xai": lambda api_key: generate_briefing_navigator_xai(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "zai": lambda api_key: generate_briefing_navigator_zai(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "openrouter": lambda api_key: generate_briefing_navigator_openrouter(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "poe": lambda api_key: generate_briefing_navigator_poe(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "openai": lambda api_key: generate_briefing_navigator_openai(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
            },
        ),
        output_builder=lambda result: {"items_count": len(result.get("picks") or []), **llm_usage_summary(result)},
    )
    return BriefingNavigatorResponse(**result)

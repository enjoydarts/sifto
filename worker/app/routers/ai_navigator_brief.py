from fastapi import APIRouter, Request
from pydantic import BaseModel

from app.services.alibaba_service import compose_ai_navigator_brief as compose_ai_navigator_brief_alibaba
from app.services.claude_service import compose_ai_navigator_brief
from app.services.deepseek_service import compose_ai_navigator_brief as compose_ai_navigator_brief_deepseek
from app.services.fireworks_service import compose_ai_navigator_brief as compose_ai_navigator_brief_fireworks
from app.services.gemini_service import compose_ai_navigator_brief as compose_ai_navigator_brief_gemini
from app.services.groq_service import compose_ai_navigator_brief as compose_ai_navigator_brief_groq
from app.services.llm_dispatch import dispatch_by_model
from app.services.mistral_service import compose_ai_navigator_brief as compose_ai_navigator_brief_mistral
from app.services.moonshot_service import compose_ai_navigator_brief as compose_ai_navigator_brief_moonshot
from app.services.openai_service import compose_ai_navigator_brief as compose_ai_navigator_brief_openai
from app.services.openrouter_service import compose_ai_navigator_brief as compose_ai_navigator_brief_openrouter
from app.services.poe_service import compose_ai_navigator_brief as compose_ai_navigator_brief_poe
from app.services.siliconflow_service import compose_ai_navigator_brief as compose_ai_navigator_brief_siliconflow
from app.services.router_observe import llm_usage_summary, run_observed_request
from app.services.xai_service import compose_ai_navigator_brief as compose_ai_navigator_brief_xai
from app.services.zai_service import compose_ai_navigator_brief as compose_ai_navigator_brief_zai

router = APIRouter()


class AINavigatorBriefCandidate(BaseModel):
    item_id: str
    title: str | None = None
    translated_title: str | None = None
    source_title: str | None = None
    summary: str
    topics: list[str] = []
    published_at: str | None = None
    score: float | None = None


class AINavigatorBriefRequest(BaseModel):
    persona: str = "editor"
    candidates: list[AINavigatorBriefCandidate]
    intro_context: dict
    model: str | None = None


class AINavigatorBriefItem(BaseModel):
    item_id: str
    comment: str
    reason_tags: list[str] = []


class AINavigatorBriefResponse(BaseModel):
    title: str
    intro: str
    summary: str
    ending: str
    items: list[AINavigatorBriefItem]
    llm: dict | None = None


@router.post("/ai-navigator-brief", response_model=AINavigatorBriefResponse)
def compose_ai_navigator_brief_endpoint(req: AINavigatorBriefRequest, request: Request):
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
                "anthropic": lambda api_key: compose_ai_navigator_brief(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    api_key=api_key,
                    model=req.model,
                ),
                "google": lambda api_key: compose_ai_navigator_brief_gemini(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "fireworks": lambda api_key: compose_ai_navigator_brief_fireworks(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "groq": lambda api_key: compose_ai_navigator_brief_groq(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "deepseek": lambda api_key: compose_ai_navigator_brief_deepseek(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "alibaba": lambda api_key: compose_ai_navigator_brief_alibaba(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "mistral": lambda api_key: compose_ai_navigator_brief_mistral(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "moonshot": lambda api_key: compose_ai_navigator_brief_moonshot(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "xai": lambda api_key: compose_ai_navigator_brief_xai(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "zai": lambda api_key: compose_ai_navigator_brief_zai(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "openrouter": lambda api_key: compose_ai_navigator_brief_openrouter(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "poe": lambda api_key: compose_ai_navigator_brief_poe(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "siliconflow": lambda api_key: compose_ai_navigator_brief_siliconflow(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "openai": lambda api_key: compose_ai_navigator_brief_openai(
                    persona=req.persona,
                    candidates=candidates,
                    intro_context=req.intro_context,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
            },
        ),
        output_builder=lambda result: {"items_count": len(result.get("items") or []), **llm_usage_summary(result)},
    )
    return AINavigatorBriefResponse(**result)

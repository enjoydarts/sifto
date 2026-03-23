from fastapi import APIRouter, Request
from pydantic import BaseModel

from app.services.alibaba_service import generate_source_navigator as generate_source_navigator_alibaba
from app.services.claude_service import generate_source_navigator
from app.services.deepseek_service import generate_source_navigator as generate_source_navigator_deepseek
from app.services.fireworks_service import generate_source_navigator as generate_source_navigator_fireworks
from app.services.gemini_service import generate_source_navigator as generate_source_navigator_gemini
from app.services.groq_service import generate_source_navigator as generate_source_navigator_groq
from app.services.llm_dispatch import dispatch_by_model
from app.services.mistral_service import generate_source_navigator as generate_source_navigator_mistral
from app.services.openai_service import generate_source_navigator as generate_source_navigator_openai
from app.services.openrouter_service import generate_source_navigator as generate_source_navigator_openrouter
from app.services.poe_service import generate_source_navigator as generate_source_navigator_poe
from app.services.router_observe import llm_usage_summary, run_observed_request
from app.services.xai_service import generate_source_navigator as generate_source_navigator_xai
from app.services.zai_service import generate_source_navigator as generate_source_navigator_zai

router = APIRouter()


class SourceNavigatorCandidate(BaseModel):
    source_id: str
    title: str
    url: str
    enabled: bool
    status: str
    last_fetched_at: str | None = None
    last_item_at: str | None = None
    total_items_30d: int = 0
    unread_items_30d: int = 0
    read_items_30d: int = 0
    favorite_count_30d: int = 0
    avg_items_per_day_30d: float = 0
    active_days_30d: int = 0
    avg_items_per_active_day_30d: float = 0
    failure_rate: float = 0


class SourceNavigatorRequest(BaseModel):
    persona: str = "editor"
    candidates: list[SourceNavigatorCandidate]
    model: str | None = None


class SourceNavigatorPick(BaseModel):
    source_id: str
    comment: str


class SourceNavigatorResponse(BaseModel):
    overview: str
    keep: list[SourceNavigatorPick] = []
    watch: list[SourceNavigatorPick] = []
    standout: list[SourceNavigatorPick] = []
    llm: dict | None = None


@router.post("/source-navigator", response_model=SourceNavigatorResponse)
def generate_source_navigator_endpoint(req: SourceNavigatorRequest, request: Request):
    candidates = [candidate.model_dump() for candidate in req.candidates]
    result = run_observed_request(
        request,
        metadata={"model": req.model or "", "persona": req.persona, "candidates_count": len(candidates)},
        input_payload={"persona": req.persona, "candidates_count": len(candidates), "model": req.model},
        call=lambda: dispatch_by_model(
            request,
            req.model,
            handlers={
                "anthropic": lambda api_key: generate_source_navigator(persona=req.persona, candidates=candidates, api_key=api_key, model=req.model),
                "google": lambda api_key: generate_source_navigator_gemini(persona=req.persona, candidates=candidates, model=str(req.model), api_key=api_key or ""),
                "fireworks": lambda api_key: generate_source_navigator_fireworks(persona=req.persona, candidates=candidates, model=str(req.model), api_key=api_key or ""),
                "groq": lambda api_key: generate_source_navigator_groq(persona=req.persona, candidates=candidates, model=str(req.model), api_key=api_key or ""),
                "deepseek": lambda api_key: generate_source_navigator_deepseek(persona=req.persona, candidates=candidates, model=str(req.model), api_key=api_key or ""),
                "alibaba": lambda api_key: generate_source_navigator_alibaba(persona=req.persona, candidates=candidates, model=str(req.model), api_key=api_key or ""),
                "mistral": lambda api_key: generate_source_navigator_mistral(persona=req.persona, candidates=candidates, model=str(req.model), api_key=api_key or ""),
                "xai": lambda api_key: generate_source_navigator_xai(persona=req.persona, candidates=candidates, model=str(req.model), api_key=api_key or ""),
                "zai": lambda api_key: generate_source_navigator_zai(persona=req.persona, candidates=candidates, model=str(req.model), api_key=api_key or ""),
                "openrouter": lambda api_key: generate_source_navigator_openrouter(persona=req.persona, candidates=candidates, model=str(req.model), api_key=api_key or ""),
                "poe": lambda api_key: generate_source_navigator_poe(persona=req.persona, candidates=candidates, model=str(req.model), api_key=api_key or ""),
                "openai": lambda api_key: generate_source_navigator_openai(persona=req.persona, candidates=candidates, model=str(req.model), api_key=api_key or ""),
            },
        ),
        output_builder=lambda result: {
            "keep_count": len(result.get("keep") or []),
            "watch_count": len(result.get("watch") or []),
            "standout_count": len(result.get("standout") or []),
            **llm_usage_summary(result),
        },
    )
    return SourceNavigatorResponse(**result)

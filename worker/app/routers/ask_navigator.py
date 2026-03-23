from fastapi import APIRouter, Request
from pydantic import BaseModel

from app.services.alibaba_service import generate_ask_navigator as generate_ask_navigator_alibaba
from app.services.claude_service import generate_ask_navigator
from app.services.deepseek_service import generate_ask_navigator as generate_ask_navigator_deepseek
from app.services.fireworks_service import generate_ask_navigator as generate_ask_navigator_fireworks
from app.services.gemini_service import generate_ask_navigator as generate_ask_navigator_gemini
from app.services.groq_service import generate_ask_navigator as generate_ask_navigator_groq
from app.services.llm_dispatch import dispatch_by_model
from app.services.mistral_service import generate_ask_navigator as generate_ask_navigator_mistral
from app.services.openai_service import generate_ask_navigator as generate_ask_navigator_openai
from app.services.openrouter_service import generate_ask_navigator as generate_ask_navigator_openrouter
from app.services.poe_service import generate_ask_navigator as generate_ask_navigator_poe
from app.services.router_observe import llm_usage_summary, run_observed_request
from app.services.xai_service import generate_ask_navigator as generate_ask_navigator_xai
from app.services.zai_service import generate_ask_navigator as generate_ask_navigator_zai

router = APIRouter()


class AskNavigatorCitation(BaseModel):
    item_id: str
    title: str
    url: str
    reason: str | None = None
    published_at: str | None = None
    topics: list[str] = []


class AskNavigatorRelatedItem(BaseModel):
    item_id: str
    title: str | None = None
    translated_title: str | None = None
    url: str
    summary: str
    topics: list[str] = []
    published_at: str | None = None


class AskNavigatorInput(BaseModel):
    query: str
    answer: str
    bullets: list[str] = []
    citations: list[AskNavigatorCitation] = []
    related_items: list[AskNavigatorRelatedItem] = []


class AskNavigatorRequest(BaseModel):
    persona: str = "editor"
    input: AskNavigatorInput
    model: str | None = None


class AskNavigatorResponse(BaseModel):
    headline: str
    commentary: str
    next_angles: list[str] = []
    llm: dict | None = None


@router.post("/ask-navigator", response_model=AskNavigatorResponse)
def generate_ask_navigator_endpoint(req: AskNavigatorRequest, request: Request):
    ask_input = {
        "query": req.input.query,
        "answer": req.input.answer,
        "bullets": req.input.bullets,
        "citations": [citation.model_dump() for citation in req.input.citations],
        "related_items": [item.model_dump() for item in req.input.related_items],
    }
    result = run_observed_request(
        request,
        metadata={"model": req.model or "", "persona": req.persona},
        input_payload={"persona": req.persona, "query": req.input.query, "model": req.model},
        call=lambda: dispatch_by_model(
            request,
            req.model,
            handlers={
                "anthropic": lambda api_key: generate_ask_navigator(persona=req.persona, ask_input=ask_input, api_key=api_key, model=req.model),
                "google": lambda api_key: generate_ask_navigator_gemini(persona=req.persona, ask_input=ask_input, model=str(req.model), api_key=api_key or ""),
                "fireworks": lambda api_key: generate_ask_navigator_fireworks(persona=req.persona, ask_input=ask_input, model=str(req.model), api_key=api_key or ""),
                "groq": lambda api_key: generate_ask_navigator_groq(persona=req.persona, ask_input=ask_input, model=str(req.model), api_key=api_key or ""),
                "deepseek": lambda api_key: generate_ask_navigator_deepseek(persona=req.persona, ask_input=ask_input, model=str(req.model), api_key=api_key or ""),
                "alibaba": lambda api_key: generate_ask_navigator_alibaba(persona=req.persona, ask_input=ask_input, model=str(req.model), api_key=api_key or ""),
                "mistral": lambda api_key: generate_ask_navigator_mistral(persona=req.persona, ask_input=ask_input, model=str(req.model), api_key=api_key or ""),
                "xai": lambda api_key: generate_ask_navigator_xai(persona=req.persona, ask_input=ask_input, model=str(req.model), api_key=api_key or ""),
                "zai": lambda api_key: generate_ask_navigator_zai(persona=req.persona, ask_input=ask_input, model=str(req.model), api_key=api_key or ""),
                "openrouter": lambda api_key: generate_ask_navigator_openrouter(persona=req.persona, ask_input=ask_input, model=str(req.model), api_key=api_key or ""),
                "poe": lambda api_key: generate_ask_navigator_poe(persona=req.persona, ask_input=ask_input, model=str(req.model), api_key=api_key or ""),
                "openai": lambda api_key: generate_ask_navigator_openai(persona=req.persona, ask_input=ask_input, model=str(req.model), api_key=api_key or ""),
            },
        ),
        output_builder=lambda result: {"has_commentary": bool(result.get("commentary")), **llm_usage_summary(result)},
    )
    return AskNavigatorResponse(**result)

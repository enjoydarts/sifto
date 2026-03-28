from fastapi import APIRouter, Request
from pydantic import BaseModel

from app.services.alibaba_service import generate_item_navigator as generate_item_navigator_alibaba
from app.services.claude_service import generate_item_navigator
from app.services.deepseek_service import generate_item_navigator as generate_item_navigator_deepseek
from app.services.fireworks_service import generate_item_navigator as generate_item_navigator_fireworks
from app.services.gemini_service import generate_item_navigator as generate_item_navigator_gemini
from app.services.groq_service import generate_item_navigator as generate_item_navigator_groq
from app.services.llm_dispatch import dispatch_by_model
from app.services.mistral_service import generate_item_navigator as generate_item_navigator_mistral
from app.services.moonshot_service import generate_item_navigator as generate_item_navigator_moonshot
from app.services.openai_service import generate_item_navigator as generate_item_navigator_openai
from app.services.openrouter_service import generate_item_navigator as generate_item_navigator_openrouter
from app.services.poe_service import generate_item_navigator as generate_item_navigator_poe
from app.services.siliconflow_service import generate_item_navigator as generate_item_navigator_siliconflow
from app.services.router_observe import llm_usage_summary, run_observed_request
from app.services.xai_service import generate_item_navigator as generate_item_navigator_xai
from app.services.zai_service import generate_item_navigator as generate_item_navigator_zai

router = APIRouter()


class ItemNavigatorArticle(BaseModel):
    item_id: str
    title: str | None = None
    translated_title: str | None = None
    source_title: str | None = None
    summary: str
    facts: list[str] = []
    published_at: str | None = None


class ItemNavigatorRequest(BaseModel):
    persona: str = "editor"
    article: ItemNavigatorArticle
    model: str | None = None


class ItemNavigatorResponse(BaseModel):
    headline: str
    commentary: str
    stance_tags: list[str] = []
    llm: dict | None = None


@router.post("/item-navigator", response_model=ItemNavigatorResponse)
def generate_item_navigator_endpoint(req: ItemNavigatorRequest, request: Request):
    article = {
        "item_id": req.article.item_id,
        "title": req.article.title,
        "translated_title": req.article.translated_title,
        "source_title": req.article.source_title,
        "summary": req.article.summary,
        "facts": req.article.facts,
        "published_at": req.article.published_at,
    }
    result = run_observed_request(
        request,
        metadata={"model": req.model or "", "persona": req.persona, "item_id": req.article.item_id},
        input_payload={"persona": req.persona, "item_id": req.article.item_id, "model": req.model},
        call=lambda: dispatch_by_model(
            request,
            req.model,
            handlers={
                "anthropic": lambda api_key: generate_item_navigator(persona=req.persona, article=article, api_key=api_key, model=req.model),
                "google": lambda api_key: generate_item_navigator_gemini(persona=req.persona, article=article, model=str(req.model), api_key=api_key or ""),
                "fireworks": lambda api_key: generate_item_navigator_fireworks(persona=req.persona, article=article, model=str(req.model), api_key=api_key or ""),
                "groq": lambda api_key: generate_item_navigator_groq(persona=req.persona, article=article, model=str(req.model), api_key=api_key or ""),
                "deepseek": lambda api_key: generate_item_navigator_deepseek(persona=req.persona, article=article, model=str(req.model), api_key=api_key or ""),
                "alibaba": lambda api_key: generate_item_navigator_alibaba(persona=req.persona, article=article, model=str(req.model), api_key=api_key or ""),
                "mistral": lambda api_key: generate_item_navigator_mistral(persona=req.persona, article=article, model=str(req.model), api_key=api_key or ""),
                "moonshot": lambda api_key: generate_item_navigator_moonshot(persona=req.persona, article=article, model=str(req.model), api_key=api_key or ""),
                "xai": lambda api_key: generate_item_navigator_xai(persona=req.persona, article=article, model=str(req.model), api_key=api_key or ""),
                "zai": lambda api_key: generate_item_navigator_zai(persona=req.persona, article=article, model=str(req.model), api_key=api_key or ""),
                "openrouter": lambda api_key: generate_item_navigator_openrouter(persona=req.persona, article=article, model=str(req.model), api_key=api_key or ""),
                "poe": lambda api_key: generate_item_navigator_poe(persona=req.persona, article=article, model=str(req.model), api_key=api_key or ""),
                "siliconflow": lambda api_key: generate_item_navigator_siliconflow(persona=req.persona, article=article, model=str(req.model), api_key=api_key or ""),
                "openai": lambda api_key: generate_item_navigator_openai(persona=req.persona, article=article, model=str(req.model), api_key=api_key or ""),
            },
        ),
        output_builder=lambda result: {"has_commentary": bool(result.get("commentary")), **llm_usage_summary(result)},
    )
    return ItemNavigatorResponse(**result)

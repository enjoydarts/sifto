from fastapi import APIRouter, Request
from pydantic import BaseModel

from app.services.alibaba_service import translate_title as translate_title_alibaba
from app.services.claude_service import translate_title as translate_title_claude
from app.services.deepseek_service import translate_title as translate_title_deepseek
from app.services.gemini_service import translate_title as translate_title_gemini
from app.services.groq_service import translate_title as translate_title_groq
from app.services.llm_dispatch import dispatch_by_model
from app.services.mistral_service import translate_title as translate_title_mistral
from app.services.openai_service import translate_title as translate_title_openai
from app.services.xai_service import translate_title as translate_title_xai
from app.services.router_observe import llm_usage_summary, run_observed_request

router = APIRouter()


class TranslateTitleRequest(BaseModel):
    title: str
    model: str | None = None


class TranslateTitleResponse(BaseModel):
    translated_title: str = ""
    llm: dict | None = None


@router.post("/translate-title", response_model=TranslateTitleResponse)
def translate_title_endpoint(req: TranslateTitleRequest, request: Request):
    result = run_observed_request(
        request,
        metadata={"model": req.model or "", "title_chars": len(req.title or "")},
        input_payload={"title": req.title, "model": req.model},
        call=lambda: dispatch_by_model(
            request,
            req.model,
            handlers={
                "anthropic": lambda api_key: translate_title_claude(req.title, api_key=api_key, model=req.model),
                "google": lambda api_key: translate_title_gemini(req.title, model=str(req.model), api_key=api_key or ""),
                "groq": lambda api_key: translate_title_groq(req.title, model=str(req.model), api_key=api_key or ""),
                "deepseek": lambda api_key: translate_title_deepseek(req.title, model=str(req.model), api_key=api_key or ""),
                "alibaba": lambda api_key: translate_title_alibaba(req.title, model=str(req.model), api_key=api_key or ""),
                "mistral": lambda api_key: translate_title_mistral(req.title, model=str(req.model), api_key=api_key or ""),
                "xai": lambda api_key: translate_title_xai(req.title, model=str(req.model), api_key=api_key or ""),
                "openai": lambda api_key: translate_title_openai(req.title, model=str(req.model), api_key=api_key or ""),
            },
        ),
        output_builder=lambda result: {
            "translated_title_present": bool(result.get("translated_title")),
            "translated_title_chars": len(result.get("translated_title") or ""),
            **llm_usage_summary(result),
        },
    )
    return result

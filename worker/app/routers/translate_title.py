from fastapi import APIRouter, Request
from pydantic import BaseModel

from app.services.claude_service import translate_title as translate_title_claude
from app.services.deepseek_service import translate_title as translate_title_deepseek
from app.services.gemini_service import translate_title as translate_title_gemini
from app.services.groq_service import translate_title as translate_title_groq
from app.services.llm_dispatch import dispatch_by_model
from app.services.openai_service import translate_title as translate_title_openai

router = APIRouter()


class TranslateTitleRequest(BaseModel):
    title: str
    model: str | None = None


class TranslateTitleResponse(BaseModel):
    translated_title: str = ""
    llm: dict | None = None


@router.post("/translate-title", response_model=TranslateTitleResponse)
def translate_title_endpoint(req: TranslateTitleRequest, request: Request):
    return dispatch_by_model(
        request,
        req.model,
        handlers={
            "anthropic": lambda api_key: translate_title_claude(req.title, api_key=api_key, model=req.model),
            "google": lambda api_key: translate_title_gemini(req.title, model=str(req.model), api_key=api_key or ""),
            "groq": lambda api_key: translate_title_groq(req.title, model=str(req.model), api_key=api_key or ""),
            "deepseek": lambda api_key: translate_title_deepseek(req.title, model=str(req.model), api_key=api_key or ""),
            "openai": lambda api_key: translate_title_openai(req.title, model=str(req.model), api_key=api_key or ""),
        },
    )

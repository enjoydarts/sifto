from fastapi import APIRouter, Request
from pydantic import BaseModel

from app.services.claude_service import translate_title as translate_title_claude
from app.services.gemini_service import translate_title as translate_title_gemini
from app.services.model_router import is_gemini_model

router = APIRouter()


class TranslateTitleRequest(BaseModel):
    title: str
    model: str | None = None


class TranslateTitleResponse(BaseModel):
    translated_title: str = ""
    llm: dict | None = None


@router.post("/translate-title", response_model=TranslateTitleResponse)
def translate_title_endpoint(req: TranslateTitleRequest, request: Request):
    if is_gemini_model(req.model):
        google_api_key = request.headers.get("x-google-api-key") or ""
        return translate_title_gemini(req.title, model=str(req.model), api_key=google_api_key)
    api_key = request.headers.get("x-anthropic-api-key") or None
    return translate_title_claude(req.title, api_key=api_key, model=req.model)

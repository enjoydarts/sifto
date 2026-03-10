from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel
from app.services.claude_service import summarize
from app.services.deepseek_service import summarize as summarize_deepseek
from app.services.gemini_service import summarize as summarize_gemini
from app.services.groq_service import summarize as summarize_groq
from app.services.llm_dispatch import dispatch_by_model
from app.services.openai_service import summarize as summarize_openai

router = APIRouter()


class SummarizeRequest(BaseModel):
    title: str | None
    facts: list[str]
    model: str | None = None
    source_text_chars: int | None = None


class SummarizeResponse(BaseModel):
    summary: str
    topics: list[str]
    translated_title: str = ""
    score: float
    score_breakdown: dict | None = None
    score_reason: str | None = None
    score_policy_version: str | None = None
    llm: dict | None = None


@router.post("/summarize", response_model=SummarizeResponse)
def summarize_endpoint(req: SummarizeRequest, request: Request):
    try:
        result = dispatch_by_model(
            request,
            req.model,
            anthropic=lambda api_key: summarize(
                req.title,
                req.facts,
                source_text_chars=req.source_text_chars,
                api_key=api_key,
                model=req.model,
            ),
            google=lambda api_key: summarize_gemini(
                req.title,
                req.facts,
                source_text_chars=req.source_text_chars,
                model=str(req.model),
                api_key=api_key,
            ),
            groq=lambda api_key: summarize_groq(
                req.title,
                req.facts,
                source_text_chars=req.source_text_chars,
                model=str(req.model),
                api_key=api_key,
            ),
            deepseek=lambda api_key: summarize_deepseek(
                req.title,
                req.facts,
                source_text_chars=req.source_text_chars,
                model=str(req.model),
                api_key=api_key,
            ),
            openai=lambda api_key: summarize_openai(
                req.title,
                req.facts,
                source_text_chars=req.source_text_chars,
                model=str(req.model),
                api_key=api_key,
            ),
        )
        return SummarizeResponse(**result)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"summarize failed: {e}")

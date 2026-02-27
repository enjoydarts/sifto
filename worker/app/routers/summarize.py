from fastapi import APIRouter, Request
from pydantic import BaseModel
from app.services.claude_service import summarize

router = APIRouter()


class SummarizeRequest(BaseModel):
    title: str | None
    facts: list[str]
    model: str | None = None
    source_text_chars: int | None = None


class SummarizeResponse(BaseModel):
    summary: str
    topics: list[str]
    score: float
    score_breakdown: dict | None = None
    score_reason: str | None = None
    score_policy_version: str | None = None
    llm: dict | None = None


@router.post("/summarize", response_model=SummarizeResponse)
def summarize_endpoint(req: SummarizeRequest, request: Request):
    api_key = request.headers.get("x-anthropic-api-key") or None
    result = summarize(
        req.title,
        req.facts,
        source_text_chars=req.source_text_chars,
        api_key=api_key,
        model=req.model,
    )
    return result

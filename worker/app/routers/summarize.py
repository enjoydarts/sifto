from fastapi import APIRouter, Request
from pydantic import BaseModel
from app.services.claude_service import summarize

router = APIRouter()


class SummarizeRequest(BaseModel):
    title: str | None
    facts: list[str]


class SummarizeResponse(BaseModel):
    summary: str
    topics: list[str]
    score: float
    llm: dict | None = None


@router.post("/summarize", response_model=SummarizeResponse)
def summarize_endpoint(req: SummarizeRequest, request: Request):
    api_key = request.headers.get("x-anthropic-api-key") or None
    result = summarize(req.title, req.facts, api_key=api_key)
    return result

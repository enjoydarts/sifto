from fastapi import APIRouter
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


@router.post("/summarize", response_model=SummarizeResponse)
def summarize_endpoint(req: SummarizeRequest):
    result = summarize(req.title, req.facts)
    return result

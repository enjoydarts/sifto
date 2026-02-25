from fastapi import APIRouter, Request
from pydantic import BaseModel
from app.services.claude_service import extract_facts

router = APIRouter()


class FactsRequest(BaseModel):
    title: str | None
    content: str
    model: str | None = None


class FactsResponse(BaseModel):
    facts: list[str]
    llm: dict | None = None


@router.post("/extract-facts", response_model=FactsResponse)
def extract_facts_endpoint(req: FactsRequest, request: Request):
    api_key = request.headers.get("x-anthropic-api-key") or None
    result = extract_facts(req.title, req.content, api_key=api_key, model=req.model)
    return FactsResponse(**result)

from fastapi import APIRouter, Request
from pydantic import BaseModel
from app.services.claude_service import extract_facts
from app.services.gemini_service import extract_facts as extract_facts_gemini
from app.services.model_router import is_gemini_model

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
    if is_gemini_model(req.model):
        google_api_key = request.headers.get("x-google-api-key") or ""
        result = extract_facts_gemini(req.title, req.content, model=str(req.model), api_key=google_api_key)
    else:
        api_key = request.headers.get("x-anthropic-api-key") or None
        result = extract_facts(req.title, req.content, api_key=api_key, model=req.model)
    return FactsResponse(**result)

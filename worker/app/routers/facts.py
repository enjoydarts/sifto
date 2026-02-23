from fastapi import APIRouter
from pydantic import BaseModel
from app.services.claude_service import extract_facts

router = APIRouter()


class FactsRequest(BaseModel):
    title: str | None
    content: str


class FactsResponse(BaseModel):
    facts: list[str]


@router.post("/extract-facts", response_model=FactsResponse)
def extract_facts_endpoint(req: FactsRequest):
    facts = extract_facts(req.title, req.content)
    return FactsResponse(facts=facts)

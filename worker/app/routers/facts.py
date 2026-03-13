from fastapi import APIRouter, Request
from pydantic import BaseModel
from app.services.claude_service import extract_facts
from app.services.alibaba_service import extract_facts as extract_facts_alibaba
from app.services.deepseek_service import extract_facts as extract_facts_deepseek
from app.services.gemini_service import extract_facts as extract_facts_gemini
from app.services.groq_service import extract_facts as extract_facts_groq
from app.services.llm_dispatch import dispatch_by_model
from app.services.mistral_service import extract_facts as extract_facts_mistral
from app.services.openai_service import extract_facts as extract_facts_openai
from app.services.router_observe import llm_usage_summary, run_observed_request

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
    result = run_observed_request(
        request,
        metadata={"model": req.model or "", "title_present": bool(req.title), "content_chars": len(req.content or "")},
        input_payload={"title": req.title, "content_chars": len(req.content or ""), "model": req.model},
        call=lambda: dispatch_by_model(
            request,
            req.model,
            handlers={
                "anthropic": lambda api_key: extract_facts(req.title, req.content, api_key=api_key, model=req.model),
                "google": lambda api_key: extract_facts_gemini(req.title, req.content, model=str(req.model), api_key=api_key or ""),
                "groq": lambda api_key: extract_facts_groq(req.title, req.content, model=str(req.model), api_key=api_key or ""),
                "deepseek": lambda api_key: extract_facts_deepseek(req.title, req.content, model=str(req.model), api_key=api_key or ""),
                "alibaba": lambda api_key: extract_facts_alibaba(req.title, req.content, model=str(req.model), api_key=api_key or ""),
                "mistral": lambda api_key: extract_facts_mistral(req.title, req.content, model=str(req.model), api_key=api_key or ""),
                "openai": lambda api_key: extract_facts_openai(req.title, req.content, model=str(req.model), api_key=api_key or ""),
            },
        ),
        output_builder=lambda result: {"facts_count": len(result.get("facts") or []), **llm_usage_summary(result)},
    )
    return FactsResponse(**result)

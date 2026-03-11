from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel

from app.services.claude_service import check_facts
from app.services.deepseek_service import check_facts as check_facts_deepseek
from app.services.gemini_service import check_facts as check_facts_gemini
from app.services.groq_service import check_facts as check_facts_groq
from app.services.llm_dispatch import dispatch_by_model
from app.services.openai_service import check_facts as check_facts_openai
from app.services.router_observe import bind_request_span, llm_usage_summary, observe_request_input, observe_request_output

router = APIRouter()


class FactsCheckRequest(BaseModel):
    title: str | None
    content: str
    facts: list[str]
    model: str | None = None


class FactsCheckResponse(BaseModel):
    verdict: str
    short_comment: str
    llm: dict | None = None


@router.post("/check-facts", response_model=FactsCheckResponse)
def check_facts_endpoint(req: FactsCheckRequest, request: Request):
    try:
        with bind_request_span(request):
            observe_request_input(
                metadata={"model": req.model or "", "facts_count": len(req.facts or []), "content_chars": len(req.content or "")},
                input_payload={"title": req.title, "facts_count": len(req.facts or []), "model": req.model},
            )
            result = dispatch_by_model(
                request,
                req.model,
                handlers={
                    "anthropic": lambda api_key: check_facts(req.title, req.content, req.facts, api_key=api_key, model=req.model),
                    "google": lambda api_key: check_facts_gemini(req.title, req.content, req.facts, model=str(req.model), api_key=api_key or ""),
                    "groq": lambda api_key: check_facts_groq(req.title, req.content, req.facts, model=str(req.model), api_key=api_key or ""),
                    "deepseek": lambda api_key: check_facts_deepseek(req.title, req.content, req.facts, model=str(req.model), api_key=api_key or ""),
                    "openai": lambda api_key: check_facts_openai(req.title, req.content, req.facts, model=str(req.model), api_key=api_key or ""),
                },
            )
            observe_request_output(
                {
                    "verdict": result.get("verdict"),
                    "short_comment_chars": len(result.get("short_comment") or ""),
                    **llm_usage_summary(result),
                },
                llm_result=result,
            )
            return FactsCheckResponse(**result)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"check_facts failed: {e}")

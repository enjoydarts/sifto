from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel
from app.services.alibaba_service import summarize as summarize_alibaba
from app.services.claude_service import summarize
from app.services.deepseek_service import summarize as summarize_deepseek
from app.services.gemini_service import summarize as summarize_gemini
from app.services.groq_service import summarize as summarize_groq
from app.services.llm_dispatch import dispatch_by_model
from app.services.mistral_service import summarize as summarize_mistral
from app.services.openai_service import summarize as summarize_openai
from app.services.router_observe import llm_usage_summary, run_observed_request

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
        result = run_observed_request(
            request,
            metadata={"model": req.model or "", "facts_count": len(req.facts or []), "source_text_chars": req.source_text_chars or 0},
            input_payload={"title": req.title, "facts_count": len(req.facts or []), "model": req.model},
            call=lambda: dispatch_by_model(
                request,
                req.model,
                handlers={
                    "anthropic": lambda api_key: summarize(
                        req.title,
                        req.facts,
                        source_text_chars=req.source_text_chars,
                        api_key=api_key,
                        model=req.model,
                    ),
                    "google": lambda api_key: summarize_gemini(
                        req.title,
                        req.facts,
                        source_text_chars=req.source_text_chars,
                        model=str(req.model),
                        api_key=api_key or "",
                    ),
                    "groq": lambda api_key: summarize_groq(
                        req.title,
                        req.facts,
                        source_text_chars=req.source_text_chars,
                        model=str(req.model),
                        api_key=api_key or "",
                    ),
                    "deepseek": lambda api_key: summarize_deepseek(
                        req.title,
                        req.facts,
                        source_text_chars=req.source_text_chars,
                        model=str(req.model),
                        api_key=api_key or "",
                    ),
                    "alibaba": lambda api_key: summarize_alibaba(
                        req.title,
                        req.facts,
                        source_text_chars=req.source_text_chars,
                        model=str(req.model),
                        api_key=api_key or "",
                    ),
                    "mistral": lambda api_key: summarize_mistral(
                        req.title,
                        req.facts,
                        source_text_chars=req.source_text_chars,
                        model=str(req.model),
                        api_key=api_key or "",
                    ),
                    "openai": lambda api_key: summarize_openai(
                        req.title,
                        req.facts,
                        source_text_chars=req.source_text_chars,
                        model=str(req.model),
                        api_key=api_key or "",
                    ),
                },
            ),
            output_builder=lambda result: {
                "topics_count": len(result.get("topics") or []),
                "summary_chars": len(result.get("summary") or ""),
                "translated_title_present": bool(result.get("translated_title")),
                **llm_usage_summary(result),
            },
        )
        return SummarizeResponse(**result)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"summarize failed: {e}")

from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel

from app.services.alibaba_service import check_summary_faithfulness as check_summary_faithfulness_alibaba
from app.services.claude_service import check_summary_faithfulness
from app.services.deepseek_service import check_summary_faithfulness as check_summary_faithfulness_deepseek
from app.services.fireworks_service import check_summary_faithfulness as check_summary_faithfulness_fireworks
from app.services.gemini_service import check_summary_faithfulness as check_summary_faithfulness_gemini
from app.services.groq_service import check_summary_faithfulness as check_summary_faithfulness_groq
from app.services.llm_dispatch import dispatch_by_model
from app.services.mistral_service import check_summary_faithfulness as check_summary_faithfulness_mistral
from app.services.openai_service import check_summary_faithfulness as check_summary_faithfulness_openai
from app.services.openrouter_service import check_summary_faithfulness as check_summary_faithfulness_openrouter
from app.services.moonshot_service import check_summary_faithfulness as check_summary_faithfulness_moonshot
from app.services.poe_service import check_summary_faithfulness as check_summary_faithfulness_poe
from app.services.xai_service import check_summary_faithfulness as check_summary_faithfulness_xai
from app.services.zai_service import check_summary_faithfulness as check_summary_faithfulness_zai
from app.services.router_observe import llm_usage_summary, run_observed_request

router = APIRouter()


class SummaryFaithfulnessRequest(BaseModel):
    title: str | None
    facts: list[str]
    summary: str
    model: str | None = None


class SummaryFaithfulnessResponse(BaseModel):
    verdict: str
    short_comment: str
    llm: dict | None = None


@router.post("/check-summary-faithfulness", response_model=SummaryFaithfulnessResponse)
def check_summary_faithfulness_endpoint(req: SummaryFaithfulnessRequest, request: Request):
    try:
        result = run_observed_request(
            request,
            metadata={"model": req.model or "", "facts_count": len(req.facts or []), "summary_chars": len(req.summary or "")},
            input_payload={"title": req.title, "facts_count": len(req.facts or []), "model": req.model},
            call=lambda: dispatch_by_model(
                request,
                req.model,
                handlers={
                    "anthropic": lambda api_key: check_summary_faithfulness(req.title, req.facts, req.summary, api_key=api_key, model=req.model),
                    "google": lambda api_key: check_summary_faithfulness_gemini(req.title, req.facts, req.summary, model=str(req.model), api_key=api_key or ""),
                    "fireworks": lambda api_key: check_summary_faithfulness_fireworks(req.title, req.facts, req.summary, model=str(req.model), api_key=api_key or ""),
                    "groq": lambda api_key: check_summary_faithfulness_groq(req.title, req.facts, req.summary, model=str(req.model), api_key=api_key or ""),
                    "deepseek": lambda api_key: check_summary_faithfulness_deepseek(req.title, req.facts, req.summary, model=str(req.model), api_key=api_key or ""),
                    "alibaba": lambda api_key: check_summary_faithfulness_alibaba(req.title, req.facts, req.summary, model=str(req.model), api_key=api_key or ""),
                    "mistral": lambda api_key: check_summary_faithfulness_mistral(req.title, req.facts, req.summary, model=str(req.model), api_key=api_key or ""),
                    "moonshot": lambda api_key: check_summary_faithfulness_moonshot(req.title, req.facts, req.summary, model=str(req.model), api_key=api_key or ""),
                    "xai": lambda api_key: check_summary_faithfulness_xai(req.title, req.facts, req.summary, model=str(req.model), api_key=api_key or ""),
                    "zai": lambda api_key: check_summary_faithfulness_zai(req.title, req.facts, req.summary, model=str(req.model), api_key=api_key or ""),
                    "openrouter": lambda api_key: check_summary_faithfulness_openrouter(req.title, req.facts, req.summary, model=str(req.model), api_key=api_key or ""),
                    "poe": lambda api_key: check_summary_faithfulness_poe(req.title, req.facts, req.summary, model=str(req.model), api_key=api_key or ""),
                    "openai": lambda api_key: check_summary_faithfulness_openai(req.title, req.facts, req.summary, model=str(req.model), api_key=api_key or ""),
                },
            ),
            output_builder=lambda result: {
                "verdict": result.get("verdict"),
                "short_comment_chars": len(result.get("short_comment") or ""),
                **llm_usage_summary(result),
            },
        )
        return SummaryFaithfulnessResponse(**result)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"check_summary_faithfulness failed: {e}")

from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel

from app.services.claude_service import check_summary_faithfulness
from app.services.deepseek_service import check_summary_faithfulness as check_summary_faithfulness_deepseek
from app.services.gemini_service import check_summary_faithfulness as check_summary_faithfulness_gemini
from app.services.groq_service import check_summary_faithfulness as check_summary_faithfulness_groq
from app.services.llm_dispatch import dispatch_by_model
from app.services.openai_service import check_summary_faithfulness as check_summary_faithfulness_openai

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
        result = dispatch_by_model(
            request,
            req.model,
            handlers={
                "anthropic": lambda api_key: check_summary_faithfulness(req.title, req.facts, req.summary, api_key=api_key, model=req.model),
                "google": lambda api_key: check_summary_faithfulness_gemini(req.title, req.facts, req.summary, model=str(req.model), api_key=api_key or ""),
                "groq": lambda api_key: check_summary_faithfulness_groq(req.title, req.facts, req.summary, model=str(req.model), api_key=api_key or ""),
                "deepseek": lambda api_key: check_summary_faithfulness_deepseek(req.title, req.facts, req.summary, model=str(req.model), api_key=api_key or ""),
                "openai": lambda api_key: check_summary_faithfulness_openai(req.title, req.facts, req.summary, model=str(req.model), api_key=api_key or ""),
            },
        )
        return SummaryFaithfulnessResponse(**result)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"check_summary_faithfulness failed: {e}")

from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel

from app.services.alibaba_service import ask_question as ask_question_alibaba
from app.services.claude_service import ask_question
from app.services.deepseek_service import ask_question as ask_question_deepseek
from app.services.fireworks_service import ask_question as ask_question_fireworks
from app.services.gemini_service import ask_question as ask_question_gemini
from app.services.groq_service import ask_question as ask_question_groq
from app.services.llm_dispatch import dispatch_by_model
from app.services.mistral_service import ask_question as ask_question_mistral
from app.services.openai_service import ask_question as ask_question_openai
from app.services.openrouter_service import ask_question as ask_question_openrouter
from app.services.xai_service import ask_question as ask_question_xai
from app.services.zai_service import ask_question as ask_question_zai
from app.services.router_observe import llm_usage_summary, run_observed_request

router = APIRouter()


class AskCandidate(BaseModel):
    item_id: str
    title: str | None = None
    translated_title: str | None = None
    url: str
    summary: str
    facts: list[str] = []
    topics: list[str] = []
    published_at: str | None = None
    similarity: float = 0.0


class AskRequest(BaseModel):
    query: str
    candidates: list[AskCandidate]
    model: str | None = None


class AskCitation(BaseModel):
    item_id: str
    reason: str


class AskResponse(BaseModel):
    answer: str
    bullets: list[str]
    citations: list[AskCitation]
    llm: dict | None = None


@router.post("/ask", response_model=AskResponse)
def ask_endpoint(req: AskRequest, request: Request):
    try:
        candidates = [c.model_dump() for c in req.candidates]
        result = run_observed_request(
            request,
            metadata={"model": req.model or "", "candidates_count": len(candidates), "query_chars": len(req.query or "")},
            input_payload={"query": req.query, "candidates_count": len(candidates), "model": req.model},
            call=lambda: dispatch_by_model(
                request,
                req.model,
                handlers={
                    "anthropic": lambda api_key: ask_question(req.query, candidates, api_key=api_key, model=req.model),
                    "google": lambda api_key: ask_question_gemini(req.query, candidates, model=str(req.model), api_key=api_key or ""),
                    "fireworks": lambda api_key: ask_question_fireworks(req.query, candidates, model=str(req.model), api_key=api_key or ""),
                    "groq": lambda api_key: ask_question_groq(req.query, candidates, model=str(req.model), api_key=api_key or ""),
                    "deepseek": lambda api_key: ask_question_deepseek(req.query, candidates, model=str(req.model), api_key=api_key or ""),
                    "alibaba": lambda api_key: ask_question_alibaba(req.query, candidates, model=str(req.model), api_key=api_key or ""),
                    "mistral": lambda api_key: ask_question_mistral(req.query, candidates, model=str(req.model), api_key=api_key or ""),
                    "xai": lambda api_key: ask_question_xai(req.query, candidates, model=str(req.model), api_key=api_key or ""),
                    "zai": lambda api_key: ask_question_zai(req.query, candidates, model=str(req.model), api_key=api_key or ""),
                    "openrouter": lambda api_key: ask_question_openrouter(req.query, candidates, model=str(req.model), api_key=api_key or ""),
                    "openai": lambda api_key: ask_question_openai(req.query, candidates, model=str(req.model), api_key=api_key or ""),
                },
            ),
            output_builder=lambda result: {
                "answer_chars": len(result.get("answer") or ""),
                "citations_count": len(result.get("citations") or []),
                **llm_usage_summary(result),
            },
        )
        return AskResponse(**result)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"ask failed: {e}")

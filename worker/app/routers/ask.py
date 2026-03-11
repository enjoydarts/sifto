from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel

from app.services.claude_service import ask_question
from app.services.deepseek_service import ask_question as ask_question_deepseek
from app.services.gemini_service import ask_question as ask_question_gemini
from app.services.groq_service import ask_question as ask_question_groq
from app.services.llm_dispatch import dispatch_by_model
from app.services.openai_service import ask_question as ask_question_openai
from app.services.router_observe import bind_request_span, llm_usage_summary, observe_request_input, observe_request_output

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
        with bind_request_span(request):
            candidates = [c.model_dump() for c in req.candidates]
            observe_request_input(
                metadata={"model": req.model or "", "candidates_count": len(candidates), "query_chars": len(req.query or "")},
                input_payload={"query": req.query, "candidates_count": len(candidates), "model": req.model},
            )
            result = dispatch_by_model(
                request,
                req.model,
                handlers={
                    "anthropic": lambda api_key: ask_question(req.query, candidates, api_key=api_key, model=req.model),
                    "google": lambda api_key: ask_question_gemini(req.query, candidates, model=str(req.model), api_key=api_key or ""),
                    "groq": lambda api_key: ask_question_groq(req.query, candidates, model=str(req.model), api_key=api_key or ""),
                    "deepseek": lambda api_key: ask_question_deepseek(req.query, candidates, model=str(req.model), api_key=api_key or ""),
                    "openai": lambda api_key: ask_question_openai(req.query, candidates, model=str(req.model), api_key=api_key or ""),
                },
            )
            observe_request_output(
                {
                    "answer_chars": len(result.get("answer") or ""),
                    "citations_count": len(result.get("citations") or []),
                    **llm_usage_summary(result),
                },
                llm_result=result,
            )
            return AskResponse(**result)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"ask failed: {e}")

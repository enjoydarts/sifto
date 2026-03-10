from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel

from app.services.claude_service import ask_question
from app.services.deepseek_service import ask_question as ask_question_deepseek
from app.services.gemini_service import ask_question as ask_question_gemini
from app.services.groq_service import ask_question as ask_question_groq
from app.services.model_router import is_deepseek_model, is_gemini_model, is_groq_model, is_openai_model
from app.services.openai_service import ask_question as ask_question_openai

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
        if is_gemini_model(req.model):
            google_api_key = request.headers.get("x-google-api-key") or ""
            result = ask_question_gemini(req.query, candidates, model=str(req.model), api_key=google_api_key)
        elif is_deepseek_model(req.model):
            deepseek_api_key = request.headers.get("x-deepseek-api-key") or ""
            result = ask_question_deepseek(req.query, candidates, model=str(req.model), api_key=deepseek_api_key)
        elif is_groq_model(req.model):
            groq_api_key = request.headers.get("x-groq-api-key") or ""
            result = ask_question_groq(req.query, candidates, model=str(req.model), api_key=groq_api_key)
        elif is_openai_model(req.model):
            openai_api_key = request.headers.get("x-openai-api-key") or ""
            result = ask_question_openai(req.query, candidates, model=str(req.model), api_key=openai_api_key)
        else:
            api_key = request.headers.get("x-anthropic-api-key") or None
            result = ask_question(req.query, candidates, api_key=api_key, model=req.model)
        return AskResponse(**result)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"ask failed: {e}")

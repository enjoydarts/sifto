from fastapi import APIRouter, Request
from pydantic import BaseModel
from app.services.claude_service import ask_question
from app.services.llm_dispatch import dispatch_by_model_async
from app.services.router_observe import llm_usage_summary, run_observed_request_async
from app.auto_dispatch import build_handler_map_async

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
async def ask_endpoint(req: AskRequest, request: Request):
    candidates = [c.model_dump() for c in req.candidates]
    result = await run_observed_request_async(
        request,
        metadata={"model": req.model or "", "candidates_count": len(candidates), "query_chars": len(req.query or "")},
        input_payload={"query": req.query, "candidates_count": len(candidates), "model": req.model},
        call=lambda: dispatch_by_model_async(
            request,
            req.model,
            handlers=build_handler_map_async(
                "ask_question",
                args_fn=lambda func, api_key: func(req.query, candidates, model=str(req.model), api_key=api_key or ""),
                anthropic_args_fn=lambda func, api_key: func(req.query, candidates, api_key=api_key, model=req.model),
            ),
        ),
        output_builder=lambda result: {
            "answer_chars": len(result.get("answer") or ""),
            "citations_count": len(result.get("citations") or []),
            **llm_usage_summary(result),
        },
    )
    return AskResponse(**result)

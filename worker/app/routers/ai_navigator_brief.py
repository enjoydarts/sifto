from fastapi import APIRouter, Request
from pydantic import BaseModel

from app.services.claude_service import compose_ai_navigator_brief
from app.services.llm_dispatch import dispatch_by_model_async
from app.services.router_observe import llm_usage_summary, run_observed_request_async
from app.auto_dispatch import build_handler_map_async

router = APIRouter()


class AINavigatorBriefCandidate(BaseModel):
    item_id: str
    title: str | None = None
    translated_title: str | None = None
    source_title: str | None = None
    summary: str
    topics: list[str] = []
    published_at: str | None = None
    score: float | None = None


class AINavigatorBriefRequest(BaseModel):
    persona: str = "editor"
    candidates: list[AINavigatorBriefCandidate]
    intro_context: dict
    model: str | None = None


class AINavigatorBriefItem(BaseModel):
    item_id: str
    comment: str
    reason_tags: list[str] = []


class AINavigatorBriefResponse(BaseModel):
    title: str
    intro: str
    summary: str
    ending: str
    items: list[AINavigatorBriefItem]
    llm: dict | None = None


@router.post("/ai-navigator-brief", response_model=AINavigatorBriefResponse)
async def compose_ai_navigator_brief_endpoint(req: AINavigatorBriefRequest, request: Request):
    candidates = [
        {
            "item_id": c.item_id,
            "title": c.title,
            "translated_title": c.translated_title,
            "source_title": c.source_title,
            "summary": c.summary,
            "topics": c.topics,
            "published_at": c.published_at,
            "score": c.score,
        }
        for c in req.candidates
    ]
    result = await run_observed_request_async(
        request,
        metadata={"model": req.model or "", "persona": req.persona, "candidates_count": len(candidates)},
        input_payload={"persona": req.persona, "candidates_count": len(candidates), "model": req.model},
        call=lambda: dispatch_by_model_async(
            request,
            req.model,
            handlers=build_handler_map_async(
                "compose_ai_navigator_brief",
                args_fn=lambda func, api_key: func(persona=req.persona, candidates=candidates, intro_context=req.intro_context, model=str(req.model), api_key=api_key or ""),
                anthropic_args_fn=lambda func, api_key: func(persona=req.persona, candidates=candidates, intro_context=req.intro_context, api_key=api_key, model=req.model),
            ),
        ),
        output_builder=lambda result: {"items_count": len(result.get("items") or []), **llm_usage_summary(result)},
    )
    return AINavigatorBriefResponse(**result)

from fastapi import APIRouter, Request
from pydantic import BaseModel

from app.services.claude_service import generate_source_navigator
from app.services.llm_dispatch import dispatch_by_model_async
from app.services.router_observe import llm_usage_summary, run_observed_request_async
from app.auto_dispatch import build_handler_map_async

router = APIRouter()


class SourceNavigatorCandidate(BaseModel):
    source_id: str
    title: str
    url: str
    enabled: bool
    status: str
    last_fetched_at: str | None = None
    last_item_at: str | None = None
    total_items_30d: int = 0
    unread_items_30d: int = 0
    read_items_30d: int = 0
    favorite_count_30d: int = 0
    avg_items_per_day_30d: float = 0
    active_days_30d: int = 0
    avg_items_per_active_day_30d: float = 0
    failure_rate: float = 0


class SourceNavigatorRequest(BaseModel):
    persona: str = "editor"
    candidates: list[SourceNavigatorCandidate]
    model: str | None = None


class SourceNavigatorPick(BaseModel):
    source_id: str
    comment: str


class SourceNavigatorResponse(BaseModel):
    overview: str
    keep: list[SourceNavigatorPick] = []
    watch: list[SourceNavigatorPick] = []
    standout: list[SourceNavigatorPick] = []
    llm: dict | None = None


@router.post("/source-navigator", response_model=SourceNavigatorResponse)
async def generate_source_navigator_endpoint(req: SourceNavigatorRequest, request: Request):
    candidates = [candidate.model_dump() for candidate in req.candidates]
    result = await run_observed_request_async(
        request,
        metadata={"model": req.model or "", "persona": req.persona, "candidates_count": len(candidates)},
        input_payload={"persona": req.persona, "candidates_count": len(candidates), "model": req.model},
        call=lambda: dispatch_by_model_async(
            request,
            req.model,
            handlers=build_handler_map_async(
                "generate_source_navigator",
                args_fn=lambda func, api_key: func(persona=req.persona, candidates=candidates, model=str(req.model), api_key=api_key or ""),
                anthropic_args_fn=lambda func, api_key: func(persona=req.persona, candidates=candidates, api_key=api_key, model=req.model),
            ),
        ),
        output_builder=lambda result: {
            "keep_count": len(result.get("keep") or []),
            "watch_count": len(result.get("watch") or []),
            "standout_count": len(result.get("standout") or []),
            **llm_usage_summary(result),
        },
    )
    return SourceNavigatorResponse(**result)

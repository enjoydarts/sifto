from fastapi import APIRouter, Request
from pydantic import BaseModel
from app.services.claude_service import check_summary_faithfulness
from app.services.llm_dispatch import dispatch_by_model_async
from app.services.router_observe import llm_usage_summary, run_observed_request_async
from app.auto_dispatch import build_handler_map_async

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
async def check_summary_faithfulness_endpoint(req: SummaryFaithfulnessRequest, request: Request):
    result = await run_observed_request_async(
        request,
        metadata={"model": req.model or "", "facts_count": len(req.facts or []), "summary_chars": len(req.summary or "")},
        input_payload={"title": req.title, "facts_count": len(req.facts or []), "model": req.model},
        call=lambda: dispatch_by_model_async(
            request,
            req.model,
            handlers=build_handler_map_async(
                "check_summary_faithfulness",
                args_fn=lambda func, api_key: func(req.title, req.facts, req.summary, model=str(req.model), api_key=api_key or ""),
                anthropic_args_fn=lambda func, api_key: func(req.title, req.facts, req.summary, api_key=api_key, model=req.model),
            ),
        ),
        output_builder=lambda result: {"verdict": result.get("verdict"), **llm_usage_summary(result)},
    )
    return SummaryFaithfulnessResponse(**result)

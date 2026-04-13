from fastapi import APIRouter, Request
from pydantic import BaseModel
from app.services.claude_service import check_facts
from app.services.llm_dispatch import dispatch_by_model_async
from app.services.router_observe import llm_usage_summary, run_observed_request_async
from app.auto_dispatch import build_handler_map_async

router = APIRouter()


class FactsCheckRequest(BaseModel):
    title: str | None
    content: str
    facts: list[str]
    model: str | None = None


class FactsCheckResponse(BaseModel):
    verdict: str
    short_comment: str
    llm: dict | None = None


@router.post("/check-facts", response_model=FactsCheckResponse)
async def check_facts_endpoint(req: FactsCheckRequest, request: Request):
    result = await run_observed_request_async(
        request,
        metadata={"model": req.model or "", "facts_count": len(req.facts or []), "content_chars": len(req.content or "")},
        input_payload={"title": req.title, "facts_count": len(req.facts or []), "model": req.model},
        call=lambda: dispatch_by_model_async(
            request,
            req.model,
            handlers=build_handler_map_async(
                "check_facts",
                args_fn=lambda func, api_key: func(req.title, req.content, req.facts, model=str(req.model), api_key=api_key or ""),
                anthropic_args_fn=lambda func, api_key: func(req.title, req.content, req.facts, api_key=api_key, model=req.model),
            ),
        ),
        output_builder=lambda result: {"verdict": result.get("verdict"), **llm_usage_summary(result)},
    )
    return FactsCheckResponse(**result)

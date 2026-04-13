from fastapi import APIRouter, Request
from pydantic import BaseModel
from app.services.claude_service import summarize
from app.services.llm_dispatch import dispatch_by_model_async
from app.services.runtime_prompt_overrides import bind_prompt_override
from app.services.router_observe import llm_usage_summary, run_observed_request_async
from app.auto_dispatch import build_handler_map_async

router = APIRouter()


class SummarizeRequest(BaseModel):
    title: str | None
    facts: list[str]
    model: str | None = None
    source_text_chars: int | None = None
    prompt: dict | None = None


class SummarizeResponse(BaseModel):
    summary: str
    topics: list[str]
    translated_title: str = ""
    score: float
    score_breakdown: dict | None = None
    score_reason: str | None = None
    score_policy_version: str | None = None
    llm: dict | None = None


@router.post("/summarize", response_model=SummarizeResponse)
async def summarize_endpoint(req: SummarizeRequest, request: Request):
    with bind_prompt_override((req.prompt or {}).get("prompt_key"), (req.prompt or {}).get("prompt_text"), (req.prompt or {}).get("system_instruction")):
        result = await run_observed_request_async(
            request,
            metadata={"model": req.model or "", "facts_count": len(req.facts or []), "source_text_chars": req.source_text_chars or 0},
            input_payload={"title": req.title, "facts_count": len(req.facts or []), "model": req.model},
            call=lambda: dispatch_by_model_async(
                request,
                req.model,
                handlers=build_handler_map_async(
                    "summarize",
                    args_fn=lambda func, api_key: func(req.title, req.facts, source_text_chars=req.source_text_chars, model=str(req.model), api_key=api_key or ""),
                    anthropic_args_fn=lambda func, api_key: func(req.title, req.facts, source_text_chars=req.source_text_chars, api_key=api_key, model=req.model),
                ),
            ),
            output_builder=lambda result: {
                "topics_count": len(result.get("topics") or []),
                "summary_chars": len(result.get("summary") or ""),
                "translated_title_present": bool(result.get("translated_title")),
                **llm_usage_summary(result),
            },
        )
    return SummarizeResponse(**result)

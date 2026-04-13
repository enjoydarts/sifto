from fastapi import APIRouter, Request
from pydantic import BaseModel
from app.services.claude_service import extract_facts
from app.services.llm_dispatch import dispatch_by_model_async
from app.services.runtime_prompt_overrides import bind_prompt_override
from app.services.router_observe import llm_usage_summary, run_observed_request_async
from app.auto_dispatch import build_handler_map_async

router = APIRouter()


class FactsRequest(BaseModel):
    title: str | None
    content: str
    model: str | None = None
    prompt: dict | None = None


class FactsResponse(BaseModel):
    facts: list[str]
    llm: dict | None = None
    facts_localization_llm: dict | None = None


@router.post("/extract-facts", response_model=FactsResponse)
async def extract_facts_endpoint(req: FactsRequest, request: Request):
    with bind_prompt_override((req.prompt or {}).get("prompt_key"), (req.prompt or {}).get("prompt_text"), (req.prompt or {}).get("system_instruction")):
        result = await run_observed_request_async(
            request,
            metadata={"model": req.model or "", "title_present": bool(req.title), "content_chars": len(req.content or "")},
            input_payload={"title": req.title, "content_chars": len(req.content or ""), "model": req.model},
            call=lambda: dispatch_by_model_async(
                request,
                req.model,
                handlers=build_handler_map_async(
                    "extract_facts",
                    args_fn=lambda func, api_key: func(req.title, req.content, model=str(req.model), api_key=api_key or ""),
                    anthropic_args_fn=lambda func, api_key: func(req.title, req.content, api_key=api_key, model=req.model),
                ),
            ),
            output_builder=lambda result: {"facts_count": len(result.get("facts") or []), **llm_usage_summary(result)},
        )
    return FactsResponse(**result)

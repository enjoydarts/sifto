from fastapi import APIRouter, Request
from pydantic import BaseModel
from app.services.llm_dispatch import dispatch_by_model_async
from app.services.router_observe import llm_usage_summary, run_observed_request_async
from app.auto_dispatch import build_handler_map_async

router = APIRouter()


class TranslateTitleRequest(BaseModel):
    title: str
    model: str | None = None


class TranslateTitleResponse(BaseModel):
    translated_title: str = ""
    llm: dict | None = None


@router.post("/translate-title", response_model=TranslateTitleResponse)
async def translate_title_endpoint(req: TranslateTitleRequest, request: Request):
    result = await run_observed_request_async(
        request,
        metadata={"model": req.model or "", "title_chars": len(req.title or "")},
        input_payload={"title": req.title, "model": req.model},
        call=lambda: dispatch_by_model_async(
            request,
            req.model,
            handlers=build_handler_map_async(
                "translate_title",
                args_fn=lambda func, api_key: func(req.title, model=str(req.model), api_key=api_key or ""),
                anthropic_args_fn=lambda func, api_key: func(req.title, api_key=api_key, model=req.model),
            ),
        ),
        output_builder=lambda result: {
            "translated_title_present": bool(result.get("translated_title")),
            "translated_title_chars": len(result.get("translated_title") or ""),
            **llm_usage_summary(result),
        },
    )
    return result

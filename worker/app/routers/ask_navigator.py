from fastapi import APIRouter, Request
from pydantic import BaseModel
from app.services.claude_service import generate_ask_navigator
from app.services.llm_dispatch import dispatch_by_model_async
from app.services.router_observe import llm_usage_summary, run_observed_request_async
from app.auto_dispatch import build_handler_map_async

router = APIRouter()


class AskNavigatorCitation(BaseModel):
    item_id: str
    title: str
    url: str
    reason: str | None = None
    published_at: str | None = None
    topics: list[str] = []


class AskNavigatorRelatedItem(BaseModel):
    item_id: str
    title: str | None = None
    translated_title: str | None = None
    url: str
    summary: str
    topics: list[str] = []
    published_at: str | None = None


class AskNavigatorInput(BaseModel):
    query: str
    answer: str
    bullets: list[str] = []
    citations: list[AskNavigatorCitation] = []
    related_items: list[AskNavigatorRelatedItem] = []


class AskNavigatorRequest(BaseModel):
    persona: str = "editor"
    input: AskNavigatorInput
    model: str | None = None


class AskNavigatorResponse(BaseModel):
    headline: str
    commentary: str
    next_angles: list[str] = []
    llm: dict | None = None


@router.post("/ask-navigator", response_model=AskNavigatorResponse)
async def generate_ask_navigator_endpoint(req: AskNavigatorRequest, request: Request):
    ask_input = {
        "query": req.input.query,
        "answer": req.input.answer,
        "bullets": req.input.bullets,
        "citations": [citation.model_dump() for citation in req.input.citations],
        "related_items": [item.model_dump() for item in req.input.related_items],
    }
    result = await run_observed_request_async(
        request,
        metadata={"model": req.model or "", "persona": req.persona},
        input_payload={"persona": req.persona, "query": req.input.query, "model": req.model},
        call=lambda: dispatch_by_model_async(
            request,
            req.model,
            handlers=build_handler_map_async(
                "generate_ask_navigator",
                args_fn=lambda func, api_key: func(persona=req.persona, ask_input=ask_input, model=str(req.model), api_key=api_key or ""),
                anthropic_args_fn=lambda func, api_key: func(persona=req.persona, ask_input=ask_input, api_key=api_key, model=req.model),
            ),
        ),
        output_builder=lambda result: {"has_commentary": bool(result.get("commentary")), **llm_usage_summary(result)},
    )
    return AskNavigatorResponse(**result)

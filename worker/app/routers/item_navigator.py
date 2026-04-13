from fastapi import APIRouter, Request
from pydantic import BaseModel

from app.services.claude_service import generate_item_navigator
from app.services.llm_dispatch import dispatch_by_model_async
from app.services.router_observe import llm_usage_summary, run_observed_request_async
from app.auto_dispatch import build_handler_map_async

router = APIRouter()


class ItemNavigatorArticle(BaseModel):
    item_id: str
    title: str | None = None
    translated_title: str | None = None
    source_title: str | None = None
    summary: str
    facts: list[str] = []
    published_at: str | None = None


class ItemNavigatorRequest(BaseModel):
    persona: str = "editor"
    article: ItemNavigatorArticle
    model: str | None = None


class ItemNavigatorResponse(BaseModel):
    headline: str
    commentary: str
    stance_tags: list[str] = []
    llm: dict | None = None


@router.post("/item-navigator", response_model=ItemNavigatorResponse)
async def generate_item_navigator_endpoint(req: ItemNavigatorRequest, request: Request):
    article = {
        "item_id": req.article.item_id,
        "title": req.article.title,
        "translated_title": req.article.translated_title,
        "source_title": req.article.source_title,
        "summary": req.article.summary,
        "facts": req.article.facts,
        "published_at": req.article.published_at,
    }
    result = await run_observed_request_async(
        request,
        metadata={"model": req.model or "", "persona": req.persona, "item_id": req.article.item_id},
        input_payload={"persona": req.persona, "item_id": req.article.item_id, "model": req.model},
        call=lambda: dispatch_by_model_async(
            request,
            req.model,
            handlers=build_handler_map_async(
                "generate_item_navigator",
                args_fn=lambda func, api_key: func(persona=req.persona, article=article, model=str(req.model), api_key=api_key or ""),
                anthropic_args_fn=lambda func, api_key: func(persona=req.persona, article=article, api_key=api_key, model=req.model),
            ),
        ),
        output_builder=lambda result: {"has_commentary": bool(result.get("commentary")), **llm_usage_summary(result)},
    )
    return ItemNavigatorResponse(**result)

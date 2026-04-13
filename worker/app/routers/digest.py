from fastapi import APIRouter, Request
from pydantic import BaseModel
from app.services.claude_service import compose_digest, compose_digest_cluster_draft
from app.services.llm_dispatch import dispatch_by_model_async
from app.services.runtime_prompt_overrides import bind_prompt_override
from app.services.router_observe import llm_usage_summary, run_observed_request_async
from app.auto_dispatch import build_handler_map_async

router = APIRouter()


class DigestItem(BaseModel):
    rank: int
    title: str | None
    url: str
    summary: str
    topics: list[str]
    score: float | None = None


class ComposeDigestRequest(BaseModel):
    digest_date: str
    items: list[DigestItem]
    model: str | None = None
    prompt: dict | None = None


class ComposeDigestResponse(BaseModel):
    subject: str
    body: str
    llm: dict | None = None


class ComposeDigestClusterDraftRequest(BaseModel):
    cluster_label: str
    item_count: int
    topics: list[str] = []
    source_lines: list[str]
    model: str | None = None


class ComposeDigestClusterDraftResponse(BaseModel):
    draft_summary: str
    llm: dict | None = None


@router.post("/compose-digest", response_model=ComposeDigestResponse)
async def compose_digest_endpoint(req: ComposeDigestRequest, request: Request):
    items = [
        {
            "rank": i.rank,
            "title": i.title,
            "url": i.url,
            "summary": i.summary,
            "topics": i.topics,
            "score": i.score,
        }
        for i in req.items
    ]
    with bind_prompt_override((req.prompt or {}).get("prompt_key"), (req.prompt or {}).get("prompt_text"), (req.prompt or {}).get("system_instruction")):
        result = await run_observed_request_async(
            request,
            metadata={"model": req.model or "", "digest_date": req.digest_date, "items_count": len(req.items or [])},
            input_payload={"digest_date": req.digest_date, "items_count": len(req.items or []), "model": req.model},
            call=lambda: dispatch_by_model_async(
                request,
                req.model,
                handlers=build_handler_map_async(
                    "compose_digest",
                    args_fn=lambda func, api_key: func(req.digest_date, items, model=str(req.model), api_key=api_key or ""),
                    anthropic_args_fn=lambda func, api_key: func(req.digest_date, items, api_key=api_key, model=req.model),
                ),
            ),
            output_builder=lambda result: {
                "subject_chars": len(result.get("subject") or ""),
                "body_chars": len(result.get("body") or ""),
                **llm_usage_summary(result),
            },
        )
    return ComposeDigestResponse(**result)


@router.post("/compose-digest-cluster-draft", response_model=ComposeDigestClusterDraftResponse)
async def compose_digest_cluster_draft_endpoint(req: ComposeDigestClusterDraftRequest, request: Request):
    result = await run_observed_request_async(
        request,
        metadata={"model": req.model or "", "cluster_label": req.cluster_label, "item_count": req.item_count, "source_lines_count": len(req.source_lines or [])},
        input_payload={"cluster_label": req.cluster_label, "item_count": req.item_count, "model": req.model},
        call=lambda: dispatch_by_model_async(
            request,
            req.model,
            handlers=build_handler_map_async(
                "compose_digest_cluster_draft",
                args_fn=lambda func, api_key: func(cluster_label=req.cluster_label, item_count=req.item_count, topics=req.topics, source_lines=req.source_lines, model=str(req.model), api_key=api_key or ""),
                anthropic_args_fn=lambda func, api_key: func(cluster_label=req.cluster_label, item_count=req.item_count, topics=req.topics, source_lines=req.source_lines, api_key=api_key, model=req.model),
            ),
        ),
        output_builder=lambda result: {
            "draft_chars": len(result.get("draft_summary") or ""),
            **llm_usage_summary(result),
        },
    )
    return ComposeDigestClusterDraftResponse(**result)

from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel

from app.services.alibaba_service import compose_digest as compose_digest_alibaba
from app.services.alibaba_service import compose_digest_cluster_draft as compose_digest_cluster_draft_alibaba
from app.services.claude_service import compose_digest, compose_digest_cluster_draft
from app.services.deepseek_service import compose_digest as compose_digest_deepseek
from app.services.deepseek_service import compose_digest_cluster_draft as compose_digest_cluster_draft_deepseek
from app.services.fireworks_service import compose_digest as compose_digest_fireworks
from app.services.fireworks_service import compose_digest_cluster_draft as compose_digest_cluster_draft_fireworks
from app.services.gemini_service import compose_digest as compose_digest_gemini
from app.services.gemini_service import compose_digest_cluster_draft as compose_digest_cluster_draft_gemini
from app.services.groq_service import compose_digest as compose_digest_groq
from app.services.groq_service import compose_digest_cluster_draft as compose_digest_cluster_draft_groq
from app.services.llm_dispatch import dispatch_by_model
from app.services.mistral_service import compose_digest as compose_digest_mistral
from app.services.mistral_service import compose_digest_cluster_draft as compose_digest_cluster_draft_mistral
from app.services.openai_service import compose_digest as compose_digest_openai
from app.services.openai_service import compose_digest_cluster_draft as compose_digest_cluster_draft_openai
from app.services.openrouter_service import compose_digest as compose_digest_openrouter
from app.services.openrouter_service import compose_digest_cluster_draft as compose_digest_cluster_draft_openrouter
from app.services.poe_service import compose_digest as compose_digest_poe
from app.services.poe_service import compose_digest_cluster_draft as compose_digest_cluster_draft_poe
from app.services.xai_service import compose_digest as compose_digest_xai
from app.services.xai_service import compose_digest_cluster_draft as compose_digest_cluster_draft_xai
from app.services.zai_service import compose_digest as compose_digest_zai
from app.services.zai_service import compose_digest_cluster_draft as compose_digest_cluster_draft_zai
from app.services.router_observe import llm_usage_summary, run_observed_request

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
def compose_digest_endpoint(req: ComposeDigestRequest, request: Request):
    try:
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
        result = run_observed_request(
            request,
            metadata={"model": req.model or "", "digest_date": req.digest_date, "items_count": len(req.items or [])},
            input_payload={"digest_date": req.digest_date, "items_count": len(req.items or []), "model": req.model},
            call=lambda: dispatch_by_model(
                request,
                req.model,
                handlers={
                    "anthropic": lambda api_key: compose_digest(
                        req.digest_date,
                        items,
                        api_key=api_key,
                        model=req.model,
                    ),
                    "google": lambda api_key: compose_digest_gemini(req.digest_date, items, model=str(req.model), api_key=api_key or ""),
                    "fireworks": lambda api_key: compose_digest_fireworks(req.digest_date, items, model=str(req.model), api_key=api_key or ""),
                    "groq": lambda api_key: compose_digest_groq(req.digest_date, items, model=str(req.model), api_key=api_key or ""),
                    "deepseek": lambda api_key: compose_digest_deepseek(req.digest_date, items, model=str(req.model), api_key=api_key or ""),
                    "alibaba": lambda api_key: compose_digest_alibaba(req.digest_date, items, model=str(req.model), api_key=api_key or ""),
                    "mistral": lambda api_key: compose_digest_mistral(req.digest_date, items, model=str(req.model), api_key=api_key or ""),
                    "xai": lambda api_key: compose_digest_xai(req.digest_date, items, model=str(req.model), api_key=api_key or ""),
                    "zai": lambda api_key: compose_digest_zai(req.digest_date, items, model=str(req.model), api_key=api_key or ""),
                    "openrouter": lambda api_key: compose_digest_openrouter(req.digest_date, items, model=str(req.model), api_key=api_key or ""),
                    "poe": lambda api_key: compose_digest_poe(req.digest_date, items, model=str(req.model), api_key=api_key or ""),
                    "openai": lambda api_key: compose_digest_openai(req.digest_date, items, model=str(req.model), api_key=api_key or ""),
                },
            ),
            output_builder=lambda result: {
                "subject_chars": len(result.get("subject") or ""),
                "body_chars": len(result.get("body") or ""),
                **llm_usage_summary(result),
            },
        )
        return ComposeDigestResponse(**result)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"compose_digest failed: {e}")


@router.post("/compose-digest-cluster-draft", response_model=ComposeDigestClusterDraftResponse)
def compose_digest_cluster_draft_endpoint(req: ComposeDigestClusterDraftRequest, request: Request):
    try:
        result = run_observed_request(
            request,
            metadata={"model": req.model or "", "cluster_label": req.cluster_label, "item_count": req.item_count, "source_lines_count": len(req.source_lines or [])},
            input_payload={"cluster_label": req.cluster_label, "item_count": req.item_count, "model": req.model},
            call=lambda: dispatch_by_model(
                request,
                req.model,
                handlers={
                    "anthropic": lambda api_key: compose_digest_cluster_draft(
                        cluster_label=req.cluster_label,
                        item_count=req.item_count,
                        topics=req.topics,
                        source_lines=req.source_lines,
                        api_key=api_key,
                        model=req.model,
                    ),
                    "google": lambda api_key: compose_digest_cluster_draft_gemini(
                        cluster_label=req.cluster_label,
                        item_count=req.item_count,
                        topics=req.topics,
                        source_lines=req.source_lines,
                        model=str(req.model),
                        api_key=api_key or "",
                    ),
                    "fireworks": lambda api_key: compose_digest_cluster_draft_fireworks(
                        cluster_label=req.cluster_label,
                        item_count=req.item_count,
                        topics=req.topics,
                        source_lines=req.source_lines,
                        model=str(req.model),
                        api_key=api_key or "",
                    ),
                    "groq": lambda api_key: compose_digest_cluster_draft_groq(
                        cluster_label=req.cluster_label,
                        item_count=req.item_count,
                        topics=req.topics,
                        source_lines=req.source_lines,
                        model=str(req.model),
                        api_key=api_key or "",
                    ),
                    "deepseek": lambda api_key: compose_digest_cluster_draft_deepseek(
                        cluster_label=req.cluster_label,
                        item_count=req.item_count,
                        topics=req.topics,
                        source_lines=req.source_lines,
                        model=str(req.model),
                        api_key=api_key or "",
                    ),
                    "alibaba": lambda api_key: compose_digest_cluster_draft_alibaba(
                        cluster_label=req.cluster_label,
                        item_count=req.item_count,
                        topics=req.topics,
                        source_lines=req.source_lines,
                        model=str(req.model),
                        api_key=api_key or "",
                    ),
                    "mistral": lambda api_key: compose_digest_cluster_draft_mistral(
                        cluster_label=req.cluster_label,
                        item_count=req.item_count,
                        topics=req.topics,
                        source_lines=req.source_lines,
                        model=str(req.model),
                        api_key=api_key or "",
                    ),
                    "xai": lambda api_key: compose_digest_cluster_draft_xai(
                        cluster_label=req.cluster_label,
                        item_count=req.item_count,
                        topics=req.topics,
                        source_lines=req.source_lines,
                        model=str(req.model),
                        api_key=api_key or "",
                    ),
                    "zai": lambda api_key: compose_digest_cluster_draft_zai(
                        cluster_label=req.cluster_label,
                        item_count=req.item_count,
                        topics=req.topics,
                        source_lines=req.source_lines,
                        model=str(req.model),
                        api_key=api_key or "",
                    ),
                    "openrouter": lambda api_key: compose_digest_cluster_draft_openrouter(
                        cluster_label=req.cluster_label,
                        item_count=req.item_count,
                        topics=req.topics,
                        source_lines=req.source_lines,
                        model=str(req.model),
                        api_key=api_key or "",
                    ),
                    "poe": lambda api_key: compose_digest_cluster_draft_poe(
                        cluster_label=req.cluster_label,
                        item_count=req.item_count,
                        topics=req.topics,
                        source_lines=req.source_lines,
                        model=str(req.model),
                        api_key=api_key or "",
                    ),
                    "openai": lambda api_key: compose_digest_cluster_draft_openai(
                        cluster_label=req.cluster_label,
                        item_count=req.item_count,
                        topics=req.topics,
                        source_lines=req.source_lines,
                        model=str(req.model),
                        api_key=api_key or "",
                    ),
                },
            ),
            output_builder=lambda result: {
                "draft_chars": len(result.get("draft_summary") or ""),
                **llm_usage_summary(result),
            },
        )
        return ComposeDigestClusterDraftResponse(**result)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"compose_digest_cluster_draft failed: {e}")

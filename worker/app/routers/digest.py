from fastapi import APIRouter, Request
from pydantic import BaseModel

from app.services.claude_service import compose_digest, compose_digest_cluster_draft

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
    api_key = request.headers.get("x-anthropic-api-key") or None
    result = compose_digest(
        req.digest_date,
        [
            {
                "rank": i.rank,
                "title": i.title,
                "url": i.url,
                "summary": i.summary,
                "topics": i.topics,
                "score": i.score,
            }
            for i in req.items
        ],
        api_key=api_key,
        model=req.model,
    )
    return ComposeDigestResponse(**result)


@router.post("/compose-digest-cluster-draft", response_model=ComposeDigestClusterDraftResponse)
def compose_digest_cluster_draft_endpoint(req: ComposeDigestClusterDraftRequest, request: Request):
    api_key = request.headers.get("x-anthropic-api-key") or None
    result = compose_digest_cluster_draft(
        cluster_label=req.cluster_label,
        item_count=req.item_count,
        topics=req.topics,
        source_lines=req.source_lines,
        api_key=api_key,
        model=req.model,
    )
    return ComposeDigestClusterDraftResponse(**result)

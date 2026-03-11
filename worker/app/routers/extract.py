from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel
from app.services.trafilatura_service import extract_body
from app.services.router_observe import run_observed_request

router = APIRouter()


class ExtractRequest(BaseModel):
    url: str


class ExtractResponse(BaseModel):
    title: str | None
    content: str
    published_at: str | None
    image_url: str | None


@router.post("/extract-body", response_model=ExtractResponse)
def extract_body_endpoint(req: ExtractRequest, request: Request):
    def call():
        try:
            result = extract_body(req.url)
        except Exception:
            # Service already tries to degrade gracefully; keep router from returning noisy 500s.
            result = None
        return result
    result = run_observed_request(
        request,
        metadata={"url": req.url},
        input_payload={"url": req.url},
        call=call,
        output_builder=lambda result: {
            "title_present": bool((result or {}).get("title")),
            "content_chars": len((result or {}).get("content") or ""),
        },
        include_llm_result=False,
    )
    if result is None:
        raise HTTPException(status_code=422, detail="Failed to extract body")
    return result

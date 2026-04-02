from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel
from app.services.trafilatura_service import extract_body
from app.services.youtube_extract_service import (
    YouTubeTranscriptUnavailableError,
    extract_body as extract_youtube_body,
    is_youtube_url,
)
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
    call_error: Exception | None = None

    def call():
        nonlocal call_error
        try:
            if is_youtube_url(req.url):
                result = extract_youtube_body(req.url)
            else:
                result = extract_body(req.url)
        except Exception as exc:
            call_error = exc
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
        if call_error is not None:
            if isinstance(call_error, YouTubeTranscriptUnavailableError):
                raise HTTPException(
                    status_code=422,
                    detail={
                        "code": "youtube_transcript_unavailable",
                        "message": str(call_error),
                        "title": call_error.title,
                        "published_at": call_error.published_at,
                        "image_url": call_error.image_url,
                    },
                )
            raise HTTPException(status_code=422, detail=str(call_error))
        raise HTTPException(status_code=422, detail="Failed to extract body")
    return result

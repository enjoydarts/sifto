from fastapi import APIRouter, HTTPException
from pydantic import BaseModel
from app.services.trafilatura_service import extract_body

router = APIRouter()


class ExtractRequest(BaseModel):
    url: str


class ExtractResponse(BaseModel):
    title: str | None
    content: str
    published_at: str | None


@router.post("/extract-body", response_model=ExtractResponse)
def extract_body_endpoint(req: ExtractRequest):
    try:
        result = extract_body(req.url)
    except Exception:
        # Service already tries to degrade gracefully; keep router from returning noisy 500s.
        result = None
    if result is None:
        raise HTTPException(status_code=422, detail="Failed to extract body")
    return result

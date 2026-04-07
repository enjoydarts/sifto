from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel, Field

from app.services.fish_speech_preprocess import DEFAULT_FISH_PREPROCESS_PROMPT_KEY, FishSpeechPreprocessService
from app.services.llm_catalog import provider_api_key_header, provider_for_model
from app.services.router_observe import llm_usage_summary, run_observed_request

router = APIRouter()


class FishSpeechPreprocessRequest(BaseModel):
    text: str
    model: str
    prompt_key: str = DEFAULT_FISH_PREPROCESS_PROMPT_KEY
    variables: dict[str, str] | None = Field(default_factory=dict)


class FishSpeechPreprocessResponse(BaseModel):
    text: str
    llm: dict | None = None


@router.post("/fish/preprocess-text", response_model=FishSpeechPreprocessResponse)
def preprocess_fish_text(req: FishSpeechPreprocessRequest, request: Request):
    try:
        service = FishSpeechPreprocessService()
        provider = provider_for_model(req.model)
        if not provider:
            raise RuntimeError(f"unsupported fish preprocess model provider: {req.model}")
        api_key_header = provider_api_key_header(provider)
        api_key = request.headers.get(api_key_header, "").strip() if api_key_header else ""
        result = run_observed_request(
            request,
            metadata={
                "model": req.model,
                "provider": provider,
                "prompt_key": req.prompt_key,
                "text_chars": len(req.text or ""),
            },
            input_payload={
                "model": req.model,
                "provider": provider,
                "prompt_key": req.prompt_key,
                "text_chars": len(req.text or ""),
            },
            call=lambda: service.preprocess(
                text=req.text,
                model=req.model,
                api_key=api_key,
                prompt_key=req.prompt_key,
                variables=req.variables or {},
            ),
            output_builder=lambda result: {
                "text_chars": len(result.get("text") or ""),
                **llm_usage_summary(result),
            },
        )
        return FishSpeechPreprocessResponse(**result)
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"fish speech preprocess failed: {exc}")

from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel

from app.services.claude_service import generate_audio_briefing_script
from app.services.feed_task_common import is_audio_briefing_script_retryable_validation_error
from app.services.llm_dispatch import dispatch_by_model_async
from app.services.runtime_prompt_overrides import bind_prompt_override
from app.services.router_observe import llm_usage_summary, run_observed_request_async
from app.auto_dispatch import build_handler_map_async

router = APIRouter()


class AudioBriefingScriptArticle(BaseModel):
    item_id: str
    title: str | None = None
    translated_title: str | None = None
    source_title: str | None = None
    summary: str
    published_at: str | None = None


class AudioBriefingScriptRequest(BaseModel):
    persona: str = "editor"
    conversation_mode: str = "single"
    host_persona: str | None = None
    partner_persona: str | None = None
    articles: list[AudioBriefingScriptArticle]
    intro_context: dict
    model: str | None = None
    target_duration_minutes: int = 20
    target_chars: int = 12000
    chars_per_minute: int = 700
    include_opening: bool = True
    include_overall_summary: bool = True
    include_article_segments: bool = True
    include_ending: bool = True
    prompt: dict | None = None


class AudioBriefingScriptSegment(BaseModel):
    item_id: str
    headline: str
    summary_intro: str
    commentary: str


class AudioBriefingScriptTurn(BaseModel):
    speaker: str
    section: str
    item_id: str | None = None
    text: str


class AudioBriefingScriptResponse(BaseModel):
    opening: str = ""
    overall_summary: str = ""
    article_segments: list[AudioBriefingScriptSegment] = []
    turns: list[AudioBriefingScriptTurn] = []
    ending: str = ""
    llm: dict | None = None


@router.post("/audio-briefing-script", response_model=AudioBriefingScriptResponse)
async def generate_audio_briefing_script_endpoint(req: AudioBriefingScriptRequest, request: Request):
    intro_context = dict(req.intro_context or {})
    intro_context["audio_briefing_conversation_mode"] = req.conversation_mode
    if req.host_persona:
        intro_context["audio_briefing_host_persona"] = req.host_persona
    if req.partner_persona:
        intro_context["audio_briefing_partner_persona"] = req.partner_persona
    articles = [
        {
            "item_id": article.item_id,
            "title": article.title,
            "translated_title": article.translated_title,
            "source_title": article.source_title,
            "summary": article.summary,
            "published_at": article.published_at,
        }
        for article in req.articles
    ]
    try:
        with bind_prompt_override((req.prompt or {}).get("prompt_key"), (req.prompt or {}).get("prompt_text"), (req.prompt or {}).get("system_instruction")):
            result = await run_observed_request_async(
                request,
                metadata={"model": req.model or "", "persona": req.persona, "conversation_mode": req.conversation_mode, "articles_count": len(articles)},
                input_payload={"persona": req.persona, "conversation_mode": req.conversation_mode, "articles_count": len(articles), "model": req.model, "target_chars": req.target_chars},
                call=lambda: dispatch_by_model_async(
                    request,
                    req.model,
                    handlers=build_handler_map_async(
                        "generate_audio_briefing_script",
                        args_fn=lambda func, api_key: func(
                            persona=req.persona, articles=articles, intro_context=intro_context,
                            target_duration_minutes=req.target_duration_minutes, target_chars=req.target_chars,
                            chars_per_minute=req.chars_per_minute, include_opening=req.include_opening,
                            include_overall_summary=req.include_overall_summary,
                            include_article_segments=req.include_article_segments,
                            include_ending=req.include_ending,
                            model=str(req.model), api_key=api_key or "",
                        ),
                        anthropic_args_fn=lambda func, api_key: func(
                            persona=req.persona, articles=articles, intro_context=intro_context,
                            target_duration_minutes=req.target_duration_minutes, target_chars=req.target_chars,
                            chars_per_minute=req.chars_per_minute, include_opening=req.include_opening,
                            include_overall_summary=req.include_overall_summary,
                            include_article_segments=req.include_article_segments,
                            include_ending=req.include_ending,
                            api_key=api_key, model=req.model,
                        ),
                    ),
                ),
                output_builder=lambda result: {"items_count": len(result.get("article_segments") or []), **llm_usage_summary(result)},
            )
    except ValueError as exc:
        if is_audio_briefing_script_retryable_validation_error(exc):
            raise HTTPException(status_code=422, detail=str(exc))
        raise
    return AudioBriefingScriptResponse(**result)

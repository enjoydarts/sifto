from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel

from app.services.alibaba_service import generate_audio_briefing_script as generate_audio_briefing_script_alibaba
from app.services.claude_service import generate_audio_briefing_script
from app.services.deepseek_service import generate_audio_briefing_script as generate_audio_briefing_script_deepseek
from app.services.fireworks_service import generate_audio_briefing_script as generate_audio_briefing_script_fireworks
from app.services.gemini_service import generate_audio_briefing_script as generate_audio_briefing_script_gemini
from app.services.groq_service import generate_audio_briefing_script as generate_audio_briefing_script_groq
from app.services.feed_task_common import is_audio_briefing_script_retryable_validation_error
from app.services.llm_dispatch import dispatch_by_model
from app.services.mistral_service import generate_audio_briefing_script as generate_audio_briefing_script_mistral
from app.services.moonshot_service import generate_audio_briefing_script as generate_audio_briefing_script_moonshot
from app.services.openai_service import generate_audio_briefing_script as generate_audio_briefing_script_openai
from app.services.openrouter_service import generate_audio_briefing_script as generate_audio_briefing_script_openrouter
from app.services.poe_service import generate_audio_briefing_script as generate_audio_briefing_script_poe
from app.services.siliconflow_service import generate_audio_briefing_script as generate_audio_briefing_script_siliconflow
from app.services.router_observe import llm_usage_summary, run_observed_request
from app.services.xai_service import generate_audio_briefing_script as generate_audio_briefing_script_xai
from app.services.zai_service import generate_audio_briefing_script as generate_audio_briefing_script_zai

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


class AudioBriefingScriptSegment(BaseModel):
    item_id: str
    headline: str
    commentary: str


class AudioBriefingScriptResponse(BaseModel):
    opening: str
    overall_summary: str
    article_segments: list[AudioBriefingScriptSegment]
    ending: str
    llm: dict | None = None


@router.post("/audio-briefing-script", response_model=AudioBriefingScriptResponse)
def generate_audio_briefing_script_endpoint(req: AudioBriefingScriptRequest, request: Request):
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
        result = run_observed_request(
            request,
            metadata={"model": req.model or "", "persona": req.persona, "articles_count": len(articles)},
            input_payload={"persona": req.persona, "articles_count": len(articles), "model": req.model, "target_chars": req.target_chars},
            call=lambda: dispatch_by_model(
                request,
                req.model,
                handlers={
                "anthropic": lambda api_key: generate_audio_briefing_script(
                    persona=req.persona,
                    articles=articles,
                    intro_context=req.intro_context,
                    target_duration_minutes=req.target_duration_minutes,
                    target_chars=req.target_chars,
                    chars_per_minute=req.chars_per_minute,
                    include_opening=req.include_opening,
                    include_overall_summary=req.include_overall_summary,
                    include_article_segments=req.include_article_segments,
                    include_ending=req.include_ending,
                    api_key=api_key,
                    model=req.model,
                ),
                "google": lambda api_key: generate_audio_briefing_script_gemini(
                    persona=req.persona,
                    articles=articles,
                    intro_context=req.intro_context,
                    target_duration_minutes=req.target_duration_minutes,
                    target_chars=req.target_chars,
                    chars_per_minute=req.chars_per_minute,
                    include_opening=req.include_opening,
                    include_overall_summary=req.include_overall_summary,
                    include_article_segments=req.include_article_segments,
                    include_ending=req.include_ending,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "fireworks": lambda api_key: generate_audio_briefing_script_fireworks(
                    persona=req.persona,
                    articles=articles,
                    intro_context=req.intro_context,
                    target_duration_minutes=req.target_duration_minutes,
                    target_chars=req.target_chars,
                    chars_per_minute=req.chars_per_minute,
                    include_opening=req.include_opening,
                    include_overall_summary=req.include_overall_summary,
                    include_article_segments=req.include_article_segments,
                    include_ending=req.include_ending,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "groq": lambda api_key: generate_audio_briefing_script_groq(
                    persona=req.persona,
                    articles=articles,
                    intro_context=req.intro_context,
                    target_duration_minutes=req.target_duration_minutes,
                    target_chars=req.target_chars,
                    chars_per_minute=req.chars_per_minute,
                    include_opening=req.include_opening,
                    include_overall_summary=req.include_overall_summary,
                    include_article_segments=req.include_article_segments,
                    include_ending=req.include_ending,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "deepseek": lambda api_key: generate_audio_briefing_script_deepseek(
                    persona=req.persona,
                    articles=articles,
                    intro_context=req.intro_context,
                    target_duration_minutes=req.target_duration_minutes,
                    target_chars=req.target_chars,
                    chars_per_minute=req.chars_per_minute,
                    include_opening=req.include_opening,
                    include_overall_summary=req.include_overall_summary,
                    include_article_segments=req.include_article_segments,
                    include_ending=req.include_ending,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "alibaba": lambda api_key: generate_audio_briefing_script_alibaba(
                    persona=req.persona,
                    articles=articles,
                    intro_context=req.intro_context,
                    target_duration_minutes=req.target_duration_minutes,
                    target_chars=req.target_chars,
                    chars_per_minute=req.chars_per_minute,
                    include_opening=req.include_opening,
                    include_overall_summary=req.include_overall_summary,
                    include_article_segments=req.include_article_segments,
                    include_ending=req.include_ending,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "mistral": lambda api_key: generate_audio_briefing_script_mistral(
                    persona=req.persona,
                    articles=articles,
                    intro_context=req.intro_context,
                    target_duration_minutes=req.target_duration_minutes,
                    target_chars=req.target_chars,
                    chars_per_minute=req.chars_per_minute,
                    include_opening=req.include_opening,
                    include_overall_summary=req.include_overall_summary,
                    include_article_segments=req.include_article_segments,
                    include_ending=req.include_ending,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "moonshot": lambda api_key: generate_audio_briefing_script_moonshot(
                    persona=req.persona,
                    articles=articles,
                    intro_context=req.intro_context,
                    target_duration_minutes=req.target_duration_minutes,
                    target_chars=req.target_chars,
                    chars_per_minute=req.chars_per_minute,
                    include_opening=req.include_opening,
                    include_overall_summary=req.include_overall_summary,
                    include_article_segments=req.include_article_segments,
                    include_ending=req.include_ending,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "xai": lambda api_key: generate_audio_briefing_script_xai(
                    persona=req.persona,
                    articles=articles,
                    intro_context=req.intro_context,
                    target_duration_minutes=req.target_duration_minutes,
                    target_chars=req.target_chars,
                    chars_per_minute=req.chars_per_minute,
                    include_opening=req.include_opening,
                    include_overall_summary=req.include_overall_summary,
                    include_article_segments=req.include_article_segments,
                    include_ending=req.include_ending,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "zai": lambda api_key: generate_audio_briefing_script_zai(
                    persona=req.persona,
                    articles=articles,
                    intro_context=req.intro_context,
                    target_duration_minutes=req.target_duration_minutes,
                    target_chars=req.target_chars,
                    chars_per_minute=req.chars_per_minute,
                    include_opening=req.include_opening,
                    include_overall_summary=req.include_overall_summary,
                    include_article_segments=req.include_article_segments,
                    include_ending=req.include_ending,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "openrouter": lambda api_key: generate_audio_briefing_script_openrouter(
                    persona=req.persona,
                    articles=articles,
                    intro_context=req.intro_context,
                    target_duration_minutes=req.target_duration_minutes,
                    target_chars=req.target_chars,
                    chars_per_minute=req.chars_per_minute,
                    include_opening=req.include_opening,
                    include_overall_summary=req.include_overall_summary,
                    include_article_segments=req.include_article_segments,
                    include_ending=req.include_ending,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "poe": lambda api_key: generate_audio_briefing_script_poe(
                    persona=req.persona,
                    articles=articles,
                    intro_context=req.intro_context,
                    target_duration_minutes=req.target_duration_minutes,
                    target_chars=req.target_chars,
                    chars_per_minute=req.chars_per_minute,
                    include_opening=req.include_opening,
                    include_overall_summary=req.include_overall_summary,
                    include_article_segments=req.include_article_segments,
                    include_ending=req.include_ending,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "siliconflow": lambda api_key: generate_audio_briefing_script_siliconflow(
                    persona=req.persona,
                    articles=articles,
                    intro_context=req.intro_context,
                    target_duration_minutes=req.target_duration_minutes,
                    target_chars=req.target_chars,
                    chars_per_minute=req.chars_per_minute,
                    include_opening=req.include_opening,
                    include_overall_summary=req.include_overall_summary,
                    include_article_segments=req.include_article_segments,
                    include_ending=req.include_ending,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                "openai": lambda api_key: generate_audio_briefing_script_openai(
                    persona=req.persona,
                    articles=articles,
                    intro_context=req.intro_context,
                    target_duration_minutes=req.target_duration_minutes,
                    target_chars=req.target_chars,
                    chars_per_minute=req.chars_per_minute,
                    include_opening=req.include_opening,
                    include_overall_summary=req.include_overall_summary,
                    include_article_segments=req.include_article_segments,
                    include_ending=req.include_ending,
                    model=str(req.model),
                    api_key=api_key or "",
                ),
                },
            ),
            output_builder=lambda result: {"items_count": len(result.get("article_segments") or []), **llm_usage_summary(result)},
        )
    except ValueError as exc:
        if is_audio_briefing_script_retryable_validation_error(exc):
            raise HTTPException(status_code=422, detail=str(exc))
        raise
    return AudioBriefingScriptResponse(**result)

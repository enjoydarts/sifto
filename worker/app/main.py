import os
import logging
import secrets
from copy import deepcopy
from contextlib import asynccontextmanager

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse
import sentry_sdk
from sentry_sdk.integrations.fastapi import FastApiIntegration
from app.routers import ai_navigator_brief, ask, ask_navigator, audio_briefing_script, audio_briefing_tts, briefing_navigator, digest, extract, facts, facts_check, feed_seed_suggestions, feed_suggestions, item_navigator, source_navigator, summary_audio_player, summarize, summary_faithfulness, translate_title, tts_markup_preprocess
from app.services.langfuse_client import flush as langfuse_flush, log_runtime_status as langfuse_log_runtime_status, span as langfuse_span, update_current as langfuse_update_current, update_current_trace as langfuse_update_current_trace
from app.services.alibaba_service import alibaba_workspace_context

_SENTRY_DSN = os.getenv("SENTRY_DSN", "").strip()
_log = logging.getLogger(__name__)


def _scrub_sentry_event(event, hint):
    # Provider exceptions and logging breadcrumbs can contain prompts and keys
    # even with local-variable capture disabled. Keep stack locations and types.
    event = deepcopy(event)
    for key in ("request", "extra", "user", "breadcrumbs", "logentry", "message", "tags", "fingerprint"):
        event.pop(key, None)
    trace = event.get("contexts", {}).get("trace", {})
    event["contexts"] = {
        "trace": {key: trace[key] for key in ("trace_id", "span_id", "parent_span_id", "op", "status") if key in trace}
    }
    for span in event.get("spans", []):
        span.pop("data", None)
        span.pop("description", None)
    for container in (event.get("exception", {}), event.get("threads", {})):
        for value in container.get("values", []):
            if "value" in value:
                value["value"] = "[Filtered]"
            for frame in value.get("stacktrace", {}).get("frames", []):
                frame.pop("vars", None)
    return event


def _configure_sentry():
    if not _SENTRY_DSN:
        return
    sentry_sdk.init(
        dsn=_SENTRY_DSN,
        environment=os.getenv("SENTRY_ENVIRONMENT", "").strip() or None,
        release=os.getenv("APP_COMMIT_SHA", "").strip() or None,
        integrations=[FastApiIntegration()],
        include_local_variables=False,
        send_default_pii=False,
        max_request_body_size="never",
        before_send=_scrub_sentry_event,
        before_send_transaction=_scrub_sentry_event,
        traces_sample_rate=float(os.getenv("SENTRY_TRACES_SAMPLE_RATE", "0")),
    )


_configure_sentry()


@asynccontextmanager
async def lifespan(app: FastAPI):
    try:
        langfuse_log_runtime_status()
    except Exception as e:
        _log.warning("failed to log langfuse runtime status: %s", e)
    yield
    langfuse_flush()


app = FastAPI(title="sifto-worker", lifespan=lifespan)


def _public_error_detail(request: Request, exc: Exception) -> str:
    internal_secret = _INTERNAL_WORKER_SECRET
    provided = str(request.headers.get("x-internal-worker-secret") or "").strip()
    if internal_secret and secrets.compare_digest(provided, internal_secret):
        # Preserve fixed failure categories needed by API fallback/check handling.
        # Provider exception bodies can contain prompts, keys and private output.
        message = str(exc).lower()
        categories = (
            (("parse failed", "short_comment missing", "output_truncated"), "LLM response parse failed"),
            (("status=429", "status 429", "rate limit"), "upstream rate limit"),
            (("status=404", "status 404"), "upstream status 404"),
            (("status=502", "status 502"), "upstream status 502"),
            (("timeout", "timed out"), "upstream timeout"),
            (("empty choices",), "upstream empty choices"),
            (("overload", "temporarily unavailable"), "upstream temporarily unavailable"),
            (("provider returned error",), "upstream provider returned error"),
        )
        for hints, detail in categories:
            if any(hint in message for hint in hints):
                return detail
    return "internal server error"


@app.exception_handler(Exception)
async def global_exception_handler(request: Request, exc: Exception):
    _log.error("unhandled exception on %s %s: %s", request.method, request.url.path, exc, exc_info=True)
    return JSONResponse(
        status_code=500,
        content={"detail": _public_error_detail(request, exc)},
        # Starlette re-raises this exception after sending the response, and
        # Uvicorn then closes the socket. Keep proxies from reusing that socket
        # for the immediate fallback request before the close reaches them.
        headers={"Connection": "close"},
    )

_INTERNAL_WORKER_SECRET = os.getenv("INTERNAL_WORKER_SECRET", "").strip()


def _worker_auth_error_status(path: str, provided: str, configured: str) -> int | None:
    if path == "/health":
        return None
    if not configured:
        return 503
    if not provided or not secrets.compare_digest(provided, configured):
        return 401
    return None


def _normalize_string_for_trace(value: str | None, limit: int | None = None) -> str:
    if value is None:
        return ""
    normalized = str(value).encode("utf-8", "replace").decode("utf-8", "replace").strip()
    if limit is None:
        return normalized
    if len(normalized) <= limit:
        return normalized
    return normalized[:limit]


@app.middleware("http")
async def require_internal_worker_secret(request: Request, call_next):
    provided = str(request.headers.get("x-internal-worker-secret") or "").strip()
    auth_error = _worker_auth_error_status(request.url.path, provided, _INTERNAL_WORKER_SECRET)
    if auth_error == 503:
        return JSONResponse(status_code=503, content={"detail": "worker authentication is not configured"})
    if auth_error == 401:
        return JSONResponse(status_code=401, content={"detail": "unauthorized"})
    if auth_error is not None:
        return JSONResponse(status_code=auth_error, content={"detail": "unauthorized"})
    with alibaba_workspace_context(request.headers.get("x-alibaba-workspace-id", "")):
        return await call_next(request)


@app.middleware("http")
async def langfuse_request_tracing(request: Request, call_next):
    auth_error = _worker_auth_error_status(request.url.path, str(request.headers.get("x-internal-worker-secret") or "").strip(), _INTERNAL_WORKER_SECRET)
    if auth_error is not None:
        return JSONResponse(status_code=auth_error, content={"detail": "unauthorized"})
    if request.url.path == "/health":
        return await call_next(request)
    user_id = _normalize_string_for_trace(request.headers.get("x-sifto-user-id"))
    provider_hint = _normalize_string_for_trace(request.headers.get("x-llm-provider"))
    model_hint = _normalize_string_for_trace(request.headers.get("x-llm-model"))
    item_id = _normalize_string_for_trace(request.headers.get("x-sifto-item-id"))
    digest_id = _normalize_string_for_trace(request.headers.get("x-sifto-digest-id"))
    source_id = _normalize_string_for_trace(request.headers.get("x-sifto-source-id"))
    purpose = _normalize_string_for_trace(request.headers.get("x-sifto-purpose"))
    metadata = {
        "path": request.url.path,
        "method": request.method,
        "user_id": user_id,
        "provider_hint": provider_hint,
        "model_hint": model_hint,
        "item_id": item_id,
        "digest_id": digest_id,
        "source_id": source_id,
        "purpose": purpose,
    }
    observation_type = "span" if request.url.path == "/extract-body" else "generation"
    with langfuse_span(
        f"worker:{request.url.path.strip('/') or 'root'}",
        metadata=metadata,
        tags=[
            "worker",
            f"path:{request.url.path}",
            f"purpose:{metadata['purpose'] or 'unknown'}",
        ],
        as_type=observation_type,
    ) as current_span:
        request.state.langfuse_span = current_span
        session_id = ""
        if item_id:
            session_id = f"item:{item_id}"
        elif digest_id:
            session_id = f"digest:{digest_id}"
        elif source_id:
            session_id = f"source:{source_id}"
        langfuse_update_current_trace(
            user_id=user_id or None,
            session_id=session_id or None,
            tags=[
                "worker",
                f"path:{request.url.path}",
                f"purpose:{purpose or 'unknown'}",
            ],
        )
        try:
            response = await call_next(request)
            langfuse_update_current(metadata={"status_code": response.status_code})
            return response
        except Exception as e:
            langfuse_update_current(level="ERROR", status_message=_normalize_string_for_trace(str(e), 500))
            raise


app.include_router(extract.router)
app.include_router(facts.router)
app.include_router(facts_check.router)
app.include_router(summarize.router)
app.include_router(summary_faithfulness.router)
app.include_router(translate_title.router)
app.include_router(audio_briefing_tts.router)
app.include_router(summary_audio_player.router)
app.include_router(tts_markup_preprocess.router)
app.include_router(audio_briefing_script.router)
app.include_router(ask.router)
app.include_router(ask_navigator.router)
app.include_router(digest.router)
app.include_router(feed_suggestions.router)
app.include_router(feed_seed_suggestions.router)
app.include_router(briefing_navigator.router)
app.include_router(ai_navigator_brief.router)
app.include_router(item_navigator.router)
app.include_router(source_navigator.router)


@app.get("/health")
def health():
    return {"status": "ok"}

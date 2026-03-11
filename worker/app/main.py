import os
import logging

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse
import sentry_sdk
from sentry_sdk.integrations.fastapi import FastApiIntegration
from app.routers import ask, digest, extract, facts, facts_check, feed_seed_suggestions, feed_suggestions, summarize, summary_faithfulness, translate_title
from app.services.langfuse_client import flush as langfuse_flush, log_runtime_status as langfuse_log_runtime_status, span as langfuse_span, update_current as langfuse_update_current, update_current_trace as langfuse_update_current_trace

_SENTRY_DSN = os.getenv("SENTRY_DSN", "").strip()
_log = logging.getLogger(__name__)
if _SENTRY_DSN:
    sentry_sdk.init(
        dsn=_SENTRY_DSN,
        environment=os.getenv("SENTRY_ENVIRONMENT", "").strip() or None,
        release=os.getenv("APP_COMMIT_SHA", "").strip() or None,
        integrations=[FastApiIntegration()],
        traces_sample_rate=float(os.getenv("SENTRY_TRACES_SAMPLE_RATE", "0")),
    )

app = FastAPI(title="sifto-worker")

_INTERNAL_WORKER_SECRET = os.getenv("INTERNAL_WORKER_SECRET", "").strip()


@app.middleware("http")
async def require_internal_worker_secret(request: Request, call_next):
    if request.url.path == "/health" or not _INTERNAL_WORKER_SECRET:
        return await call_next(request)
    provided = request.headers.get("x-internal-worker-secret", "")
    if provided != _INTERNAL_WORKER_SECRET:
        return JSONResponse(status_code=401, content={"detail": "unauthorized"})
    return await call_next(request)


@app.middleware("http")
async def langfuse_request_tracing(request: Request, call_next):
    if request.url.path == "/health":
        return await call_next(request)
    metadata = {
        "path": request.url.path,
        "method": request.method,
        "user_id": request.headers.get("x-sifto-user-id", ""),
        "provider_hint": request.headers.get("x-llm-provider", ""),
        "model_hint": request.headers.get("x-llm-model", ""),
        "item_id": request.headers.get("x-sifto-item-id", ""),
        "digest_id": request.headers.get("x-sifto-digest-id", ""),
        "source_id": request.headers.get("x-sifto-source-id", ""),
        "purpose": request.headers.get("x-sifto-purpose", ""),
    }
    with langfuse_span(
        f"worker:{request.url.path.strip('/') or 'root'}",
        metadata=metadata,
        tags=[
            "worker",
            f"path:{request.url.path}",
            f"purpose:{metadata['purpose'] or 'unknown'}",
        ],
    ) as current_span:
        request.state.langfuse_span = current_span
        session_id = ""
        if metadata["item_id"]:
            session_id = f"item:{metadata['item_id']}"
        elif metadata["digest_id"]:
            session_id = f"digest:{metadata['digest_id']}"
        elif metadata["source_id"]:
            session_id = f"source:{metadata['source_id']}"
        langfuse_update_current_trace(
            user_id=metadata["user_id"] or None,
            session_id=session_id or None,
            tags=[
                "worker",
                f"path:{request.url.path}",
                f"purpose:{metadata['purpose'] or 'unknown'}",
            ],
        )
        try:
            response = await call_next(request)
            langfuse_update_current(metadata={"status_code": response.status_code})
            return response
        except Exception as e:
            langfuse_update_current(level="ERROR", status_message=str(e)[:500])
            raise


@app.on_event("shutdown")
def flush_langfuse():
    langfuse_flush()


@app.on_event("startup")
def log_langfuse_status():
    try:
        langfuse_log_runtime_status()
    except Exception as e:
        _log.warning("failed to log langfuse runtime status: %s", e)

app.include_router(extract.router)
app.include_router(facts.router)
app.include_router(facts_check.router)
app.include_router(summarize.router)
app.include_router(summary_faithfulness.router)
app.include_router(translate_title.router)
app.include_router(ask.router)
app.include_router(digest.router)
app.include_router(feed_suggestions.router)
app.include_router(feed_seed_suggestions.router)


@app.get("/health")
def health():
    return {"status": "ok"}

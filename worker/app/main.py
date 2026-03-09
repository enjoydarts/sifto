import os

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse
import sentry_sdk
from sentry_sdk.integrations.fastapi import FastApiIntegration
from app.routers import ask, digest, extract, facts, feed_seed_suggestions, feed_suggestions, summarize, translate_title

_SENTRY_DSN = os.getenv("SENTRY_DSN", "").strip()
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

app.include_router(extract.router)
app.include_router(facts.router)
app.include_router(summarize.router)
app.include_router(translate_title.router)
app.include_router(ask.router)
app.include_router(digest.router)
app.include_router(feed_suggestions.router)
app.include_router(feed_seed_suggestions.router)


@app.get("/health")
def health():
    return {"status": "ok"}

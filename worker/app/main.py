import os

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse
from app.routers import digest, extract, facts, feed_seed_suggestions, feed_suggestions, summarize, translate_title

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
app.include_router(digest.router)
app.include_router(feed_suggestions.router)
app.include_router(feed_seed_suggestions.router)


@app.get("/health")
def health():
    return {"status": "ok"}

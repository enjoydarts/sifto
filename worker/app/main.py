from fastapi import FastAPI
from app.routers import digest, extract, facts, summarize

app = FastAPI(title="sifto-worker")

app.include_router(extract.router)
app.include_router(facts.router)
app.include_router(summarize.router)
app.include_router(digest.router)


@app.get("/health")
def health():
    return {"status": "ok"}

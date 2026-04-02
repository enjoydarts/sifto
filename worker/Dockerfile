FROM python:3.12-slim

WORKDIR /app

COPY worker/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt \
    && pip install --no-cache-dir -U yt-dlp \
    && pip install --no-cache-dir bgutil-ytdlp-pot-provider==1.3.1

COPY worker/app/ ./app/
COPY shared/ ./shared/

ENV LLM_CATALOG_PATH=/app/shared/llm_catalog.json

EXPOSE 8000

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]

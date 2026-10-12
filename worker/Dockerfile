FROM python:3.12.15-slim

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends curl \
    && rm -rf /var/lib/apt/lists/*

COPY worker/requirements.txt .
RUN pip install --no-cache-dir --require-hashes -r requirements.txt

COPY worker/app/ ./app/
COPY shared/ ./shared/
COPY worker/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

RUN chmod 755 /usr/local/bin/docker-entrypoint.sh

ENV LLM_CATALOG_PATH=/app/shared/llm_catalog.json

EXPOSE 8000

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]

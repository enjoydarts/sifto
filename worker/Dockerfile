FROM python:3.12.15-slim AS base

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends liblzma5 \
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

# Mounted development sources retain the existing root execution workflow.
FROM base AS dev

FROM base AS runner
RUN python -m pip uninstall --yes pip \
    && groupadd --gid 10001 sifto \
    && useradd --uid 10001 --gid sifto --create-home --home-dir /home/sifto sifto \
    && mkdir -p /home/sifto/.cache \
    && chown 10001:10001 /home/sifto/.cache
ENV HOME=/home/sifto
ENV XDG_CACHE_HOME=/home/sifto/.cache
ENV PYTHONDONTWRITEBYTECODE=1
USER 10001:10001

# Keep Coolify readiness checks independent of curl and its OS dependencies.
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD ["python", "-c", "import http.client; c = http.client.HTTPConnection('127.0.0.1', 8000, timeout=3); c.request('GET', '/health'); r = c.getresponse(); raise SystemExit(0 if r.status == 200 else 1)"]

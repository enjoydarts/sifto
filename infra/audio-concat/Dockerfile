FROM python:3.12-slim AS base

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg ca-certificates liblzma5 \
    && update-ca-certificates --fresh \
    && rm -rf /var/lib/apt/lists/*

# Botocore otherwise selects its own bundle, which retains the AAA root.
ENV AWS_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt

COPY infra/audio-concat/requirements.txt .
RUN pip install --no-cache-dir --require-hashes -r requirements.txt

COPY infra/audio-concat/app/ ./app/

CMD ["python", "-m", "app.main"]

FROM base AS dev

FROM base AS runner
RUN python -m pip uninstall --yes pip

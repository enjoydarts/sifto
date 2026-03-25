FROM python:3.12-slim

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY infra/audio-concat/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY infra/audio-concat/app/ ./app/

CMD ["python", "-m", "app.main"]

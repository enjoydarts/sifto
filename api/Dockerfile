FROM golang:1.27.1-alpine AS builder
ARG APP_COMMIT_SHA=unknown

WORKDIR /src

COPY api/ ./api/
COPY shared/ ./shared/
WORKDIR /src/api
RUN go mod download && \
    go build -o /out/server ./cmd/server

FROM golang:1.27.1-alpine AS dev

WORKDIR /app

COPY api/ ./

RUN go mod download && \
    go install github.com/air-verse/air@v1.67.4

ENV LLM_CATALOG_PATH=/shared/llm_catalog.json

EXPOSE 8080

CMD ["air", "-c", ".air.toml"]

FROM alpine:latest@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6 AS runner
ARG APP_COMMIT_SHA=unknown

RUN apk add --no-cache ca-certificates && \
    addgroup -g 10001 sifto && \
    adduser -D -u 10001 -G sifto -h /home/sifto sifto

WORKDIR /app
ENV APP_COMMIT_SHA=${APP_COMMIT_SHA}
ENV LLM_CATALOG_PATH=/app/shared/llm_catalog.json
ENV HOME=/home/sifto

COPY --from=builder /out/server .
COPY --from=builder /src/shared ./shared

USER 10001:10001

EXPOSE 8080

CMD ["./server"]

FROM golang:alpine AS builder
ARG APP_COMMIT_SHA=unknown

WORKDIR /src

COPY api/ ./api/
COPY shared/ ./shared/
WORKDIR /src/api
RUN go mod download && \
    go build -o /out/server ./cmd/server

FROM golang:alpine AS dev

WORKDIR /app

COPY api/ ./

RUN go mod download && \
    go install github.com/air-verse/air@latest

ENV LLM_CATALOG_PATH=/shared/llm_catalog.json

EXPOSE 8080

CMD ["air", "-c", ".air.toml"]

FROM alpine:latest
ARG APP_COMMIT_SHA=unknown

RUN apk add --no-cache ca-certificates

WORKDIR /app
ENV APP_COMMIT_SHA=${APP_COMMIT_SHA}
ENV LLM_CATALOG_PATH=/app/shared/llm_catalog.json

COPY --from=builder /out/server .
COPY --from=builder /src/shared ./shared

EXPOSE 8080

CMD ["./server"]

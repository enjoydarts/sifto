FROM golang:alpine AS builder

WORKDIR /app

# Copy all sources first, then get deps (so go.mod is updated in-place)
COPY . .
RUN go get \
    github.com/go-chi/chi/v5@latest \
    github.com/golang-jwt/jwt/v5@latest \
    github.com/inngest/inngestgo@latest \
    github.com/jackc/pgx/v5@latest \
    github.com/mmcdole/gofeed@latest && \
    go mod tidy && \
    go build -o bin/server ./cmd/server

FROM golang:alpine AS dev

WORKDIR /app

COPY . .

RUN go get \
    github.com/go-chi/chi/v5@latest \
    github.com/golang-jwt/jwt/v5@latest \
    github.com/inngest/inngestgo@latest \
    github.com/jackc/pgx/v5@latest \
    github.com/mmcdole/gofeed@latest && \
    go mod tidy

EXPOSE 8080

CMD ["go", "run", "./cmd/server"]

FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /app/bin/server .

EXPOSE 8080

CMD ["./server"]

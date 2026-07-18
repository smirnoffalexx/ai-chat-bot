# --- build stage ---
FROM golang:1.26.5-alpine AS build

WORKDIR /src

# Download dependencies first for better layer caching.
COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

# Static, stripped binary.
ENV CGO_ENABLED=0 GOOS=linux
RUN go build -trimpath -ldflags="-s -w" -o /out/bot ./cmd/bot

# --- runtime stage ---
FROM alpine:latest

# CA certificates for outbound HTTPS to the Telegram and Anthropic APIs.
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 app

WORKDIR /app
COPY --from=build /out/bot /app/bot

USER app

# All configuration comes from environment variables. On Railway, set them in
# the service's Variables tab. Locally:
#   docker run --rm \
#     -e TELEGRAM_TOKEN=... -e ANTHROPIC_API_KEY=... -e ALLOWED_USER_IDS=123456789 \
#     ai-chat-bot
# (or: docker run --rm --env-file .env ai-chat-bot)
ENTRYPOINT ["/app/bot"]

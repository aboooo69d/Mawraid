# ---- Stage 1: Build ----
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Download dependencies first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy application source
COPY . .

# Build a static, stripped Linux binary
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/nova-api ./cmd/api

# ---- Stage 2: Runtime ----
FROM alpine:3 AS runtime

# CA certificates for PostgreSQL TLS, Cloudflare R2, and Turnstile HTTPS calls; tzdata for timezones
RUN apk add --no-cache ca-certificates tzdata

# Run as an unprivileged user
RUN addgroup -S appgroup && adduser -S -G appgroup appuser

WORKDIR /app

COPY --from=builder --chown=appuser:appgroup /out/nova-api /app/nova-api

USER appuser

ENV PORT=8080 \
    GIN_MODE=release

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- "http://127.0.0.1:${PORT:-8080}/health" >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/app/nova-api"]

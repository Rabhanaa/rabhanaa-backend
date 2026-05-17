# =============================================================================
# MULTI-STAGE DOCKERFILE — rabhana auction API
# Builds from source inside the container. No pre-built binary required.
# =============================================================================

ARG GO_VERSION=1.25

# --- Stage 1: download modules (cached when go.mod/go.sum unchanged) ---------
FROM golang:${GO_VERSION}-alpine3.21 AS deps
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# --- Stage 2: compile static binary ------------------------------------------
FROM golang:${GO_VERSION}-alpine3.21 AS builder
WORKDIR /app
COPY --from=deps /go/pkg/mod /go/pkg/mod
COPY go.mod go.sum ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -extldflags '-static'" \
    -trimpath \
    -o bin/api \
    ./api/main.go

# --- Stage 3: minimal runtime image ------------------------------------------
FROM alpine:3.21 AS final

RUN apk add --no-cache ca-certificates tzdata \
 && addgroup -S appgroup \
 && adduser -S -G appgroup -D -H appuser

WORKDIR /app

COPY --from=builder /app/bin/api ./api
RUN mkdir -p uploads && chown appuser:appgroup uploads

ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown
LABEL org.opencontainers.image.title="rabhana-api" \
      org.opencontainers.image.description="Rabhana auction platform — Go API" \
      org.opencontainers.image.revision="${GIT_COMMIT}" \
      org.opencontainers.image.created="${BUILD_TIME}" \
      org.opencontainers.image.base.name="alpine:3.21"

USER appuser
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["./api"]

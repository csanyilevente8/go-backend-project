# syntax=docker/dockerfile:1

# ---- Build stage -----------------------------------------------------------
FROM golang:1.25-alpine AS build
WORKDIR /src

# Cache modules first.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Static build (CGO disabled) so it runs on a minimal/distroless runtime.
# Build for linux/amd64 to match the GKE nodes (set by buildx --platform).
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/server ./cmd/server

# ---- Runtime stage ---------------------------------------------------------
# Distroless static: tiny, no shell, non-root by default.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /app

COPY --from=build /out/server /app/server

EXPOSE 8080

# DB connection + CORS are supplied via environment variables at runtime
# (DB_HOST, DB_PORT, DB_NAME, DB_USERNAME, DB_PASSWORD, CORS_ALLOWED_ORIGINS).
ENTRYPOINT ["/app/server"]

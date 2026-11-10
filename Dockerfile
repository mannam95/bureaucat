# Build frontend
FROM oven/bun:1.4.4-slim AS frontend
WORKDIR /app/web
COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile
COPY web .
RUN bun run generate

# Build backend
FROM golang:1.27.3-trixie AS backend
ARG VERSION=dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Copy Nuxt generated static files to embed location
COPY --from=frontend /app/web/.output/public ./cmd/bureaucat/dist
# Copy migrations to embed location
RUN cp -r migrations/* ./cmd/bureaucat/migrations/
# Regenerate the OpenAPI spec from the handler annotations, so /docs and
# /api/v1/openapi.json can never ship stale (version pinned to go.mod).
RUN go run github.com/swaggo/swag/cmd/swag init -g cmd/bureaucat/main.go -o docs --parseDependency=false
RUN go build -ldflags "-X main.Version=${VERSION}" -o bureaucat ./cmd/bureaucat

# Runtime
FROM debian:trixie-slim
RUN set -e \
  && apt update \
  && apt install -y \
    ca-certificates \
  && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=backend /app/bureaucat .
EXPOSE 1341
CMD ["./bureaucat", "serve", "--migrate"]

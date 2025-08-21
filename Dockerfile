# syntax=docker/dockerfile:1.7

# Build stage
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder
WORKDIR /src

# Cache dependencies first
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy sources
COPY . .

# Build static binary for target platform
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/ssh-reverseproxy ./cmd/ssh-reverseproxy

# Runtime stage
FROM scratch
WORKDIR /app

COPY --from=builder /out/ssh-reverseproxy /usr/local/bin/ssh-reverseproxy

# Default SSH port exposed by this proxy (can be overridden by SSH_PORT/SSH_LISTEN_ADDR)
EXPOSE 2222/tcp

# The application loads optional .env from CWD.
# For DB-only mapping, set:
#   -e SSH_DB_DSN=postgres://user:pass@host:5432/db?sslmode=disable \
#   -e SSH_DB_TABLE=proxy_mappings \
#   -e SSH_HOST_KEY_PATH=/config/ssh/hostkey_ed25519 \
#   -e SSH_KNOWN_HOSTS=/config/ssh/known_hosts \
# And mount your config dir: -v $(pwd)/config:/config

ENTRYPOINT ["/usr/local/bin/ssh-reverseproxy"]

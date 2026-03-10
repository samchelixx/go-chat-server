# ─────────────────────────────────────────────────────────────────────────────
# Stage 1: Build
#   Use the official Go image as the build environment.
#   The result is a statically linked Linux binary — no runtime dependencies.
# ─────────────────────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

# Install git so `go mod download` can fetch modules over HTTPS.
RUN apk add --no-cache git

WORKDIR /app

# Copy dependency manifests first. Docker caches this layer separately,
# so subsequent builds are fast when only source files change.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source and build the binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /chat-server ./cmd/server

# ─────────────────────────────────────────────────────────────────────────────
# Stage 2: Runtime
#   Use a minimal Alpine image — final image is ~20 MB.
# ─────────────────────────────────────────────────────────────────────────────
FROM alpine:3.20

# ca-certificates allows TLS connections (e.g. to external APIs or DBs).
RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy only the compiled binary and the frontend assets.
COPY --from=builder /chat-server .
COPY web/ ./web/

# The server listens on this port — must match PORT in your .env.
EXPOSE 8080

# Run as a non-root user for better container security.
RUN adduser -D -H chatuser
USER chatuser

ENTRYPOINT ["./chat-server"]

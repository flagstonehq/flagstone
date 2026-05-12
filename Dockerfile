# =============================================================================
# Flagstone — Multi-stage Docker build
# =============================================================================
# Stage 1: Build the Go binary
# Stage 2: Copy into a minimal scratch image (~5MB final)
#
# Build:  docker build -t flagstone .
# Run:    docker run -p 8080:8080 flagstone
# =============================================================================

# --- Build stage ---
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Cache dependencies first (these change less often than source code)
COPY go.mod go.sum* ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/flagstone ./cmd/flagstone

# --- Runtime stage ---
FROM alpine:3.20

# Needed for HTTPS calls and timezone data
RUN apk add --no-cache ca-certificates tzdata

# Non-root user for security
RUN adduser -D -u 1000 flagstone
USER flagstone

COPY --from=builder /bin/flagstone /usr/local/bin/flagstone
COPY --from=builder /src/migrations /migrations

EXPOSE 8080

ENTRYPOINT ["flagstone"]

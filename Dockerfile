# =============================================================================
# Flagstone — Multi-stage Docker build
# =============================================================================
# Stage 1: Build the Go binary
# Stage 2: Copy into a minimal scratch image (~5MB final)
#
# Build:  docker build -t flagstone .
# Run:    docker run -p 8080:8080 flagstone
# =============================================================================

FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/flagstone ./cmd/flagstone \
 && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/seed      ./cmd/seed

ARG MIGRATE_VERSION=4.18.3
RUN wget -qO- "https://github.com/golang-migrate/migrate/releases/download/v${MIGRATE_VERSION}/migrate.linux-amd64.tar.gz" \
    | tar xz -C /bin/ migrate

FROM alpine:3.24

RUN apk add --no-cache ca-certificates tzdata wget

RUN adduser -D -u 1000 flagstone
USER flagstone

COPY --from=builder /bin/flagstone /usr/local/bin/flagstone
COPY --from=builder /bin/seed      /usr/local/bin/seed
COPY --from=builder /bin/migrate   /usr/local/bin/migrate
COPY --from=builder /src/migrations /migrations

EXPOSE 8080

ENTRYPOINT ["flagstone"]

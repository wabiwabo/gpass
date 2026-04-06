FROM golang:1.22-alpine AS builder

WORKDIR /app

# Cache dependency downloads
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

# Reproducible, statically linked binary with stripped debug symbols
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /bff .

# --- Runtime stage ---
FROM alpine:3.20

# Security: run as non-root user
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g 1000 -S bff \
    && adduser -u 1000 -S bff -G bff

COPY --from=builder /bff /usr/local/bin/bff

USER bff:bff

EXPOSE 4000

HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
    CMD ["/usr/local/bin/bff", "-version"] || exit 1

ENTRYPOINT ["/usr/local/bin/bff"]

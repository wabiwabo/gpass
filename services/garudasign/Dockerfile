FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o garudasign .

FROM alpine:3.21

RUN apk --no-cache add ca-certificates && \
    addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup && \
    mkdir -p /data/signing && \
    chown -R appuser:appgroup /data/signing

COPY --from=builder /build/garudasign /usr/local/bin/garudasign

USER appuser

EXPOSE 4007

ENTRYPOINT ["garudasign"]

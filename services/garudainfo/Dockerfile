FROM golang:1.22-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o garudainfo .

FROM alpine:3.20

RUN apk --no-cache add ca-certificates \
    && adduser -D -u 1000 appuser

WORKDIR /app

COPY --from=builder /build/garudainfo .

USER appuser

EXPOSE 4003

ENTRYPOINT ["./garudainfo"]

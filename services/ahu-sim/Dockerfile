FROM golang:1.22-alpine AS builder

WORKDIR /build

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o ahu-sim .

FROM alpine:3.19

RUN apk --no-cache add ca-certificates \
    && adduser -D -u 1000 appuser

WORKDIR /app

COPY --from=builder /build/ahu-sim .

USER appuser

EXPOSE 4004

ENTRYPOINT ["./ahu-sim"]

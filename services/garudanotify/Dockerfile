FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o garudanotify .

FROM alpine:3.21

RUN apk --no-cache add ca-certificates && \
    addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

COPY --from=builder /build/garudanotify /usr/local/bin/garudanotify

USER appuser

EXPOSE 4011

ENTRYPOINT ["garudanotify"]

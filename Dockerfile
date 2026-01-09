FROM golang:alpine AS builder

WORKDIR /app
COPY . /app

RUN apk add --no-cache gcc libstdc++ musl-dev
RUN go mod download
RUN go build -o bot ./cmd/bot

FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ffmpeg python3 py3-pip && \
    pip3 install --no-cache-dir yt-dlp --break-system-packages

COPY --from=builder /app/bot /app/bot
COPY .env /app/.env

CMD ["/app/bot"]
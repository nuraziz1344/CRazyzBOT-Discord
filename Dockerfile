FROM golang:alpine AS builder

RUN apk add --no-cache gcc libstdc++ musl-dev pkgconfig opus-dev opusfile-dev

WORKDIR /app
COPY . /app

RUN go mod download
RUN go build -o bot ./cmd/bot

FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ffmpeg opus opusfile python3 py3-pip && \
    pip3 install --no-cache-dir yt-dlp --break-system-packages

COPY --from=builder /app/bot /app/bot
COPY start.sh /app/start.sh
RUN chmod +x /app/start.sh

CMD ["/app/start.sh"]
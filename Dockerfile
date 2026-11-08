FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev pkgconfig opus-dev opusfile-dev

WORKDIR /app

# Cache Go modules layer separately from source code changes
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /app/bot ./cmd/bot

FROM alpine:latest

RUN apk add --no-cache ca-certificates ffmpeg opus opusfile python3 py3-pip && \
    addgroup -g 10001 -S botgroup && \
    adduser -u 10001 -S -G botgroup -h /home/botuser botuser

WORKDIR /app
RUN chown botuser:botgroup /app

COPY --chown=botuser:botgroup --from=builder /app/bot /app/bot
COPY --chown=botuser:botgroup entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/bot /app/entrypoint.sh

USER botuser
ENV PATH="/home/botuser/.local/bin:${PATH}"

# Install yt-dlp in user space so non-root entrypoint can self-update it at container start
RUN pip3 install --no-cache-dir --user --break-system-packages yt-dlp

ENTRYPOINT ["/app/entrypoint.sh"]

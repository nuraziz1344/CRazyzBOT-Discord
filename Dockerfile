from golang:alpine

workdir /app
copy . /app

run apk add --no-cache gcc libstdc++ musl-dev ffmpeg
run go get .
run go build -o bot

# from scratch

# workdir /app
# copy go-tube /app/bot
# copy .env /app/.env

cmd ["/app/bot"]
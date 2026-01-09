.PHONY: build run clean test docker-build docker-run

# Build the application
build:
	go build -o bin/bot ./cmd/bot

# Run the application
run:
	go run ./cmd/bot

# Clean build artifacts
clean:
	rm -rf bin/

# Run tests
test:
	go test -v ./...

# Build Docker image
docker-build:
	docker build -t discord-music-bot .

# Run Docker container
docker-run:
	docker run --rm --env-file .env discord-music-bot

# Install dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Run linter
lint:
	go vet ./...

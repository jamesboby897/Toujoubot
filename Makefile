# Makefile for discord-youtube-bot

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=discord-youtube-bot
MAIN_PATH=./main.go

# Build directory
BUILD_DIR=./build

# Make sure build directory exists
$(shell mkdir -p $(BUILD_DIR))

.PHONY: all build clean test run deps tidy help update-deps

all: test build

build: 
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) -v $(MAIN_PATH)

test: 
	$(GOTEST) -v ./...

clean: 
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -rf ./audio/*.dca

run:
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) -v $(MAIN_PATH)
	./$(BUILD_DIR)/$(BINARY_NAME)

deps:
	$(GOGET) -v ./...

tidy:
	$(GOMOD) tidy

update-deps:
	$(GOGET) -u ./...
	$(GOMOD) tidy

# Download yt-dlp if it doesn't exist
download-yt-dlp:
	mkdir -p cmd/yt-dlp
	if [ ! -f cmd/yt-dlp/yt-dlp ]; then \
		echo "Downloading yt-dlp..."; \
		curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o cmd/yt-dlp/yt-dlp; \
		chmod +x cmd/yt-dlp/yt-dlp; \
	fi

# Setup creates necessary directories and downloads dependencies
setup: deps download-yt-dlp
	mkdir -p audio

# Docker targets
docker-build:
	docker build -t $(BINARY_NAME) .

docker-run:
	docker run --rm -it --env-file .env $(BINARY_NAME)

help:
	@echo "Available commands:"
	@echo "  make build         - Build the application"
	@echo "  make build-linux   - Build for Linux"
	@echo "  make test          - Run tests"
	@echo "  make clean         - Clean build files"
	@echo "  make run           - Build and run the application"
	@echo "  make deps          - Get dependencies"
	@echo "  make tidy          - Tidy go.mod"
	@echo "  make update-deps   - Update dependencies"
	@echo "  make setup         - Initial setup (create dirs, download yt-dlp)"
	@echo "  make download-yt-dlp - Download yt-dlp executable"
	@echo "  make docker-build  - Build Docker image"
	@echo "  make docker-run    - Run Docker container"
# Makefile for discord-youtube-bot

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=toujoubot
MAIN_PATH=./main.go

# Build directory
BUILD_DIR=./build

# Make sure build directory exists
$(shell mkdir -p $(BUILD_DIR))

.PHONY: all build clean test run tidy help

all: test build

build: 
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) -v $(MAIN_PATH)

test: 
	$(GOTEST) -v ./...

clean: 
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -rf ./audio/

run:
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) -v $(MAIN_PATH)
	./$(BUILD_DIR)/$(BINARY_NAME)

tidy:
	$(GOMOD) tidy

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
	@echo "  make tidy          - Tidy go.mod"
	@echo "  make download-yt-dlp - Download yt-dlp executable"
	@echo "  make docker-build  - Build Docker image"
	@echo "  make docker-run    - Run Docker container"
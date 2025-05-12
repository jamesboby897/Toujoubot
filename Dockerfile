# Build stage
FROM golang:1.20-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git curl

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o discord-youtube-bot main.go

# Download yt-dlp
RUN mkdir -p cmd/yt-dlp && \
    curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o cmd/yt-dlp/yt-dlp && \
    chmod +x cmd/yt-dlp/yt-dlp

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates python3

# Create audio directory
RUN mkdir -p /app/audio

# Set working directory
WORKDIR /app

# Copy binary and yt-dlp from builder stage
COPY --from=builder /app/discord-youtube-bot .
COPY --from=builder /app/cmd/yt-dlp/yt-dlp ./cmd/yt-dlp/

# Ensure yt-dlp is executable
RUN chmod +x ./cmd/yt-dlp/yt-dlp

# Run the application
CMD ["./discord-youtube-bot"]
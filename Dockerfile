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

# Install runtime dependencies
RUN apk add --no-cache make

# Build the application
RUN make

# Final stage
FROM alpine:latest

# Set working directory
WORKDIR /app

# Copy binary and yt-dlp from builder stage
COPY --from=builder /app/toujoubot .

# Ensure yt-dlp is executable
RUN chmod +x ./cmd/yt-dlp/yt-dlp

# Run the application
CMD ["./toujoubot"]
# Multi-stage build for Worker Service
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git protobuf protobuf-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Generate protobuf code
RUN make proto

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o bin/worker cmd/server/main.go

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S worker && \
    adduser -u 1001 -S worker -G worker

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/bin/worker .

# Change ownership to non-root user
RUN chown -R worker:worker /app

# Switch to non-root user
USER worker

# Expose health check port
EXPOSE 8082

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8082/health || exit 1

# Run the application
CMD ["./worker"]

# Stage 1: Build the application
FROM golang:1.20-alpine AS builder

# Install git (required for Go modules)
RUN apk update && apk add --no-cache git

# Set working directory
WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY ./ ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o gpgenie ./cmd/gpgenie

# Stage 2: Create the final image
FROM alpine:latest

# Install necessary certificates (if using HTTPS with database or APIs)
RUN apk --no-cache add ca-certificates

# Create a non-root user
RUN addgroup -S gpgenie && adduser -S gpgenie -G gpgenie

# Set working directory
WORKDIR /app

# Copy the built binary from the builder
COPY --from=builder /app/gpgenie .

# Copy configuration files (ensure you mount them via volumes or use environment variables)
# Alternatively, you can copy a default config here
# COPY ./config /app/config

# Change ownership to non-root user
RUN chown gpgenie:gpgenie gpgenie

# Switch to non-root user
USER gpgenie

# Expose any necessary ports (if applicable)
# EXPOSE 8080

# Command to run the executable
ENTRYPOINT ["./gpgenie"]

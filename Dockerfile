# Build stage
FROM golang:1.24-alpine AS builder

# Install git and build dependencies
RUN apk add --no-cache git build-base

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./



# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o bin/formpf .

# Runtime stage
FROM alpine:3.18 AS runner

RUN apk add -U tzdata

# Create user and necessary directories
RUN adduser -D -g '' formpf \
 && mkdir -p /var/storage/formpf/


# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/bin /app/bin


# Copy config directory if it exists
COPY --from=builder /app/config ./config
COPY --from=builder /app/static ./static
# Copy entrypoint script
COPY --from=builder /app/entrypoint.sh /app/entrypoint.sh
COPY --from=builder /app/.env ./.env

RUN chmod +x /app/entrypoint.sh

# Expose port
EXPOSE 8080

# Use entrypoint script
ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["./bin/formpf"]
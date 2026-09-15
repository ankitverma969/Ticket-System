# Multi-stage Dockerfile for Ticket System backend

# 1. Builder Stage
FROM golang:alpine AS builder

WORKDIR /app

# Install git and ca-certificates if needed for module downloads
RUN apk add --no-cache git ca-certificates

# Cache dependencies layer
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Compile static Linux binary
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s" -o /app/bin/server ./cmd/server

# 2. Minimal Runtime Stage
FROM alpine:3.21

WORKDIR /app

# Install ca-certificates for outbound TLS (e.g. MongoDB Atlas)
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy compiled binary from builder
COPY --from=builder /app/bin/server /app/server

# Run as non-root user
USER appuser

# Expose default application port
EXPOSE 8080

# Run binary
ENTRYPOINT ["/app/server"]

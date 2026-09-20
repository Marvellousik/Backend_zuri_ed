# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates

# Copy go.mod, go.sum and vendor directory
COPY go.mod go.sum ./
COPY vendor/ ./vendor/

# Copy source code
COPY services/analytics/ ./services/analytics/
COPY shared/ ./shared/

# Build the application using vendor
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -installsuffix cgo -o analytics-service ./services/analytics/cmd/main.go

# Final stage
FROM alpine:3.19

# Install ca-certificates and tzdata
RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/analytics-service .
RUN chown -R appuser:appgroup /app

USER appuser

# Expose port 8083
EXPOSE 8083

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8083/health || exit 1

# Run the service
CMD ["./analytics-service"]

# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates

# Copy go.mod, go.sum and vendor directory
COPY go.mod go.sum ./
COPY vendor/ ./vendor/

# Copy shared packages
COPY shared/ ./shared/

# Copy source code
COPY services/notification-service/ ./services/notification-service/

# Build the application using vendor
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -installsuffix cgo -o notification-service ./services/notification-service/

# Final stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

WORKDIR /app

# Copy the binary
COPY --from=builder /app/notification-service .
RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 8084

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8084/health || exit 1

CMD ["./notification-service"]

# API Gateway

The API Gateway is the fortress wall of Zuri. It stands on port 8080 and handles all incoming traffic.

## Features

- **JWT Validation**: RS256 token validation using public key from User Service
- **Rate Limiting**: Redis-based sliding windows (100 RPM normal, 20 RPM AI endpoints)
- **Header Injection**: Automatically injects `X-User-ID` header for downstream services
- **Circuit Breaker**: Protects against cascading failures (3 failures = 60s cooldown)
- **CORS**: Configurable cross-origin resource sharing
- **Request Proxying**: Routes requests to appropriate microservices
- **Structured Logging**: Correlation ID propagation across all requests

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        API Gateway (8080)                    │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │   Routes    │→ │ Middleware  │→ │   Reverse Proxy     │ │
│  │             │   │ • JWT Auth  │   │ • Circuit Breaker   │ │
│  │             │   │ • Rate Limit│   │ • Header Injection  │ │
│  │             │   │ • CORS      │   │ • Error Handling    │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        ▼                     ▼                     ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│ User Service │    │Content Service│   │   Analytics  │
│   (8081)     │    │   (8082)     │    │   (8083)     │
└──────────────┘    └──────────────┘    └──────────────┘
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8080 | HTTP server port |
| `REDIS_URL` | localhost:6379 | Redis connection |
| `RATE_LIMIT_RPM` | 100 | Rate limit for normal endpoints |
| `AI_RATE_LIMIT_RPM` | 20 | Rate limit for AI endpoints |
| `CIRCUIT_BREAKER_THRESHOLD` | 3 | Failures before opening circuit |
| `CIRCUIT_BREAKER_TIMEOUT` | 60s | Time before half-open state |
| `ALLOWED_ORIGINS` | http://localhost:3000 | CORS allowed origins |
| `USER_SERVICE_URL` | http://localhost:8081 | User Service URL |
| `CONTENT_SERVICE_URL` | http://localhost:8082 | Content Service URL |
| `ANALYTICS_SERVICE_URL` | http://localhost:8083 | Analytics Service URL |
| `AI_ORCHESTRATOR_URL` | http://localhost:5000 | AI Orchestrator URL |

## Routing Table

### Public Routes (No Auth)

| Method | Path | Destination |
|--------|------|-------------|
| GET | `/health` | Gateway health check |
| POST | `/api/v1/auth/register` | User Service |
| POST | `/api/v1/auth/login` | User Service |
| POST | `/api/v1/auth/refresh` | User Service |
| POST | `/api/v1/auth/verify-email` | User Service |
| GET | `/api/v1/auth/public-key` | User Service |

### Protected Routes (JWT Required)

| Method | Path | Destination |
|--------|------|-------------|
| GET | `/api/v1/users/me` | User Service |
| POST | `/api/v1/auth/logout` | User Service |
| GET | `/api/v1/courses` | Content Service |
| POST | `/api/v1/courses` | Content Service |
| GET | `/api/v1/quizzes` | Content Service |
| POST | `/api/v1/quizzes/:id/submit` | Analytics Service |
| POST | `/api/v1/ai/generate/quiz` | AI Orchestrator |
| POST | `/api/v1/ai/chat` | AI Orchestrator |

## Middleware Chain

1. **Recovery** - Panic recovery
2. **CORS** - Cross-origin handling
3. **Correlation ID** - Request tracking
4. **Rate Limiting** - Redis-based limits
5. **Logger** - Structured logging
6. **JWT Auth** - Token validation (protected routes only)

## Circuit Breaker States

```
┌─────────┐    3 failures    ┌─────────┐   60s timeout   ┌───────────┐
│  CLOSED │ ───────────────→ │  OPEN   │ ──────────────→ │ HALF-OPEN │
│(normal) │                  │(reject) │                 │ (testing) │
└─────────┘←──────────────── └─────────┘←─────────────── └───────────┘
   ↑          success                            failure
   └──────────────────────────────────────────────────────────────┘
```

## Running Locally

```bash
# Start dependencies
docker-compose -f infra/docker-compose.yml up -d postgres redis

# Start User Service (required for public key)
docker run -d --name user-service --network host \
  -e DATABASE_URL=postgres://zuri:zuri_secret@localhost:5432/zuri \
  -e REDIS_URL=localhost:6379 \
  -e PRIVATE_KEY_ENCRYPTION_KEY=your-secure-master-key-min-32-chars-long-1234 \
  zuri/user-service

# Start Gateway
go run ./services/gateway/cmd/main.go

# Test
curl http://localhost:8080/health
```

## Testing with Docker Compose

```bash
# Start all services
docker-compose -f infra/docker-compose-full.yml up -d

# Test Gateway
curl http://localhost:8080/health

# Test through Gateway
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}'
```

## Rate Limit Headers

The Gateway adds these headers to responses:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 2026-03-17T12:00:00Z
```

## Error Responses

### 401 Unauthorized
```json
{
  "error": "invalid token"
}
```

### 429 Too Many Requests
```json
{
  "error": "rate limit exceeded"
}
```

### 503 Service Unavailable (Circuit Open)
```json
{
  "error": "service temporarily unavailable"
}
```

## Monitoring

Health check endpoint returns status of all upstream services:

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "service": "gateway",
  "status": "healthy",
  "upstream": {
    "user": "healthy",
    "content": "healthy",
    "analytics": "healthy"
  }
}
```

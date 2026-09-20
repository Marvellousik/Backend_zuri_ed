---
name: zuri-go-services
description: >-
  Use this skill when implementing, refactoring, debugging, or testing Go microservices in Zuri
  (API Gateway, User, Content, Analytics, Notification, and Sync services), or when working with
  shared Go packages under shared/pkg/.
---

# Zuri Go Microservices Development & Maintenance

This skill guides the development, refactoring, testing, and operational debugging of all Go-based backend services in the Zuri platform.

---

## 1. Services Architecture & Ports

| Service | Port | Framework | Responsibility | Key Models & Tables |
| :--- | :--- | :--- | :--- | :--- |
| **Gateway** | `:8080` | Echo v4 | Single entry point, RS256 JWT validation, Redis rate-limiting (100 RPM standard / 20 RPM AI), circuit breaker (3 failures $\rightarrow$ 60s cooldown), reverse proxy | Proxy routes, Circuit breaker state |
| **User** | `:8081` | Echo v4 | User registration, login, profile, password reset, refresh token rotation, session management | `auth.users`, `auth.refresh_tokens`, `auth.jwt_keys`, `auth.user_sessions` |
| **Content** | `:8082` | Echo v4 | Courses, learning materials, quizzes, flashcards, MinIO object storage uploads | `content.courses`, `content.materials`, `content.quizzes`, `content.flashcards` |
| **Analytics** | `:8083` | Echo v4 | Quiz attempts, study streaks, topic mastery, learning goals (`FlexibleDate`), AI usage webhooks | `analytics.quiz_attempts`, `analytics.learning_goals`, `analytics.topic_mastery` |
| **Notification** | `:8084` | Gin v1.9 | SMTP HTML transactional emails, Firebase FCM push notifications, quiet hours | `notification.notifications`, `notification.preferences` |
| **Sync** | `:8085` | Gin v1.9 | WebSocket real-time broadcast, presence tracking, room hub | `sync.messages`, WebSocket Hub |

---

## 2. Shared Packages (`shared/pkg/`)

All Go microservices import shared utilities using the root module prefix `zuri/shared/pkg/...`:

- `zuri/shared/pkg/auth`: RS256 JWT generation/verification, bcrypt hashing (cost factor 12), AES-256-GCM master key encryption for stored private keys.
- `zuri/shared/pkg/config`: Environment variable loading with type conversion, defaults, and validation.
- `zuri/shared/pkg/database`: GORM + PostgreSQL connection pooling, exponential retries, logging.
- `zuri/shared/pkg/redis`: Redis client initialization, sliding-window rate limiters, pub/sub.
- `zuri/shared/pkg/logger`: Structured JSON logging with `go.uber.org/zap`, correlation ID injection (`X-Correlation-ID`).
- `zuri/shared/pkg/middleware`: Request logging, JWT auth middleware, internal API key validation (`X-Internal-Key`).

> [!IMPORTANT]
> Never duplicate database connection, JWT validation, or password hashing logic in individual services. Always reuse `zuri/shared/pkg/`.

---

## 3. Standard Service Directory Layout

Each Echo service (`user`, `content`, `analytics`, `gateway`) follows a clean layered structure:
```text
services/<name>/
├── cmd/
│   └── main.go          # Entry point: Config -> DB/Redis -> Repos -> Services -> Handlers -> Routes
├── internal/
│   ├── handler/         # HTTP handlers receiving Echo context
│   ├── service/         # Business logic and cross-domain orchestration
│   ├── repository/      # Database queries (GORM)
│   ├── model/           # Data structs, JSON tags, and GORM annotations
│   └── middleware/      # Service-specific middlewares
├── pkg/
│   └── config/          # Service-specific environment config struct
└── Dockerfile           # Multi-stage production container build
```

---

## 4. Key Workflows & Procedures

### A. Adding a New Route or Endpoint
1. Define the domain struct in `services/<service>/internal/model/<entity>.go`.
2. Define interface methods and GORM queries in `services/<service>/internal/repository/<entity>_repository.go`.
3. Implement business logic and input validation in `services/<service>/internal/service/<entity>_service.go`.
4. Implement the HTTP handler in `services/<service>/internal/handler/<entity>_handler.go`.
5. Register the route in `services/<service>/cmd/main.go`.
6. If externally accessible, add route forwarding in Gateway (`services/gateway/cmd/main.go` and `services/gateway/internal/proxy/reverse_proxy.go`).

### B. Building All Microservices
Verify all Go services compile cleanly without missing packages:
```bash
make build
```
The compiled binaries will be output to `bin/`. Clean up with `make clean`.

### C. Running Unit Tests
Unit tests use the external test package pattern (e.g. `package service_test`):
```bash
# Run all Go unit tests across all services
make test

# Or run tests with HTML coverage report
make test-coverage
```

### D. Formatting & Linting
```bash
# Format all Go source files
make fmt

# Run golangci-lint
make lint
```

---

## 5. Security & Upstream Conventions

- **Internal Service Auth**: When calling downstream services from Gateway or between services, always pass `X-Internal-Key: <INTERNAL_API_KEY>` and `X-User-ID: <USER_ID>`.
- **Row-Level Security**: Services must verify ownership using the injected `X-User-ID` header rather than trusting request payloads.
- **CORS Handling**: CORS is handled centrally at the API Gateway (`:8080`). Individual backend services must not set duplicate CORS headers.

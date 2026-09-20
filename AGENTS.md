# Zuri — Agent Guide

This document provides the essential context an AI coding agent needs to work effectively in the Zuri codebase.

---

## Project Overview

Zuri is an AI-powered learning platform built as a polyglot microservices system. It provides course management, quizzes, flashcards, AI chat (Gemini), RAG-based document retrieval, text-to-speech, and real-time sync.

**High-level flow:**
```
Client (Next.js) → API Gateway (Go, :8080) → Backend Microservices (Go/Python) → PostgreSQL / Redis / MinIO
```

---

## Technology Stack

| Layer | Technology |
|-------|------------|
| API Gateway & Core Services | Go 1.23 — Echo (v4) and Gin (v1.9) |
| AI Services | Python — FastAPI |
| Frontend | Next.js 16.1.6, React 19.2.3, TypeScript, Tailwind CSS v4 |
| ORM / DB | GORM + PostgreSQL 15 (pgvector image) |
| Cache / PubSub | Redis 7 |
| Object Storage | MinIO |
| AI / LLM | Google Gemini API, LangChain, LangGraph |
| Vector Search | pgvector (PostgreSQL extension) |

**Key dependency files:**
- `go.mod` / `go.sum` — Go module definition (module: `zuri`), Go 1.23
- `Frontend/package.json` — Next.js frontend dependencies
- `zuri-Python Services/requirements.txt` — Shared Python dependencies for AI microservices
- `zuri-ai-main/requirements.txt` — AI Monolith dependencies
- `zuri-Python Services/services/<name>/requirements.txt` — Per-service Python deps (where committed)

---

## Repository Layout

```
zuri/
├── services/                     # Go microservices
│   ├── gateway/                  # API Gateway (Echo) — routing, rate limit, circuit breaker
│   ├── user/                     # User Service (Echo) — auth, profiles, sessions
│   ├── content/                  # Content Service (Echo) — courses, materials, quizzes, flashcards
│   ├── analytics/                # Analytics Service (Echo) — quiz grading, streaks, goals
│   ├── notification-service/     # Notification Service (Gin) — email, push (FCM)
│   └── sync-service/             # Sync Service (Gin) — WebSocket, presence, CDC
├── shared/pkg/                   # Shared Go libraries
│   ├── auth/                     # JWT RS256, bcrypt, AES-256-GCM key encryption
│   ├── config/                   # Env loader with validation and defaults
│   ├── database/                 # PostgreSQL + GORM, connection pooling, retries
│   ├── redis/                    # Redis client, rate limiting, pub/sub
│   ├── logger/                   # Structured JSON logging (Zap), correlation IDs
│   └── middleware/               # Auth middleware, request logging, CORS
├── zuri-Python Services/   # Python AI microservices (FastAPI)
│   └── services/
│       ├── orchestrator/         # AI Orchestrator (:5005) — Gemini chat, quiz/summary/flashcard generation
│       ├── ingestion/            # Ingestion (:5002) — PDF parsing, chunking, embeddings
│       ├── retrieval/            # Retrieval (:5003) — vector search (pgvector), RAG context
│       ├── audio/                # Audio (:5004) — speech-to-text, text-to-speech
│       └── evaluation/           # Evaluation (:5006) — quiz grading, analytics, feedback
├── zuri-ai-main/           # Python AI Monolith (FastAPI, :8000)
│   ├── api.py                    # Main FastAPI entry point
│   ├── reading_assistant/        # Reading engine, TTS, job manager, routes
│   ├── study_buddy/              # Flashcards, quizzes, routes
│   ├── writing_assistant/        # Transcription, note generation, routes
│   └── shared/                   # AI cache utilities
├── Frontend/                     # Next.js frontend (:3000)
│   ├── src/app/                  # App Router routes (auth, main, api)
│   ├── src/components/           # React components (ui, chat, goals, landing, ...)
│   ├── src/services/             # API client wrappers
│   ├── src/store/                # Zustand state management
│   ├── src/lib/                  # Utilities, integrations, sanitization
│   ├── prisma/                   # Prisma schema (minimal: WaitlistEntry only)
│   └── middleware.ts             # Next.js middleware (protected paths, AI proxy headers)
├── docs/                         # Technical documentation hub
│   ├── api/                      # API and service specs
│   ├── integration/              # Frontend & mobile integration guides
│   ├── deployment/               # Database and deployment runbooks
│   └── demo/                     # Demo scripts
├── tests/                        # Tests, smoke scripts & test fixtures
│   ├── integration/              # Python integration tests
│   ├── e2e/                      # PowerShell gateway smoke test scripts
│   ├── fixtures/                 # Test payloads and fixtures
│   └── scripts/                  # Standalone database & service test scripts
├── scripts/                      # Developer utility scripts (check_syntax.py)
├── infra/                        # Infrastructure as Code
│   ├── docker-compose.yml        # Full local stack (mix of build + pre-built images)
│   ├── docker-compose-full.yml   # Full stack using services/user/Dockerfile
│   ├── docker-compose.core.yml   # Core services subset
│   ├── migrations/               # SQL schema migrations (001-009)
│   │   └── supabase/             # Supabase schema variants
│   ├── deploy/                   # Alternate deployment configs (render-supabase.yaml)
│   └── .env.example              # Environment variable template
├── go.mod / go.sum               # Go module definition
├── Makefile                      # Build, test, migrate, lint helpers
├── render.yaml                   # Root deployment blueprint for Render
└── README.md
```

---

## Service Architecture

### Go Services

| Service | Port | Framework | Responsibility |
|---------|------|-----------|----------------|
| Gateway | 8080 | Echo | JWT validation, rate limiting, circuit breaker, reverse proxy to upstreams |
| User | 8081 | Echo | RS256 JWT auth, registration, profile, session management |
| Content | 8082 | Echo | Courses, materials, quizzes, flashcards; async PDF ingestion via Redis |
| Analytics | 8083 | Echo | Quiz attempts, study streaks, topic mastery, learning goals, AI usage |
| Notification | 8084 | Gin | Email (SMTP), push (FCM), reminders, quiet hours |
| Sync | 8085 | Gin | WebSocket real-time sync, presence, CDC |

### Python AI Services

| Service | Port | Framework | Responsibility |
|---------|------|-----------|----------------|
| AI Orchestrator | 5005 | FastAPI | Gemini API proxy, generation endpoints (quiz, summary, flashcards), chat |
| Ingestion | 5002 | FastAPI | PDF parsing, text chunking, embedding generation (all-MiniLM-L6-v2) |
| Retrieval | 5003 | FastAPI | Vector search with pgvector, RAG context assembly |
| Audio | 5004 | FastAPI | Speech-to-text, text-to-speech |
| Evaluation | 5006 | FastAPI | Quiz grading, analytics computation, feedback |
| AI Monolith | 8000 | FastAPI | Reading assistant, study buddy, writing assistant |

### Infrastructure

| Component | Port | Purpose |
|-----------|------|---------|
| PostgreSQL | 5432 | auth, content, analytics, notification, sync, ai schemas |
| Redis | 6379 | Cache, sessions, rate limiting, pub/sub |
| MinIO | 9000/9001 | S3-compatible file storage for materials |

---

## Build, Test, and Run Commands

### Go Backend

```bash
# Install dependencies
go mod download
go mod tidy

# Build (Makefile currently only builds user-service)
make build

# Run tests
make test                    # runs: go test -v ./services/user/internal/...
go test -v ./services/user/internal/...

# Test with coverage
make test-coverage
Giv
# Format code
go fmt ./...
make fmt

# Run linter (requires golangci-lint)
make lint

# Run a service locally (requires local postgres + redis)
go run ./services/user/cmd/main.go
go run ./services/gateway/cmd/main.go
# etc.
```

### Frontend

```bash
cd Frontend

# Install dependencies
npm install

# Development server
npm run dev

# Build
npm run build

# Lint
npm run lint

# Database (Prisma)
npm run db:generate
npm run db:push
npm run db:studio
npm run db:migrate
```

### Full Stack (Docker Compose)

```bash
# Start everything (infra/docker-compose.yml defines 14 containers)
cd infra
docker-compose up -d

# Check status
docker-compose ps
curl http://localhost:8080/health

# View logs
docker-compose logs gateway --tail=50
docker-compose logs ai-orchestrator --tail=50

# Restart a service
docker-compose restart gateway

# Rebuild after code changes
docker-compose build notification-service && docker-compose up -d notification-service

# Stop everything
docker-compose down
```

### Database Migrations

```bash
# Requires golang-migrate installed
make migrate-up
make migrate-down
make migrate-create   # interactive prompt for name
```

Migration files live in `infra/migrations/`:
- `001_create_auth_schema.sql`
- `001_create_auth_schema_supabase.sql`
- `002_create_content_schema.sql`
- `003_create_analytics_schema.sql`
- `004_create_notification_schema.sql`
- `005_create_sync_schema.sql`
- `006_create_ai_schema.sql`
- `007_add_role_column.sql`

### Python Syntax Check

```bash
python scripts/check_syntax.py
# or via Makefile
make check-syntax
```

Validates Python AST for core AI service files under `zuri-Python Services/`.

---

## Code Organization Conventions

### Go Services

Each Go service (except `notification-service` and `sync-service`) follows a layered hexagonal-like structure:

```
services/<name>/
├── cmd/main.go          # Entry point: config → db/redis → repos → services → handlers → routes
├── internal/
│   ├── handler/         # HTTP handlers (Echo/Gin context)
│   ├── service/         # Business logic
│   ├── repository/      # Data access (GORM/sqlx)
│   ├── model/           # Domain structs
│   └── middleware/      # Service-specific middleware
└── pkg/config/          # Service-specific env configuration
```

**`notification-service`** and **`sync-service`** use a flatter layout:

```
services/<name>/
├── main.go
├── handlers/
├── models/
├── services/   (notification only)
└── websocket/  (sync only)
```

### Go Code Style

- Package comments are present (e.g., `// Package main is the entry point for the API Gateway.`).
- Structured logging with `go.uber.org/zap` is mandatory. Use `logger.Info/Error/Fatal` with `zap.Field`.
- Correlation IDs are propagated via context (`X-Correlation-ID` header).
- Graceful shutdown pattern is standard: capture `SIGINT`/`SIGTERM`, use `echo.Shutdown` with a 30s timeout (or `srv.Shutdown` for Gin).
- Services using Echo register `echomiddleware.Recover()`; Gin services use `gin.Recovery()`.
- CORS is configured explicitly per service; in practice, CORS is handled by the Gateway to avoid duplicate headers.
- **Important**: `notification-service` and `sync-service` use `godotenv.Load()` directly and `sqlx.Connect("postgres", ...)` rather than the shared `database` package.

### Frontend

- **App Router** with route groups: `(auth)` for unauthenticated pages, `(main)` for authenticated dashboard pages.
- **State**: Zustand for client state; React Query (TanStack) for server state.
- **Styling**: Tailwind CSS v4 + `class-variance-authority` + `tailwind-merge` for component variants.
- **UI Components**: Radix UI primitives wrapped in `src/components/ui/`.
- **API**: Next.js API routes in `src/app/api/` act as proxies or BFF endpoints; external calls go to the Go gateway at `NEXT_PUBLIC_API_GATEWAY_URL` (default `http://localhost:8080`).
- **Security headers** (CSP, X-Frame-Options, etc.) are defined in `next.config.ts`.
- **Middleware** (`Frontend/middleware.ts`) marks protected paths and injects `x-zuri-internal-caller` for `/api/ai/*` routes. Actual JWT validation happens client-side because tokens are stored in `localStorage` via Zustand persist.
- **Prisma**: The schema (`Frontend/prisma/schema.prisma`) is minimal and currently only defines `WaitlistEntry`. Most data operations go through the Go gateway, not a local Prisma client.

### Python AI Services

- Each service is a standalone FastAPI app with a `main.py` entry point (AI Monolith uses `api.py`).
- `requirements.txt` is per-service where committed. The root `zuri-Python Services/requirements.txt` contains shared dependencies.
- The AI monolith (`zuri-ai-main/`) is a single FastAPI app (`api.py`) with sub-routers for reading, study, and writing assistants. It uses an `AIWorker` background worker and SQLAlchemy for DB access.
- The orchestrator includes a built-in model router with cost tracking for Gemini models (`gemini-2.5-flash-lite`, `gemini-2.5-flash`, `gemini-2.5-pro`).

---

## Testing Strategy

### Current State

- **Go**: Minimal test coverage. Only `services/user/internal/service/user_service_test.go` exists. The `Makefile` only runs tests in the user service path.
- **Frontend**: A small set of integration/unit tests exist in `src/lib/__tests__/`, `src/services/__tests__/`, and page-level `__tests__/` directories (chat, dashboard, settings, text-to-speech).
- **E2E / Smoke**: `tests/e2e/test-gateway.ps1` (and `test-gateway-enhanced.ps1`) provides a PowerShell smoke test that registers a user, checks the public key endpoint, attempts login, verifies 401 on protected routes, and inspects rate-limit headers.

### How to Add Tests

- **Go**: Add `*_test.go` files next to the code under test. Run with `go test -v ./services/<name>/internal/...`. Use `stretchr/testify` (already in `go.mod`). Tests in this codebase use the **external test package** pattern (`package service_test`), not `package service`.
- **Frontend**: Place `.test.ts` or `.test.tsx` files in `__tests__/` folders within the feature directory or in `src/lib/__tests__/`. Uses Jest-style `describe`/`it`/`expect`.

---

## Security Considerations

- **Authentication**: RS256 JWT (RSA-2048). The User Service generates and signs tokens. The Gateway loads the public key from User Service at startup to validate tokens. Private keys are AES-256-GCM encrypted at rest.
- **Passwords**: bcrypt with cost 12.
- **Rate Limiting**: Redis-backed sliding window. Default RPM is 100; AI endpoints are limited to 20 RPM.
- **Circuit Breaker**: Gateway uses a circuit breaker for Python AI service calls (3 failures = 60s cooldown).
- **Internal Communication**: `X-User-ID` and `X-Internal-Key` headers are injected for service-to-service calls. `X-User-ID` is used for row-level access control.
- **Token Lifecycle**: Refresh token rotation with reuse detection. Tokens are blacklisted on logout. Access tokens expire in 15 minutes; refresh tokens expire in 30 days.
- **Frontend**: CSP headers are strict. In development `unsafe-eval` is allowed for Next.js/HMR; production CSP removes it. `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, and `Permissions-Policy` are enforced.
- **Secrets**: Never commit `.env` files. The root `.env` and `infra/.env` are gitignored. Use `infra/.env.example` as a template.

---

## Environment Variables

Key variables (see `infra/.env.example` and docker-compose for full list):

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes | — | PostgreSQL connection string |
| `REDIS_URL` | Yes | `localhost:6379` | Redis address |
| `PRIVATE_KEY_ENCRYPTION_KEY` | Yes | — | AES master key for JWT private key (32+ chars) |
| `GEMINI_API_KEY` | Yes (AI) | — | Google Gemini API key |
| `INTERNAL_API_KEY` | No | `dev-internal-key` | Key for service-to-service auth |
| `RATE_LIMIT_RPM` | No | `100` | Normal rate limit |
| `AI_RATE_LIMIT_RPM` | No | `20` | AI endpoint rate limit |
| `ACCESS_TOKEN_TTL` | No | `15m` | JWT access token lifetime |
| `REFRESH_TOKEN_TTL` | No | `720h` | Refresh token lifetime (30 days) |
| `BCRYPT_COST` | No | `12` | bcrypt cost factor |
| `LOG_LEVEL` | No | `info` | Logging level (debug, info, warn, error) |
| `AI_SERVICE_TIMEOUT` | No | `120s` | Timeout for AI Monolith calls |
| `SMTP_HOST` / `SMTP_PORT` / `SMTP_USERNAME` / `SMTP_PASSWORD` | No | — | Email notification config |
| `FIREBASE_SERVICE_ACCOUNT_PATH` | No | — | FCM push notification config |
| `MINIO_ENDPOINT` / `MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY` / `MINIO_BUCKET` | Yes (content) | — | Object storage config |

---

## Deployment

The primary deployment target is **Docker Compose**.

- `infra/docker-compose.yml` defines the full production-like local stack. Note that some Python AI services (orchestrator, retrieval, evaluation, ingestion) reference pre-built Docker images rather than local build contexts.
- `infra/docker-compose-full.yml` uses the root `Dockerfile.user-service` for the user service.
- `infra/docker-compose.core.yml` provides a subset of core services.
- Each Go service has its own `Dockerfile` inside `services/<name>/`.
- Each Python service has its own `Dockerfile` inside `zuri-Python Services/services/<name>/`.
- The AI monolith has `zuri-ai-main/Dockerfile`.
- Health checks are configured on every container.
- Services restart `unless-stopped`.

There is no Kubernetes or Terraform configuration in this repository.

---

## Important Notes for Agents

1. **Do not assume all Python services have `requirements.txt` committed.** Some (e.g., orchestrator, audio) only have a `Dockerfile` and `main.py`. If you add dependencies, create or update the `requirements.txt` and the `Dockerfile`.
2. **Gateway is the single entry point.** All public traffic goes through `:8080`. Never expose individual backend service ports externally in production.
3. **Frontend rewrites** in `next.config.ts` proxy `/api/v1/*` and `/health` to the gateway. Next.js API routes in `src/app/api/` handle unmatched paths first.
4. **Database schemas are fixed** across SQL migration files in `infra/migrations/`. If you change models, you must update the corresponding migration or create a new one.
5. **Shared packages** in `shared/pkg/` are imported as `zuri/shared/pkg/<name>`. Do not duplicate auth, config, or database logic inside individual services.
6. **Go vendor directory exists.** The project vendors Go dependencies. Running `go mod tidy` or `go mod download` will update the module cache; vendor can be refreshed with `go mod vendor` if needed.
7. **Go version is 1.23** (per `go.mod`). The root `Dockerfile.test` uses Go 1.21-alpine, which may need updating if newer language features are used.
8. **AI Monolith entry point is `api.py`**, not `main.py`. It registers sub-routers from `reading_assistant/`, `study_buddy/`, and `writing_assistant/`.
9. **Prisma schema is minimal.** The frontend Prisma client only manages `WaitlistEntry`. Do not assume Prisma models exist for users, courses, or other domain entities — those are managed by the Go backend.
10. **CORS is handled at the Gateway.** Individual backend services explicitly avoid setting CORS to prevent duplicate headers.

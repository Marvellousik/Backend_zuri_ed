# Zuri - Complete Service Documentation

> **Version:** 1.0 | **Last Updated:** 2026-04-21 | **Total Services:** 12 backend + 1 frontend + 3 infra

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Go Microservices](#1-api-gateway)
3. [Python AI Services](#7-ai-orchestrator)
4. [Frontend](#13-frontend-nextjs)
5. [Infrastructure](#infrastructure)
6. [Request Flow](#request-flow)
7. [Environment Variables](#environment-variables-reference)
8. [Database Schema](#database-schema-summary)

---

## Architecture Overview

```
Client (Next.js) -> Gateway :8080 -> Go Services -> PostgreSQL/Redis/MinIO
                          -> Python AI Services (Gemini, RAG, Audio)
```

| Layer | Services | Tech |
|-------|----------|------|
| Gateway | Single entry point | Go, Echo |
| Core Go | User, Content, Analytics | Go, Echo |
| Utility Go | Notification, Sync | Go, Gin |
| Python AI | Orchestrator, Ingestion, Retrieval, Audio, Evaluation, Monolith | Python, FastAPI |
| Data | PostgreSQL 15 (pgvector), Redis 7, MinIO/S3 | Docker |

---

## 1. API Gateway

| Property | Value |
|----------|-------|
| **Port** | 8080 |
| **Framework** | Echo v4 |
| **Entry** | `services/gateway/cmd/main.go` |
| **Image** | `zuri/gateway:latest` |

**Responsibility:** Single public entry point. All backend ports should be internal-only in production.

**Key Features:**
- JWT RS256 validation (public key loaded from User Service at startup)
- Redis-backed sliding-window rate limiting (100 RPM default, 20 RPM for AI)
- Daily AI quota: `X-Quota-Limit` / `X-Quota-Remaining` headers
- Circuit breaker: 3 failures = 60s cooldown for AI calls
- Injects `X-Internal-Key` + `X-User-ID` for service-to-service auth

**Public Routes (no auth):** `/health`, `/api/v1/auth/*`, `/api/v1/auth/public-key`

**Protected Routes (JWT required):** All `/api/v1/*` except auth paths. Proxies to:
- User Service (`/users/*`, `/auth/logout`)
- Content Service (`/courses`, `/materials`, `/quizzes`, `/flashcard-decks`)
- Analytics Service (`/analytics/*`, `/quiz-attempts/*`)
- AI Orchestrator (`/ai/generate/*`, `/ai/chat`)
- AI Monolith (`/writing/*`, `/reading/*`, `/study/*`)
- Retrieval (`/ai/retrieve`), Audio (`/ai/speech-to-text`, `/ai/text-to-speech`, `/ai/languages`)
- Notification (`/notifications/*`), Sync (`/sync/*`, `/presence/*`, `/ws`)

**Env vars:** `PORT`, `REDIS_URL`, `RATE_LIMIT_RPM`, `AI_RATE_LIMIT_RPM`, `AI_DAILY_QUOTA`, `CIRCUIT_BREAKER_THRESHOLD`, `INTERNAL_API_KEY`, all `*_SERVICE_URL` upstreams.

---

## 2. User Service

| Property | Value |
|----------|-------|
| **Port** | 8081 |
| **Framework** | Echo v4 |
| **Entry** | `services/user/cmd/main.go` |
| **Image** | `zuri/user-service:latest` |

**Responsibility:** Authentication, profiles, sessions, JWT key lifecycle.

**Features:**
- bcrypt cost 12 password hashing
- RS256 JWT signing (RSA-2048, private key AES-256-GCM encrypted at rest)
- Access token: 15 min | Refresh token: 30 days with rotation + reuse detection
- Email verification: 6-digit code stored in PostgreSQL + Redis
- Multi-device session tracking with revoke capability

**Public endpoints:** `/api/v1/auth/register`, `/login`, `/refresh`, `/verify-email`, `/resend-verification`, `/forgot-password`, `/reset-password`, `/public-key`

**Protected endpoints:** `/api/v1/auth/logout`, `/logout-all`, `/users/me`, `/users/me/change-password`, `/users/me/sessions`

**Env vars:** `DATABASE_URL`, `REDIS_URL`, `PRIVATE_KEY_ENCRYPTION_KEY` (32+ chars), `BCRYPT_COST`, `ACCESS_TOKEN_TTL`, `REFRESH_TOKEN_TTL`, `BYPASS_EMAIL_VERIFICATION` (false in prod).

---

## 3. Content Service

| Property | Value |
|----------|-------|
| **Port** | 8082 |
| **Framework** | Echo v4 |
| **Entry** | `services/content/cmd/main.go` |
| **Image** | `zuri/content-service:latest` |

**Responsibility:** Courses, materials, quizzes, flashcard decks. Orchestrates file uploads via presigned MinIO URLs.

**Endpoints:** `/api/v1/courses`, `/materials`, `/quizzes`, `/flashcard-decks`, plus nested question/card routes.

**Env vars:** `DATABASE_URL`, `REDIS_URL`, `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_BUCKET`, `INGESTION_SERVICE_URL`, `INTERNAL_API_KEY`.

---

## 4. Analytics Service

| Property | Value |
|----------|-------|
| **Port** | 8083 |
| **Framework** | Echo v4 |
| **Entry** | `services/analytics/cmd/main.go` |
| **Image** | `zuri/analytics-service:latest` |

**Responsibility:** Quiz attempts, study streaks, topic mastery, learning goals, AI usage tracking.

**Endpoints:** `/api/v1/quiz-attempts/*`, `/api/v1/analytics/*` (study-streak, study-stats, topic-mastery, goals, ai-usage, ai-interactions).

**Env vars:** `DATABASE_URL`, `REDIS_URL`, `INTERNAL_API_KEY`.

---

## 5. Notification Service

| Property | Value |
|----------|-------|
| **Port** | 8084 |
| **Framework** | Gin v1.9 |
| **Entry** | `services/notification-service/main.go` |
| **Image** | `zuri/notification-service:latest` |

**Responsibility:** Email (SMTP), push notifications (FCM), preferences, reminders, quiet hours.

**Architecture:** Background worker polls `notification.queue` every 10 seconds.

**Endpoints:** `/api/v1/notifications/preferences`, `/devices/register`, `/reminders`, `/history`.

**Env vars:** `DATABASE_URL`, `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM`, `FIREBASE_SERVICE_ACCOUNT_PATH`.

---

## 6. Sync Service

| Property | Value |
|----------|-------|
| **Port** | 8085 |
| **Framework** | Gin v1.9 |
| **Entry** | `services/sync-service/main.go` |
| **Image** | `zuri/sync-service:latest` |

**Responsibility:** WebSocket real-time sync, presence tracking, CDC events.

**Endpoints:** `/api/v1/sync/*`, `/api/v1/presence/*`, `/api/v1/ws` (WebSocket upgrade).

**Env vars:** `DATABASE_URL`.

---

## 7. AI Orchestrator

| Property | Value |
|----------|-------|
| **Port** | 5005 |
| **Framework** | FastAPI |
| **Entry** | `zuri-Python Services/services/orchestrator/main.py` |

**Responsibility:** Primary AI proxy. Manages Gemini API calls, conversation history, dynamic model routing, cost tracking.

**Dynamic Model Routing:**
- `flash-lite`: < 1,500 chars, simple chat/summary
- `flash`: default balanced model
- `pro`: > 12,000 chars or complex generation tasks

**Pricing (per 1K tokens):** flash-lite $0.00010/$0.00040, flash $0.00030/$0.00250, pro $0.00125/$0.01000.

**Endpoints:** `/health`, `/api/v1/ai/generate/quiz`, `/generate/summary`, `/generate/flashcards`, `/ai/chat`, `/ai/conversation/:id`.

**Env vars:** `GEMINI_API_KEY`, `INTERNAL_API_KEY`, `DEFAULT_MODEL`, `ALLOWED_ORIGINS`.

---

## 8. Ingestion Service

| Property | Value |
|----------|-------|
| **Port** | 5002 |
| **Framework** | FastAPI |
| **Entry** | `zuri-Python Services/services/ingestion/main.py` |

**Responsibility:** Document processing pipeline: PDF parsing -> text extraction -> chunking (500 chars, 50 overlap) -> embeddings (all-MiniLM-L6-v2, 384-dim) -> storage.

**Endpoints:** `/health`, `/api/v1/ingest`.

**Key modules:** `parser.py`, `chunker.py`, `embedder.py`, `models.py`.

**Env vars:** `INTERNAL_API_KEY`.

---

## 9. Retrieval Service

| Property | Value |
|----------|-------|
| **Port** | 5003 |
| **Framework** | FastAPI |
| **Entry** | `zuri-Python Services/services/retrieval/main.py` |

**Responsibility:** Semantic search and vector retrieval for RAG. Serves relevant chunks to the Orchestrator.

**Search modes:** `pgvector` (production) or `json_fallback` (dev).

**Endpoints:** `/health`, `/api/v1/ai/retrieve`.

**Env vars:** `DATABASE_URL`, `INTERNAL_API_KEY`.

---

## 10. Audio Service

| Property | Value |
|----------|-------|
| **Port** | 5004 |
| **Framework** | FastAPI |
| **Entry** | `zuri-Python Services/services/audio/main.py` |

**Responsibility:** Speech-to-text and text-to-speech.

**Features:**
- STT: `speech_recognition` + `pydub` + ffmpeg (supports MP3, WAV, M4A, OGG, FLAC, AAC, up to 50 MB)
- TTS: `gTTS` (Google Text-to-Speech, free, no API key)

**Endpoints:** `/health`, `/api/v1/ai/speech-to-text`, `/api/v1/ai/text-to-speech`, `/api/v1/ai/languages`.

**System deps:** `ffmpeg` (installed via Dockerfile).

**Env vars:** `INTERNAL_API_KEY`.

---

## 11. Evaluation Service

| Property | Value |
|----------|-------|
| **Port** | 5006 |
| **Framework** | FastAPI |
| **Entry** | `zuri-Python Services/services/evaluation/main.py` |

**Responsibility:** Quiz grading, analytics computation, AI interaction logging, user feedback.

**Endpoints:** `/health`, `/api/v1/evaluation/grade`, `/answer-key`, `/log-interaction`, `/feedback`, `/stats`.

**Env vars:** `DATABASE_URL`, `INTERNAL_API_KEY`.

---

## 12. AI Monolith

| Property | Value |
|----------|-------|
| **Port** | 8000 |
| **Framework** | FastAPI |
| **Entry** | `zuri-ai-main/api.py` |
| **Image** | `zuri/ai-service:latest` |

**Responsibility:** Combined Reading Assistant, Study Buddy, and Writing Assistant. Runs background `AIWorker` for async job processing.

**Subsystems:**
- `reading_assistant/` - Document analysis, summary, vocab extraction, TTS
- `study_buddy/` - Flashcard & quiz generation
- `writing_assistant/` - Transcription, note generation

**Background Worker:** `AIWorker` processes Redis-backed job queue with retry logic (max 3) and dead-letter queue.

**Endpoints:**
- Reading: `POST /reading/analyse`, `GET /reading/{id}`
- Study: `POST /study/flashcards`, `POST /study/quiz`, `GET /study/history`
- Writing: `POST /writing/transcribe`, `POST /writing/notes`, `GET /writing/history`
- Jobs: `GET /jobs/{job_id}`

**Env vars:** `DATABASE_URL`, `REDIS_URL`, `GOOGLE_API_KEY`, `GEMINI_API_KEY`, `GROQ_API_KEY`, `WEAVIATE_URL`, `WEAVIATE_API_KEY`, `COHERE_API_KEY`, `INTERNAL_API_KEY`, `ALLOWED_ORIGINS`.

---

## 13. Frontend (Next.js)

| Property | Value |
|----------|-------|
| **Framework** | Next.js 16.1.6, React 19.2.3, TypeScript |
| **Styling** | Tailwind CSS v4 |
| **State** | Zustand (client), React Query (server) |
| **Port** | 3000 |
| **Entry** | `Frontend/src/app/` (App Router) |

**Structure:**
- `src/app/(auth)/` - Login, register
- `src/app/(main)/` - Dashboard
- `src/app/api/` - BFF/proxy routes
- `src/components/ui/` - 50 Radix UI primitives
- `src/services/` - API client wrappers (`api.ts`, `http.ts`, `websocket.ts`)
- `src/store/authStore.ts` - Zustand auth state (localStorage persisted)

**Middleware:** Marks protected paths, injects `x-zuri-internal-caller` for `/api/ai/*`. JWT validation is client-side.

**Key deps:** `next`, `react`, `tailwindcss`, `zustand`, `@tanstack/react-query`, `prisma` (minimal: `WaitlistEntry` only).

---

## Infrastructure

### PostgreSQL 15 (pgvector)

| Property | Value |
|----------|-------|
| **Image** | `postgres:15-alpine` with pgvector |
| **Port** | 5432 |
| **Container** | `zuri-postgres` |

**Schemas & migrations:**
| Schema | Purpose | Migration |
|--------|---------|-----------|
| `auth` | Users, JWT keys, tokens, sessions | `001_create_auth_schema.sql` |
| `content` | Courses, materials, quizzes, flashcards | `002_create_content_schema.sql` |
| `analytics` | Quiz attempts, study sessions, goals | `003_create_analytics_schema.sql` |
| `notification` | Preferences, queue, history | `004_create_notification_schema.sql` |
| `sync` | Connections, events, presence | `005_create_sync_schema.sql` |
| `ai` | AI monolith user sessions | `006_create_ai_schema.sql` |

### Redis 7

| Property | Value |
|----------|-------|
| **Image** | `redis:7-alpine` |
| **Port** | 6379 |
| **Container** | `zuri-redis` |

**Usage patterns:**
| Pattern | Key Prefix | TTL |
|---------|-----------|-----|
| Rate limiting | `ratelimit:user:{id}` | 1 min |
| Daily AI quota | `quota:user:{id}:{date}` | 48h |
| Email verification | `email_verification:{user_id}` | 24h |
| Token blacklist | `token:blacklist:{jti}` | Token expiry |
| AI cache | `ai:cache:{sha256}` | 1h |
| Job queue | `ai:queue:pending` | - |
| Dead letter | `ai:queue:dead_letter` | - |

### MinIO / S3

| Property | Value |
|----------|-------|
| **Local** | `minio/minio` on ports 9000/9001 |
| **Production** | AWS S3 or Cloudflare R2 |
| **Bucket** | `zuri-materials` |

Content Service generates presigned URLs for direct client upload/download.

---

## Request Flow

### Registration -> Login -> AI Chat

```
POST /api/v1/auth/register
  -> Gateway -> User Service
  -> Hash password, generate verification code
  -> Store in PostgreSQL (auth.users)

POST /api/v1/auth/login
  -> Gateway -> User Service
  -> Validate password, generate RS256 JWT pair
  -> Store refresh token in PostgreSQL

POST /api/v1/ai/chat
  -> Gateway: validate JWT (RS256)
  -> Gateway: rate limit check (Redis sliding window)
  -> Gateway: increment daily quota, set X-Quota-* headers
  -> Gateway -> AI Orchestrator (circuit breaker protected)
  -> Orchestrator: validate X-Internal-Key
  -> Orchestrator: route to appropriate Gemini model
  -> Stream response back through Gateway
```

### Upload PDF -> Generate Flashcards

```
POST /api/v1/materials
  -> Content Service creates record, returns presigned MinIO URL

Client uploads to MinIO

Content triggers Ingestion Service
  -> Extract text, chunk, embed -> PostgreSQL (pgvector)

POST /api/v1/study/flashcards
  -> Gateway -> AI Monolith
  -> Retrieve relevant chunks via Retrieval Service
  -> Generate flashcards via Gemini
```

---

## Environment Variables Reference

| Variable | Required By | Description |
|----------|-------------|-------------|
| `PORT` | All services | HTTP server port |
| `ENVIRONMENT` | Go services | `development` or `production` |
| `LOG_LEVEL` | All services | `debug`, `info`, `warn`, `error` |
| `DATABASE_URL` | All Go + most Python | PostgreSQL connection string |
| `REDIS_URL` | Gateway, User, Content, AI Monolith | Redis address |
| `PRIVATE_KEY_ENCRYPTION_KEY` | User Service | AES-256-GCM master key (32+ chars) |
| `INTERNAL_API_KEY` | Gateway, all Python services | Service-to-service auth key |
| `BCRYPT_COST` | User Service | Password hash cost (default: 12) |
| `ACCESS_TOKEN_TTL` | User Service | JWT access token lifetime (default: 15m) |
| `REFRESH_TOKEN_TTL` | User Service | Refresh token lifetime (default: 720h) |
| `BYPASS_EMAIL_VERIFICATION` | User Service | `true` = auto-verify (dev only) |
| `SMTP_HOST` / `SMTP_PORT` / `SMTP_USERNAME` / `SMTP_PASSWORD` / `SMTP_FROM` | Notification Service | Email SMTP config |
| `FIREBASE_SERVICE_ACCOUNT_PATH` | Notification Service | Firebase service account JSON path |
| `MINIO_ENDPOINT` / `MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY` / `MINIO_BUCKET` | Content Service | Object storage config |
| `GEMINI_API_KEY` | AI Orchestrator, AI Monolith | Google Gemini API key |
| `GOOGLE_API_KEY` | AI Monolith | Legacy Google API key |
| `GROQ_API_KEY` | AI Monolith | Groq API key for Whisper STT |
| `WEAVIATE_URL` / `WEAVIATE_API_KEY` | AI Monolith | Weaviate Cloud config |
| `COHERE_API_KEY` | AI Monolith | Cohere embeddings API key |
| `RATE_LIMIT_RPM` | Gateway | Default rate limit (default: 100) |
| `AI_RATE_LIMIT_RPM` | Gateway | AI endpoint rate limit (default: 20) |
| `AI_DAILY_QUOTA` | Gateway | Daily AI requests per user (default: 50) |
| `AI_SERVICE_TIMEOUT` | Gateway | AI monolith call timeout (default: 120s) |
| `ALLOWED_ORIGINS` | Gateway, Python services | Comma-separated CORS origins |
| `NEXT_PUBLIC_API_GATEWAY_URL` | Frontend | Gateway URL for API calls |

---

## Database Schema Summary

### Auth Schema

| Table | Purpose |
|-------|---------|
| `auth.users` | Accounts, password hashes, email verification status |
| `auth.jwt_keys` | RSA key pairs (encrypted private key) |
| `auth.refresh_tokens` | Refresh token storage with rotation tracking |
| `auth.sessions` | Active user sessions per device |
| `auth.password_resets` | Password reset token storage |

### Content Schema

| Table | Purpose |
|-------|---------|
| `content.courses` | Learning courses |
| `content.materials` | Uploaded files metadata |
| `content.quizzes` | Quiz definitions |
| `content.quiz_questions` | Individual questions |
| `content.flashcard_decks` | Flashcard collections |
| `content.flashcards` | Individual flashcards |

### Analytics Schema

| Table | Purpose |
|-------|---------|
| `analytics.quiz_attempts` | User quiz attempt records |
| `analytics.quiz_answers` | Per-question answer submissions |
| `analytics.study_sessions` | Time-tracked study periods |
| `analytics.topic_mastery` | Spaced repetition mastery scores |
| `analytics.learning_goals` | User-defined objectives |
| `analytics.ai_interactions` | AI usage and cost tracking |

### Notification Schema

| Table | Purpose |
|-------|---------|
| `notification.preferences` | Per-user notification settings |
| `notification.queue` | Pending notification jobs |
| `notification.history` | Sent notification log |
| `notification.scheduled_reminders` | Future reminder definitions |

### Sync Schema

| Table | Purpose |
|-------|---------|
| `sync.connections` | WebSocket connection records |
| `sync.events` | CRUD events for offline sync |
| `sync.device_state` | Per-device sync checkpoints |
| `sync.presence` | Online/offline status |

### AI Schema

| Table | Purpose |
|-------|---------|
| `ai.user_sessions` | AI monolith session tracking |

---

## Service Dependency Graph

```
                    +-------------+
                    |   Client    |
                    |  (Next.js)  |
                    +------+------+
                           |
                    +------v------+
                    |   Gateway   |
                    |   :8080     |
                    +------+------+
                           |
        +---------+--------+--------+---------+
        |         |        |        |         |
   +----v---+ +---v---+ +--v---+ +--v----+ +--v------+
   |  User  | |Content| |Analytics| |Notify| |  Sync  |
   | :8081  | | :8082 | | :8083  | |:8084 | | :8085  |
   +---+----+ +---+---+ +--+---+ +--+----+ +---+-----+
       |          |        |        |         |
       +----------+--------+--------+---------+
                           |
                    +------v------+
                    | PostgreSQL  |
                    |   + Redis   |
                    +------+------+
                           |
        +---------+--------+--------+---------+
        |         |        |        |         |
   +----v---+ +---v---+ +--v---+ +--v----+ +--v------+
   |Orchestr| |Ingest | |Retrieval| |Audio | |Evaluati|
   | :5005  | | :5002 | | :5003  | |:5004 | | :5006  |
   +--------+ +-------+ +--------+ +------+ +--------+
        |
   +----v----+
   | AI Mono |
   | :8000   |
   +---------+
```

---

## Deployment Notes

### Local Development

```bash
cd infra
docker-compose up -d
```

Starts all 14 containers. Check health at `http://localhost:8080/health`.

### Production Hardening

- Remove backend port mappings (only Gateway :80/:443 public)
- Change `INTERNAL_API_KEY` to random 32+ char string
- Set `BYPASS_EMAIL_VERIFICATION=false`
- Configure exact `ALLOWED_ORIGINS` (no wildcard `*`)
- Enable PostgreSQL SSL (`sslmode=require`)
- Use managed secrets (AWS/GCP Secret Manager)
- Set `LOG_LEVEL=warn` or `error`

### Scaling Considerations

| Service | Stateless? | Horizontally Scalable? | Bottleneck |
|---------|-----------|----------------------|------------|
| Gateway | Yes | Yes | Redis connection pool |
| User Service | Yes | Yes | DB write capacity |
| Content Service | Yes | Yes | MinIO throughput |
| Analytics Service | Yes | Yes | DB write capacity |
| Notification Service | No (worker) | Partial | Worker singleton |
| Sync Service | No (WebSocket) | Partial | WebSocket hub state |
| AI Orchestrator | Yes | Yes | Gemini API rate limits |
| AI Monolith | No (worker) | Partial | Worker singleton |
| Ingestion | Yes | Yes | Embedding model load |
| Retrieval | Yes | Yes | pgvector query perf |
| Audio | Yes | Yes | File I/O, ffmpeg CPU |
| Evaluation | Yes | Yes | DB write capacity |

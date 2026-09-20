# Zuri — AI-Powered Learning Platform

A microservices-based learning platform with 6 Go services, 5 Python AI services, PostgreSQL, Redis, and MinIO.

---

## Architecture

```
Client → API Gateway (8080) → Microservices → PostgreSQL / Redis / MinIO
```

### Go Services

| Service | Port | Description |
|---------|------|-------------|
| API Gateway | 8080 | JWT validation, rate limiting, circuit breaker, routing |
| User Service | 8081 | Auth (RS256 JWT), registration, profile, sessions |
| Content Service | 8082 | Courses, materials, quizzes, flashcards |
| Analytics Service | 8083 | Quiz grading, study streaks, topic mastery, AI usage |
| Notification Service | 8084 | Push (FCM), email (SMTP), reminders, quiet hours |
| Sync Service | 8085 | WebSocket real-time sync, presence, CDC |

### Python AI Services

| Service | Port | Description |
|---------|------|-------------|
| AI Orchestrator | 5005 | Gemini API, quiz/summary/flashcard generation, chat |
| Ingestion | 5002 | PDF parsing, text chunking, embedding generation |
| Retrieval | 5003 | Vector search (pgvector), RAG context assembly |
| Audio | 5004 | Speech-to-text, text-to-speech |
| Evaluation | 5006 | Quiz grading, analytics, feedback |

### Infrastructure

| Component | Port | Purpose |
|-----------|------|---------|
| PostgreSQL | 5432 | auth, content, analytics, notification, sync schemas |
| Redis | 6379 | Cache, sessions, rate limiting, pub/sub |
| MinIO | 9000/9001 | S3-compatible file storage |

### Request Flow

```
Client → Gateway (JWT + rate limit) → Service → PostgreSQL/Redis
                                    → Python AI (circuit breaker protected)
```

### Database Schemas

- `auth` — users, jwt_keys, refresh_tokens, sessions, password_resets
- `content` — courses, materials, quizzes, quiz_questions, flashcard_decks, flashcards
- `analytics` — quiz_attempts, quiz_answers, study_sessions, topic_mastery, learning_goals, ai_interactions
- `notification` — preferences, queue, history, scheduled_reminders
- `sync` — connections, events, device_state, change_log, presence

---

## Prerequisites

- Docker Desktop (running)
- curl (comes with Windows 10+)
- Git

---

## Quick Start

```batch
cd infra
docker-compose up -d
```

Wait ~60 seconds for all services to start, then verify:

```batch
docker-compose ps
curl http://localhost:8080/health
```

Expected: all 14 containers running, gateway status `"healthy"` with all upstream services healthy.

---

## Demo (CMD)

### Step 1: Health Check

```batch
curl http://localhost:8080/health
```

Expected:
```json
{"service":"gateway","status":"healthy","upstream":{"analytics":"healthy","content":"healthy","notification":"healthy","sync":"healthy","user":"healthy"}}
```

### Step 2: Register a User

```batch
curl -X POST http://localhost:8080/api/v1/auth/register -H "Content-Type: application/json" -d "{\"email\":\"demo2@example.com\",\"password\":\"DemoPass123!\",\"first_name\":\"Demo\",\"last_name\":\"User\"}"
```

### Step 3: Login

```batch
curl -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d "{\"email\":\"demo2@example.com\",\"password\":\"DemoPass123!\"}"
```

Copy the `access_token` value from the response and save it:

```batch
set TOKEN=eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiMWUyNTM0YTAtZTlkMi00ZGUxLWFjOWItYzE1NzQ5NWRmY2FlIiwiZW1haWwiOiJkZW1vMkBleGFtcGxlLmNvbSIsInRva2VuX3R5cGUiOiJhY2Nlc3MiLCJzdWIiOiIxZTI1MzRhMC1lOWQyLTRkZTEtYWM5Yi1jMTU3NDk1ZGZjYWUiLCJleHAiOjE3NzQ1MjUzOTMsIm5iZiI6MTc3NDUyNDQ5MywiaWF0IjoxNzc0NTI0NDkzLCJqdGkiOiI1NTBiZTVjMS1mOWVhLTQ3MmItOGU4NC1hMGQwYzUwYzQ5OTkifQ.jMoDalB96zl9IYW7zJOrn1PTjhwGczsmZ8SOU_HKrSFyHxul_hR7QJQLctxuwCyieKxrfiLW9axcVD3SKBzWRQnokAkEaLYWNKKHdg8FjNyERaqfNIGbawPBUvbEr9CpWz2fxZrgE8mjBix0X7gbvYGa9ftXElwGTEq88oX5wY_Beiek9ONK6w9Kx_uRiRsmAkGjU8Tm4xsCqHTT45aHsfeYo4osZdXidtVTzCjY3sCsTQ0WfcGeCQcJSUlXA3KwXZFcBL55FOfnII3K2H16zz_cPffvex6K-_i9yIymTPlFIQw0iuyo5Zbfx_-AsPf46UnmV0DrACnsrg
```

### Step 4: AI Chat (Gemini)

```batch
curl -X POST http://localhost:8080/api/v1/ai/chat -H "Content-Type: application/json" -H "Authorization: Bearer %TOKEN%" -d "{\"query\":\"What is machine learning?\",\"user_id\":\"test\",\"context_chunks\":[]}"
```

### Step 5: Document Retrieval (RAG)

```batch
curl -X POST http://localhost:8080/api/v1/ai/retrieve -H "Content-Type: application/json" -H "Authorization: Bearer %TOKEN%" -d "{\"query\":\"neural networks\",\"user_id\":\"test\",\"top_k\":3}"
```

### Step 6: Audio Languages

```batch
curl http://localhost:8080/api/v1/ai/languages -H "Authorization: Bearer %TOKEN%"
```

### Step 7: Create a Course

```batch
curl -X POST http://localhost:8080/api/v1/courses -H "Content-Type: application/json" -H "Authorization: Bearer %TOKEN%" -d "{\"name\":\"Machine Learning 101\",\"description\":\"Intro to ML\",\"semester\":\"Fall\",\"year\":2026}"
```

### Step 8: Upload Material

```batch
curl -X POST http://localhost:8080/api/v1/materials -H "Content-Type: application/json" -H "Authorization: Bearer %TOKEN%" -d "{\"title\":\"My Notes\",\"description\":\"Study notes\",\"content_type\":\"pdf\",\"file_size\":10000}"
```

### Step 9: Get User Profile

```batch
curl http://localhost:8080/api/v1/users/me -H "Authorization: Bearer %TOKEN%"
```

### Step 10: Check Study Stats

```batch
 ```

---

## Demo (PowerShell)

```powershell
# Login and save token
$body = @{email="e2etest@example.com"; password="TestPass123!"} | ConvertTo-Json
$login = Invoke-RestMethod -Uri http://localhost:8080/api/v1/auth/login -Method POST -Body $body -ContentType "application/json"
$token = $login.data.access_token
$headers = @{Authorization = "Bearer $token"; "Content-Type" = "application/json"}

# AI Chat
$chat = Invoke-RestMethod -Uri http://localhost:8080/api/v1/ai/chat -Method POST -Body (@{query="What is AI?"; user_id=$login.data.user.id; context_chunks=@()} | ConvertTo-Json) -Headers $headers
Write-Host "AI: $($chat.response)"

# Retrieval
$ret = Invoke-RestMethod -Uri http://localhost:8080/api/v1/ai/retrieve -Method POST -Body (@{query="machine learning"; user_id=$login.data.user.id; top_k=3} | ConvertTo-Json) -Headers $headers
Write-Host "Found $($ret.results.Count) chunks"

# Languages
$langs = Invoke-RestMethod -Uri http://localhost:8080/api/v1/ai/languages -Headers $headers
Write-Host "Languages: $($langs.supported_languages.PSObject.Properties.Name -join ', ')"
```

---

## Automated Test Scripts

```batch
:: Quick test (CMD)
test-cmd.bat

:: One-liner test (CMD)
test-oneliner.bat

:: PowerShell comprehensive test
.\e2e-comprehensive-test.ps1
```

---

## API Reference

### Public Endpoints (No Auth)

| Method | Path | Service |
|--------|------|---------|
| GET | `/health` | Gateway health + upstream status |
| POST | `/api/v1/auth/register` | Register user |
| POST | `/api/v1/auth/login` | Login, returns JWT |
| POST | `/api/v1/auth/refresh` | Refresh access token |
| POST | `/api/v1/auth/verify-email` | Verify email with code |
| POST | `/api/v1/auth/forgot-password` | Request password reset |
| POST | `/api/v1/auth/reset-password` | Reset password |
| GET | `/api/v1/auth/public-key` | Get JWT public key |

### Protected Endpoints (Bearer Token Required)

**User**
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/users/me` | Get profile |
| PUT | `/api/v1/users/me` | Update profile |
| POST | `/api/v1/users/me/change-password` | Change password |
| GET | `/api/v1/users/me/sessions` | List sessions |
| DELETE | `/api/v1/users/me/sessions/:id` | Revoke session |
| POST | `/api/v1/auth/logout` | Logout |

**Content**
| Method | Path | Description |
|--------|------|-------------|
| GET/POST | `/api/v1/courses` | List/create courses |
| GET/PUT/DELETE | `/api/v1/courses/:id` | Course CRUD |
| GET/POST | `/api/v1/materials` | List/create materials |
| GET | `/api/v1/materials/:id` | Get material |
| GET/POST | `/api/v1/quizzes` | List/create quizzes |
| GET/POST | `/api/v1/flashcard-decks` | List/create flashcard decks |

**Analytics**
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/quizzes/:id/start` | Start quiz attempt |
| POST | `/api/v1/quiz-attempts/:id/answers` | Submit answers |
| POST | `/api/v1/quiz-attempts/:id/complete` | Complete attempt |
| GET | `/api/v1/analytics/study-streak` | Get study streak |
| GET | `/api/v1/analytics/study-stats` | Get study statistics |
| GET | `/api/v1/analytics/topic-mastery` | Get topic mastery |
| GET/POST | `/api/v1/analytics/goals` | Learning goals |
| GET | `/api/v1/analytics/ai-usage` | AI token usage |

**AI**
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/ai/chat` | Chat with Gemini AI |
| POST | `/api/v1/ai/generate/quiz` | Generate quiz from text |
| POST | `/api/v1/ai/generate/summary` | Generate summary |
| POST | `/api/v1/ai/generate/flashcards` | Generate flashcards |
| POST | `/api/v1/ai/retrieve` | Vector search (RAG) |
| POST | `/api/v1/ai/speech-to-text` | Transcribe audio |
| GET | `/api/v1/ai/languages` | Supported languages |

**Notifications**
| Method | Path | Description |
|--------|------|-------------|
| GET/PUT | `/api/v1/notifications/preferences` | Notification preferences |
| POST | `/api/v1/notifications/devices/register` | Register push device |
| GET/POST | `/api/v1/notifications/reminders` | Study reminders |
| GET | `/api/v1/notifications/history` | Notification history |

**Sync**
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/sync/state` | Device sync state |
| POST | `/api/v1/sync/ack` | Acknowledge sync |
| GET/POST | `/api/v1/sync/events` | Sync events |
| GET/PUT | `/api/v1/presence` | User presence |
| GET | `/api/v1/ws` | WebSocket connection |

---

## Shared Go Packages

Located in `shared/pkg/`:

| Package | Purpose |
|---------|---------|
| `auth` | JWT RS256, bcrypt, AES-256-GCM key encryption, RSA utilities |
| `config` | Environment variable loading with validation and defaults |
| `database` | PostgreSQL + GORM, connection pooling, retry, transactions |
| `redis` | Redis client, rate limiting, pub/sub, sorted sets, streams |
| `logger` | Structured JSON logging (Zap), correlation ID propagation |
| `middleware` | Auth middleware, request logging, CORS |

---

## Go-Python Integration

**Synchronous (HTTP):** Gateway → AI Orchestrator, Retrieval, Audio
- 30s timeout, circuit breaker (3 failures = 60s cooldown)
- `X-User-ID` and `X-Internal-Key` headers injected

**Asynchronous (Redis):** Content → Ingestion (PDF processing)
- Content Service publishes to Redis queue
- Ingestion Service processes and calls webhook when done

**Data Models:** All request/response models aligned between Go structs and Python Pydantic models (see `services/gateway/internal/clients/models.go`).

---

## Environment Variables

Key variables (set in `infra/.env` or docker-compose):

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | (required) | PostgreSQL connection string |
| `REDIS_URL` | localhost:6379 | Redis address |
| `PRIVATE_KEY_ENCRYPTION_KEY` | (required) | AES master key for JWT private key (32+ chars) |
| `GEMINI_API_KEY` | (required for AI) | Google Gemini API key |
| `RATE_LIMIT_RPM` | 100 | Normal rate limit |
| `AI_RATE_LIMIT_RPM` | 20 | AI endpoint rate limit |
| `ACCESS_TOKEN_TTL` | 15m | JWT access token lifetime |
| `REFRESH_TOKEN_TTL` | 720h | Refresh token lifetime (30 days) |

---

## Security

- JWT RS256 (RSA-2048) — private key AES-256-GCM encrypted at rest
- bcrypt password hashing (cost 12)
- Redis sliding window rate limiting
- Circuit breaker for AI services
- Refresh token rotation with reuse detection
- Token blacklisting on logout
- `X-User-ID` header injection (row-level access control)

---

## Project Structure

```
Backend_lexi/
├── services/                     # Go microservices
│   ├── gateway/                  # API Gateway (Echo)
│   ├── user/                     # User Service (Echo)
│   ├── content/                  # Content Service (Echo)
│   ├── analytics/                # Analytics Service (Echo)
│   ├── notification-service/     # Notification Service (Gin)
│   └── sync-service/             # Sync Service (Gin)
├── shared/pkg/                   # Shared Go packages (auth, database, redis, logger, middleware)
├── zuri-Python Services/   # Python AI microservices (FastAPI)
│   └── services/
│       ├── orchestrator/         # AI Orchestrator (FastAPI)
│       ├── ingestion/            # Ingestion Service (FastAPI)
│       ├── retrieval/            # Retrieval Service (FastAPI)
│       ├── audio/                # Audio Service (FastAPI)
│       └── evaluation/           # Evaluation Service (FastAPI)
├── zuri-ai-main/           # Python AI Monolith (FastAPI)
│   ├── reading_assistant/        # Reading engine & TTS
│   ├── study_buddy/              # Flashcards & quizzes
│   └── writing_assistant/        # Notes & transcription
├── docs/                         # Technical documentation & integration guides
│   ├── api/                      # API & service architecture specs
│   ├── integration/              # Frontend, mobile & staging integration guides
│   ├── deployment/               # Database & Render deployment runbooks
│   └── demo/                     # Product demo scripts
├── tests/                        # Automated tests, smoke scripts & test fixtures
│   ├── integration/              # Python & cross-service integration tests
│   ├── e2e/                      # Gateway smoke test scripts
│   ├── fixtures/                 # Test payloads and mock files
│   └── scripts/                  # Standalone verification scripts
├── scripts/                      # Developer utility scripts (syntax verification)
├── infra/                        # Infrastructure & deployment configuration
│   ├── migrations/               # SQL schema migrations (001-009)
│   │   └── supabase/             # Supabase schema variants
│   ├── deploy/                   # Deployment blueprints (render-supabase.yaml)
│   ├── docker-compose.yml        # Full local stack
│   └── .env
├── vendor/                       # Go vendor dependencies
├── go.mod / go.sum
├── Makefile
├── render.yaml
└── README.md
```

Detailed documentation index: [docs/README.md](file:///workspaces/Backend_lexi/docs/README.md)  
Test suite details: [tests/README.md](file:///workspaces/Backend_lexi/tests/README.md)

---

## Useful Commands

```bash
# Start all services with Docker Compose
cd infra && docker-compose up -d

# Stop all services
cd infra && docker-compose down

# View logs
docker-compose logs gateway --tail=50
docker-compose logs ai-orchestrator --tail=50

# Restart a service
docker-compose restart gateway

# Build all Go binaries
make build

# Run all Go unit tests
make test

# Run Go tests and Python syntax verification
make test-all

# Clean build artifacts
make clean

# Check container status
docker-compose ps
```

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| "Connection refused" | Run `docker-compose up -d` and wait 60s |
| "401 Unauthorized" | Token expired — login again |
| "rate limit exceeded" | Wait 60s or check Redis: `docker exec zuri-redis redis-cli ping` |
| "503 Service Unavailable" | Circuit breaker open — wait 60s or `docker-compose restart gateway` |
| Gateway shows "degraded" | Check which upstream is unhealthy in the health response |
| Docker build fails | Ensure Docker Desktop is running, try `docker-compose build --no-cache` |
| Database connection failed | `docker logs zuri-postgres` to check |
| Port already in use | `netstat -ano \| findstr :8080` to find the process |

# Zuri — AI Monolith Integration Plan

**Goal:** Connect the AI team's FastAPI monolith (`zuri-ai-main/`) to the backend team's Go microservices infrastructure so everything runs together via `docker-compose up -d`.

**Date:** 2026-03-31
**Status:** ✅ All tasks complete

---

## Task Tracker

| # | Priority | Task | Files | Status |
|---|----------|------|-------|--------|
| 1 | P0 | Fix requirements.txt encoding (UTF-16 → UTF-8) | `zuri-ai-main/requirements.txt` | ✅ DONE |
| 2 | P0 | Create Dockerfile for AI monolith | `zuri-ai-main/Dockerfile` | ✅ DONE |
| 3 | P0 | Add ai-service to docker-compose + env vars | `infra/docker-compose.yml`, `infra/.env` | ✅ DONE |
| 4 | P1 | Create AI schema migration | `infra/migrations/006_create_ai_schema.sql` | ✅ DONE |
| 5 | P1 | Update database.py to use `ai` schema | `zuri-ai-main/database.py` | ✅ DONE |
| 6 | P1 | Add `AI_SERVICE_URL` to Gateway env in compose | `infra/docker-compose.yml` | ✅ DONE |
| 7 | P2 | Make Weaviate connection lazy in reading_engine.py | `zuri-ai-main/reading_assistant/reading_engine.py` | ✅ DONE |
| 8 | P2 | Fix `_chunk_text` call bug in reading_engine.py | `zuri-ai-main/reading_assistant/reading_engine.py` | ✅ DONE |
| 9 | P2 | Make Weaviate connection lazy in zuricore.py | `zuri-ai-main/zuricore.py` | ✅ DONE |
| 10 | P1 | Fix notification service route prefix + user_id type | `services/notification-service/handlers/handlers.go`, `models/models.go` | ✅ DONE |
| 11 | P1 | Fix sync service user_id type + device_id handling | `services/sync-service/handlers/handlers.go` | ✅ DONE |
| 12 | P0 | Add /health endpoint to AI monolith | `zuri-ai-main/api.py` | ✅ DONE |
| 13 | P0 | Create .gitignore, .env.example, .dockerignore | Root + infra + AI monolith | ✅ DONE |
| 14 | P0 | Create frontend integration guide | `FRONTEND_INTEGRATION_GUIDE.md` | ✅ DONE |

---

## Task Details

### Task 1 — Fix requirements.txt encoding

**Problem:** The file is UTF-16 encoded with null bytes. `pip install -r requirements.txt` will fail inside Docker.

**Fix:** Re-create the file as plain UTF-8 with the same package list.

**Affected file:** `zuri-ai-main/requirements.txt`

---

### Task 2 — Create Dockerfile for AI monolith

**Problem:** The AI monolith has no Dockerfile and cannot be deployed as a container.

**Fix:** Create a Dockerfile that:
- Uses `python:3.11-slim` base image
- Installs system dependencies (ffmpeg for audio, build tools for native packages)
- Installs Python requirements
- Copies application code
- Runs `uvicorn api:app --host 0.0.0.0 --port 8000`

**Affected file:** `zuri-ai-main/Dockerfile`

---

### Task 3 — Add ai-service to docker-compose

**Problem:** The AI monolith is not in `infra/docker-compose.yml` and has no env var configuration.

**Fix:**
- Add `ai-service` container definition with build context `../zuri-ai-main`
- Set all required env vars: `DATABASE_URL`, `GOOGLE_API_KEY`, `GROQ_API_KEY`, `GEMINI_API_KEY`, `WEAVIATE_URL`, `WEAVIATE_API_KEY`, `COHERE_API_KEY`
- Add health check, network, restart policy
- Depends on `postgres` (healthy)
- Add placeholder env vars to `infra/.env`

**Affected files:** `infra/docker-compose.yml`, `infra/.env`

---

### Task 4 — Create AI schema migration

**Problem:** The AI monolith's `user_sessions` table auto-creates in the `public` schema. The backend already has `auth.user_sessions` — name collision risk.

**Fix:** Create `infra/migrations/006_create_ai_schema.sql` that:
- Creates the `ai` schema
- Creates the `user_sessions` table explicitly in `ai` schema
- Matches the SQLAlchemy model columns exactly

**Affected file:** `infra/migrations/006_create_ai_schema.sql`

---

### Task 5 — Update database.py to use `ai` schema

**Problem:** The `UserSession` model has `__tablename__ = "user_sessions"` with no schema, so it creates in `public`.

**Fix:**
- Add `__table_args__ = {"schema": "ai"}` to the `UserSession` model
- Update `DATABASE_URL` default to match the shared PostgreSQL connection string used by other services
- Ensure `Base.metadata.create_all()` creates in the `ai` schema

**Affected file:** `zuri-ai-main/database.py`

---

### Task 6 — Add AI_SERVICE_URL to Gateway env in compose

**Problem:** The Gateway config reads `AI_SERVICE_URL` (default `http://localhost:8000`), but docker-compose doesn't set it. Inside Docker, `localhost` won't resolve to the AI container.

**Fix:** Add `AI_SERVICE_URL=http://ai-service:8000` to the gateway service's environment block in docker-compose.

**Affected file:** `infra/docker-compose.yml` (done together with Task 3)

---

### Task 7 — Make Weaviate connection lazy in reading_engine.py

**Problem:** `reading_engine.py` connects to Weaviate Cloud at **module import time** (top-level code). If env vars are missing or Weaviate is unreachable, the entire AI service crashes on startup.

**Fix:**
- Move the Weaviate client initialization into a lazy singleton function
- The connection is established on first request, not on import
- Add error handling so the service starts even if Weaviate is temporarily unavailable
- The `ReaadingEngine` class methods access the client via the lazy getter

**Affected file:** `zuri-ai-main/reading_assistant/reading_engine.py`

---

### Task 8 — Fix _chunk_text call bug in reading_engine.py

**Problem:** `ReaadingEngine.store_document()` calls `ZuriEngine._chunk_text(self, text=...)` passing its own `self` (a `ReaadingEngine` instance) as the first arg to a `ZuriEngine` method. This works by accident because `_chunk_text` doesn't actually use `self`, but it's a bug.

**Fix:** Make `_chunk_text` a `@staticmethod` on `ZuriEngine` and call it as `ZuriEngine._chunk_text(text=...)` without passing `self`.

**Affected files:**
- `zuri-ai-main/zuricore.py` (make `_chunk_text` a staticmethod)
- `zuri-ai-main/reading_assistant/reading_engine.py` (fix the call site)

---

### Task 9 — Make Weaviate connection lazy in zuricore.py

**Problem:** `zuricore.py` has global `wclient`, `collection`, `model` variables that are only initialized when `_init_clients()` is called from `run_cli()`. The web server path (`api.py`) imports `ZuriEngine` from `zuricore.py` — the globals remain `None`. If any web endpoint tries to use `contextual_chat` or `ingest_material`, it will crash with `NoneType` errors.

**Fix:**
- Make `_init_clients()` callable from the web context (not just CLI)
- Add a lazy initialization pattern so the Weaviate client connects on first use
- Add graceful error handling if Weaviate env vars are not configured

**Affected file:** `zuri-ai-main/zuricore.py`

---

## Architecture Context

### What's Already Working

- Gateway routes `/api/v1/writing/*`, `/api/v1/reading/*`, `/api/v1/study/*` → AI monolith via `AIClient`
- `AIClient` (`services/gateway/internal/clients/ai_client.go`) matches the AI team's API contract
- `AIHandler` (`services/gateway/internal/handlers/ai_handler.go`) extracts `user_id` from JWT and passes it correctly
- All 6 Go services + 3 infra containers (PostgreSQL, Redis, MinIO) work in docker-compose
- 5 backend Python services (orchestrator, retrieval, audio, ingestion, evaluation) work separately

### What's NOT Changing

- Go services code (gateway, user, content, analytics, notification, sync)
- Backend Python services (orchestrator, retrieval, audio, ingestion, evaluation)
- Shared Go packages (`shared/pkg/`)
- Existing migrations (001–005)
- Gateway routing logic and AIClient/AIHandler code

### Decision: Backend Python Services vs AI Monolith

Both teams built overlapping features. Current plan: **keep both running**.

| Path | Routes To | Purpose |
|------|-----------|---------|
| `/api/v1/writing/*` | AI monolith (:8000) | Live transcription + notes (Groq Whisper, LangChain) |
| `/api/v1/reading/*` | AI monolith (:8000) | Document analysis + TTS (Weaviate RAG, Gemini TTS) |
| `/api/v1/study/*` | AI monolith (:8000) | Flashcards + quizzes (LangGraph pipelines) |
| `/api/v1/ai/chat` | Orchestrator (:5005) | Simple Gemini chat |
| `/api/v1/ai/generate/*` | Orchestrator (:5005) | Basic quiz/summary/flashcard generation |
| `/api/v1/ai/retrieve` | Retrieval (:5003) | pgvector semantic search |
| `/api/v1/ai/speech-to-text` | Audio (:5004) | SpeechRecognition STT |
| `/api/v1/ai/languages` | Audio (:5004) | Supported language list |

The AI monolith features are significantly more sophisticated. The backend Python services can be deprecated later once the monolith is proven stable.

---

## Environment Variables Required by AI Monolith

| Variable | Purpose | Source |
|----------|---------|--------|
| `DATABASE_URL` | PostgreSQL connection | Shared with other services |
| `GOOGLE_API_KEY` | LangChain Google GenAI (flashcards, quizzes, reading) | AI team |
| `GEMINI_API_KEY` | Gemini direct API (zuricore, TTS) | AI team |
| `GROQ_API_KEY` | Groq Whisper STT (writing assistant) | AI team |
| `WEAVIATE_URL` | Weaviate Cloud cluster URL | AI team |
| `WEAVIATE_API_KEY` | Weaviate Cloud auth | AI team |
| `COHERE_API_KEY` | Cohere embeddings for Weaviate | AI team |

---
name: zuri-infra-ops
description: >-
  Use this skill when managing local or cloud infrastructure for Zuri, including Docker Compose
  multi-service stacks, PostgreSQL schema migrations with pgvector, Redis caching, MinIO storage,
  and Render deployment configurations.
---

# Zuri Infrastructure, Migrations & Deployment Operations

This skill guides local orchestration, database migrations, storage management, and deployment pipelines across the Zuri platform.

---

## 1. Core Infrastructure Components & Ports

| Component | Image | Default Port | Volumes & Storage | Purpose |
| :--- | :--- | :--- | :--- | :--- |
| **PostgreSQL** | `pgvector/pgvector:pg15` | `:5432` | `postgres_data:/var/lib/postgresql/data` | Primary database hosting schemas: `auth`, `content`, `analytics`, `notification`, `sync`, `ai` |
| **Redis** | `redis:7-alpine` | `:6379` | `redis_data:/data` | Session cache, sliding-window rate limiting, Celery/job queues, and pub/sub |
| **MinIO** | `minio/minio:latest` | `:9000` (API), `:9001` (Console) | `minio_data:/data` | S3-compatible object storage for course materials and PDFs |
| **Gateway** | `zuri/gateway:latest` | `:8080` | None | Edge reverse proxy, CORS, JWT verification, rate limiting |

---

## 2. Docker Compose Stacks (`infra/`)

Zuri provides tailored Docker Compose configurations for different stages:

- **`infra/docker-compose.yml`**: Full production-like local stack running all 14 containers (Postgres, Redis, MinIO, 6 Go microservices, 5 Python AI services, AI Monolith).
- **`infra/docker-compose.core.yml`**: Lightweight setup containing only core storage dependencies (`postgres`, `redis`, `minio`) for local service development.
- **`infra/docker-compose-full.yml`**: Stack using local build contexts for all Go services.
- **`infra/docker-compose.staging.yml`**: Staging override configuration with production logging and health check intervals.

### Common Docker Commands
```bash
# Start full local stack in background
cd infra && docker-compose up -d

# Start only core dependencies (for running individual Go services locally)
cd infra && docker-compose -f docker-compose.core.yml up -d

# View live service logs
docker-compose logs gateway --tail=50 -f
docker-compose logs ai-orchestrator --tail=50 -f

# Check health check status across all containers
docker-compose ps

# Rebuild a single service after code changes
docker-compose build notification-service && docker-compose up -d notification-service

# Stop and teardown infrastructure
cd infra && docker-compose down
```

---

## 3. Database Migrations Runbook

Migration files are located in `infra/migrations/`.

### A. Canonical Migration Sequence
The canonical schema files execute sequentially during database initialization (`docker-entrypoint-initdb.d`):
1. `001_create_auth_schema.sql` — Users, sessions, refresh tokens, JWT keys.
2. `002_create_content_schema.sql` — Courses, materials, flashcards, quizzes.
3. `003_create_analytics_schema.sql` — Quiz attempts, topic mastery, learning goals.
4. `004_create_notification_schema.sql` — In-app notifications, preferences, quiet hours.
5. `005_create_sync_schema.sql` — WebSocket message logs, room states.
6. `006_create_ai_schema.sql` — AI interaction logging, generation jobs.
7. `007_add_role_column.sql` — User role column (`student`, `instructor`, `admin`).
8. `008_create_document_chunks.sql` — `ai.zuri_chunks` table with 1024-dim pgvector indexing.
9. `009_update_learning_goals.sql` — Auto-progress tracking (`current_value`, `goal_type`).

### B. Supabase Namespace Variants (`infra/migrations/supabase/`)
When deploying to Supabase, custom schema names are used (e.g. `zuri_auth`, `zuri_content`) to prevent conflicts with Supabase's internal schemas.
- All Supabase-specific SQL files are isolated in `infra/migrations/supabase/`.
- Never place `*_zuri.sql` or `*_supabase.sql` files directly in `infra/migrations/`, as Docker Compose would execute both sets on startup.

### C. Migration Commands via Makefile
```bash
# Apply pending migrations using golang-migrate
make migrate-up

# Roll back the most recent migration
make migrate-down

# Generate a new migration file pair
make migrate-create
```

---

## 4. Render Cloud Deployment

Render deployment is configured through Infrastructure-as-Code blueprints:

- **`render.yaml` (Root)**: The standard Render Blueprint defining all containerized web services, background workers, and managed Redis.
- **`infra/deploy/render-supabase.yaml`**: The alternate blueprint configured to connect to an external Supabase PostgreSQL database instead of Render Postgres.

> [!IMPORTANT]
> Render Blueprints require `render.yaml` to remain in the repository root. All service `dockerfilePath` and `dockerContext` directives are relative to the repository root.

---

## 5. Health Check & Troubleshooting Cheatsheet

| Issue | Verification Command | Resolution |
| :--- | :--- | :--- |
| **Gateway unhealthy** | `curl -i http://localhost:8080/health` | Check which upstream service is down in the JSON response; restart the upstream container. |
| **Postgres connection failure** | `docker exec zuri-postgres pg_isready -U zuri` | Ensure container is healthy; inspect logs with `docker logs zuri-postgres`. |
| **Redis connection timeout** | `docker exec zuri-redis redis-cli ping` | Ensure port `6379` is open and container is not restarting. |
| **MinIO bucket not found** | `curl http://localhost:9000/minio/health/live` | Ensure `MINIO_BUCKET=zuri-materials` exists; verify MinIO credentials in `.env`. |
| **Vector search fails** | `docker exec -it zuri-postgres psql -U zuri -c "\dx"` | Ensure extension `vector` is installed: `CREATE EXTENSION IF NOT EXISTS vector;`. |

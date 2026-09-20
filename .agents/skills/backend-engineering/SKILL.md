---
name: backend-engineering
description: Use for any backend work in this repo — Go microservices, Python AI services, APIs, databases, Docker, CI/CD, deployment, observability. Covers the full backend surface area so nothing is skipped just because it wasn't explicitly asked for. Trigger for any task touching services, endpoints, data layer, infra config, or pipelines.
---

# Backend Engineering (Go + Python microservices, Docker, CI/CD)

This repo's stack: Go for core microservices, Python for AI services, Docker for local dev and
deployment, CI/CD for build/test/deploy. Apply the relevant section(s) below even when the task
description doesn't explicitly mention them — a "add an endpoint" task still implies auth,
validation, logging, and tests, whether or not that was spelled out.

## API design

- REST by default unless the service already uses gRPC internally — match what's there.
- Version APIs from day one (`/v1/...`) even if there's only one version so far.
- Consistent error response shape across all services (status code + machine-readable error code
  + human message). Don't invent a new error format per endpoint.
- Idempotency keys for any POST that creates a resource and could plausibly be retried.
- Request/response validation at the handler boundary, not scattered through business logic.

## Service boundaries & communication

- Keep each microservice's responsibility narrow; don't let one service reach into another's
  database directly — go through its API.
- For inter-service calls: set timeouts, add retries with backoff for transient failures, and
  add circuit-breaking or at least a sane failure mode if a downstream service is down (don't
  let one slow dependency cascade into a full outage).
- Use structured events/queues (not synchronous chains) for anything that doesn't need an
  immediate response — e.g., transcription, embedding generation, notification sends.

## Data layer

- Every migration is written as a real migration file (up/down), never a manual schema edit.
- Index columns used in WHERE/JOIN/ORDER BY on tables expected to grow.
- Explicit connection pool limits per service — don't rely on driver defaults.
- Soft-delete vs hard-delete decided deliberately per table, not by accident.
- For the AI services specifically: batch embedding calls, cap prompt sizes deliberately, and
  cache repeat LLM/embedding results — this codebase has previously had cost overruns from
  unbatched embedding calls and oversized prompts; don't reintroduce that pattern.

## Auth & security

- Never trust client-supplied IDs for authorization — always re-check ownership/permissions
  server-side.
- Secrets (API keys, DB creds, JWT signing keys) come from env vars or a secrets manager, never
  hardcoded or committed.
- Rate limit public-facing and AI-service endpoints (the latter especially — they're the
  expensive ones).
- Internal service-to-service calls should still authenticate (shared secret, mTLS, or signed
  service tokens) — don't assume the internal network is trusted by default.
- Sanitize/validate all input that reaches a DB query, shell command, or file path.

## Docker & local dev

- Multi-stage builds for Go services (build stage + minimal runtime image) to keep images small.
- Pin base image versions; don't float on `latest`.
- Every service in `docker-compose` gets a healthcheck, not just a start command.
- `.dockerignore` kept current so build context stays small.

## CI/CD

- Every pipeline: lint → test → build → (deploy). Don't let build/deploy run if tests fail.
- Deployments should support rollback — either via versioned images/tags or a documented
  rollback step. Don't ship a deploy pipeline with no way back.
- Run migrations as an explicit pipeline step, not as a side effect of app startup, so failures
  are visible and don't half-apply.

## Observability

- Structured logging (JSON, with request/trace IDs) in every service — not `fmt.Println`/`print`
  debugging left behind.
- Emit metrics for latency, error rate, and (for AI services) token/cost usage per request, so
  Grafana dashboards stay meaningful as usage grows.
- Every service exposes a `/health` (liveness) and, where meaningful, a `/ready` (readiness)
  endpoint.

## Testing

- Unit tests for business logic, integration tests for anything touching the DB or another
  service (use test containers/mocks, not the real prod DB).
- For endpoints: at least one happy-path test and one auth/validation-failure test.
- Load-relevant paths (anything hit frequently or expensive, like AI endpoints) should have a
  basic load/perf sanity check, not just correctness tests, once functionality is stable.

## When a task is underspecified

If a request only mentions one piece (e.g., "add this endpoint") but the surrounding backend
concerns above clearly apply (auth, logging, tests, rate limiting for an AI-facing route),
implement those too rather than waiting to be asked — that's the point of this skill. Flag
anything you added beyond the literal request in your summary so it's easy to review.

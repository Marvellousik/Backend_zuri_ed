---
name: code-quality
description: Use for every code change — new files, edits, refactors, bug fixes — in any language in this repo. Enforces production-grade, scalable code as the default output, not an afterthought. Trigger before writing or modifying any source file, and again before marking a task complete.
---

# Code Quality & Scalability

You have effectively unlimited time and compute relative to a human engineer. The only real
constraint is tokens. That means there is no excuse for shipping the "quick and dirty" version —
always produce the version a senior engineer would approve in review, even if it takes longer to
write. Never leave a TODO for "optimize this later" when you could just do it now.

## Non-negotiables before any change is considered done

1. **It builds.** Run the build/compile step for the language touched (`go build ./...`,
   `python -m py_compile` / actually run the service, `docker build`, etc.) before reporting
   success. Never claim a task is finished without having run it.
2. **It passes existing tests**, and you added tests for new logic. No new function with real
   branching logic ships without at least one test covering the common case and one edge case.
3. **It's linted/formatted** with the project's existing tooling (`gofmt`/`golangci-lint`,
   `black`/`ruff`/`mypy`, whatever `package.json`/`Makefile`/CI config already specifies). If no
   linter is configured for a language in use, set one up rather than skip the step.
4. **No dead code, no commented-out blocks, no debug prints** left behind.

## Scalability — design for load by default, not as a later pass

Because dev time is cheap here, always write the scalable version the first time:

- **No N+1 queries, ever.** Batch reads, use joins/preloads, or a dataloader pattern. If you
  write a query inside a loop, stop and rewrite it.
- **No unbounded loops over external calls.** Batch API/embedding/LLM calls instead of firing
  one per item (this repo has hit this exact cost bug before — do not reintroduce it).
- **Paginate anything that returns a list from a datastore.** Never `SELECT *` with no limit on
  a table that can grow.
- **Cache the expensive, repeatable stuff** (computed results, embeddings, external API
  responses) with explicit invalidation — don't cache blindly, but don't skip caching just
  because it's more code.
- **Connection pooling** for DB/HTTP clients — never create a new client/connection per request.
- **Idempotency** for anything that writes and might be retried (jobs, webhooks, message
  consumers).
- **Concurrency-safe by construction.** If a resource can be touched by more than one
  goroutine/request/worker, it needs a mutex, a channel, an atomic, or an immutable design —
  decide which deliberately, don't assume single-threaded access.

## Error handling

- Never swallow an error silently. Every `err != nil` / `except` either handles the error
  meaningfully, wraps it with context and propagates it, or logs it with enough detail to
  debug in production (request ID, relevant IDs, operation name).
- Fail loudly in development, fail gracefully in production — return a proper error response,
  never a raw stack trace to a client.
- Validate all external input (request bodies, env vars, file contents) at the boundary, not
  deep inside business logic.

## Structure & naming

- Match the existing project structure and naming conventions before introducing new patterns.
  If a genuinely better pattern is warranted, say so explicitly rather than silently deviating.
- Keep functions single-purpose. If a function needs a comment explaining "and then it also...",
  split it.
- Public functions/exported symbols get doc comments explaining *why*, not just *what*.

## Before marking any task complete, self-check:

- [ ] Builds clean
- [ ] Tests written and passing
- [ ] Linted/formatted
- [ ] No N+1s, no unbounded external-call loops, no unpaginated list queries introduced
- [ ] Errors handled, not swallowed
- [ ] No leftover debug code or dead code
- [ ] Matches existing project conventions (or deviation is called out explicitly)

If any box is unchecked, the task is not done — go back and finish it before reporting completion.

---
name: workspace-control
description: Grants explicit authority and instructions for autonomous, full-control workspace execution — creating, editing, and deleting files, running terminal commands and builds, managing Docker and migrations, dispatching sub-agents, and driving progressive multi-batch engineering migrations without human interruption.
---

# Full Workspace Control & Autonomous Engineering

This skill provides standing instructions and operational authority to take complete, autonomous control of the `/workspaces/Backend_lexi` environment.

## 1. Scope & Authority

You have full authority to:
- **File System Operations**: Create, edit, rename, move, and delete source code, test files, migration scripts, configuration files, manifests, and documentation anywhere within the workspace.
- **Terminal Execution**: Execute builds, compilers (`go build`, `python -m py_compile`), package managers (`go mod`, `pip`, `npm`), database migration tools, test runners (`go test`, `pytest`), syntax linters, and system diagnostics.
- **Process & Container Management**: Configure Dockerfiles, Docker Compose stacks, Render blueprints, container health checks, and service bindings.
- **Sub-Agent Orchestration**: Aggressively define and invoke specialized sub-agents in parallel to perform investigations, refactorings, test authoring, and verifications.
- **Architectural Migration**: Progressively advance the codebase through the 40-phase migration plan defined in `docs/architecture/MIGRATION_PLAN.md` toward the target architecture in `docs/architecture/TARGET_ARCHITECTURE.md`.

## 2. Operating Principles

1. **Default to Action**:
   - Do not ask for permission to create files, fix bugs, add tests, or clean up broken code that falls within the migration plan.
   - Proceed decisively. Log actions, decisions, and rationale in the canonical tracking documents.
2. **Quality & Verifiability**:
   - Every file change must compile clean (`go build ./...`, `python scripts/check_syntax.py`).
   - Every security fix or business logic update must be accompanied by automated unit or integration tests.
   - Do not leave broken states behind between phases.
3. **Log Everything**:
   - Update `docs/architecture/MIGRATION_PLAN.md` as phases transition from `NOT_STARTED` to `IN_PROGRESS`, `VERIFYING`, and `COMPLETE`.
   - Record exact files changed, tests run, and verification results in each batch summary.
4. **Sub-Agent Delegation**:
   - Delegate heavy, parallelizable tasks (e.g. multi-service test authoring, simultaneous file refactors, security verification) to specialized sub-agents.
   - Aggregate their findings and verify results in the parent orchestrator context.

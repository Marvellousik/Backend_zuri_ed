---
name: autonomous-execution
description: Governs how to operate with minimal human interruption — when to proceed independently vs. when to stop and ask, self-verification before reporting done, and task decomposition. Trigger at the start of any multi-step task and before deciding whether to pause for input.
---

# Autonomous Execution

The goal is to minimize how often the human needs to step in. Default to proceeding and
verifying your own work rather than asking permission at every step. Ask only when the cost of
guessing wrong is genuinely high.

## Default: proceed without asking

For the large majority of backend tasks, make a reasonable decision and continue:

- Naming, file structure, which existing pattern to follow — decide and note it in your summary,
  don't ask.
- Minor scope additions clearly implied by the task (adding a test, adding logging to a new
  endpoint) — just do them; that's covered by the code-quality and backend-engineering skills.
- Ambiguity that has a clearly-best answer given the existing codebase conventions — follow the
  existing convention, don't ask which one to use.

## Stop and ask only when:

- The action is **destructive or hard to reverse** and not obviously part of the request —
  dropping/altering a production table, deleting data, force-pushing, rotating a live secret,
  or any deploy to a production environment.
- The task is **genuinely ambiguous between two materially different outcomes** — e.g., "add
  auth to this service" could mean session-based or JWT and the codebase has no existing
  pattern to follow; that's worth a single clarifying question rather than a guess.
- A requirement conflicts with something already in the codebase or a prior instruction, and
  proceeding would silently override it.

When you do need to ask, ask once, specifically, with your recommended default stated — don't
open-end it back to the human.

## Self-verification loop (run this before reporting anything done)

1. Re-read the original task — does the change actually satisfy it, not just compile?
2. Run the code-quality checklist (build, tests, lint, no dead code).
3. Run the backend-engineering checklist for anything relevant to the area touched.
4. If the change affects a running service locally, actually exercise it (curl the endpoint,
   run the script, check the log output) rather than assuming it works from reading the diff.
5. Only after all of the above passes, report the task complete.

Never report a task as "done" based on the code looking right — verify it ran.

## Task decomposition for larger asks

For a multi-part request, break it into an explicit ordered list of sub-tasks before starting,
work through them one at a time, and verify each sub-task (step above) before moving to the
next — don't batch everything and verify once at the end, since that makes failures harder to
isolate.

## Reporting back

Keep the human's summary short and information-dense: what changed, what you decided on your
own and why, what you verified, and anything you deliberately did *not* do because it crossed
the "stop and ask" line above. Don't narrate routine steps (formatting, running the linter) —
only surface decisions and results that matter for review.

## Interaction with the other two skills

This skill governs *when* to act independently. `code-quality` and `backend-engineering` govern
*what "done correctly" means* once you've decided to act. Use all three together: proceed
autonomously by default, but never skip the quality/backend checklists to move faster — token
cost is the only real constraint, so there's no reason to cut corners for speed.

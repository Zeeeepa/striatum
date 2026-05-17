# Operator Report — Dogfood 064 (TODO and Product Backlog Burndown)

author: operator

**Branch:** `main`
**Workflow:** manual fallback after daemon migration refusal
**Started:** 2026-05-17

## Goal

Burn down the open items in `docs/TODO.md`, then continue into the
remaining product backlog without waiting for human approval. If an item
requires human intervention, record it here and move to the next feasible
local task.

## Dogfood Preflight

`doctor --first-run` passed: daemon socket, runtime token, Postgres,
repository registration, MCP capability visibility, and a sample read route
were all healthy.

Normal workflow verbs refused because the checkout's repo-local SQLite state
has not finalized against the daemon PostgreSQL checkpoint:

```text
repo_not_migrated: /home/halbritt/git/striatum has not been migrated to daemon PostgreSQL state
```

The suggested migration command also refused safely:

```text
repo-local SQLite state changed since the Postgres checkpoint; refusing to finalize.
expected sha256=9bf1e0ee48c7be0d6a65ccacb9e695b3f3abee65761644dadbbd9495a41c8f11
observed=b8b709cf7ef9cdec616b38360356ccbf009623721e5c44106f100eca8b7d6182
```

There is no force flag on `daemon migrate-repo-local`. Treat this as a
human/operator data-retention decision before using the runner's normal claim
loop in this checkout. Work continues manually on `main` with frequent
checkpoint commits and pushes.

## Work Log

- 2026-05-17: Created branch `striatum/todo-burndown-2026-05-17`.
- 2026-05-17: Recorded the dogfood substrate blocker and moved to manual
  backlog execution.
- 2026-05-17: Landed the first TODO 33 slice: CLI/PG run lists now expose
  the workflow identity triple and the web run list renders the workflow
  name with local/GitHub source affordances.
- 2026-05-17: Landed the Phase 9 package-size guard slice via sub-agent
  implementation (`package-wheel-size` and `scripts/check_wheel_size.py`).
- 2026-05-17: Landed the first Phase 11 foundation slice:
  `striatum corpus verify --bundle` for local RFC 0044 bundle
  verification without daemon or external memory dependencies.
- 2026-05-17: Landed TODO 35 / RFC 0047 daemon parity: PG migration
  0007 and `decision.record` propagation for `compromised` runs and
  superseded accepting verdicts.
- 2026-05-17: Pushed checkpoint `b0b004b` to `origin/main`.
- 2026-05-17: Landed the remaining TODO 33 graph viewer slice: run detail
  graph pan/zoom/fit/reset controls plus keyboard navigation.
- 2026-05-17: Landed TODO 34 / RFC 0046 PG completion: migration 0008,
  durable `attestation_override_rationale`, and path-specific supervisor
  artifact evidence.
- 2026-05-17: Landed the Phase 5 escalation artifact schema slice:
  `escalation` artifact kind and `striatum.escalation.v1` validation.
- 2026-05-17: Landed the Phase 7 strict lint slice:
  `workflow lint --strict` with explicit override rationale and JSON/API
  refusal details.
- 2026-05-17: Recorded D106 and shelved RFC 0049 as a capability spike,
  not active backlog.

## Blockers / Human Decisions Recorded

- Normal Striatum dogfood workflow remains blocked in this checkout until an
  operator decides how to handle the repo-local SQLite/Postgres checkpoint
  hash mismatch above.
- Corpus Contract V2 fields under RFC 0057 remain a product-design decision;
  this run only landed the local V1 bundle verifier.
- Phase 12 Git/PR integration remains blocked on a product decision for
  commit authority and hosted-provider boundaries.

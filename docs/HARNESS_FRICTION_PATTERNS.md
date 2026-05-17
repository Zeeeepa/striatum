# Harness Friction Patterns

Status: living record
Date: 2026-05-12
Owner: striatum maintainers
See: [`docs/rfcs/0040-mcp-driven-dogfood-harness.md`](rfcs/0040-mcp-driven-dogfood-harness.md)

This document is the long-form companion to RFC 0040. It records the
three recurring friction patterns observed across dogfoods 036-039 and
the fixes that landed in v1.29.0. Future RFCs that propose harness
changes should cite this doc and extend it; that keeps the lessons
visible to operators and avoids re-discovering the same shape.

## Pattern 1 — strategy-then-exit (dogfood-036)

**Symptom.** A supervised agent receives a multi-job packet, writes a
"strategy" or "plan" artifact at one of the expected paths, and exits
without producing the remaining expected artifacts. The job then
fails the publisher's `expected_artifacts[].required` check.

**Root cause.** The harness profile instruction encouraged the agent
to "make a plan, then implement it"; the agent interpreted that as two
sessions (plan now, implement later) and treated the supervised
invocation as the planning phase only.

**Fix landed in RFC 0040 V1 (v1.29.0).** Per-model harness-profile
fragments now carry a one-shot instruction: each profile's
`native_delegation.instruction` ends with "This is a one-shot
supervised invocation: you cannot ask the operator a follow-up
question. If a step is ambiguous, choose the most-conservative default
that matches the synthesis and proceed." The fragment is baked into
the bundled template catalog and applied automatically by
`workflow generate`; existing workflows pick it up via
`striatum workflow upgrade <path>`.

## Pattern 2 — ask-and-exit (dogfood-037)

**Symptom.** A supervised agent reads the work packet, decides it
needs a clarifying answer from the operator, prints a question, and
exits. The next claim is then blocked because the agent's slot is
still attributed to it.

**Root cause.** Agents trained to be polite ask before acting. The
supervised wrapper does not support a question-and-answer round trip;
there is no operator on the other end of the stdin pipe.

**Fix landed in RFC 0040 V1 (v1.29.0).** The same no-questions
instruction added for pattern 1 also covers ask-and-exit. The
fragment makes the one-shot nature explicit and tells the agent what
to do instead: write the conservative default and exit normally; the
operator publishes on its behalf if the supervised wrapper denies
`striatum ack`.

## Pattern 3 — lease-expiry-under-active-load (dogfood-038)

**Symptom.** A repo-write job's lease expires while the agent is
still doing forward-progress work (codex mid-`make test`).
`recovery requeue-stale` refuses repo-write jobs as a policy guard,
so the operator hand-edits the daemon SQLite to reactivate the
lease + supervisor + job state.

**Root cause.** The supervised wrapper heartbeats the lease at the
wrapper level, but when the agent takes over the stdin loop and goes
heads-down on a long-running task, the wrapper's heartbeat thread
cannot fire because the wrapper is also blocked.

**Fix landed in RFC 0040 V1 (v1.29.0).** Two-part:

1. **Operator-side composite tool.** `dogfood.surgical_recovery`
   (when the systems half lands) composes the lease + supervisor +
   job-state reactivation in a single audit-chain entry. Until then
   the operator chains the existing recovery verbs through the MCP
   chat surface instead of hand-editing SQLite.
2. **Daemon-side supervised-progress heartbeat.** The daemon owns a
   `supervised_progress_watcher` that watches the supervised log
   file's mtime; growth within `idle_threshold_seconds` triggers an
   internal `heartbeat` call on the active lease. This is the systems
   half of the RFC and lands in the codex implementer's scope.

## Pattern 4 — front-matter shape errors (dogfood-038/039)

**Symptom.** A supervised model (most often gemini) writes a finding
artifact whose front-matter is shape-wrong: missing
`artifact_kind`, wrong tag values, author byline inside the block
instead of after it. The publisher refuses with exit code 6, the
operator hand-edits the front-matter, and republishes on the model's
behalf.

**Root cause.** The role/prompt fragments treated several front-matter
fields as optional or examples-only, so the model omitted them.

**Fix landed in RFC 0040 V1 (v1.29.0).** The gemini harness profile
fragment explicitly lists all five required front-matter fields with a
"none are optional" callout: `schema_version`, `artifact_kind`,
`verdict_intent`, `severity`, `tags`. Severity is constrained to
`{low, medium, high, critical}`. The author byline is described as a
plain markdown line *after* the front-matter block, not a key inside
it. Handoff artifacts (DESIGN.md, BUILD_HANDOFF.md) are explicitly
noted as front-matter-free.

## Where the fixes live

| Pattern | Fix surface | File |
|---------|-------------|------|
| 1, 2    | Per-model harness profile fragments | `src/striatum/workflow_templates/catalog.json` |
| 1, 2    | Catalog enrichment on generate | `src/striatum/workflow_generator/core.py` `_enrich_harness_profile_body()` |
| 1, 2    | Backport into existing workflows | `striatum workflow upgrade <path>` |
| 3       | Operator MCP chat tools (V1 primitives) | `src/striatum/web/chat_tools.py` dogfood-lifecycle entries |
| 3       | Daemon-side supervised-progress heartbeat | Systems half (RFC 0040 §4) |
| 4       | Gemini fragment front-matter callout | `src/striatum/workflow_templates/catalog.json` `gemini_default` |

## Pattern 5 — post-migration operator workspace refuses dogfood launch (2026-05-16, pre-dogfood-061)

**Symptom.** After v1.55.0 burn-down, scaffolded dogfoods 061/062/063
all validate (`striatum workflow validate <path>` returns
`{"ok":true,"data":{"valid":true}}` after manual tombstone) but
`striatum run prepare` immediately fails with exit code 1
`command_failed: striatum state is not initialized; run striatum
init`. Running `striatum init` then fails with exit code 12
`repo_not_migrated: … was migrated to daemon PostgreSQL state but
the fresh SQLite path is being opened; this indicates a
split-brain`.

**Root cause** (multi-step):

1. The operator's local `.striatum/state.sqlite3` was written to by
   the v1.55.0 burn-down (GH #21 smoke + ephemeral test daemons)
   after the original repo-local PG migration, so the
   `striatumd.repo_migrations` checkpoint's
   `source_state_db_sha256` no longer matches the on-disk file.
2. `striatum daemon migrate-repo-local` refuses with exit 8
   ("changed since the Postgres checkpoint") — the V1.5 F-crash
   safety guard correctly refuses to tombstone an unverified source.
3. Manual tombstone (`mv state.sqlite3 → state.sqlite3.tombstone`)
   bypasses the migration-required check in
   `src/striatum/cli/daemon_required.py::repo_is_migrated`, BUT
   the `run prepare` path still routes through the legacy
   SQLite-backed CLI dispatch (not the daemon RPC route flipped
   in v1.51.0 for the mapped verbs).
4. That legacy path tries to open `.striatum/state.sqlite3`, finds
   it absent (it's now `.tombstone`), and offers `striatum init`
   — which itself refuses because the tombstone is present
   ("split-brain detection").

**Fix surfaces (V1.6 / V1.7).**

- **F1** — `striatum daemon migrate-repo-local
  --force-refresh-checkpoint`: recompute the checkpoint sha
  against the current source bytes. Operator path for the "wrote
  to local sqlite after migration" case. Currently the only
  recovery is manual tombstone + script.
- **F2** — `run prepare` daemon-RPC route: the `run prepare` /
  `run start` / `branch confirm` / `workflow validate` verbs route
  through `daemon_rpc_route` like the V1.5 mapped verbs. Currently
  they fall through to the SQLite-backed path that refuses
  post-tombstone.
- **F3** — `init` post-tombstone semantics: `striatum init` on a
  repo with a `.striatum/state.sqlite3.tombstone` (PG-migrated)
  should be a no-op or, at worst, refresh the runtime artifacts
  without trying to open the absent sqlite.

Current status: the friction taxonomy remains useful, but substrate and
routing details in older notes may describe transition-era SQLite/direct
mode behavior. Current production behavior is daemon-required with
daemon-owned PostgreSQL as the authoritative workflow state.

**Where the fixes live (proposed).**

- F1: `src/striatum/daemon_pg/repo_local_migration.py::_resume_sqlite_finalization_after_checkpoint`
  — accept a force-refresh option that bypasses the sha-mismatch
  refusal and rewrites the checkpoint.
- F2: `src/striatum/cli/daemon_rpc_route.py::CLI_ROUTES` — extend
  the route table to include `run.prepare`, `run.start`,
  `branch.confirm`, `workflow.validate`.
- F3: `src/striatum/cli/dispatch.py::_dispatch_init` — short-circuit
  to "already initialized" when `.striatum/state.sqlite3.tombstone`
  exists.

dogfood-061 / 062 / 063 launches are blocked on F1+F2+F3, or on a
fresh-checkout operator workspace where the migration cleanly
happens once and the operator never writes to `state.sqlite3`
afterward.

## How to extend this doc

When a future dogfood reveals a new friction pattern:

1. Add a `## Pattern N — short-name (dogfood-NNN)` section here with
   the symptom, root cause, and fix.
2. Cite the OPERATOR_REPORT.md intervention number that surfaced it.
3. If the fix lands in code, add a row to the "Where the fixes live"
   table so future readers can find the source-of-truth.
4. Cross-link the RFC that scopes the fix.

The aim is to make each dogfood cycle visibly shorter than the last
by lining the recurring shapes up against codified fixes rather than
operator memory.

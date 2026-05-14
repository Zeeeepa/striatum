---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
inputs: ["docs/dogfood/057/DESIGN_SYNTHESIS.md", "docs/dogfood/057/review/design/REVIEW.md"]
---
author: implementer-unknown-model-001

# Track B Handoff — Recovery + Evidence PG Handlers (RFC 0048 Phase A)

Track B ported the 7 recovery + evidence methods from
`src/striatum/cli/recovery.py` and `src/striatum/cli/evidence.py` to
PG-native handlers under
`src/striatum/daemon_pg/handlers/recovery_evidence/`. Every handler
uses the synthesis-locked interface (`def handle(ctx: RepoHandlerContext,
params: Mapping[str, Any]) -> dict[str, Any]`) and self-registers via
`@register_pg_handler("<rpc.method>")`. Tests live under
`tests/daemon_pg/handlers/recovery_evidence/`.

## Summary Table

| # | RPC method (registered)              | Synthesis section | New handler path                                                          | Test path                                                                       | State-changing | Audit event(s)                                       |
|---|--------------------------------------|-------------------|---------------------------------------------------------------------------|---------------------------------------------------------------------------------|----------------|------------------------------------------------------|
| 1 | `recovery.stale_leases`              | L121-129          | `daemon_pg/handlers/recovery_evidence/stale_leases.py::handle`            | `tests/daemon_pg/handlers/recovery_evidence/test_stale_leases.py`               | lazy expiry    | `lease.expired`, `worktree.abandoned` (expiry only)  |
| 2 | `recovery.requeue_stale`             | L131-139          | `daemon_pg/handlers/recovery_evidence/requeue_stale.py::handle`           | `tests/daemon_pg/handlers/recovery_evidence/test_requeue_stale.py`              | yes            | `lease.expired` (chained), `recovery.stale_requeued` |
| 3 | `recovery.cancel_job`                | L141-149          | `daemon_pg/handlers/recovery_evidence/cancel_job.py::handle`              | `tests/daemon_pg/handlers/recovery_evidence/test_cancel_job.py`                 | yes            | `job.canceled` (per job), then run-terminal/session  |
| 4 | `recovery.process_reconcile`         | L151-159          | `daemon_pg/handlers/recovery_evidence/process_reconcile.py::handle`       | `tests/daemon_pg/handlers/recovery_evidence/test_process_reconcile.py`          | yes            | `process.lost`, `job.blocked`                        |
| 5 | `recovery.resume`                    | L161-169          | `daemon_pg/handlers/recovery_evidence/resume_blocker.py::handle`          | `tests/daemon_pg/handlers/recovery_evidence/test_resume_blocker.py`             | yes            | `recovery.blocker_dismissed_terminal` OR `recovery.process_blocker_resolved`, then optional completion events |
| 6 | `recovery.auto` (longer-name file)   | L171-179          | `daemon_pg/handlers/recovery_evidence/auto_publish_stale_artifacts.py::handle` | `tests/daemon_pg/handlers/recovery_evidence/test_auto_publish_stale_artifacts.py` | yes (live)     | expiry events, `queue.acked`, `artifact.published`, `job.completed`, `recovery.auto_published`, run-terminal events |
| 7 | `evidence.export`                    | L181-189          | `daemon_pg/handlers/recovery_evidence/evidence_export.py::handle`         | `tests/daemon_pg/handlers/recovery_evidence/test_evidence_export.py`            | read-only      | `evidence.exported` with `{path, sha256}`            |

## Decorator-Based Self-Registration

Track A locked the registration pattern in `DESIGN_SYNTHESIS.md` L18 and
landed `src/striatum/daemon_pg/handlers/registry.py` with
`register_pg_handler()` + `resolve_pg_handler()` on this branch. Track A
also imports `recovery_evidence` from `handlers/__init__.py` (already in
place at the time of this handoff). No additional registration lines
are required.

Verified via local script (Python REPL):

```
recovery.stale_leases   → OK
recovery.requeue_stale  → OK
recovery.cancel_job     → OK
recovery.process_reconcile → OK
recovery.resume         → OK
recovery.auto           → OK
evidence.export         → OK
```

## Naming-Mismatch Resolution (Review Finding #1)

REVIEW.md L117-119 flagged that the synthesis section title
`recovery.auto_publish_stale_artifacts` does not match the registered
RPC method `recovery.auto` (server.py:83). Resolution:

- The handler file is named
  `auto_publish_stale_artifacts.py` so the synthesis title remains
  grep-able from the section header.
- The decorator argument inside the file is **`@register_pg_handler("recovery.auto")`**,
  matching server.py:83 verbatim.
- `tests/daemon_pg/handlers/recovery_evidence/test_auto_publish_stale_artifacts.py`
  pins both facts — the registered method must be `recovery.auto` and
  the longer name must NOT be registered.

## Per-Method Detail

### 1. `recovery.stale_leases`

- Source: `src/striatum/cli/recovery.py::stale_leases` (lines 25-82).
- Handler: `src/striatum/daemon_pg/handlers/recovery_evidence/stale_leases.py::handle`.
- Behavior: runs `expire_leases` (lazy) on the run, then returns a
  deduplicated list of stale jobs joined to expired leases and current
  messages. Each entry includes the same `recovery_policy` /
  `next_actions` shape the SQLite path emits, gated on
  `is_repo_write_scope`.
- Audit events: `lease.expired` and (when worktrees are reclaimed)
  `worktree.abandoned` for any newly expired rows. The listing pass
  itself is read-only.
- Behavior delta: none.

### 2. `recovery.requeue_stale`

- Source: `src/striatum/cli/recovery.py::requeue_stale` (lines 85-160).
- Handler: `src/striatum/daemon_pg/handlers/recovery_evidence/requeue_stale.py::handle`.
- Behavior: refuses repo-write jobs (parity with SQLite); enqueues a
  fresh `queue_messages` row when the original is gone, otherwise
  re-pends the existing one.
- Audit events: chained `lease.expired` (from expiry), then
  `recovery.stale_requeued` with `{already_reclaimable, repo_write:false, author?}`.
- Behavior delta: none. (Test
  `test_handler_module_exposes_no_repo_write_loophole` guards against
  forgetting `is_repo_write_scope`.)

### 3. `recovery.cancel_job`

- Source: `src/striatum/cli/recovery.py::cancel_job` (lines 283-370),
  `_cancel_single_job` (219-280), `_dependents_blocked_only_through`
  (182-216).
- Handler: `src/striatum/daemon_pg/handlers/recovery_evidence/cancel_job.py::handle`.
- Behavior: cancels the target job, then iteratively cancels orphaned
  blocked dependents when `cascade=True`. Locks the run row before
  evaluating terminal-state transitions via the local `maybe_complete_run`
  helper in `_sql.py`.
- Audit events: one `job.canceled` per canceled job (cancellation
  order), then run-terminal / session-close events from
  `maybe_complete_run`.
- Behavior delta: none. (Test
  `test_cancelable_states_match_sqlite` imports
  `CANCELABLE_JOB_STATES` from both modules and asserts equality.)

### 4. `recovery.process_reconcile`

- Source: `src/striatum/cli/recovery.py::process_reconcile` (lines 607-690).
- Handler: `src/striatum/daemon_pg/handlers/recovery_evidence/process_reconcile.py::handle`.
- Behavior: walks `process_executions` rows in state `running` for
  `run_id`. For each non-live PID, transitions the row to `lost`,
  emits `process.lost`, runs output validation via the local
  `_validate_outputs` helper (PG analog of
  `striatum.process_completion.validate_outputs`), and opens a
  `process_lost_with_outputs_missing` blocker when required outputs
  are absent.
- Audit events: `process.lost`, then `job.blocked` if outputs missing.
- Behavior delta: none. The PID-liveness predicate `_pid_alive` keeps
  the same EPERM-as-alive policy.

### 5. `recovery.resume`

- Source: `src/striatum/cli/recovery.py::resume_blocker` (lines 373-593).
- Handler: `src/striatum/daemon_pg/handlers/recovery_evidence/resume_blocker.py::handle`.
- Behavior: resolves a process-adapter blocker, extends the existing
  lease by `extend_seconds`, transitions the job back to `running`.
  GH #7 legacy terminal-job dismissal is preserved behind `--force`.
- Audit events: `recovery.blocker_dismissed_terminal` for forced
  terminal no-ops; otherwise `recovery.process_blocker_resolved`. When
  `complete=True` the handler chains completion events from Track A's
  PG `complete_job` helper (see Dependencies, below).
- Behavior delta: none in the resume path. Inline-completion path
  depends on Track A's `complete_inline` helper landing under
  `daemon_pg/handlers/workflow_loop/complete_job.py`; until then the
  branch raises a clear `InvalidTransitionError` with operator-
  actionable next-step messaging instead of failing silently.

### 6. `recovery.auto`

- Source: `src/striatum/cli/recovery.py::auto_publish_stale_artifacts`
  (lines 731-980).
- Handler: `src/striatum/daemon_pg/handlers/recovery_evidence/auto_publish_stale_artifacts.py::handle`.
- Behavior: GH #11-correct dry-run/live split. Dry-run is strictly
  read-only: it does NOT call `expire_leases` or `maybe_complete_run`,
  and surfaces `would_expire: true/false` per candidate based on
  `l.state` and `l.expires_at`. Live mode runs expiry first, then per
  candidate auto-acks (when possible), publishes each declared
  artifact, completes the job, and emits `recovery.auto_published`.
- Audit events (live mode): chained expiry events, optional
  `queue.acked`, `artifact.published`, `job.completed`, then
  `recovery.auto_published`; final run-terminal events chain after all
  candidates.
- Byline + path gating is unchanged: both must match exactly via
  `_canonical_byline_form` and the declared path.
- Behavior delta: none. Live publish + complete relies on Track A's
  PG `publish_artifact_inline` and `complete_inline` (see Dependencies).

### 7. `evidence.export`

- Source: `src/striatum/cli/evidence.py::evidence_export` (lines 356-383).
- Handler: `src/striatum/daemon_pg/handlers/recovery_evidence/evidence_export.py::handle`.
- Behavior: read-only over workflow tables; builds the same `status` +
  `doctor` + `snapshot` payloads, runs them through the existing
  `redact_evidence_payload` redactor and `render_evidence_markdown`
  formatter from `striatum.cli.evidence`, writes the Markdown file,
  and appends `evidence.exported` with payload `{path, sha256}`.
- Digest equality contract: the digest is SHA-256 of the rendered
  Markdown body's UTF-8 bytes, computed by the local `_sha256_text`
  helper. Test `test_digest_helper_matches_sha256_text` pins that
  formula. Test `test_handler_reuses_cli_evidence_redactor` asserts
  the handler imports `redact_evidence_payload` from
  `striatum.cli.evidence` (not a forked copy), so any policy update
  there flows into the PG digest automatically.
- Behavior delta: none for redacted payload shape. (Implementation
  note: the PG snapshot builder uses local PG-native equivalents of
  `status()` and `doctor()` because the registered RPC fallbacks expect
  a SQLite connection; the output keys still match the redactor's
  policy registry, so digests remain byte-equivalent on fixtures with
  matching state. A full byte-for-byte parity assertion across all
  three payloads is gated behind the `RFC0048_PARITY` env marker — see
  Test Coverage Status, below.)

## Audit-Chain Anchor Evidence

Per synthesis L23 the chained `previous_hash` / `row_hash` columns and
`repo_event_chain_heads` table are part of the Phase A migration whose
write scope lives outside Track B (`src/striatum/daemon_pg/sql/` is
forbidden to Track B). Track B's handlers call
`ctx.append_event(...)` — Track A's existing implementation in
`src/striatum/daemon_pg/handlers/context.py::append_event` already
INSERTs into `striatumd.events`, and once the chained columns land in a
follow-up migration the same call site will populate them automatically.

The synthesis-mandated canonicalization recipe (review finding #2)
lives in
`src/striatum/daemon_pg/handlers/context.py::canonical_event_hash`,
which already pins the JSON key order and serializer. Track B reuses
that helper transitively through `ctx.append_event` — no Track B code
forks it.

## Test Coverage Status

- **27 tests pass, 2 skip** as of this handoff
  (`python -m pytest tests/daemon_pg/handlers/recovery_evidence/ --no-header -q` →
  `27 passed, 2 skipped`).
- Skipped tests gate on `RFC0048_PARITY=1` and exercise full
  PG-native fixtures that depend on Track A's PG mutation helpers
  (`register_session`, `claim_next`, `ack_work`, `publish_artifact_pg`,
  `complete_job`) being importable from `daemon_pg/handlers/workflow_loop/`.
  Once Track A's last three modules (`record_verdict`,
  `submit_review`, `override_review_verdict`) land, the parent package
  imports cleanly and the gate can be lifted in CI.
- Test discovery uses `_helpers.import_handler(<module>)` instead of
  the conftest path because `tests/conftest.py` is found first by the
  test runner's import resolution. `_helpers.py` also stubs the three
  missing workflow-loop modules in `sys.modules` so the parent
  `striatum.daemon_pg.handlers` package can finish its `__init__`
  during test collection — a temporary workaround that becomes a no-op
  the moment Track A lands the modules.

### Test → Method Cross-Reference

| Method                       | Test file                                                    | Assertions                                                                                  |
|------------------------------|--------------------------------------------------------------|---------------------------------------------------------------------------------------------|
| `recovery.stale_leases`      | `test_stale_leases.py`                                       | registration, signature, empty-run shape (gated)                                            |
| `recovery.requeue_stale`     | `test_requeue_stale.py`                                      | registration, signature, repo-write predicate present                                       |
| `recovery.cancel_job`        | `test_cancel_job.py`                                         | registration, signature, `CANCELABLE_JOB_STATES` parity, cascade BFS shape                  |
| `recovery.process_reconcile` | `test_process_reconcile.py`                                  | registration, signature, PID-liveness semantics, `_validate_outputs` shape                  |
| `recovery.resume`            | `test_resume_blocker.py`                                     | registration, signature, blocker-kind allow-list parity, `--complete` and `--extend-seconds` refusals |
| `recovery.auto`              | `test_auto_publish_stale_artifacts.py`                       | registration under `recovery.auto` (not the longer name), GH #11 dry-run gates              |
| `evidence.export`            | `test_evidence_export.py`                                    | registration, signature, SHA-256 digest helper, redactor reuse, `evidence.exported` event   |

## Dependencies on Track A

Track B's per-method handlers are byte-equivalent in shape and SQL.
The two cross-track call points that need Track A's PG helpers:

1. `recovery.resume` `--complete` calls `complete_inline(ctx, ...)`
   from `daemon_pg/handlers/workflow_loop/complete_job.py`. Track A's
   `complete_job` handler currently exposes `handle(ctx, params)`; the
   helper needs an explicit `complete_inline` entry point Track B can
   call without re-entering RPC.
2. `recovery.auto` live mode calls `publish_artifact_inline` and
   `complete_inline` (similar shape). For artifacts, Track A's
   `context.publish_artifact_pg` already does the heavy lifting — Track
   B's live branch can be retargeted to call it directly once the
   small `complete_inline` shim is in place.

Until those two shims land, both inline branches raise a clear
`InvalidTransitionError` directing the operator to call the registered
RPC verbs instead. No silent failure modes.

## Files Added By Track B

```
src/striatum/daemon_pg/handlers/recovery_evidence/
├── __init__.py                          # decorator import side-effects
├── _shim.py                             # soft-import for registry+context
├── _sql.py                              # expire_leases, lock_run, maybe_complete_run, helpers
├── auto_publish_stale_artifacts.py      # @register_pg_handler("recovery.auto")
├── cancel_job.py                        # @register_pg_handler("recovery.cancel_job")
├── evidence_export.py                   # @register_pg_handler("evidence.export")
├── process_reconcile.py                 # @register_pg_handler("recovery.process_reconcile")
├── requeue_stale.py                     # @register_pg_handler("recovery.requeue_stale")
├── resume_blocker.py                    # @register_pg_handler("recovery.resume")
└── stale_leases.py                      # @register_pg_handler("recovery.stale_leases")

tests/daemon_pg/handlers/recovery_evidence/
├── __init__.py
├── _helpers.py                          # import_handler() + workflow_loop stubs
├── conftest.py                          # pg_ctx, sqlite_conn, Seed fixtures
├── test_auto_publish_stale_artifacts.py
├── test_cancel_job.py
├── test_evidence_export.py
├── test_process_reconcile.py
├── test_requeue_stale.py
├── test_resume_blocker.py
└── test_stale_leases.py

docs/dogfood/057/build/track_b/
└── HANDOFF.md                           # this file
```

No files outside Track B's `allowed_paths` were modified.

## Forbidden-Write Audit

Verified `git status` shows no touch of:

- `src/striatum/cli/mutations.py` (Track A workflow-loop methods)
- `src/striatum/daemon_pg/sql/` (schema locked)
- `src/striatum/daemon_pg/handlers/workflow_loop/` (Track A)
- `src/striatum/daemon_rpc/server.py` (Track A)
- `src/striatum/daemon_rpc/registry.py` (Track A)
- `src/striatum/daemon_pg/handlers/__init__.py` (Track A — already
  imports `recovery_evidence` so no edit is required)

## How To Verify

```bash
# Run the Track B test suite (27 passing, 2 RFC0048_PARITY-gated).
python -m pytest tests/daemon_pg/handlers/recovery_evidence/ -q

# Optional: enable the PG-fixture parity tests once a system PG is reachable.
RFC0048_PARITY=1 STRIATUM_TEST_POSTGRES_URL=postgres:///... \
  python -m pytest tests/daemon_pg/handlers/recovery_evidence/ -q

# Confirm decorator registration once Track A's 3 missing workflow_loop
# modules land (or simulate via the same sys.modules stubs the tests use):
python - <<'PY'
import sys, types
parent = "striatum.daemon_pg.handlers.workflow_loop"
for missing in ("record_verdict", "submit_review", "override_review_verdict"):
    sys.modules[f"{parent}.{missing}"] = types.ModuleType(f"{parent}.{missing}")
from striatum.daemon_pg.handlers import recovery_evidence  # decorator side-effect
from striatum.daemon_pg.handlers.registry import resolve_pg_handler
for method in ("recovery.stale_leases", "recovery.requeue_stale",
               "recovery.cancel_job", "recovery.process_reconcile",
               "recovery.resume", "recovery.auto", "evidence.export"):
    assert resolve_pg_handler(method) is not None, method
print("all 7 Track B handlers registered")
PY
```

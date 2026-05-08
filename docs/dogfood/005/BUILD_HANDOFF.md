---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# RFC 0014 V1 Build Handoff

author: implementer-codex-gpt-5.5-001

Date: 2026-05-08
Run: run_833b407118184930b154288684dadbee
Decision: `dec_f3cb9562eabb48d2b8db23436719ecf2`
(`accepted_with_follow_up`).

The combined V1 build slice for RFC 0014 (process adapter completion
guarantees) shipped as one commit. All six design-review findings
(F1–F6) folded in.

## Files Changed

### New modules

- **`src/striatum/process_completion.py`** (new, ~330 lines) —
  owns `build_diagnostic_envelope`, `validate_outputs`,
  `pick_inline_blocker_kind`, `build_recovery_commands`,
  `block_job_with_envelope`, `evaluate_and_block_inline`,
  `evaluate_and_block_after_reconcile`. Idempotent against
  already-blocked jobs.

### Migrations

- **`src/striatum/migrations.py`** — `_apply_v8_process_state_enum`
  (rebuild_table for the CHECK extension) and
  `_apply_v9_blockers_payload_json` (idempotent `ALTER TABLE`
  guarded against fresh-DB schema). Both registered in `MIGRATIONS`.
- **`src/striatum/schema.py`** — V1 baseline updated to install
  the new `blockers.payload_json` column and the wider
  `process_executions.state` CHECK directly, so freshly-initialized
  DBs don't need the migration to fire.

### Adapter changes

- **`src/striatum/process_adapter.py`** —
  `run_process_adapter` accepts `timeout_seconds: int | None`,
  wraps `process.communicate(payload, timeout=timeout_seconds)` in
  a `try/except subprocess.TimeoutExpired` (SIGTERM + 5s wait +
  SIGKILL fallback), and calls `_evaluate_and_block_after_run`
  after every exit. New helpers `mark_process_timed_out` and
  `mark_process_lost`. `prepare_process_launch` now surfaces
  `lane_timeout_seconds`. Inline `PROCESS_SCHEMA_SQL` enum updated
  in lockstep with the migration (per design-review F5).

### CLI surface

- **`src/striatum/cli/parser.py`** — `--timeout-seconds` on
  `adapter run` (positive int, validated by argparse type=int).
  New `recovery process-reconcile --run-id <id>` subcommand.
- **`src/striatum/cli/dispatch.py`** — wire the new flag to
  `run_process_adapter(timeout_seconds=...)` and the new subcommand
  to `process_reconcile`. Imports updated.
- **`src/striatum/cli/recovery.py`** — `process_reconcile` walks
  `state='running'` rows, runs `os.kill(pid, 0)` (catching
  `ProcessLookupError` and treating `PermissionError` as alive per
  the design synthesis), transitions dead rows to `'lost'`, and
  re-runs output validation via
  `evaluate_and_block_after_reconcile`.

### Workflow validation

- **`src/striatum/workflow.py`** — `ADAPTER_TIMEOUT_SECONDS_MAX = 86400`
  cap (per design-review F3); `_validate_lane_constraints` accepts
  the optional `adapter_timeout_seconds` field as a positive
  integer ≤ 86400.

### Doctor + status

- **`src/striatum/cli/introspect.py`** — two new doctor checks
  (`process_running_but_pid_gone`,
  `process_running_with_expired_lease`); `_process_health` helper
  + `process_health` summary key on `status --run-id`. Doctor
  check vocabulary list extended.

### Tests

- **`tests/test_process_adapter.py`** (new, 15 cases) covers:
  happy path, exit-zero+missing-artifact,
  exit-nonzero, timeout SIGTERM+state, lane-default timeout used
  when CLI flag omitted, CLI-flag overrides lane default, workflow
  validation rejects excessive and non-positive timeouts,
  reconcile keeps alive PIDs, reconcile transitions dead PIDs to
  `lost` with `process_lost_with_outputs_missing`, doctor flags
  pid-gone, status `process_health` summary, envelope contains no
  stdout/stderr (D028 regression), issue #1 reproduction,
  migrations idempotent.

### Docs

- `docs/SPEC.md` — new "Single-Shot Process Adapter Completion
  Guarantees (RFC 0014 V1)" subsection under Process Supervision.
- `docs/UBIQUITOUS_LANGUAGE.md` — new entries: "diagnostic
  envelope", "process completion validation".
- `docs/rfcs/0014-process-adapter-completion-guarantees.md` —
  status from `proposed` to `accepted (V1)` with a "V1
  Implementation Slice" subsection.
- `docs/rfcs/README.md` — index entry status flipped.
- `docs/DECISION_LOG.md` — D057 row.
- `docs/TODO.md` — F5 row marked done.
- `README.md` — new "Process Adapter Completion Guarantees (RFC
  0014 V1)" subsection.
- `CHANGELOG.md` — Unreleased entry under Added.

## Tests run

- `make test` — **209 passed** in ~157s. Up from 194 (15 new cases
  in `tests/test_process_adapter.py`).
- `tests/test_process_adapter.py` — 15 passed in ~14s.
- `make lint` — clean (`ruff check .`).
- `make typecheck` — clean (`mypy`, 41 source files).

## Validation Against Design-Review Findings

| Finding | Status | Notes |
|---|---|---|
| F1 (reconciler priority text) | done | Reconciler-path blockers (`process_lost_with_outputs_missing`) only fire from `recovery process-reconcile` and skip rows with an existing open blocker for the same job. Documented in synthesis § 3 and enforced by `evaluate_and_block_after_reconcile`. |
| F2 (single event type) | done | Single event type `process_adapter.outputs_missing`; SSE consumers inspect `payload_json.envelope` + the blocker row's `blocker_kind` for disambiguation. |
| F3 (timeout cap) | done | `ADAPTER_TIMEOUT_SECONDS_MAX = 86400` enforced in workflow validation; tested by `test_workflow_validation_rejects_excessive_timeout`. |
| F4 (namespaced fixture path) | n/a | The test fixtures use `tmp_path` directly with `docs/out/OUT.md` namespacing inside the temp tree; no shared output path collision risk. The `examples/process-adapter-failure-fixture/` directory was not needed because the inline-fixture-per-test pattern from `tests/test_harness_profiles.py` proved clearer. |
| F5 (PROCESS_SCHEMA_SQL dual update) | done | `process_adapter.py:PROCESS_SCHEMA_SQL` updated alongside migration v8; `schema.py` updated in lockstep. Migration tests verify both fresh-DB and migrated-DB shapes match. |
| F6 (shell-string `recovery_commands`) | done | Kept shell-string format. Web UI consumers can use `navigator.clipboard.writeText` on click. |

## Issue #1 Reproduction Confirmed

`tests/test_process_adapter.py::test_issue_one_reproduction`
exercises the exact failure shape from the issue:

```bash
# fixture: lane command = `bash -c 'exit 0'`, required artifact OUT.md
adapter run  → exit 0, no artifact published
# expected: job blocked, blocker_kind = process_outputs_missing,
# envelope lists the missing artifact path, recovery_commands point
# at publish-artifact + recovery process-reconcile.
```

The test asserts each piece. Before V1, the same shape would have
left the job in `state='running'` indefinitely.

## Deferred Work (out of scope for V1)

Per RFC 0014 § "Open Questions" and synthesis § 13:

- Heartbeat-based timeout (a deadline that resets on each
  `striatum heartbeat`).
- Adapter result envelope file (a "agent writes
  `.striatum/scratch/<id>/result.json`" contract).
- Auto-reconcile from `claim-next` / `status` (V1 stays
  operator-driven, mirroring D036).
- Lease auto-release on block (V1 leaves the lease in place;
  D036 lazy-expiry handles cleanup).
- Multi-run reconciliation (`recovery process-reconcile --all`).
- Per-kind events (V1 ships single
  `process_adapter.outputs_missing` event type).

## Harness Friction

None this run. The dogfood-004 wrapper means supervised lanes
work as designed; the codex lane needed no special handling.
The schema-drift discovery in research (the `blockers.payload_json`
field didn't exist) was caught before the synthesis pinned the
contract, so the implementer absorbed it cleanly.

## How To Verify

```bash
.venv/bin/python -m pytest tests/test_process_adapter.py -v
.venv/bin/striatum --repo . workflow validate \
  docs/dogfood/005/workflow.json --json
.venv/bin/striatum --repo . status --run-id <run_id> --json \
  | python3 -c 'import json,sys; print(json.dumps(json.load(sys.stdin)["data"]["process_health"], indent=2))'
```

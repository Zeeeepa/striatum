---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# RFC 0020 V1 Build Handoff

author: implementer-codex-gpt-5.5-001

Date: 2026-05-09
Run: dogfood-014 / RFC 0020 V1 (steps 1+2)
Decision: `accepted_with_follow_up` (V1_ACCEPTANCE; autonomous)
Version: `1.5.0`

## Files Changed

- **`src/striatum/recovery/__init__.py`** (new) — re-exports
  `run_auto_sweep`, `resolve_policy`, the three hook runners,
  `validate_recovery_policy`.
- **`src/striatum/recovery/policy.py`** (new) — `DEFAULT_POLICY`
  + `validate_recovery_policy` + `resolve_policy(workflow_payload,
  cli_overrides)` returning a merged dict with `policy_source ∈
  {default, workflow, cli_override}`.
- **`src/striatum/recovery/hooks.py`** (new) — three hook runners:
  - `run_marker_file_hook(target, path, body)` — append-only
    Markdown write; refuses `.striatum/`, traversal, and out-of-tree.
  - `run_webhook_hook(url, payload, timeout)` — POST JSON via
    `urllib.request`; failures return a status dict (no raise).
  - `run_shell_hook(command, env, cwd, timeout)` — runs through
    `subprocess.run` with stdout/stderr to DEVNULL (D028).
- **`src/striatum/recovery/auto.py`** (new) — `run_auto_sweep` is
  a pure orchestrator over existing primitives. Sweep order:
  expire_leases → process_reconcile (when policy permits) →
  autonomous review-only requeue (D036-classified) → checkpoint
  timeout escalation → eligible-blocker doctor flag.
- **`src/striatum/workflow.py`** — validator wires
  `validate_recovery_policy` for the optional top-level
  `recovery_policy` block.
- **`src/striatum/cli/parser.py`** — `recovery auto` subparser
  with `--dry-run`, `--autonomous-review-requeue`,
  `--autonomous-process-reconcile`, `--max-requeue`,
  `--checkpoint-timeout`, `--eligible-after`, `--json`.
- **`src/striatum/cli/dispatch.py`** — wires `recovery auto`,
  reads `recovery_policy` from the run's workflow snapshot,
  resolves with CLI overrides, calls `run_auto_sweep`. New
  `json_loads` import.
- **`tests/test_recovery_auto.py`** (new, 21 cases) — policy
  validator (8 cases), hook runners (6 cases), end-to-end sweep
  (4 cases via `_drive_to_human_checkpoint`), workflow validator
  (1 case), `resolve_policy` (3 cases).
- **`docs/SPEC.md`** § Recovery — adds the `recovery auto`
  paragraph.
- **`docs/CLI_REFERENCE.md`** — adds `recovery auto`.
- **`docs/rfcs/0020-autonomous-stalled-run-recovery.md`** —
  status → `accepted (V1; step 3 deferred)`.
- **`docs/rfcs/README.md`** — index reflects new status + D066.
- **`docs/DECISION_LOG.md`** — D066 (intentionally tight: one
  sentence per cell, modeling the cleanup direction the user
  flagged for the next pass).
- **`docs/TODO.md`** — F14.
- **`pyproject.toml`** + **`src/striatum/__init__.py`** 1.4.1 →
  1.5.0.
- **`CHANGELOG.md`** 1.5.0 section.

## Verification

- `make lint` clean.
- `make typecheck` clean (56 source files, +4 from the new
  recovery package).
- `make test` — 309 passed (288 baseline + 21 new).
- Smoke-tested against `run_b0ee5ebc07ba48ddaecd22d80fd5a541`
  (this run): `striatum recovery auto --run-id <id> --dry-run`
  returns `{ok: true, data: {actions: [], escalations: [],
  still_stuck: [], policy_source: "default", dry_run: true}}`.
  No state changes.

## Notes For The Reviewer

- **D036 honored**: `is_repo_write` classifier is the gate.
  Repo-write stale work always lands in `still_stuck` with
  `reason: repo_write_requires_operator_inspection`; never in
  `actions` even with `--autonomous-review-requeue`.
- **`policy_source` field**: tells operators reading sweep logs
  whether the workflow declared a policy or inherited defaults.
  Useful when debugging "why did this sweep not act?"
- **Hook isolation**: hook runners are pure (target +
  payload/url/command in, status dict out); the orchestrator
  injects them via `hook_runner` parameter so tests can stub.
- **`recovery auto` is a mutation verb**: the service's
  `is_read_command` whitelist does not include it, so the
  `--allow-mutations` gate covers it for free. Verified by
  inspection.
- **Step 3 deferred**: `recovery watch` (long-lived daemon) is
  not in this PR. Cron + step 1 covers the overnight-stall
  case the RFC was written to address.
- **DECISION_LOG row D066**: I wrote it tight on purpose
  (one-sentence-per-cell) because the user flagged the
  cleanup pass. This shows the shape; the cleanup PR can
  retrofit older D-rows to the same form.

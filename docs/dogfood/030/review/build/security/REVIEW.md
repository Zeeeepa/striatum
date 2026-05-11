---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["security", "provenance", "rfc-0026", "rfc-0027"]
---

# Security Review — Dogfood 030 (RFC 0026 + RFC 0027 build)

status: complete
date: 2026-05-11
author: operator

## Scope

Security posture review of the shipped RFC 0026 lane-liveness attestation
scope and RFC 0027 provenance-mode honest-surfacing guardrail. Focus areas:
unattested-session byline minting, attestation bypass resistance, downgrade
behavior, patch/review digest binding (deferred-scope honesty),
`require_attested_lane` enforcement, migration safety, write-scope/
transcript posture, and documentation honesty.

Inputs reviewed:

- `src/striatum/identity.py`
- `src/striatum/artifacts.py`
- `src/striatum/cli/mutations.py` (run start, register_session, submit_review)
- `src/striatum/db.py` (`_enforce_required_attestation_for_verdict`,
  `record_review_verdict`)
- `src/striatum/workflow.py` (`_validate_provenance_mode`,
  `_validate_require_attested_lane`, `_validate_path_policy`)
- `src/striatum/migrations.py` (`_apply_v12_lane_attestation_columns`)
- `src/striatum/supervisor.py` (start path, pid_start_time capture)
- `src/striatum/cli/introspect.py` (`_provenance_mode_for_status`,
  `_session_attestation_summaries`)
- `tests/test_cli_mvp.py` (attestation + provenance assertions)
- `tests/test_supervise.py` (supervisor start records pid_start_time)
- `docs/SPEC.md`, RFC 0026, RFC 0027, dogfood-030 BUILD_HANDOFF, RFC index,
  CHANGELOG, DECISION_LOG, UBIQUITOUS_LANGUAGE, README, TODO.

## Verdict

`accept_with_findings`. The shipped surface honors the prevention-only
threat model described in RFC 0026 and the honest-mode-surfacing posture
described in RFC 0027. Bypass resistance against the in-scope threat
(good-faith operator slipping into forgery through low-friction CLI paths)
is materially better than V0; sealed-mode overclaim risk is closed by an
explicit refusal-to-start in `run_start`. Findings below are
non-blocking: one documentation-hygiene issue and two adversarial-path
test gaps in otherwise well-structured code.

## Security Invariants Verified

### Unattested sessions cannot mint lane-typed bylines

`expected_author_line` (`src/striatum/artifacts.py:569-594`) consults
`session_lane_attestation(..., mark_lost=True)` and threads `attested` into
`artifact_author_identity`, which returns `author: operator` (or
`author: operator [self-declared: <label>]`) when `attested=False`. The
publisher's `validate_optional_markdown_author_line`
(`src/striatum/artifacts.py:487-508`) then refuses any published Markdown
whose title-block or YAML front-matter `author:` line does not match the
expected operator byline, with the existing exit-code 6 error pattern.
This closes the documented frictionless slip — an unattested session that
attempts to commit `author: reviewer-codex-gpt-5.5-001` is rejected before
the artifact row is inserted, the event is emitted, or the file is
treated as durable provenance.

The same downgrade applies at verdict-read time: verdicts are not
denormalized; bylines are reconstructed via `artifact_author_identity`,
so all read surfaces (`status`, `evidence export`, run summary, dashboard,
web UI session block via `_session_attestation_summaries`) display the
unattested operator byline rather than a falsified lane byline.

### Lane-liveness attestation is bypass-resistant against the declared scope

`session_lane_attestation` (`src/striatum/identity.py:133-182`) attests
only when **all** of the following hold:

- A `process_supervisors` row exists for the session in **state=`attached`**
  (`starting`/`detached` do not qualify — V1 implementation notes call
  this out).
- The supervisor row's `run_id` and `session_id` match the session.
- The recorded `pid` is alive (`os.kill(pid, 0)` via `_pid_alive`).
- `pid_start_time` recorded at attach time matches the current Linux
  `/proc/{pid}/stat` field 22 (`process_start_time`). PID reuse with a
  different start time fails closed with `pid_identity_mismatch`.
- Platforms without `/proc/{pid}/stat` fail closed with
  `pid_identity_unavailable`.
- The supervisor row's `command_json` equals the lane command stored in
  the immutable `workflow_snapshots.workflow_json` for this run's
  `sessions.lane_id`. A supervisor whose recorded command does not match
  the snapshot's `lanes.<lane_id>.command` fails closed with
  `lane_command_mismatch`.

The pid-identity + snapshot-command binding together close the
process-substitution avenues that V0 lacked: a long-running shell session
cannot be silently re-attached as a different lane's supervisor, and PID
reuse cannot resurrect a dead supervisor's attestation.

Linkage to the supervisor start path: `supervise.start_supervised_session`
(`src/striatum/supervisor.py:80-226`) records `command_json` from the
caller-supplied launch command, transitions to `attached` only after
`process_start_time(pid)` is captured under the same transaction, and
falls back to `lost` on early-exit. The `lane_attestation` field returned
from the start RPC accurately reports `"attested"` only when
`pid_start_time` is non-NULL.

### `require_attested_lane` refuses unattested side effects on review jobs

Both gating chokepoints exist:

- `prevalidate_submit_review` (`src/striatum/cli/mutations.py:736-780`) and
  `publish_artifact`'s `_enforce_required_attestation_for_artifact`
  (`src/striatum/artifacts.py:597-612`) refuse before any artifact row is
  inserted when the workflow job sets `require_attested_lane: true` and
  the calling session is unattested.
- `record_review_verdict`'s `_enforce_required_attestation_for_verdict`
  (`src/striatum/db.py:1631-1648`) refuses before the verdict row is
  inserted, the gate is processed, or `verdict.recorded` is emitted.

Workflow validation (`_validate_require_attested_lane`,
`src/striatum/workflow.py:1345-1356`) refuses `require_attested_lane: true`
on non-review jobs in V1, matching the RFC 0026 V1 scope and avoiding the
producer-side-patch overreach the RFC flagged.

The error messages name `striatum supervise start --session-id <id>`,
which matches the recovery path described in SPEC.md.

### `provenance_mode` is named honestly and fails closed

- `_validate_provenance_mode` (`src/striatum/workflow.py:949-997`) accepts
  only the closed set `{advisory, attested_bylines, sealed_patch}`, treats
  absent as `advisory`, requires non-empty repo-relative
  `protected_paths` and `operator_writable_paths` for sealed workflows,
  rejects `..` traversal and absolute paths, refuses `.striatum/` as a
  protected source path, and rejects overlap between protected and
  operator-writable trees via `_path_prefix` symmetric check.
- `run_start` (`src/striatum/cli/mutations.py:187-196`) refuses any run
  whose snapshotted workflow declares `provenance_mode: sealed_patch`
  with a clear non-overstating error. Silent downgrade to advisory is
  explicitly disallowed and tested
  (`test_sealed_patch_mode_validates_but_refuses_to_start_without_containment`).
- Status surfaces (`_provenance_mode_for_status`,
  `src/striatum/cli/introspect.py:212-239`) read the actual snapshot
  value rather than echoing operator assertions.

No patch capture, hash-bound verdict target, apply gate, receipt signing,
key custody, or sealed local commit was shipped — and none is implied by
the runner. SPEC.md, BUILD_HANDOFF, RFC 0027 V1 implementation status,
and CHANGELOG all explicitly disclaim those guarantees.

### Migration safety

Migration v12 (`_apply_v12_lane_attestation_columns`) is forward-only,
idempotent against fresh DBs (PRAGMA `table_info` membership check),
and adds two nullable columns. Old supervisors without `pid_start_time`
fail-closed via `_inactive_reason → pid_identity_unavailable`, which is
the safe direction: pre-v12 attached supervisors are treated as
unattested rather than honoring an attestation the runner cannot
verify. Existing advisory workflows continue unchanged. The migration
preserves D006/D009 (CLI-owned writes) — no new direct-SQLite write
paths were introduced.

### Write-scope hygiene and transcript posture preserved

- Dogfood-030 jobs declare `.striatum/` under `forbidden_paths` and
  `repo_write: false` (review jobs) where appropriate.
- `_validate_path_policy` explicitly refuses `.striatum/` as a
  `protected_paths` source entry — `.striatum/` is runner state, not a
  source artifact.
- No transcript-capture hooks were introduced. D028 stance preserved.
  Lane `transcripts` adapter constraints in the workflow remain
  `"transcripts": "off"` for the advisory request and "enforced" only
  as the workflow-declared advisory constraint; no broad transcript
  capture surface ships with this change.
- `validate_artifact_front_matter` is read-only — the publisher never
  mutates files.

### Documentation honesty

SPEC.md § "Provenance Modes" is explicit that `advisory` does not
prevent direct source edits, `attested_bylines` does not prove artifact
bytes came from a model process, and `sealed_patch` is reserved for a
future hard-containment mode with `run start` refusal as the
fail-closed posture. RFC 0026 Non-Goals explicitly enumerate
adversarial-operator and cryptographic-linkage out-of-scope items. RFC
0027 V1 Implementation Status enumerates every deferred surface
(patch capture, hash-bound verdict targets, apply gate, receipts, key
management, containment mechanism, signed local commits). README and
CHANGELOG quote the same constrained guarantees.

## Findings

### F1 — `D080` decision-id collision in `docs/DECISION_LOG.md` (severity: low)

`docs/DECISION_LOG.md` now contains two rows with id `D080`: line 24
records "Accept RFC 0026 V1 plus RFC 0027 Phase 2 guardrails" (added by
this build) and line 81 records the pre-existing "Accept RFC 0024 V4:
pause/resume + per-job mutations." The highest pre-existing id is
`D082` (line 79); the new entry should have used `D083`.

Impact: documentation-traceability only. No runtime behavior depends on
decision-log ids; references in BUILD_HANDOFF, RFC 0026, RFC 0027,
CHANGELOG, and SPEC all point to "D080" but only the table is the
authoritative ledger. Because two rows now share the id, a reader who
follows a "see D080" link is left to disambiguate by date or row order,
which weakens the audit trail this RFC family is supposed to
strengthen.

Recommended fix: renumber the new row to `D083` (or the next available
id), and update the textual references in
`docs/dogfood/030/BUILD_HANDOFF.md`,
`docs/rfcs/0026-lane-attestation-and-operator-byline-honesty.md`,
`docs/rfcs/0027-sealed-patch-provenance-mode.md`,
`docs/SPEC.md` (none currently cite D080 by id), and `CHANGELOG.md` if
applicable. Non-blocking; a docs-only follow-up PR is sufficient.

### F2 — Missing adversarial test for `pid_identity_mismatch` (severity: low)

`session_lane_attestation` returns `attested=False` with
`reason="pid_identity_mismatch"` when the recorded `pid_start_time` does
not equal the current process's start token, defending against PID
reuse. `tests/test_supervise.py:test_supervise_status_marks_lost_when_pid_disappears`
covers the simpler `pid_gone` path, but no test exercises the PID-reuse
identity-mismatch path directly. Because this is the core defense
against the "long-running shell session impersonates a dead supervisor"
threat, a direct test (e.g. mutate `process_supervisors.pid_start_time`
to a known-wrong value and assert `attested=False`
plus `mark_lost=True` transitions the row to `lost`) would lock the
invariant down.

Non-blocking — the code path is small, well-typed, and visited by
`_inactive_reason` whenever attestation is consulted.

### F3 — Missing adversarial test for `lane_command_mismatch` (severity: low)

Same shape as F2 for `_session_lane_command`: no test exercises the
"supervisor command does not match the snapshotted lane command" path
that defends against supervising the wrong binary under a session whose
operator-asserted lane claims a model the supervisor is not running.
Recommended fix: a test that registers an attached supervisor whose
`command_json` is rewritten to a different command list, then asserts
attestation is `unattested` with `reason="lane_command_mismatch"`.

Non-blocking — the comparison is structural (`json.loads` of stored
command vs the workflow snapshot's command list) and the surrounding
code paths are tested at a higher level.

## Issues Not Found

- No path was found by which an unattested session could publish an
  artifact with a lane-typed byline. The byline is derived at publish
  time via `expected_author_line` → `session_lane_attestation` and
  enforced by `validate_optional_markdown_author_line`. Both the file
  byline and the recorded `artifacts.author_line` column reflect what
  was actually published, not the workflow's expected line (HARNESS-003
  byline integrity).
- No path was found by which `provenance_mode: sealed_patch` could
  silently downgrade to advisory at run start.
- No path was found by which `require_attested_lane: true` could be
  bypassed for `publish-artifact`, `submit-review`, or `verdict`. All
  three gates call the same attestation check and refuse before side
  effects.
- No path was found by which an operator-declared `--operator-label`
  could shape itself into an attested-looking byline.
  `validate_operator_label` blocks `^[a-z0-9._-]{1,64}$` violations,
  reserved attestation words, active lane ids, and the
  `<role>-<model>-<ordinal>` shape via `ATTESTED_BYLINE_BODY_RE`. Tests
  `test_register_session_rejects_deceptive_operator_labels` cover the
  documented adversarial cases.
- No change to D028 (no transcript capture) and no widening of the
  artifact write surface. The publisher remains read-only against files.
- No change to D006/D009 (CLI-owned SQLite writes). All new writes are
  routed through existing mutation paths.

## Recommendation

Accept the implementation with the three low-severity findings recorded
above. None blocks a downstream consumer of the RFC 0026/0027 V1
surface, and none represents an overclaim of provenance guarantees in
the runner, schema, status surfaces, or docs. F1 should be cleaned up
in a follow-up docs commit; F2 and F3 are useful belt-and-suspenders
tests for the adversarial-path code that already exists.

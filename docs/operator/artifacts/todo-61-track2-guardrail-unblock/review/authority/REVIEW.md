---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
tags: ["authority_boundary", "daemon_authority", "legacy_sqlite_quarantine", "todo-61-track2-guardrail-unblock"]
---

# Authority-Boundary Review: todo-61-track2-guardrail-unblock

author: reviewer-claude-code-001

## Scope and Posture

Fresh-context, document-only review under posture
`custom:authority_boundary`. Objective: verify that the guardrail unblock
preserves daemon/PostgreSQL authority, does not reopen production SQLite,
and does not decide TODO 55, 56, 59, or 60.

Documents reviewed:

- `docs/operator/artifacts/todo-61-track2-guardrail-unblock/build/authority_guardrails/HANDOFF.md`
- `docs/operator/artifacts/todo-61-track2-guardrail-unblock/build/recovery_evidence/HANDOFF.md`
- `docs/operator/artifacts/todo-61-track2-test-debt/map/MAP.md`
- `docs/operator/artifacts/todo-61-track2-test-debt/build/guardrail/HANDOFF.md`
- `docs/operator/artifacts/todo-61-track2-test-debt/review/authority/REVIEW.md`
- `docs/operator/artifacts/todo-61-track2-test-debt/review/regression/REVIEW.md`
- `docs/architecture/COMMAND_AUTHORITY_MATRIX.md`
- `AGENTS.md`, `docs/SPEC.md`, `docs/TODO.md`, `docs/ROADMAP.md`,
  `docs/operator/BRIEF.md`, `docs/HOW_TO_AGENT.md`,
  `docs/rfcs/0068-go-production-daemon-port.md`,
  `docs/rfcs/0069-pg-only-daemon-global-surfaces.md`,
  `README.md`.

No source files, run reports, decision-log entries, or other artifacts
were consulted.

## Overall Verdict

`accept` — info severity. The two handoffs in this packet close the F1
finding from the prior test-debt authority review and the F1/F3 findings
from the regression review without touching production module code,
without restoring SQLite as live-state authority, and without deciding
any of the four blocked product TODOs.

The guardrail-unblock work is the correct authority-direction move for
RFC 0068 / RFC 0069: it activates a previously-disabled architecture
test, brings `COMMAND_AUTHORITY_MATRIX.md` into sync with the active
Go daemon contract, and tightens (not loosens) the legacy-SQLite
allowlist. No findings rise above informational hygiene notes.

## Authority-Boundary Assessment

### Daemon/PostgreSQL Authority Is Preserved And Strengthened

- `authority_guardrails/HANDOFF.md:5-10` removes the module-level
  `pytest.skip("legacy sqlite eradicated", ...)` from
  `tests/architecture/test_authority_guardrails.py`, so the command
  authority guardrails run again. This is the
  unblock the test-debt MAP and authority review both called for
  (`todo-61-track2-test-debt/map/MAP.md:35-47`,
  `todo-61-track2-test-debt/review/authority/REVIEW.md:122-150`).
  Re-enabling the guardrail strengthens the boundary by making
  command-authority drift detectable in CI again.
- `authority_guardrails/HANDOFF.md:10-26` introduces a narrow
  `GO_ONLY_DAEMON_METHODS` classification for eight active Go-only
  daemon contract methods that lacked classification once the test
  ran: `artifact.backfill_blob`, `artifact.get_content`,
  `artifact.list_for_run`, `corpus.fetch_historical_dogfood_file`,
  `corpus.list_historical_dogfood_files`,
  `corpus.list_historical_dogfoods`,
  `corpus.migrate_historical_dogfood_file`, and `work.await_packet`.
  The matrix already documents these methods with explicit
  `pg / go / sqlite_dependency` columns showing
  Python-not-implemented + Go-real + no SQLite dependency
  (`COMMAND_AUTHORITY_MATRIX.md:71-167`). Classifying them keeps
  the test honest about RFC 0068's Python-daemon-retirement reality
  without granting any new authority to non-daemon code paths.
- The new matrix rows for the blob, historical-dogfood, and
  `work.await_packet` methods land alongside their existing
  cohort entries (read methods at `read` capability, mutations at
  `write` or `claim`, all `scope=single_repo`, all `Go=real`,
  all `SQLite dependency=no`). This is consistent with the
  daemon-authoritative posture in `AGENTS.md` Product Boundary and
  `docs/SPEC.md`, and with the
  COMMAND_AUTHORITY_MATRIX legend that names `pg`/`real` as the
  production authority columns.
- The handoff records the deletion of the
  `src/striatum/legacy_sqlite/repo_local_migration.py` entry from the
  direct-PostgreSQL bootstrap allowlist
  (`authority_guardrails/HANDOFF.md:18-23`). Because the file no longer
  exists in the tree, the allowlist entry was stale; removing it
  *tightens* the allowlist surface rather than expanding any escape
  hatch.

### Legacy SQLite Stays Quarantined; No Production SQLite Is Reopened

- The authority-guardrail change does not touch
  `src/striatum/` production code; it only edits
  `tests/architecture/test_authority_guardrails.py` and
  `docs/architecture/COMMAND_AUTHORITY_MATRIX.md`. Verification
  (`authority_guardrails/HANDOFF.md:28-32`) is limited to architecture
  and ruff checks.
- The `GO_ONLY_DAEMON_METHODS` set is described as "intentionally
  narrow" with the explicit forward-pointer that "future daemon
  methods still need an explicit matrix row and classification when
  they are added"
  (`authority_guardrails/HANDOFF.md:36-38`). This matches RFC 0068's
  posture of treating new contract methods as visible authority
  decisions rather than absorbing them under a blanket exemption.
- `recovery_evidence/HANDOFF.md:5-16` narrows the legacy-SQLite skip in
  `tests/daemon_pg/handlers/recovery_evidence/conftest.py` from
  module-level `allow_module_level=True` to a fixture-scoped skip on
  the `sqlite_conn` fixture only. The handoff explicitly notes that the
  docstring is corrected so it "no longer claims the whole
  recovery-evidence parity rig is live SQLite/PG parity." This closes
  F3 from the regression review without restoring SQLite as a
  production authority: PG-only tests now collect, and the legacy
  SQLite dependency is only opted into when a test explicitly requests
  the `sqlite_conn` fixture. The two newly-visible stale assertion
  failures in `test_cancelable_states` and
  `test_process_adapter_blocker_kinds`
  (`recovery_evidence/HANDOFF.md:31-41`) are correctly flagged as
  out-of-scope for this packet's write scope rather than papered over.

### TODO 55, 56, 59, 60 Are Not Decided

- The handoff explicitly states "No TODO 55, 56, 59, or 60 decision was
  made." (`authority_guardrails/HANDOFF.md:26`). The recovery-evidence
  handoff makes no reference to any of those TODOs either.
- Cross-checking against `docs/TODO.md`: TODO 55 is RFC 0064 workflow
  risk lint / review diversity (accepted-risk persistence), TODO 56 is
  the auto-finalize default policy, TODO 59 is RFC 0066
  replay/archive/corpus v2, TODO 60 is RFC 0067 optional Git/PR
  integration. None of these surfaces is touched by edits to the
  architecture guardrail test, the recovery-evidence conftest, or the
  command authority matrix. The two changed test files exercise
  guardrail enforcement and PG handler coverage respectively, and the
  matrix edits document existing Go contract methods rather than
  proposing or accepting any new policy.
- The new matrix rows align with active Go daemon handlers already
  named in `COMMAND_AUTHORITY_MATRIX.md` (`Registered Daemon Methods`
  table, lines 71-173) and with the RFC 0068 / RFC 0069 trajectory
  documented in `docs/rfcs/0068-go-production-daemon-port.md` and
  `docs/rfcs/0069-pg-only-daemon-global-surfaces.md`. They do not
  introduce any new product capability.

## Findings

No blocking findings. Two informational hygiene notes follow; neither
changes the verdict.

### N1. Forward-pointer to per-handler matrix-row hygiene

- Severity: info
- Tags: `documentation_hygiene`, `command_authority_matrix`
- Evidence: `authority_guardrails/HANDOFF.md:36-38` notes that the
  `GO_ONLY_DAEMON_METHODS` classification is intentionally narrow and
  future daemon methods still need an explicit matrix row plus
  classification when they land. The matrix already has executable
  guardrails in
  `tests/architecture/test_authority_guardrails.py` and
  `go/cmd/striatumd/handler_coverage_test.go`
  (`COMMAND_AUTHORITY_MATRIX.md:17-24`).
- Why this is info: the policy is restated correctly inside the
  handoff; nothing about this packet defeats it. Future contributors
  adding daemon methods should keep both layers — Python guardrail
  classification and matrix row — in lockstep, as already required.

### N2. Forward-pointer to the two newly-visible stale recovery-evidence assertions

- Severity: info
- Tags: `documentation_hygiene`, `recovery_evidence`,
  `forward_pointer`
- Evidence: `recovery_evidence/HANDOFF.md:31-41` flags two failing
  assertions outside this packet's write scope:
  `test_cancel_job.py::test_cancelable_states` (expects only
  `pending`, `claimed`, `blocked`; current handler also includes
  `queued`, `running`, `stale_lease`, `waiting_human`) and
  `test_resume_blocker.py::test_process_adapter_blocker_kinds`
  (expects the older process blocker kind set; current handler
  exposes the expanded post-cutover set).
- Why this is info: the handoff correctly leaves these outside the
  conftest-narrowing write scope, which preserves the authority
  boundary of the current packet. They are pre-existing handler/test
  drift, not regressions introduced here. They should be addressed in
  a follow-up Track 2 batch under the recovery-evidence handler scope.

## Out-of-Scope (Not Reviewed)

- I did not read the on-disk bodies of
  `tests/architecture/test_authority_guardrails.py`,
  `tests/daemon_pg/handlers/recovery_evidence/conftest.py`, or any
  source module. The review posture is `document_only` and `fresh`;
  divergence between the handoff claims and on-disk test bodies should
  surface from a regression or build-test track rather than from this
  verdict.
- I did not consult run-state, the decision log, ledgers, or session
  artifacts beyond the documents listed above.

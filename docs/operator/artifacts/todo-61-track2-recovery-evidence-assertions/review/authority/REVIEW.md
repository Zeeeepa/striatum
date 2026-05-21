---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
tags: ["authority_boundary", "daemon_authority", "recovery_evidence", "todo-61-track2-recovery-evidence-assertions"]
---

# Authority-Boundary Review: todo-61-track2-recovery-evidence-assertions

author: reviewer-claude-code-001

## Scope and Posture

Fresh-context, document-only review under posture
`custom:authority_boundary`. Objective: verify that the recovery-evidence
assertion cleanup preserves daemon/PostgreSQL authority, does not restore
production SQLite, keeps changes bounded to recovery-evidence tests and
handoff artifacts, and does not decide TODO 55, 56, 59, or 60.

Documents reviewed:

- `docs/operator/artifacts/todo-61-track2-recovery-evidence-assertions/build/resume_blocker/HANDOFF.md`
- `docs/operator/artifacts/todo-61-track2-recovery-evidence-assertions/build/cancel_job/HANDOFF.md`
- `docs/operator/artifacts/todo-61-track2-guardrail-unblock/build/recovery_evidence/HANDOFF.md`
- `docs/operator/artifacts/todo-61-track2-guardrail-unblock/review/regression/REVIEW.md`
- `docs/operator/artifacts/todo-61-track2-guardrail-unblock/review/authority/REVIEW.md`
- `docs/operator/artifacts/todo-61-track2-test-debt/map/MAP.md`
- `AGENTS.md`, `docs/SPEC.md`, `docs/TODO.md`, `docs/ROADMAP.md`,
  `docs/operator/BRIEF.md`, `docs/HOW_TO_AGENT.md`,
  `docs/INDEX.md`, `docs/UBIQUITOUS_LANGUAGE.md`, `docs/DECISION_LOG.md`,
  `docs/rfcs/0068-go-production-daemon-port.md`,
  `docs/rfcs/0069-pg-only-daemon-global-surfaces.md`,
  `README.md`.

No source files, run reports, decision-log entries, or other artifacts
were consulted.

## Overall Verdict

`accept` — info severity. The two build handoffs in this packet close
the two stale-assertion forward-pointers (N2) recorded by the prior
guardrail-unblock authority review without touching production handler
code, without restoring SQLite as live-state authority, and without
deciding any of the four blocked product TODOs.

The cleanup is the correct authority-direction follow-up to the
guardrail-unblock packet: with the recovery-evidence conftest narrowed
from a module-level SQLite skip to a fixture-scoped skip, the two
previously-hidden tests now had to assert against the current
daemon/PostgreSQL handler contract or fail. The handoffs report
asserting against the current contract.

## Authority-Boundary Assessment

### Daemon/PostgreSQL Authority Is Preserved And Reaffirmed

- `resume_blocker/HANDOFF.md:6-22` updates
  `tests/daemon_pg/handlers/recovery_evidence/test_resume_blocker.py` so
  the `PROCESS_ADAPTER_BLOCKER_KINDS` assertion matches the active Go
  daemon handler's expanded post-cutover blocker set
  (`process_outputs_missing`, `process_review_verdict_missing`,
  `process_exit_nonzero`, `process_timeout_exceeded`,
  `process_lost_with_outputs_missing`), and the
  `PROCESS_EXIT_BLOCKER_KINDS` assertion matches the active
  exit-only subset (`process_exit_nonzero`, `process_timeout_exceeded`).
  This realigns the test with the daemon/PG handler reality flagged in
  the prior guardrail-unblock packet
  (`todo-61-track2-guardrail-unblock/build/recovery_evidence/HANDOFF.md:37-41`,
  `todo-61-track2-guardrail-unblock/review/authority/REVIEW.md:170-187`).
- `cancel_job/HANDOFF.md:6-19` updates
  `tests/daemon_pg/handlers/recovery_evidence/test_cancel_job.py` so the
  `test_cancelable_states` assertion matches the active daemon
  `recovery.cancel_job` contract: `blocked`, `queued`, `claimed`,
  `running`, `stale_lease`, `waiting_human`. This is the same expanded
  set the prior guardrail-unblock handoff identified as exposed when
  the conftest skip was narrowed
  (`todo-61-track2-guardrail-unblock/build/recovery_evidence/HANDOFF.md:34-36`).
- Both handoffs explicitly note "No production handler code was
  changed." (`resume_blocker/HANDOFF.md:31`,
  `cancel_job/HANDOFF.md:21-22`). The authority direction here is
  test-to-handler: the daemon contract was already authoritative and
  the tests were stale; this packet aligns the tests, not the other
  way around. That preserves the AGENTS.md Product Boundary in which
  the daemon-owned PostgreSQL instance is the authoritative live
  state and tests verify that contract rather than dictate it.

### Legacy SQLite Stays Quarantined; No Production SQLite Is Reopened

- Both handoffs report only test-file edits inside
  `tests/daemon_pg/handlers/recovery_evidence/`. No `src/striatum/`
  module, no production schema, no migration, and no live-state
  authority surface was touched.
- The docstring corrections described in each handoff move the changed
  files away from SQLite-parity language. `resume_blocker/HANDOFF.md:20-22`
  adjusts the file docstring and the `--complete` refusal test docstring
  so they describe the current PG contract rather than SQLite parity;
  `cancel_job/HANDOFF.md:20-21` changes the module docstring "from
  SQLite parity language to current daemon/PostgreSQL contract
  language." This continues the authority-direction trend started in
  the prior packet
  (`todo-61-track2-guardrail-unblock/build/recovery_evidence/HANDOFF.md:14-16`),
  which removed the conftest's SQLite-parity framing.
- The lingering `1 skipped` line in `resume_blocker/HANDOFF.md:39-40`
  is consistent with the fixture-scoped `sqlite_conn` skip that the
  guardrail-unblock packet preserved as the only legitimate legacy
  SQLite opt-in path
  (`todo-61-track2-guardrail-unblock/review/authority/REVIEW.md:111-124`).
  No new module-level SQLite skip is reintroduced, and no test claims
  SQLite as a live-state authority.
- The MAP for Track 2 keeps `tests/architecture/test_legacy_sqlite_quarantine.py`
  active and called for tightening
  (`todo-61-track2-test-debt/map/MAP.md:33,45-47`); this packet does
  not touch the quarantine guardrail or its allowlist, so the
  legacy-SQLite scanner posture is unchanged.

### Changes Are Bounded To Recovery-Evidence Tests And Handoff Artifacts

- The combined edit surface reported by the two handoffs is exactly:
  `tests/daemon_pg/handlers/recovery_evidence/test_resume_blocker.py`
  and `tests/daemon_pg/handlers/recovery_evidence/test_cancel_job.py`,
  plus the two new HANDOFF.md artifacts under
  `docs/operator/artifacts/todo-61-track2-recovery-evidence-assertions/build/`.
  Both files sit inside the recovery-evidence test directory the prior
  packet unblocked.
- Neither handoff reports touching production code, the
  COMMAND_AUTHORITY_MATRIX, the guardrail tests, the conftest, or any
  shared fixture. The cleanup is per-file and per-assertion.
- The cross-handoff verification math is internally consistent: the
  prior packet reported the recovery-evidence shard at 40 passed /
  1 skipped / 2 failed
  (`todo-61-track2-guardrail-unblock/build/recovery_evidence/HANDOFF.md:26-29`);
  the `cancel_job` handoff after its fix reports 41 passed / 1 skipped
  / 1 failed (`cancel_job/HANDOFF.md:30`); the `resume_blocker` handoff
  after its fix reports 42 passed / 1 skipped
  (`resume_blocker/HANDOFF.md:39-40`). The two stale-assertion
  forward-pointers from N2 of the prior authority review are closed
  with no remaining failures in that shard.

### TODO 55, 56, 59, 60 Are Not Decided

- Neither handoff references any of these TODOs. Per the prior
  authority review's cross-check against `docs/TODO.md`
  (`todo-61-track2-guardrail-unblock/review/authority/REVIEW.md:126-146`),
  TODO 55 is RFC 0064 workflow risk lint / review diversity, TODO 56
  is the auto-finalize default policy, TODO 59 is RFC 0066
  replay/archive/corpus v2, and TODO 60 is RFC 0067 optional Git/PR
  integration. None of these surfaces is touched by an assertion
  update inside two recovery-evidence handler tests.
- The cleanup neither proposes nor adopts any new policy. It only
  brings two test files into alignment with the active Go daemon
  handler contract documented in
  `docs/rfcs/0068-go-production-daemon-port.md` and
  `docs/rfcs/0069-pg-only-daemon-global-surfaces.md`. No product
  capability is introduced or retired.

## Findings

No blocking findings. Two informational hygiene notes follow; neither
changes the verdict.

### N1. Task-prompt path mismatch is a workflow-scaffold note, not an authority issue

- Severity: info
- Tags: `documentation_hygiene`, `workflow_scaffolding`
- Evidence: `resume_blocker/HANDOFF.md:24-29` notes that the work
  packet's `task_prompt.path` pointed at
  `prompts/fix_resume_blocker_assertions.md`, which is not in the
  repo, and that the implementer used the matching prompt at
  `docs/operator/workflows/todo-61-track2-recovery-evidence-assertions/prompts/fix_resume_blocker_assertions.md`
  instead. The cancel_job handoff makes no equivalent note, implying
  the same resolution applied silently.
- Why this is info: this is a workflow-scaffold pointer error in the
  packet, not an authority-boundary breach. The implementer documented
  the substitution, used the correct prompt body, and did not
  improvise scope. Future Track 2 packets should emit `task_prompt`
  paths under the workflow's `prompts/` directory or copy the prompts
  into the top-level `prompts/` tree, but that is operator-side
  hygiene rather than an authority decision.

### N2. Forward-pointer to broader recovery-evidence assertion drift discipline

- Severity: info
- Tags: `documentation_hygiene`, `recovery_evidence`,
  `forward_pointer`
- Evidence: the two assertions corrected here had drifted from the
  current daemon handler contract while hidden behind the prior
  conftest module-level skip
  (`todo-61-track2-guardrail-unblock/build/recovery_evidence/HANDOFF.md:31-41`).
  The same risk exists wherever else module-level skips currently
  quarantine handler-level tests: assertions can drift unnoticed once
  visibility is restored.
- Why this is info: this packet's bounded scope is correct, and the
  Track 2 MAP already calls for narrowing remaining module-level
  skips
  (`todo-61-track2-test-debt/map/MAP.md:35-47`). The general
  recommendation is to keep follow-up Track 2 unblock packets paired
  with assertion-cleanup packets so newly-exposed tests do not stay
  red in CI between batches. This does not change the verdict of this
  packet.

## Out-of-Scope (Not Reviewed)

- I did not read the on-disk bodies of
  `tests/daemon_pg/handlers/recovery_evidence/test_resume_blocker.py`
  or
  `tests/daemon_pg/handlers/recovery_evidence/test_cancel_job.py`, or
  any source module. The review posture is `document_only` and
  `fresh`; divergence between the handoff claims and on-disk test
  bodies should surface from a regression or build-test track rather
  than from this verdict.
- I did not consult run-state, the decision log, ledgers, or session
  artifacts beyond the documents listed above.
- I did not verify the `pytest` counts reported in either handoff;
  the assessment of internal consistency between the prior shard
  numbers and the two new shard numbers is a documentary cross-check,
  not a re-run.

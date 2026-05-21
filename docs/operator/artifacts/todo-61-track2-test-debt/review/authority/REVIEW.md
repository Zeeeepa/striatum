---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["authority_boundary", "daemon_authority", "legacy_sqlite_quarantine", "todo-61-track2-test-debt"]
---

# Authority-Boundary Review: todo-61-track2-test-debt (first batch)

author: reviewer-claude-code-001

## Scope and Posture

Fresh-context, document-only review of the first TODO 61 Track 2 batch
under posture `custom:authority_boundary`. The objective is to verify
that the batch preserves daemon/PostgreSQL authority, keeps legacy SQLite
isolated as explicit fixtures only, and does not decide TODO 55, 56, 59,
or 60.

Documents reviewed:

- `docs/operator/artifacts/todo-61-track2-test-debt/map/MAP.md`
- `docs/operator/artifacts/todo-61-track2-test-debt/build/cli_mvp/HANDOFF.md`
- `docs/operator/artifacts/todo-61-track2-test-debt/build/artifact_process/HANDOFF.md`
- `docs/operator/artifacts/todo-61-track2-test-debt/build/service/HANDOFF.md`
- `docs/operator/artifacts/todo-61-track2-test-debt/build/guardrail/HANDOFF.md`

Boundary references read for context only:

- `AGENTS.md`, `docs/SPEC.md`, `docs/TODO.md`, `docs/operator/BRIEF.md`,
  `docs/HOW_TO_AGENT.md`, `docs/operator/artifacts/todo-61-62-cleanup/plan/CLEANUP_PLAN.md`,
  `docs/operator/artifacts/todo-61-62-cleanup/final/SUMMARY.md`,
  `docs/rfcs/0068-go-production-daemon-port.md`,
  `docs/rfcs/0069-pg-only-daemon-global-surfaces.md`.

No source files, run reports, or other artifacts were consulted.

## Overall Verdict

`accept_with_findings` — low severity. The four batch handoffs and the
synthesis map are consistent with RFC 0068/0069 and the daemon-authority
boundary documented in `AGENTS.md` and `docs/SPEC.md`. Legacy SQLite is
demoted to explicit per-test or per-fixture quarantines with named
reasons, no production module is touched, and the four blocked product
TODOs (55, 56, 59, 60) are not decided. The single low-severity finding
is a forward-pointer about a residual authority-guardrail skip the MAP
called out but this batch did not close.

## What the Batch Looks Like Across the Authority Boundaries

### Daemon/PostgreSQL Authority Is Preserved

- The MAP recommends converting publish, service, and adapter coverage
  to daemon handler / PG harness coverage, and quarantining only
  intentionally historical SQLite assertions
  (`map/MAP.md:25-33`, `map/MAP.md:50-59`).
- `cli_mvp/HANDOFF.md:5-19` removes the broad module-level
  `pytest.skip("legacy sqlite eradicated", ...)` and the
  `striatum.legacy_sqlite.db` import, then leaves current CLI/workflow
  authoring tests visible. It explicitly states the remaining skipped
  tests "should be converted in later focused batches to daemon/PostgreSQL
  handler or CLI-dispatch coverage instead of restoring production SQLite
  authority" (`cli_mvp/HANDOFF.md:33-37`). That is the correct authority
  direction for RFC 0068.
- `artifact_process/HANDOFF.md:9-19` reports that current front-matter,
  workflow validator, fail-closed adapter-run, diagnostic-envelope, and
  lane-environment assertions now run, while historical SQLite
  adapter-run and migration fixtures are quarantined per test. No
  production SQLite path is restored.
- `service/HANDOFF.md:5-17` converts `tests/test_service.py` to active
  daemon-routed unit coverage; legacy-package imports are removed and
  the SQLite tripwire is now achieved by patching stdlib `sqlite3.connect`
  directly, "so daemon-routed service handlers still fail if they attempt
  any SQLite open without depending on the retired package." The
  per-fixture skip reason
  `"historical repo-local SQLite service fixture quarantined after Go/PG
  cutover"` names the boundary it sits behind.
- `guardrail/HANDOFF.md:5-25` updates
  `tests/architecture/test_legacy_sqlite_quarantine.py` to fail the
  architecture test if any of the four converted files reintroduces
  imports of `striatum.legacy_sqlite`, `striatum.db`, or
  `striatum.migrations`, or restores a broad module-level
  `pytest.skip(..., allow_module_level=True)`. The stdlib `sqlite3`
  tripwire is narrowed the same way.

### Legacy SQLite Is Reduced To Explicit Fixtures

- All four converted files now hide legacy SQLite dependencies at the
  granularity of individual test cases with named skip reasons rather
  than at the module level. The pattern matches RFC 0068 §SQLite
  Eradication and RFC 0069's "explicit one-way import fixture" posture
  rather than a supported compatibility mode.
- The guardrail records residual legacy SQLite fixture imports outside
  this batch in an explicit future-batch allowlist
  (`guardrail/HANDOFF.md:21-37`) covering recovery evidence fixtures,
  RFC 0043 split-brain exit-code coverage, corpus/export tests,
  dashboard/web tests, skill install tests, supervision tests, and
  worktree isolation tests. That allowlist makes the remaining debt
  visible rather than blanket-approved, which is the authority-boundary
  outcome RFC 0068 asks for.
- No handoff reports any production-module change. The `service`
  handoff explicitly notes "No production service fallback policy,
  architecture guardrail, or other test file was changed in this slice"
  (`service/HANDOFF.md:27-29`). The other three handoffs stay inside
  their named test file plus, for the guardrail batch, the architecture
  guardrail file.

### TODO 55, 56, 59, 60 Are Not Decided

- TODO 55 (RFC 0064 risk lint + review diversity, accepted-risk
  persistence), TODO 56 (auto-finalize default policy), TODO 59
  (RFC 0066 replay/archive/corpus v2), and TODO 60 (RFC 0067 optional
  Git/PR integration) are listed as blocked or partial in
  `docs/TODO.md`. None of the five batch documents references those
  TODOs, RFCs, or their underlying product decisions. The batch scope is
  strictly the four named test files plus the architecture quarantine
  guardrail.

## Findings

### F1. MAP-flagged authority-guardrail skip not closed in this batch

- Severity: low
- Tags: `authority_boundary`, `architecture_guardrail`,
  `forward_pointer`
- Evidence: `map/MAP.md:35-47` lists
  `tests/architecture/test_authority_guardrails.py:3` as also carrying a
  module-level `pytest.skip("legacy sqlite eradicated", ...)` and
  states: "The authority guardrail skip is outside the four requested
  conversion files but is part of the architecture guardrail surface.
  It currently disables the command-authority drift checks entirely, so
  it should be removed or narrowed before treating Track 2 as complete."
  None of `cli_mvp/HANDOFF.md`, `artifact_process/HANDOFF.md`,
  `service/HANDOFF.md`, or `guardrail/HANDOFF.md` reports touching that
  file. The guardrail handoff updates only
  `tests/architecture/test_legacy_sqlite_quarantine.py`.
- Why this is low rather than higher: the four files explicitly named
  for this batch are all converted, the legacy-SQLite quarantine
  guardrail is actively enforcing on those files, and the residual
  allowlist makes the broader gap visible. The authority-guardrails
  module-level skip predates this batch and is preserved, not
  introduced, here. It is the next obvious Track 2 step rather than a
  regression in the current slice.
- Suggested follow-up (not blocking this verdict): close the
  `tests/architecture/test_authority_guardrails.py` module-level skip in
  the next Track 2 batch, or narrow it to a named per-test quarantine
  with an explicit reason mirroring the convention adopted in the four
  converted files. That closes the command-authority drift surface the
  MAP called out.

### F2. Per-test quarantine reasons should remain consistent

- Severity: info
- Tags: `documentation_hygiene`, `legacy_sqlite_quarantine`
- Evidence: `service/HANDOFF.md:15-18` adopts a single named skip reason
  (`"historical repo-local SQLite service fixture quarantined after
  Go/PG cutover"`). The artifact/process and CLI MVP handoffs do not
  report the exact skip-reason wording they used. This is not a
  boundary regression — all three describe per-test (not module-level)
  quarantines — but future Track 2 batches should converge on a single
  named reason vocabulary so the architecture guardrail can eventually
  tighten on the reason string. Pure documentation/hygiene note, no
  action required for this batch.

## Out-of-Scope (Not Reviewed)

- I did not read the converted test files themselves. The review posture
  is `document_only` and `fresh`, so any divergence between the handoff
  claims and the on-disk test bodies is not in scope for this verdict
  and should surface from the regression-review track.
- I did not consult run-state, decision-log, or session artifacts beyond
  the documents listed above.

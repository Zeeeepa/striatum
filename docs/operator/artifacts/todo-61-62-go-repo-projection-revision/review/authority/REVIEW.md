---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["authority_boundary", "daemon_authority", "product_boundary", "operator_boundary", "todo-61-62-go-repo-projection-revision"]
---

# Authority-Boundary Review: todo-61-62-go-repo-projection-revision
author: reviewer-claude-code-001

## Scope and Posture

Fresh-context, document-only review of the state-path projection revision
that closes F1 from the regression-risk review of TODO 61-62 cleanup. The
"Go repo projection" framing in the job title refers to aligning the
Python repository projection with the directory-shaped projection that
the Go daemon and Python MCP already emit, not to a change in Go code.

Posture is `custom:authority_boundary`. The objective is to verify that
the fix:

1. Preserves daemon-owned PostgreSQL as the live-state authority.
2. Does not reopen repo-local SQLite as a production behavior path.
3. Does not decide the blocked TODO 55, 56, 59, or 60 product questions.

Inputs reviewed:

- `docs/operator/artifacts/todo-61-62-cleanup/build/HANDOFF.md`
- `docs/operator/artifacts/todo-61-62-cleanup/review/regression/REVIEW.md`
- `docs/operator/artifacts/todo-61-62-cleanup-revision/build/HANDOFF.md`
- `docs/operator/artifacts/todo-61-62-cleanup-revision/review/REVIEW.md`
- `docs/TODO.md`
- `docs/ROADMAP.md`
- `docs/architecture/COMMAND_AUTHORITY_MATRIX.md`
- `docs/rfcs/0068-go-production-daemon-port.md`
- `docs/rfcs/0069-pg-only-daemon-global-surfaces.md`
- `AGENTS.md`

No other artifacts, ledgers, run reports, or source files were consulted.

## Overall Verdict

`accept` — low severity. As represented in the input document set, the
revision is a scope-disciplined output normalization that preserves all
three boundaries under review. The single observation below is
forward-pointing hygiene, not an authority regression.

## Why The Revision Holds The Authority Boundaries

### 1. Daemon / PostgreSQL authority is preserved

The cleanup-revision handoff and its existing regression-risk review
both state the fix is output normalization only:

- `repo_list_pg` and `repo_resolve_pg` now serialize repository rows
  through a shared `_repository_projection` helper in
  `src/striatum/daemon_pg/repositories.py`, with the same helper covering
  the duplicate-registration return from `repo_add_pg`.
- The stored `striatumd.repositories.state_db_path` column is not
  rewritten. Older registry rows keep their historical value; the
  helper substitutes the `.striatum/` operational scratch directory
  in the projection that leaves Python clients.

This matches the authority shape required by RFC 0068 (Go is the
production daemon core; Python remains acceptable as a client) and
RFC 0069 (daemon-global surfaces are PostgreSQL-backed and fail closed
rather than opening SQLite). The CLI/RPC clients that the F1 finding
called out as divergent now see the same directory-shaped projection
the Go daemon and Python MCP already emit, which is the alignment
asked for in `docs/operator/artifacts/todo-61-62-cleanup/review/
regression/REVIEW.md` § F1 and § Recommendations.

`docs/architecture/COMMAND_AUTHORITY_MATRIX.md` shows `repo.list`,
`repo.resolve`, and `repo.add` continuing to route through PG-native
handlers with `real` Go authority and no CLI fallback; the revision
does not change any registry method, capability, scope, fallback
classification, or check id, which is why the revision handoff
correctly states it required no authority-matrix update.

### 2. No reopened repo-local SQLite production behavior

The revision handoff explicitly says the stored PG row is not
rewritten and no migration history is touched. The existing review
adds two independent confirmations from the same input set:

- `STRIATUM_SQLITE_CONNECT_TRIPWIRE=1` regression coverage in
  `tests/daemon_pg/test_repo_registration.py` and
  `tests/test_mcp_capability_scope_e2e.py` confirms no SQLite
  connection is opened during repository listing/resolution or MCP
  resource reads — i.e., the projection helper does not even
  speculatively touch a `.striatum/retired-local-state` file when one is
  named in the legacy column.
- `src/striatum/service_command_policy.py` has been further
  restricted to require `STRIATUM_LEGACY_SERVICE_FIXTURE=1` for the
  legacy test-harness escape. That is a tightening, not a loosening,
  of the production fallback boundary RFC 0068 / 0069 require.

These match the COMMAND_AUTHORITY_MATRIX § Direct PostgreSQL
Bootstrap/Admin Plane invariants and the RFC 0068 acceptance criterion
that "production daemon/client paths do not open repo-local SQLite
or the legacy daemon registry". The cleanup-revision keeps SQLite
strictly inside the named one-way import / fixture / quarantine
modules already enumerated in `docs/TODO.md` items 61-62 and
`docs/ROADMAP.md` § 4.5 / § 4.6.

### 3. Blocked TODO product questions remain undecided

The revision handoff states plainly: "I did not restore the deleted
legacy package, reopen SQLite import windows, or decide the blocked
TODO 55, 56, 59, or 60 product questions." The existing
regression-risk review of the revision reinforces this under
§ F3 Scope Discipline: no implementation work was performed on
Track 2 (test debt), Track 3 (corpus/templates), or blocked TODO
55, 56, 59, or 60.

Cross-checking against `docs/TODO.md` and `docs/ROADMAP.md`:

- **TODO 55 / RFC 0064 Phase 7** — accepted-risk persistence remains
  blocked on a product decision about the durable authority surface
  (decision artifact linkage, daemon audit row, workflow metadata,
  or another explicit home). The revision touches no policy-decision
  surface (no validator behavior change, no `workflow lint` shape
  change, no `--accepted-risk-decision-id` semantics).
- **TODO 56 / Phase 8** — default auto-finalize policy remains
  blocked on live-dogfood confidence. The revision touches no
  `recovery.auto_finalize` code path, no workflow opt-in surface,
  and no PG evidence `publish_origin` semantics.
- **TODO 59 / RFC 0066 Phase 11** — Corpus Contract V2 fields remain
  blocked on RFC 0057 decisions. The revision touches no corpus
  exporter, archive, or replay surface.
- **TODO 60 / RFC 0067 Phase 12** — optional Git/PR integration
  remains blocked on a product decision on commit authority and
  hosted-provider boundaries. The revision touches no provider,
  apply, or hosted-integration surface.

The revision's footprint (`src/striatum/daemon_pg/repositories.py`
projection helper + a dedicated regression test in
`tests/daemon_pg/test_repo_registration.py`) is mechanically incapable
of deciding any of those four product questions.

## Findings

### F1 (low) — F2 regression-review residuals remain open after this revision

This is informational, not a regression of the revision under review.
The original `docs/operator/artifacts/todo-61-62-cleanup/review/
regression/REVIEW.md` carried four findings; this revision exists to
close F1 (structural projection divergence) and the existing review
confirms it does so. Per the revision handoff and the existing
revision review, F2 (Track 2 test debt — 69 imports of
`striatum.legacy_sqlite` across 45 test files; quarantine guardrail
coverage for `tests/`), F3 (Track 3 corpus export still hardcoding
`legacy_sqlite_fixture` / `PRAGMA user_version`), and F4 (stale
`docs/architecture/COMMAND_AUTHORITY_MATRIX.md` rename note) are
still open.

None of those are authority-boundary regressions; they are the broader
RFC 0068 / RFC 0069 residuals already named in `docs/TODO.md` items 61
("Go default; Python daemon module deleted; broad direct repo-local
fixture opens converted") and 62 ("guardrail residuals only"). They
also align with `docs/ROADMAP.md` § 1 ("remaining SQLite eradication
is legacy compatibility module, migration/import fixture, and
in-memory unit-fixture cleanup") and § 5.9 step 13.

Operator follow-up suggestion: when the next cleanup or revision
ships, fold the F2 quarantine guardrail extension to `tests/` and
the F4 matrix-rename refresh into that delta so the original review's
ledger closes cleanly.

### F2 (info) — Output normalization is a deliberate, non-rewriting choice

The revision normalizes the projection rather than rewriting the
underlying `striatumd.repositories.state_db_path` value. That is
consistent with RFC 0068 § Non-Goals (no permanent dual-core
divergence; SQLite is allowed only as one-way migration fixture
material) and RFC 0069 § Acceptance Criteria (fail closed when a
PostgreSQL DTO is missing; no SQLite fallback). It also preserves the
auditability of how older repositories were registered, which the
revision handoff calls out explicitly ("migration history and older
registry rows remain intact").

This is the right modeling choice for an output projection; a
back-fill of the stored column would be a separate write-authority
decision and would correctly belong in a future revision under
explicit operator scope, not in this cleanup.

## What I Looked For And Did Not Find

- **Daemon authority bypass.** No new client opens daemon PostgreSQL
  outside the `Direct PostgreSQL Bootstrap/Admin Plane` entries
  already curated in the authority matrix; the revision changes only
  the projection inside the already-allowed daemon PG repository
  helpers.
- **Reopened SQLite production fallback.** No production fallback is
  added; the revision retains the existing
  `STRIATUM_SQLITE_CONNECT_TRIPWIRE` regression coverage and the
  tightened `STRIATUM_LEGACY_SERVICE_FIXTURE` requirement is a
  narrowing, not a widening.
- **New `api.invoke` or CLI-shaped MCP tool.** Not introduced; the
  revision footprint is constrained to `daemon_pg/repositories.py`
  and a single test file, and the existing revision review confirms
  no new RPC method, capability, or check id ships.
- **Product-boundary creep.** No hosted services, telemetry, or
  external persistence introduced; the revision is internal Python
  client output shaping over an existing PG row.
- **Operator-boundary regression.** The fix does not change how
  operators invoke commands, how `.striatum/` is treated as
  operational scratch, or how `escalation` / `decision` artifacts are
  produced; D103/D104/D107 operator semantics are untouched.
- **Decision on TODO 55/56/59/60.** Confirmed above; the revision's
  surface area cannot touch any of those product questions.

## Suggested Disposition

`accept`. The revision closes F1 from the regression-risk review with
a surgical, well-scoped projection helper, preserves daemon-owned
PostgreSQL authority, does not reopen repo-local SQLite production
behavior, and does not decide any of the blocked TODO 55, 56, 59, or
60 product questions. The single forward-pointing observation about
the remaining F2 / F3 / F4 residuals from the original regression
review is hygiene, not an authority-boundary regression, and is
already tracked under `docs/TODO.md` items 61-62 and
`docs/ROADMAP.md` § 5.9.

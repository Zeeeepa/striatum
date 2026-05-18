# RFC 0071: Operator Diagnostics and Cutover Evidence

Status: implemented
Date: 2026-05-17
Context: [RFC 0043](0043-postgres-as-sole-substrate-and-daemon-required-runtime.md), [RFC 0058](0058-operator-progress-surface.md), [RFC 0068](0068-go-production-daemon-port.md), [RFC 0069](0069-pg-only-daemon-global-surfaces.md), [RFC 0070](0070-daemon-client-service-boundary.md)

## Problem

After the P0/P1 authority cleanup, operators still need concise evidence that
the daemon/PostgreSQL cutover is complete and that remaining exceptions are
bounded. The current docs and guardrails are useful, but they require reading
multiple files and test names.

## Goals

- Add cutover evidence that is useful after, not before, RFC 0069 and RFC 0070.
- Make migration success and legacy quarantine visible without reopening direct
  SQLite paths.
- Keep generated or diagnostic tooling subordinate to the source contract.

## Non-Goals

- Build archive/replay corpus V2 before RFC 0057 decisions land.
- Persist accepted workflow-lint risks before the durable authority decision.
- Implement hosted Git/PR mutation integration.

## Proposal

The implemented diagnostic slice provides:

1. `daemon doctor --repo <path> --authority --json` detail with a structured
   report confirming PG rows, event-chain anchoring, tombstone state, and
   absence of production SQLite opens.
2. `doctor --authority --json` daemon doctor detail that reports live-state
   authority by surface.
3. A curated daemon method authority matrix with stricter drift tests; D108
   rejected full generation for the current slice.

## Acceptance Criteria

- Operators can run one command to verify a repository's PG cutover state.
- The report names migration/test-only SQLite exceptions separately from
  production authority.
- Authority diagnostics fail closed when daemon/PostgreSQL is unavailable.
- Documentation points operators to diagnostics without implying direct DB
  inspection is acceptable.

## Implementation Notes

- D108 resolves the generated-versus-curated authority-matrix question:
  `docs/architecture/COMMAND_AUTHORITY_MATRIX.md` remains curated for
  authority/status classification, while generated contract tables and
  executable architecture tests own drift-prone facts.
- `striatum daemon doctor --authority --json` now emits
  `striatum.authority_report.v1` with PostgreSQL status, legacy SQLite
  registry status, daemon method fallback counts, allowed migration/test-only
  SQLite exceptions, and remediation recommendations.
- The daemon doctor treats legacy SQLite registry access as retired production
  behavior. Any registry details in diagnostics are quarantine/fixture
  evidence, not a live fallback.
- `striatum daemon doctor --repo <path> --authority --json` now emits a
  verify-only `striatum.repo_cutover_report.v1` with repository registration,
  migration checkpoint, destination counts, raw source/tombstone/sentinel
  state, event-chain anchor health, and bounded SQLite exception notes. The
  verifier does not open SQLite as a database.
- The same command includes the report in doctor output and includes a
  repository-cutover summary in `striatum.authority_report.v1`.
- `tests/architecture/test_authority_guardrails.py` now verifies that every
  CLI route label from `contracts/daemon_methods.json` appears in the curated
  command matrix and that CLI fallback cells match the runtime daemon RPC
  route map.

## Open Questions

None for the accepted diagnostic slice. Future diagnostics may extend the
report, but the generated-versus-curated matrix policy and daemon-doctor
repository mirror are settled for RFC 0071 V1.

## Domain Modeling

This RFC adds diagnostic projections over existing aggregate state. It does
not add a new source of truth.

# RFC 0071: Operator Diagnostics and Cutover Evidence

Status: partially implemented
Date: 2026-05-17
Context: [RFC 0043](0043-postgres-as-sole-substrate-and-daemon-required-runtime.md), [RFC 0058](0058-operator-progress-surface.md), [RFC 0069](0069-pg-only-daemon-global-surfaces.md), [RFC 0070](0070-daemon-client-service-boundary.md)

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

After RFC 0069 and RFC 0070 land:

1. Extend `daemon migrate-repo-local` or add `repo verify-cutover` with a
   structured report confirming PG rows, event-chain anchoring, tombstone
   state, and absence of production SQLite opens.
2. Add `doctor --authority --json` or equivalent daemon doctor detail that
   reports live-state authority by surface.
3. Generate the stable daemon method authority matrix from
   `contracts/daemon_methods.json` plus checked-in override metadata, or keep
   the current hand-maintained matrix with stricter drift tests if generation
   proves too costly.

## Acceptance Criteria

- Operators can run one command to verify a repository's PG cutover state.
- The report names migration/test-only SQLite exceptions separately from
  production authority.
- Authority diagnostics fail closed when daemon/PostgreSQL is unavailable.
- Documentation points operators to diagnostics without implying direct DB
  inspection is acceptable.

## Implementation Notes

- `striatum daemon doctor --authority --json` now emits
  `striatum.authority_report.v1` with PostgreSQL status, legacy SQLite
  registry status, daemon method fallback counts, allowed migration/test-only
  SQLite exceptions, and remediation recommendations.
- The daemon doctor treats a production-disabled legacy SQLite registry as the
  expected post-cutover state when PostgreSQL doctor is healthy.
- `striatum daemon migrate-repo-local --from sqlite --to pg --repo <path>
  --verify-cutover --json` now emits `striatum.repo_cutover_report.v1` with
  repository registration, migration checkpoint, destination counts, raw
  source/tombstone/sentinel state, event-chain anchor health, and bounded
  SQLite exception notes. The verifier does not open SQLite as a database.

## Open Questions

- Should repository-specific cutover verification also be mirrored in daemon
  doctor, or is the migration command diagnostic sufficient?
- How much of `docs/architecture/COMMAND_AUTHORITY_MATRIX.md` should be
  generated versus curated?

## Domain Modeling

This RFC adds diagnostic projections over existing aggregate state. It does
not add a new source of truth.

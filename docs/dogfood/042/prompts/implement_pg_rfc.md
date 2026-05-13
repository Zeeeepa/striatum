# Track C Implement: Draft RFC 0042 Body (codex)

Blocked until `review_pg_design` returns an accepting verdict.

Write `docs/rfcs/0042-repo-local-state-to-postgres.md` from `docs/dogfood/042/track_c/DESIGN_SYNTHESIS.md`.

The RFC 0042 body must include:

- Status: `proposed`
- Context links to D006, D007, D028 (superseded), D082, D086, D087, D088, D093 (encoded in parallel session by operator), RFC 0028, RFC 0030, RFC 0033, RFC 0039
- Problem (per-repo `.striatum/state.sqlite3` silos; daemon-first product shape per D082; multi-repo schema)
- Goals (acceptance-criteria-bearing)
- Non-Goals (NO user-level auth changes; D083 single-user trust unchanged; no cross-machine; no bundled PG)
- Proposal (detailed enough for a future dogfood to implement):
  - Schema changes (add `repo_id` to every repo-local table)
  - Composite keys
  - New CLI verb: `striatum daemon migrate-repo-local --from sqlite --to pg --repo <path> [--keep-sqlite-readonly] [--dry-run]`
  - `.striatum/` directory becomes operational scratch only
  - Daemon mandatory; CLI refuses cleanly without daemon
  - RFC 0039 scope revision: Go daemon as gateway for ALL repo-local ops from day 1
  - Migration ordering + rollback
  - Audit chain integrity preserved
- Acceptance Criteria (concrete bullets)
- Implementation Plan
- D006/D007/D028 supersession statement
- Open Questions
- Domain Modeling

**Do NOT update `docs/rfcs/README.md`, `docs/TODO.md`, `CHANGELOG.md`, or `docs/DECISION_LOG.md`** — the `consolidate_phase_1` job handles those.

D093 (encoded in parallel session) is the explicit supersession decision-log entry; reference it but do not modify the decision log here.

**Use sub-agents aggressively**: spawn one sub-agent per RFC section (Context, Problem, Goals, Non-Goals, Proposal subsections — schema, CLI verb, `.striatum/` role, daemon-mandatory, RFC 0039 scope revision, migration ordering, audit chain — Acceptance Criteria, Implementation Plan, Open Questions, Domain Modeling) so they run in parallel. Reconcile into the final RFC body yourself. Dispatch additional sub-agents to enumerate every repo-local table in `src/striatum/repo_state/sql/*.sql` and every CLI verb that touches repo-local state, so the schema migration list and CLI behavior section are exhaustive.

Output: `docs/rfcs/0042-repo-local-state-to-postgres.md` + `docs/dogfood/042/track_c/build/HANDOFF.md`.

One-shot supervised invocation. If `striatum ack` is denied, write the artifacts and exit normally.

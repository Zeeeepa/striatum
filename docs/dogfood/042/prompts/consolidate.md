# Phase 1 Consolidation Prompt (codex)

Runs after all three tracks complete their build reviews. Produces the cross-cutting doc updates that no single track owned, plus the combined BUILD_HANDOFF for the multi-phase dogfood.

**Your scope:**

- `docs/rfcs/README.md` — add entries for RFC 0042 (repo-local PG) and RFC 0044 (Engram Phase 1). Bump RFC 0039 status to `accepted (V1 Steps 1+2 implemented)` if Track A's build review accepts.
- `docs/TODO.md` — mark progress on items 20, 21 (RFC 0040 V1.5, RFC 0038 V1.5 still pending) + add follow-up entries for Phase 2 (Go daemon Steps 3-6) and for the future RFC 0042 V1 implementation, RFC 0044 V1 implementation.
- `CHANGELOG.md` — Unreleased entries:
  - Track A: Go daemon Steps 1+2 landed in `go/` (Phase 1 partial).
  - Track B: RFC 0044 drafted (Engram Phase 1 implementation spec).
  - Track C: RFC 0042 drafted (repo-local state to Postgres).
- `docs/dogfood/042/BUILD_HANDOFF.md` — combined handoff covering all three tracks. Read each track's HANDOFF.md (`track_a/build/systems/HANDOFF.md`, `track_a/build/glue/HANDOFF.md`, `track_b/build/HANDOFF.md`, `track_c/build/HANDOFF.md`) and synthesize.
- `docs/dogfood/042/PHASE_1_OPERATOR_NOTES.md` — operator-narrative section that the human OPERATOR_REPORT can pull from (the operator writes OPERATOR_REPORT.md per D091 incrementally; these notes are a summary of what shipped per track).

Cite the Track A/B/C build review verdicts in the BUILD_HANDOFF. If any review returned `needs_revision` or `reject`, surface that visibly (the operator decides whether to override per dogfood-040/041 patterns).

**Use sub-agents aggressively**: spawn one sub-agent per output file (RFC README update, TODO update, CHANGELOG update, BUILD_HANDOFF, PHASE_1_OPERATOR_NOTES) so they run in parallel. Spawn additional sub-agents to read each track's HANDOFF.md and per-track build-review verdicts so the synthesis is grounded.

**Do NOT touch**:

- Other RFC body files (Track B owns RFC 0044; Track C owns RFC 0042).
- The track-specific dogfood subdirs (those are owned by track jobs).
- `docs/DECISION_LOG.md` (D093 was encoded by the parallel session; do not modify here).

Output: the listed files. Plain markdown line byline.

One-shot supervised invocation. If `striatum ack` is denied, write the artifacts and exit normally.

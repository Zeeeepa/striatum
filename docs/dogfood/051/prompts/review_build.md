# Build Review — RFC 0039 V1.6

Read:
- `docs/dogfood/051/DESIGN_SYNTHESIS.md`
- `docs/dogfood/051/build/HANDOFF.md`
- The source files the HANDOFF cites (`go/pkg/supervisor/`,
  `go/pkg/db/`, `go/go.mod`, `.github/workflows/ci.yml`).

Posture is supplied in your work packet (`threat_model`,
`ergonomics_dx`, or adversarial). Write to your assigned
`docs/dogfood/051/review/build/<lane>/REVIEW.md` with v1 finding front
matter.

Required checks:
- F-pty: PTY path no longer returns sentinel; `creack/pty` dep added
  cleanly.
- F-pid-recycling: start-time check is read from a real source
  (`/proc/<pid>/stat`), not just timestamp comparison.
- F-perms: pidfile + scratch dir created with 0600/0700.
- F-store: Postgres-backed `PointerStore` round-trips upsert + get +
  mark-lost.
- F-ci: Go binary verification step fails when binary absent.

Findings should cite file:line. Verdict: `accept`,
`accept_with_findings`, or `needs_revision`.

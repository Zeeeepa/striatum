# Track A Implement: Go Core (codex)

Blocked until `review_go_design` returns an accepting verdict.

Implement Steps 1+2 from `docs/dogfood/042/track_a/DESIGN_SYNTHESIS.md`. **You write Go code; claude writes Python glue + docs.** Do NOT cross into Python scope.

**Your scope (codex Go-side):**

- `go/cmd/striatumd/main.go` — daemon entry point.
- `go/pkg/rpc/{envelope,registry,capability,server}.go` — envelope-v1 wire protocol, method registry, capability table, RPC server framework.
- `go/pkg/db/{connection,migrations,audit}.go` — Postgres connection pool, migration loader (reads from `src/striatum/daemon_pg/sql/*.sql` or embeds), audit-chain hash helper.
- `go/go.mod` + `go/go.sum`.
- `go/Makefile` — `build`, `test`, `lint` targets.
- `docs/dogfood/042/track_a/build/systems/HANDOFF.md` summarizing your shipped scope.

Use sub-agents aggressively for parallelizable Go work (one sub-agent per `go/pkg/<package>/`).

**Do NOT write to**: anything outside your `allowed_paths`. Specifically not `tests/_harness/`, `src/striatum/`, `docs/HOW_TO_HUMAN.md`, `docs/SPEC.md`, etc.

Verification: `cd go && make test` passes. Daemon binary builds with `go build ./cmd/striatumd`. Read-only RPC verbs work end-to-end against a test Postgres.

This is a one-shot supervised invocation. Do not ask follow-ups. If `striatum ack` is denied, write the HANDOFF and exit normally.

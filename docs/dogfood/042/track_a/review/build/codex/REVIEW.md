---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["threat_model", "rfc-0039", "go-daemon", "build", "track_a"]
---

author: reviewer-codex-gpt-5.5-003

# Track A Build Review: Go Daemon Steps 1+2

Verdict: needs_revision.

Trust boundaries reviewed: the Go daemon's local Unix socket boundary, envelope-v1 request parsing, capability-token authorization, daemon-owned PostgreSQL audit state, harness daemon-core selection, and the Go module/build surface. Attack surfaces reviewed: unauthenticated RPC calls, cross-repo capability scope checks, audit-chain append integrity under concurrent RPC requests, Postgres URL and toolchain handling, and the harness path that is supposed to prove Go/Python daemon parity.

## Findings

### F1. Go RPC authorization is fail-open in the daemon entrypoint

Severity: high.

`go/cmd/striatumd/main.go:50` constructs the server and `go/cmd/striatumd/main.go:53` installs `rpc.AllowAllAuthorizer{}`. That authorizer returns `Decision: "allowed"` for every method regardless of the supplied token in `go/pkg/rpc/capability.go:25`. As a result, every non-hello method that reaches `Server.Handle` bypasses the capability table entirely, including repository-scoped read routes and future mutating routes. This violates RFC 0030's capability-bound method registry posture and directly crosses the daemon socket trust boundary: possession of a local socket connection becomes sufficient authority.

The fix should wire a PostgreSQL-backed authorizer that validates token hash, revocation, expiry, required capability, and repository scope before any route dispatch. Keep `AllowAllAuthorizer` test-only or remove it from production construction.

### F2. `daemon_core="go"` cannot launch through the harness

Severity: high.

`tests/_harness/daemon.py:117` invokes the Go binary with `--db-url` and `--migrations-dir`, but `go/cmd/striatumd/main.go:24` defines `--socket`, `--postgres-url`, `--migrate`, and `--describe`; there is no `--db-url` or `--migrations-dir`. Even before that, `tests/_harness/daemon.py:23` expects the default binary at `go/bin/striatumd`, while `go/Makefile:3` runs plain `go build ./cmd/striatumd`, which emits `go/striatumd` rather than `go/bin/striatumd`. The default Go-core path therefore fails unless the operator supplies a custom `STRIATUMD_GO_BIN`, and even then flag parsing fails.

This blocks the RFC 0039 Step 1+2 acceptance path that says `MultiRepoHarness(daemon_core="go")` can boot the Go daemon. Align the binary output path and the harness flags with the Go CLI, then add a smoke test that actually instantiates `MultiRepoHarness(daemon_core="go")`.

### F3. The Go parity target is declared but not wired

Severity: high.

RFC 0039 says `make test-multi-repo CORE=go` should run the RFC 0035 harness against the Go core, but the top-level `Makefile` has no `CORE` plumbing for `test-multi-repo`, and `tests/conftest.py:16` constructs `MultiRepoHarness(...)` without passing `daemon_core`. I found no pytest parametrization for the Go core. The current suite therefore continues to exercise the Python daemon only, while the RFC status text claims the harness "gained a daemon_core parameter so e2e fixtures can target either core."

This leaves the Go daemon outside the acceptance evidence. Add a parametrized fixture or an explicit Go-core test target and make it run at least the Step 1+2 smoke/read-only assertions. If the intent is only to land the constructor field in this dogfood, the RFC status text should say that Go-core CI parity is still deferred.

### F4. The Go audit append helper can break the hash chain under concurrent RPC calls

Severity: medium.

`go/pkg/db/audit.go:57` reads the current chain head in one database command, computes the next row hash in process, then `go/pkg/db/audit.go:87` appends the row and updates `audit_chain_head` in a separate command. There is no transaction or row-level lock around "read previous hash -> insert row -> update chain head." Two concurrent RPC requests can read the same previous hash, append two rows with identical `previous_hash`, and leave a chain that fails verification in `src/striatum/daemon_pg/audit.py:51`.

This should reuse a single database-side helper or perform the append inside one transaction that locks `audit_chain_head` before reading it. The threat model should treat the audit chain as a synchronization boundary, not only a hash-format compatibility check.

### F5. Postgres access shells out to ambient `psql`

Severity: medium.

`go/pkg/db/connection.go:110` and `go/pkg/db/connection.go:119` execute `psql` from `PATH` for every DB command. That makes the daemon depend on an unpinned external executable that is not represented in `go.mod` or `go.sum`, weakens the single-binary distribution claim, and creates an environment-injection surface for tests or operators with a polluted `PATH`. The Go module currently has no third-party dependencies and an empty `go.sum`, which is good for module supply-chain review, but the runtime database dependency has moved outside Go's module integrity model.

Use a Go Postgres driver or constrain this explicitly as a temporary test-only implementation with a tracked follow-up. Production daemon DB access should not depend on arbitrary `psql` resolution.

## Verification

I ran `make -C go test`; it passed for `go/cmd/striatumd`, `go/pkg/db`, and `go/pkg/rpc`. I did not run the multi-repo harness against `daemon_core="go"` because the launch path is mechanically inconsistent as described above.

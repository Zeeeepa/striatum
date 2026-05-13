# Implement Track A: RFC 0039 Phase 2 — CLI integration + mutating verbs in Go (codex)

Blocked until `review_design` returns an accepting verdict.

Implement Track A per `docs/dogfood/049/DESIGN_SYNTHESIS.md`. **You write Go (under `go/`) plus a narrow slice of Python (`src/striatum/cli/daemon.py` + `src/striatum/cli/parser.py`).** Sister Track B (supervisor + distribution + CI, claude) runs in parallel — do not cross into its write scope.

**Your scope (codex):**

- `src/striatum/cli/parser.py` — add the `--core {python,go}` flag to the `daemon start` subparser. Honor `STRIATUM_DAEMON_CORE` env var as the default value source. Default is `python` (no implicit flip).
- `src/striatum/cli/daemon.py` — branch on `args.core` to launch either the Python daemon (existing path) or the Go binary. Resolve the Go binary in this order: shipped wheel binary (sister Track B owns `src/striatum/_daemongo/` resolver — call it via the public interface they define, or via lazy import that gracefully handles absent-on-disk during parallel dev), `STRIATUMD_GO_BIN` env override, `go/bin/striatumd` for in-tree dev. Subprocess launch via `os.execv` or `subprocess.Popen` per the synthesis decision. The Python CLI client continues to speak RFC 0030 envelope-v1 over the Unix socket regardless of daemon language.
- `go/pkg/rpc/registry.go` — register every mutation in `src/striatum/cli/mutations.py` per RFC 0043 §5 table with the same capability binding as `src/striatum/daemon_rpc/registry.py`: `session.register`, `work.claim_next` / `ack` / `heartbeat` / `complete` / `block` / `release`, `artifact.publish`, `review.submit` / `verdict`, `decision.record`, `checkpoint.resolve`, `recovery.requeue_stale` / `cancel_job` / `resume`, `worktree.create`, `branch.confirm`, `run.prepare` / `start` / `pause` / `resume` / `cancel`, `workflow.validate` / `generate`. Each method writes its audit row via `go/pkg/db/audit.go` (already landed in V1.5 F4).
- `go/pkg/apply/{receipt.go,service.go}` — apply receipt schema + signing-key wiring + fail-closed authority semantics mirroring `src/striatum/daemon_apply/{apply_service,signing_key}.py`. RFC 0031 threat model preserved.
- `go/pkg/mcp/{capabilities.go,tools.go}` — RFC 0032 MCP capability-gated `tools/call` + `tools/list` filter + audit row append. Mirror `src/striatum/daemon_rpc/mcp.py`.
- `go/pkg/crossrepo/{prepare.go,lifecycle.go}` — RFC 0032 cross-repo run lifecycle. Mirror `src/striatum/daemon_rpc/multi_repo.py`.
- `go/cmd/striatumd/main.go` — wire the new registry / apply / mcp / crossrepo packages into the serving daemon (the existing main currently only wires read-only routes).
- `go/go.mod` + `go/go.sum` — any new Go runtime dependencies (e.g. additional jackc/pgx subpackages, MCP-related libs). If Track B's HANDOFF arrives mid-flight with a creack/pty require line, fold it here.
- `tests/test_daemon_go_mutations.py` — Python harness end-to-end: `MultiRepoHarness(daemon_core="go")` exercising the mutation surface (claim, ack, publish, complete, verdict, recovery). Mirror existing `tests/test_daemon_go_*.py` shape.
- `tests/cli/`, `tests/daemon_rpc/` — `--core go` parser tests, `STRIATUM_DAEMON_CORE` env precedence tests, daemon-launch dispatch tests.
- `go/pkg/{rpc,apply,mcp,crossrepo}/*_test.go` — Go unit tests for each cluster.
- `docs/dogfood/049/build/track_a/HANDOFF.md` — handoff summarizing shipped scope, files touched, test results (both `go test ./...` and `make test`), deviations from the synthesis (if any) with one-line rationale.

**Use sub-agents aggressively** — one per concern, dispatched in parallel:

- Sub-agent CLI integration: parser flag + dispatch hook + subprocess launcher + binary resolver.
- Sub-agent RPC registry expansion: cluster A (claim: session.register, work.*), cluster B (write: artifact.publish, decision.record, checkpoint.resolve), cluster C (review: review.submit, review.verdict), cluster D (run admin: run.*, workflow.*), cluster E (recovery: recovery.*, worktree.create, branch.confirm). Each cluster gets its own sub-agent.
- Sub-agent apply service: receipt schema + signing-key + service dispatcher.
- Sub-agent MCP: capabilities filter + tools/call + tools/list + audit append.
- Sub-agent cross-repo: prepare + lifecycle.
- Sub-agent Go unit tests: per-package `*_test.go`.
- Sub-agent Python harness end-to-end: `tests/test_daemon_go_mutations.py` + CLI tests.

Reconcile sub-agent outputs yourself before writing HANDOFF.

**Do NOT touch**: `go/pkg/supervisor/`, `go/Makefile`, top-level `Makefile`, `.github/workflows/`, `src/striatum/_daemongo/`, `src/striatum/cli/dispatch.py`, `src/striatum/cli/mutations.py`, `src/striatum/daemon.py`, `src/striatum/daemon_supervisor/` (sister Track B owns those, plus shared dispatcher/mutations stay stable). **Do NOT write to**: README / TODO / CHANGELOG / RFC index / SPEC / HOW_TO. Operator handles those manually after the dogfood lands.

**Backward-compat (non-negotiable)**: the Python daemon must keep working. `daemon_core` parameter defaults to `python`. `--core go` is opt-in only. Existing test fixtures continue to pass against `daemon_mode=on` and `daemon_core="python"`.

**D094 framing**: per RFC 0043 Postgres is the sole substrate and the daemon is required. The Go daemon implements RFC 0030 over the **same Postgres schema** as the Python daemon. The two cores are mutually exclusive at runtime via pidfile + socket-path lock.

Verification: `make lint`, `make typecheck`, `make test` all pass for the Python slice. `cd go && go build ./... && go test ./...` clean. `make test-multi-repo CORE=go` exercises the mutation surface against an ephemeral Postgres.

One-shot supervised invocation. Do not ask follow-ups. If `striatum ack` is denied, write the HANDOFF and exit normally.

## Byline discipline

The work packet supplies an exact `author: <slug>` line. Copy it verbatim. Plain Markdown line, lowercase `author:`, NO bold, NO italics, NO lane prefix. Slug shape: `implementer-unknown-model-<NN>`.

# Design Prompt: RFC 0039 Phase 2 — Go daemon Steps 3-6

Produce DESIGN.md at the path your work packet specifies (under `docs/dogfood/049/design/<lane>/`).

Read `docs/rfcs/0039-go-daemon-core.md` first — focus on §Implementation Plan Steps 3-6, §Acceptance Criteria, §V1.5 Deltas (what already landed in dogfood-047), and the Phase 1 status block at the top of §Implementation Plan. Also skim:

- `docs/rfcs/0030-daemon-rpc-server-and-version-skew-protocol.md` — envelope-v1, version handshake, method registry.
- `docs/rfcs/0031-daemon-owned-supervision-and-sealed-apply-boundary.md` — supervisor metadata, apply-receipt schema.
- `docs/rfcs/0032-cross-repo-workflows-and-mcp-mutation-capabilities.md` — MCP capability gating + cross-repo lifecycle.
- `docs/rfcs/0033-storage-substrate-rewrite-for-daemon-v2.md` — daemon Postgres schema, audit chain.
- `docs/rfcs/0035-multi-repo-test-harness-for-cross-repo-workflows.md` — RFC 0035 harness shape.
- `docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md` — RFC 0043 §5 mutation registry table (the list of methods Track A must register on the Go core).
- `docs/dogfood/047/PHASE_1_OPERATOR_NOTES.md` — what Phase 1 / V1.5 actually shipped, the codex F1-F5 findings, the codex/codex anti-pattern history (5 instances) + the codex-reviewer-of-claude-implementer pattern (D099 + D101).
- The existing `go/` tree — `go/cmd/striatumd/main.go`, `go/pkg/rpc/{registry,server,auth_pg,envelope,capability}.go`, `go/pkg/db/{audit,connection,migrations}.go`.
- `src/striatum/daemon.py` (foreground supervision, process boot, signal handling), `src/striatum/daemon_supervisor/` (supervisor pointers), `src/striatum/daemon_apply/` (apply service), `src/striatum/daemon_rpc/{registry,mcp,multi_repo,server,capability}.py` (RPC server + MCP + cross-repo), `src/striatum/cli/{daemon,parser,dispatch,mutations}.py`.
- `tests/_harness/daemon.py`, `tests/conftest.py`, `tests/test_daemon_go_*.py`.

Design the implementation across **two tracks**:

**Track A — CLI integration + mutating verbs in Go (codex Go):**

- Wire `striatum daemon start --core go` in `src/striatum/cli/daemon.py` + `src/striatum/cli/parser.py`. Honor `STRIATUM_DAEMON_CORE` env var; resolve binary via `STRIATUMD_GO_BIN` override or `go/bin/striatumd` default (matching `tests/_harness/daemon.py`). The Python CLI client continues to speak RFC 0030 envelope-v1 over the Unix socket regardless of daemon language.
- Extend `go/pkg/rpc/registry.go` to register every mutation in `src/striatum/cli/mutations.py` per RFC 0043 §5 table: `session.register`, `work.claim_next` / `ack` / `heartbeat` / `complete` / `block` / `release`, `artifact.publish`, `review.submit` / `verdict`, `decision.record`, `checkpoint.resolve`, `recovery.requeue_stale` / `cancel_job` / `resume`, `worktree.create`, `branch.confirm`, `run.prepare` / `start` / `pause` / `resume` / `cancel`, `workflow.validate` / `generate`. Capability binding matches `src/striatum/daemon_rpc/registry.py` exactly.
- Land `go/pkg/apply/{receipt.go,service.go}` mirroring `src/striatum/daemon_apply/` (apply receipt schema + fail-closed authority semantics per RFC 0031).
- Land `go/pkg/mcp/{capabilities.go,tools.go}` mirroring `src/striatum/daemon_rpc/mcp.py` — RFC 0032 capability-gated `tools/call` + `tools/list` filter + audit row append.
- Land `go/pkg/crossrepo/{prepare.go,lifecycle.go}` mirroring `src/striatum/daemon_rpc/multi_repo.py` (RFC 0032 cross-repo run lifecycle).
- Tests: `go test go/pkg/{rpc,apply,mcp,crossrepo}/...` for unit coverage; Python-side `tests/test_daemon_go_mutations.py` exercising `MultiRepoHarness(daemon_core="go")` end-to-end across the mutation surface.

**Track B — Supervisor + distribution + CI (claude Go + Python harness):**

- Land `go/pkg/supervisor/{pointer.go,liveness.go,pty.go}` mirroring `src/striatum/daemon.py` supervisor + `src/striatum/daemon_supervisor/` with `os/exec` + `creack/pty`. Implement start, attach, heartbeat from supervised-progress signal, packet delivery via FIFO (byte-compatible with the Python wrapper protocol — read the Python wrapper code under `.striatum/bin/*-supervised-wrapper.sh` or its source generator under `src/striatum/`), stop, lost-detection via pidfile + supervisor pointer in Postgres, deterministic SIGTERM cleanup using a signal channel + waitgroup drain.
- Distribution: cross-compile `linux-amd64`, `linux-arm64`, `darwin-amd64`, `darwin-arm64` binaries in CI. Top-level `Makefile` gains `daemon-go-build` / `daemon-go-install` / `daemon-go-release` targets; `go/Makefile` produces `go/bin/striatumd<-platform>`.
- Package-data shim under `src/striatum/_daemongo/` that ships the per-platform Go binary inside the Python wheel (binary-payload pattern like `psycopg[binary]`). CLI binary resolver: shipped wheel binary first, then `STRIATUMD_GO_BIN` override, then `go/bin/striatumd` for in-tree dev. `pyproject.toml` package-data + `MANIFEST.in` updates as needed.
- CI matrix `daemon_core={python,go}` on Linux + macOS: extend `.github/workflows/` with two explicit jobs per OS (`CORE=python`, `CORE=go`) running `make test-multi-repo` against ephemeral Postgres. Wire `tests/test_daemon_go_supervisor.py` covering supervised lane start, packet delivery, heartbeat, lost-detection, SIGTERM cleanup.

Cover concretely per track: exact file paths, function-level entry points (cite current names from the existing Go tree and Python source), capability mapping per RPC method, FIFO packet schema fields, CI job names, wheel package-data layout, test paths. Cite the RFC 0030/0031/0032 patterns being mirrored. Hand-waving "we add a method" without a pinpoint citation is grounds for review to bounce.

**Backward-compat (non-negotiable)**: the Python daemon must keep working. `daemon_core` parameter defaults to `python`. Step 6 only adds the `--core go` opt-in path; flipping the default to `go` is a separate future RFC per RFC 0039 §9 Phase 2 (post-this-dogfood).

**D094 framing (non-negotiable)**: per RFC 0043 the daemon is the sole substrate and Postgres is the sole substrate. The Go daemon implements RFC 0030 over the **same Postgres schema** as the Python daemon. There is no SQLite path, no per-language substrate, no parallel daemon code path. The two cores are mutually exclusive in a given run (per RFC 0039 §3) — pidfile + socket-path lock prevents concurrent daemons.

Out of scope: rewriting the Python CLI in Go; Windows daemon support; multi-machine / hosted-mode daemon (D083); cryptographic non-repudiation on the apply path (RFC 0031 threat model preserved); Prometheus metrics; flipping the `--core go` default. README / TODO / CHANGELOG / SPEC / HOW_TO updates are operator-only after the dogfood lands.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim. Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes, NO lane prefix. Lowercase `author:`. Slug shape: `<role>-unknown-model-<NN>`.

One-shot supervised invocation. Write the artifact directly. If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.

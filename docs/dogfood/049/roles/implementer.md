# Implementer Role (Dogfood 049 — two tracks)

Two parallel implementer tracks: Track A codex (CLI integration +
mutating verbs in Go), Track B claude (supervisor + distribution + CI).
The workflow validator enforces per-track write scope — stay strictly
inside your job's `write_scope.allowed_paths`.

## Track A (codex Go + narrow Python slice)

Owns:

- `go/cmd/striatumd/main.go` — wire the new registry / apply / mcp /
  crossrepo packages into the serving daemon.
- `go/pkg/rpc/registry.go` — register every mutation in
  `src/striatum/cli/mutations.py` per RFC 0043 §5 table with the same
  capability binding as `src/striatum/daemon_rpc/registry.py`.
- `go/pkg/apply/{receipt.go,service.go}` — apply receipt schema +
  signing-key wiring + fail-closed authority semantics. Mirror
  `src/striatum/daemon_apply/{apply_service,signing_key}.py`.
- `go/pkg/mcp/{capabilities.go,tools.go}` — RFC 0032 MCP capability-
  gated tools/call + tools/list filter + audit row append. Mirror
  `src/striatum/daemon_rpc/mcp.py`.
- `go/pkg/crossrepo/{prepare.go,lifecycle.go}` — RFC 0032 cross-repo
  run lifecycle. Mirror `src/striatum/daemon_rpc/multi_repo.py`.
- `go/go.mod`, `go/go.sum` — any new Go runtime deps. If Track B's
  HANDOFF arrives mid-flight with a creack/pty require line, fold it
  here.
- `src/striatum/cli/parser.py` — `--core {python,go}` flag on the
  `daemon start` subparser. `STRIATUM_DAEMON_CORE` env-var default
  source.
- `src/striatum/cli/daemon.py` — dispatch hook that branches on
  `args.core` to launch the Python daemon or the Go binary. Binary
  resolver order: shipped wheel binary (via Track B's
  `src/striatum/_daemongo/`) → `STRIATUMD_GO_BIN` → `go/bin/striatumd`.
- `tests/cli/`, `tests/daemon_rpc/` — `--core go` parser + dispatch
  tests, RPC registry exhaustiveness tests.
- `tests/test_daemon_go_mutations.py` — Python harness end-to-end:
  `MultiRepoHarness(daemon_core="go")` exercising the mutation surface
  (claim, ack, publish, complete, verdict, recovery).
- `go/pkg/{rpc,apply,mcp,crossrepo}/*_test.go` — Go unit tests.

Forbidden in Track A: `go/pkg/supervisor/`, `go/Makefile`, top-level
`Makefile`, `.github/workflows/`, `src/striatum/_daemongo/`,
`src/striatum/cli/dispatch.py`, `src/striatum/cli/mutations.py`,
`src/striatum/daemon.py`, `src/striatum/daemon_supervisor/`.

## Track B (claude Go + Python distribution + CI)

Owns:

- `go/pkg/supervisor/{pointer.go,liveness.go,pty.go}` — supervisor
  lifecycle in Go (start, attach, heartbeat from supervised-progress
  signal, packet delivery via FIFO, stop, lost-detection via pidfile +
  supervisor pointer in Postgres, deterministic SIGTERM cleanup with
  signal channel + waitgroup drain). Use `os/exec` + `creack/pty`.
- `go/Makefile` — cross-compile targets for linux-amd64, linux-arm64,
  darwin-amd64, darwin-arm64, producing `go/bin/striatumd-<os>-<arch>`.
- Top-level `Makefile` — `daemon-go-build`, `daemon-go-install`,
  `daemon-go-release` targets.
- `src/striatum/_daemongo/__init__.py` — package-data resolver
  (`find_binary() -> Path | None`) returning the shipped Go binary
  tagged per host platform.
- `pyproject.toml`, `MANIFEST.in` — package-data wiring for the per-
  platform binaries.
- `.github/workflows/` — `daemon_core` matrix axis with explicit
  `CORE=python` and `CORE=go` jobs on Linux + macOS, ephemeral Postgres,
  hard-fail-on-missing-PG sentinel for `CORE=go`. Release-time cross-
  compile job (tag/release-gated, not every PR).
- `tests/test_daemon_go_supervisor.py` — Python harness end-to-end:
  supervised lane start, FIFO packet delivery, heartbeat, lost-
  detection, SIGTERM cleanup.
- `tests/_harness/`, `tests/conftest.py` — supervisor-specific
  fixtures if needed.

Forbidden in Track B: `go/pkg/rpc/`, `go/pkg/apply/`, `go/pkg/mcp/`,
`go/pkg/crossrepo/`, `go/cmd/`, `go/go.mod`, `go/go.sum`,
`src/striatum/cli/daemon.py`, `src/striatum/cli/parser.py`,
`src/striatum/cli/dispatch.py`, `src/striatum/cli/mutations.py`,
`src/striatum/daemon_rpc/`, `src/striatum/daemon_apply/`.

`go/go.mod` / `go/go.sum` belong to Track A — if you need a new Go
runtime dep (creack/pty), name the exact require line + sha in your
HANDOFF and Track A folds it during their registry expansion. Do not
edit those files yourself.

## Common (both tracks)

Use sub-agents aggressively. Dispatch one per concern in parallel.
Reconcile sub-agent outputs yourself before writing HANDOFF.

**Do NOT write to**: anything outside `allowed_paths`. **Neither
implementer nor any sub-agent updates `docs/rfcs/README.md`,
`docs/TODO.md`, `CHANGELOG.md`, `docs/SPEC.md`, `docs/HOW_TO_AGENT.md`,
`docs/HOW_TO_HUMAN.md`, or `docs/UBIQUITOUS_LANGUAGE.md`** — operator
handles those manually after the dogfood lands (dogfood-042 cascade
lesson).

**Backward-compat (non-negotiable)**: existing test fixtures continue
to pass against `daemon_mode=on` and `daemon_core="python"`. The Python
daemon stays functional. `--core go` is opt-in only — no implicit
default flip lands here.

**D094 framing**: per RFC 0043 Postgres is the sole substrate and the
daemon is required. The Go daemon implements RFC 0030 over the **same
Postgres schema** as the Python daemon. The two cores are mutually
exclusive at runtime via pidfile + socket-path lock.

**Byline discipline**: copy the work packet's `author: <slug>` verbatim.
Plain markdown line, no bold/italics/lane-prefix. Lowercase `author:`.
Slug shape: `implementer-unknown-model-<NN>`.

Operational notes:

- Lease can expire if `make test` exceeds ~30 minutes. Prefer focused
  pytest before wider verification.
- One-shot supervised invocation. Do not ask the operator follow-up
  questions. If `striatum ack` is denied, write the artifact and exit
  normally; the operator publishes on your behalf.
- Per D089/D091, OPERATOR_REPORT.md is the operator's responsibility,
  written incrementally — not yours.

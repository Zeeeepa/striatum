# Implement Track B: RFC 0039 Phase 2 — Supervisor + distribution + CI (claude Go + Python harness)

Blocked until `review_design` returns an accepting verdict.

Implement Track B per `docs/dogfood/049/DESIGN_SYNTHESIS.md`. **You write Go (under `go/pkg/supervisor/` and the `go/Makefile`) plus the Python distribution shim, top-level Makefile, CI workflow files, and supervisor test fixture.** Sister Track A (CLI integration + mutating verbs, codex) runs in parallel — do not cross into its write scope.

**Your scope (claude):**

- `go/pkg/supervisor/{pointer.go,liveness.go,pty.go}` — supervisor lifecycle in Go: start, attach, heartbeat from supervised-progress signal, packet delivery via FIFO (byte-compatible with the Python wrapper protocol — read the wrapper source at `src/striatum/...` that generates `.striatum/bin/*-supervised-wrapper.sh`), stop, lost-detection via pidfile + supervisor pointer in Postgres, deterministic SIGTERM cleanup using a signal channel + waitgroup drain. Use `os/exec` + `creack/pty` (the well-trodden Go PTY library per RFC 0039 §6).
- `go/Makefile` — cross-compile targets for `linux-amd64`, `linux-arm64`, `darwin-amd64`, `darwin-arm64`, producing `go/bin/striatumd-<os>-<arch>`. The existing `build` target (producing `go/bin/striatumd` for the current host) stays.
- Top-level `Makefile` — `daemon-go-build`, `daemon-go-install`, `daemon-go-release` targets. `daemon-go-build` invokes `go/Makefile build`; `daemon-go-install` copies `go/bin/striatumd` into `src/striatum/_daemongo/` for in-tree wheel testing; `daemon-go-release` cross-compiles all four platforms and copies the per-platform binaries into the wheel package-data tree for the release pipeline.
- `src/striatum/_daemongo/__init__.py` — package-data resolver. Public function (e.g. `find_binary() -> Path | None`) that returns the path to the shipped Go binary, tagged per host platform (`sys.platform` + `platform.machine()`). Returns `None` if the binary is not shipped (sdist install or missing platform), so the CLI dispatch (Track A) can fall through to `STRIATUMD_GO_BIN` → `go/bin/striatumd`. **Do NOT** import `go/...` Go source; the resolver only locates the binary.
- `pyproject.toml` — `[tool.setuptools.package-data]` (or `[tool.hatch.build.targets.wheel.shared-data]`, whichever this project uses — read it first) update to include `src/striatum/_daemongo/binaries/*` so the per-platform binary ships in the wheel. Build-system metadata only; do NOT touch project name / version / dependencies on the same edit.
- `MANIFEST.in` — line for `recursive-include src/striatum/_daemongo *` if needed.
- `.github/workflows/` — extend the existing CI workflow(s) with a `daemon_core` matrix axis: `CORE=python` and `CORE=go` as **explicit jobs** (not in-process parametrization), on Linux + macOS. Each runs `make test-multi-repo CORE=$CORE` against ephemeral Postgres. For `CORE=go`, add a hard-fail-on-missing-PG sentinel so an all-skipped pass cannot occur (per dogfood-047 F3 finding). Add a release-time cross-compile job that runs `make daemon-go-release` and uploads the four per-platform binaries as artifacts (gated by tag push or release event, not every PR).
- `tests/test_daemon_go_supervisor.py` — Python harness end-to-end: `MultiRepoHarness(daemon_core="go")` exercising supervised lane start, packet delivery via FIFO, heartbeat from supervised-progress, lost-detection (kill the subprocess, assert recovery), SIGTERM cleanup (TERM the daemon, assert no orphan PTYs). Mirror the existing `tests/test_daemon_go_*.py` shape.
- `tests/_harness/`, `tests/conftest.py` — extend if the existing `daemon_core` fixture needs new helpers for supervisor-specific tests (e.g. FIFO inspection, pidfile assertion).
- `docs/dogfood/049/build/track_b/HANDOFF.md` — handoff summarizing shipped scope, files touched, test results (`go test ./...`, `make test`, `make test-multi-repo CORE=go`), the exact `creack/pty` require line + sha (or whatever new Go runtime dep you need) for Track A to fold into `go/go.mod`, deviations from the synthesis (if any) with one-line rationale.

**Use sub-agents aggressively** — one per concern, dispatched in parallel:

- Sub-agent supervisor pty: `go/pkg/supervisor/pty.go` (os/exec + creack/pty).
- Sub-agent supervisor pointer: `go/pkg/supervisor/pointer.go` (pidfile + supervisor pointer in Postgres via `go/pkg/db/`).
- Sub-agent supervisor liveness: `go/pkg/supervisor/liveness.go` (heartbeat + lost-detection + SIGTERM cleanup).
- Sub-agent go/Makefile: cross-compile targets for four platforms.
- Sub-agent top-level Makefile: daemon-go-build / daemon-go-install / daemon-go-release.
- Sub-agent package-data shim: `src/striatum/_daemongo/__init__.py` + `pyproject.toml` + `MANIFEST.in`.
- Sub-agent CI workflow: `.github/workflows/` matrix axis + release-time cross-compile job.
- Sub-agent supervisor test: `tests/test_daemon_go_supervisor.py`.

Reconcile sub-agent outputs yourself before writing HANDOFF.

**Do NOT touch**: `go/pkg/rpc/`, `go/pkg/apply/`, `go/pkg/mcp/`, `go/pkg/crossrepo/`, `go/cmd/`, `go/go.mod`, `go/go.sum`, `src/striatum/cli/daemon.py`, `src/striatum/cli/parser.py`, `src/striatum/cli/dispatch.py`, `src/striatum/cli/mutations.py`, `src/striatum/daemon_rpc/`, `src/striatum/daemon_apply/` — sister Track A owns those, and `go.mod`/`go.sum` belong to Track A so they handle the dep fold. **Do NOT write to**: README / TODO / CHANGELOG / RFC index / SPEC / HOW_TO. Operator handles those manually after the dogfood lands.

**Backward-compat (non-negotiable)**: the Python daemon must keep working. `daemon_core` parameter defaults to `python`. `--core go` is opt-in only. Existing test fixtures continue to pass against `daemon_mode=on` and `daemon_core="python"`. The CI matrix must not break existing single-core jobs.

**D094 framing**: per RFC 0043 Postgres is the sole substrate and the daemon is required. The Go daemon implements RFC 0030 over the **same Postgres schema** as the Python daemon. The supervisor pointer table is the same `striatumd.process_supervisor_pointers` already in the daemon schema.

Verification: `cd go && go build ./pkg/supervisor/... && go test ./pkg/supervisor/...` clean. `make daemon-go-build` produces `go/bin/striatumd`. `make daemon-go-release` produces all four per-platform binaries. `make test-multi-repo CORE=go` exercises the supervisor surface against ephemeral Postgres. `make lint`, `make typecheck`, `make test` clean for the Python slice.

One-shot supervised invocation. Do not ask follow-ups. If `striatum ack` is denied, write the HANDOFF and exit normally.

## Byline discipline

The work packet supplies an exact `author: <slug>` line. Copy it verbatim. Plain Markdown line, lowercase `author:`, NO bold, NO italics, NO lane prefix. Slug shape: `implementer-unknown-model-<NN>`.

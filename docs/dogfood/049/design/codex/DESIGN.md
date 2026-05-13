# RFC 0039 Phase 2 Design — Codex

author: designer-unknown-model-001

## Scope

This design implements RFC 0039 Phase 2 Steps 3-6 without changing the product boundary: `daemon_core` remains opt-in, the default stays `python`, and both daemon cores speak RFC 0030 envelope-v1 over the same RFC 0033 / RFC 0043 PostgreSQL substrate. There is no SQLite fallback, no second Go-only schema, and no default flip to Go in this dogfood.

The current tree already has the Phase 1/V1.5 base: `go/cmd/striatumd/main.go` accepts `--socket`, `--postgres-url`, `--migrate`, `--describe`, and `--migrations-sha-source`; `go/pkg/db/{connection,migrations,audit}.go` uses `pgx/v5`, applies embedded daemon migrations, and appends transactional audit rows; `go/pkg/rpc/{envelope,server,auth_pg}.go` implements envelope-v1, handshake enforcement, Postgres token authorization, and audit recording. Phase 2 should build on those entry points rather than introduce a parallel launcher or alternate RPC router.

## Track A — CLI Integration And Mutating Go RPC

Track A owns `src/striatum/cli/{parser,daemon}.py`, `go/pkg/rpc/registry.go`, and new Go packages `go/pkg/{apply,mcp,crossrepo}/`.

First, `src/striatum/cli/parser.py` should add `daemon start --core {python,go}` with default resolution from `STRIATUM_DAEMON_CORE`, falling back to `python`. `src/striatum/cli/daemon.py::dispatch_daemon` should take over the `daemon start` branch, resolve the selected core, and, for `go`, launch the Go binary with `subprocess.Popen` using the same socket path and Postgres URL semantics as the Python daemon. Binary resolution should be deterministic: packaged wheel binary under `striatum._daemongo` first, then `STRIATUMD_GO_BIN`, then repo-local `go/bin/striatumd` for editable checkouts. The command line passed to Go should match `tests/_harness/daemon.py`: `--socket <runtime/striatumd.sock> --postgres-url <url> --migrations-sha-source src/striatum/daemon_pg/sql`. The Python daemon path remains the existing behavior for `striatum daemon start`.

`go/cmd/striatumd/main.go` should fail closed when no Postgres URL is configured. Dogfood-047 identified the current no-URL `AllowAllAuthorizer{}` branch as a production auth/audit fallback bug. For Phase 2, `AllowAllAuthorizer` must remain test-only; serving mode requires a configured Postgres URL so `PostgresAuthorizer` and `AuditRecorder` are always active. The Go daemon should keep `--describe` free of that requirement because it is a local metadata inspection command, not a serving daemon.

Second, `go/pkg/rpc/registry.go` must be reconciled with `src/striatum/daemon_rpc/registry.py`, not with the older aliases. The current Go registry still exposes `ack`, `heartbeat`, `claim_next`, `publish_artifact`, `verdict`, and `submit_review` as primary entries and maps several mutations to `write` where Python now maps them to `claim` or `admin`. The canonical Phase 2 registry entries are:

| Method | Capability | Scope |
|---|---|---|
| `session.register`, `session.close` | `claim` | `single_repo` |
| `work.claim_next`, `work.ack`, `work.heartbeat`, `work.release` | `claim` | `single_repo` |
| `supervise.start`, `supervise.send`, `supervise.stop` | `claim` | `single_repo` |
| `work.send_message`, `work.block`, `work.complete` | `write` | `single_repo` |
| `artifact.publish`, `worktree.create`, `worktree.release` | `write` | `single_repo` |
| `workflow.init`, `workflow.generate`, `workflow.upgrade` | `write` | `single_repo` |
| `workflow.validate`, `workflow.plan`, `workflow.graph`, `workflow.templates.*`, `workflow.generate.preview` | `read` | `single_repo` |
| `review.submit`, `review.verdict` | `review` | `single_repo` |
| `review.override`, `decision.record`, `checkpoint.resolve`, `branch.confirm`, `run.prepare`, `run.start`, `run.pause`, `run.resume`, `run.cancel`, `run.retry_job`, `repo.init` | `admin` | `single_repo` |
| `recovery.stale_leases`, `recovery.requeue_stale`, `recovery.cancel_job`, `recovery.process_reconcile`, `recovery.resume`, `recovery.auto`, `recovery.watch` | `recovery` | `single_repo` |
| `apply.reviewed_patch` | `apply` | `single_repo` |
| `apply.receipt.show`, `apply.receipt.verify` | `read` | `single_repo` |
| `repo.add`, `repo.remove`, `daemon.token.*`, `daemon.key.rotate`, `daemon.shutdown`, `daemon.migrate`, `daemon.migrate_repo_local` | `admin` | `daemon_global` |
| `cross_repo.list`, `cross_repo.describe`, `cross_repo.why` | `read` | `cross_repo` |
| `cross_repo.cancel` | `recovery` | `cross_repo` |

The deprecated aliases can remain marked `Deprecated: true` for one release, but `daemon.describe` must publish the dotted RFC 0043 names as the supported surface. `go/pkg/rpc/server.go::route` should keep direct built-ins for `daemon.describe`, `apply.*`, `cross_repo.*`, and MCP adapter calls, then use registered handlers for the rest. The absence of a registered handler should still return `method_unknown` with exit-code-10 semantics through the envelope error.

Third, mutating routes need one implementation boundary. The shortest safe Phase 2 implementation is a Go RPC method layer that translates envelope params to the existing daemon-required CLI/API mutation service during this dogfood, then replaces hot paths with native Go later only where needed. The Go daemon can shell to `striatum` only as an explicit compatibility adapter if it preserves D094 by calling the Python CLI as a client of the same Postgres-backed daemon substrate, not by opening `.striatum/state.sqlite3`. The better target for this slice is a `go/pkg/rpc/cliadapter.go` handler that executes the installed Python module with `--repo <repo_root>` for methods that still have only Python business logic, with params serialized to argv exactly as `src/striatum/daemon_rpc/server.py::_params_to_args` does. This keeps the externally observable RFC 0030 registry, capability, and audit behavior in Go while avoiding a second, rushed reimplementation of every workflow invariant.

Fourth, `go/pkg/apply/{receipt.go,service.go}` should mirror `src/striatum/daemon_apply/apply_service.py` and RFC 0031. `apply.reviewed_patch` fails with `sealed_key_missing` unless the signing key is loaded, then fails with `apply_gate_unsatisfied` until the reviewed-patch apply gate is implemented. `apply.receipt.show` and `apply.receipt.verify` return `receipt_missing` for absent receipts. `receipt.go` should define the Postgres-facing receipt model: `receipt_id`, `repository_id`, `run_id`, `job_id`, `patch_artifact_id`, `patch_sha256`, `base_tree`, `result_tree`, `signing_key_id`, `signature`, `state`, `denial_reason`, and timestamps. This preserves the RFC 0031 threat model: sealed apply is an AI guardrail, not local-operator non-repudiation.

Fifth, `go/pkg/mcp/{capabilities.go,tools.go}` should mirror the RFC 0032 posture: `tools/list` filters `rpc.SortedMethods()` by token capability and repository scope, while `tools/call` re-authorizes through the same `PostgresAuthorizer` and appends an audit row through `AuditRecorder.RecordRPC`. Visibility is not authority. Tests should assert read-only tokens cannot see or call write/review/claim/admin/recovery tools, crafted calls return `capability_missing`, and both allow and deny paths produce hash-chain audit rows.

Sixth, `go/pkg/crossrepo/{prepare.go,lifecycle.go}` should mirror the current Python cross-repo lifecycle surface that `src/striatum/daemon_rpc/server.py::_route_cross_repo` only partially routes. `prepare.go` owns repository accessibility checks, participant rows, per-repo local run creation, and `cross_repo_runs` creation. `lifecycle.go` owns `list`, `describe`, `why`, `cancel`, and crash recovery reconciliation. The Go implementation must use the existing Postgres tables and per-repo pointer semantics from RFC 0032/RFC 0035; it must not invent cross-repo artifact provenance or atomic file mutations.

Track A tests:

- `go test ./pkg/rpc/... ./pkg/apply/... ./pkg/mcp/... ./pkg/crossrepo/...`.
- `tests/test_daemon_go_mutations.py` using `MultiRepoHarness(daemon_core="go")` to register a session, claim, ack, heartbeat, publish, complete, submit review/verdict, record a decision, resolve a checkpoint, requeue/cancel/resume recovery paths, create/release worktrees, confirm a branch, prepare/start/pause/resume/cancel runs, and validate/generate workflows.
- A regression that `striatumd` serving without `--postgres-url` exits non-zero, closing the dogfood-047 fail-open branch.

## Track B — Supervisor, Distribution, And CI

Track B owns new `go/pkg/supervisor/{pointer,liveness,pty}.go`, packaging under `src/striatum/_daemongo/`, Makefile targets, and CI.

The supervisor package should mirror `src/striatum/supervisor.py` and `src/striatum/daemon_supervisor/{pointer,progress_watcher}.py` but store authoritative supervisor rows in Postgres per RFC 0031/D094. `pointer.go` writes the repo-visible supervisor pointer only through daemon-mediated mutation; its state model should match the Python helper: `starting`, `attached`, `detached`, `lost`, `stopped`, with `attached` requiring `pid_start_time`. `liveness.go` owns pid/start-time verification and lost detection. `pty.go` owns `os/exec` + `github.com/creack/pty` process start, SIGTERM/SIGKILL cleanup, and FIFO packet delivery.

The FIFO protocol must be byte-compatible with the existing wrappers under `.striatum/bin/*-supervised-wrapper.sh` and `src/striatum/supervisor.py`: the daemon creates `.striatum/scratch/<supervisor_id>/stdin.pipe` mode `0600`, opens it so the child does not see EOF, and writes one UTF-8 JSON work-packet line per delivery with a trailing newline. The packet body is the stored `work_packets.packet_json` object from `striatum.db.build_packet`, including `packet_version`, `packet_id`, `run`, `job`, `lease`, `commands`, `expected_artifacts`, `write_scope`, `adapter_constraints`, `lane_attestation`, optional `review_policy`, and optional `harness_profile`. The supervisor never parses agent stdout for workflow state and never captures transcripts; progress is derived only from wrapper log file mtimes.

Supervised-progress heartbeat should port the `ProgressWatcherConfig` values from `src/striatum/daemon_supervisor/progress_watcher.py`: poll interval 30s, refresh threshold 60s, idle threshold 600s, heartbeat extension 900s. The Go watcher checks the supervised process is alive, finds the newest relevant `*.log` under the supervisor scratch path, heartbeats the active lease when mtime is fresh, emits idle/lost events when stale or dead, and uses a per-job advisory guard equivalent to `progress_advisory_lock` so surgical recovery and progress refresh cannot race.

Daemon shutdown should be deterministic. `go/cmd/striatumd/main.go` already uses `signal.NotifyContext`; Track B should add a supervisor manager with a `sync.WaitGroup`, a shutdown channel, and bounded drain. On SIGTERM the manager stops accepting new supervisor starts, sends SIGTERM to attached children, waits up to the configured grace period, sends SIGKILL to remaining children, marks rows `stopped` or `lost`, unlinks FIFOs, flushes final audit rows, and only then returns from `main`.

Distribution should ship both editable-development and wheel-installed paths. `go/Makefile` should grow:

- `build`: current host binary at `go/bin/striatumd`.
- `release`: cross-compile `go/bin/striatumd-linux-amd64`, `go/bin/striatumd-linux-arm64`, `go/bin/striatumd-darwin-amd64`, `go/bin/striatumd-darwin-arm64`.
- `install`: copy the host binary to a configurable `DESTDIR`/`PREFIX`.

The top-level `Makefile` should add `daemon-go-install` and `daemon-go-release` alongside the existing `daemon-go-build`, `daemon-go-test`, and `daemon-go-lint`.

The package-data shim should be:

```text
src/striatum/_daemongo/
  __init__.py
  bin/
    linux-amd64/striatumd
    linux-arm64/striatumd
    darwin-amd64/striatumd
    darwin-arm64/striatumd
```

`src/striatum/_daemongo/__init__.py` should expose `resolve_binary()` based on `platform.system()` and `platform.machine()`, chmod the extracted path if needed, and return `None` when the wheel lacks a matching payload. `pyproject.toml` package data should include `striatum._daemongo = ["bin/*/striatumd"]`; `MANIFEST.in` should include the same tree for sdists. The resolver order used by `src/striatum/cli/daemon.py` is packaged binary, then `STRIATUMD_GO_BIN`, then in-tree `go/bin/striatumd`.

CI should use explicit jobs rather than an implicit matrix that can skip unnoticed. `.github/workflows/ci.yml` should keep the existing Python/UI job, then add `multi-repo-python` and `multi-repo-go` jobs on `ubuntu-latest` and `macos-latest` with system Postgres available, `make daemon-go-build` before the Go job, and `make test-multi-repo CORE=<core>`. `make test-multi-repo CORE=go` must fail if Postgres is missing or if all selected multi-repo tests skip; dogfood-047 found that skip-all returned success. `.github/workflows/release.yml` should run `make daemon-go-release` and upload the four Go binaries alongside the Python dist artifacts.

Track B tests:

- `go test ./pkg/supervisor/...` covering FIFO creation, one-line packet delivery, PTY-backed child start, pid/start-time liveness, progress-heartbeat state transitions, and SIGTERM drain.
- `tests/test_daemon_go_supervisor.py` using `MultiRepoHarness(daemon_core="go")` to start a supervised lane, claim a packet, assert `supervisor_delivery` and `supervisor.packet_delivered`, touch a wrapper log and assert heartbeat extension, remove/kill the child and assert lost detection, and stop the daemon to verify child cleanup.
- CI sentinel test that `STRIATUM_MULTI_REPO_DAEMON_CORE=go` actually launched `go/bin/striatumd` or the wheel binary, not the Python daemon.

## Integration Order

1. Reconcile `go/pkg/rpc/registry.go` with `src/striatum/daemon_rpc/registry.py`, including capability fixes and deprecated aliases.
2. Make Go serving mode require Postgres and add the no-URL fail-closed regression.
3. Wire `daemon start --core go` through `src/striatum/cli/{parser,daemon}.py` and prove `MultiRepoHarness(daemon_core="go")` still boots.
4. Add Track A compatibility handlers and tests for the RFC 0043 mutation table.
5. Add `go/pkg/apply`, then `go/pkg/mcp`, then `go/pkg/crossrepo`, because MCP and cross-repo both depend on the finalized method registry and audit behavior.
6. Add `go/pkg/supervisor` and the Python harness supervisor e2e.
7. Add package-data resolver, Makefile targets, and CI/release jobs.

## Review Risks

The main implementation risk is trying to rewrite all Python mutation business logic natively in Go in one dogfood. That would duplicate the scheduler, workflow validator, artifact publisher, recovery, and worktree invariants while D094 is already moving all state through a daemon-required substrate. The safer Phase 2 bar is externally observable daemon parity: Go owns RPC authorization, registry, audit, MCP gating, process supervision, and binary distribution, while individual repo-local mutation semantics can route through the existing Python command implementation until native Go ports are explicitly scheduled.

The second risk is accidentally preserving the Phase 1 fail-open branch. Any Go daemon that can serve RPC without Postgres is not Phase 2 compliant. No Postgres means no capability table, no audit chain, and no D094 substrate.

The third risk is CI evidence that only looks green because Postgres-backed tests skipped. `make test-multi-repo CORE=go` needs a sentinel that at least one Go-core harness test executed against a real Postgres URL, and CI should wire ephemeral/system Postgres before running it.

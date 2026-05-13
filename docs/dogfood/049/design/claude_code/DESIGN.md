author: designer-unknown-model-001

# RFC 0039 Phase 2 — Go Daemon Steps 3-6 (claude_code design)

## 0. Scope, sequencing, and what V1.5 already shipped

RFC 0039 V1 (dogfood-042) landed Steps 1+2: `go/cmd/striatumd/` binds the
RFC 0030 envelope-v1 socket and serves the read-only verbs registered in
`go/pkg/rpc/registry.go:76-128` (`daemon.hello`, `daemon.describe`, `status`,
`why`, `doctor`, `dashboard`, `dashboard.all`, `evidence.export`, `repo.add`,
`repo.list`, `repo.remove`, plus admin/cross-repo read methods). V1.5
(dogfood-047) added F1-F5 from `docs/dogfood/047/PHASE_1_OPERATOR_NOTES.md`:
`go/pkg/db/connection.go:175-207` uses `pgx/v5 v5.7.2` with
`application_name = "striatumd-go/<daemon_version>"`,
`go/pkg/db/audit.go:55-197` does a single `READ COMMITTED` transaction with
`SELECT ... FOR UPDATE` on `striatumd.audit_chain_head`,
`go/pkg/rpc/auth_pg.go:30-135` is the production `PostgresAuthorizer` that
mirrors `src/striatum/daemon_rpc/capability.py` denial vocabulary,
`go/cmd/striatumd/main.go:21-91` accepts the locked flag surface
(`--socket / --postgres-url / --migrate / --describe /
--migrations-sha-source`) and writes the binary to `go/bin/striatumd` via
`go/Makefile:1-16`, and `Makefile:82-92` wires
`make test-multi-repo CORE=go` against the
`STRIATUM_MULTI_REPO_DAEMON_CORE` fixture in `tests/conftest.py:18-25`.

V1.5 still left **five named correctness holes** the codex reviewer
recorded (`docs/dogfood/047/PHASE_1_OPERATOR_NOTES.md:119-153`): no-Postgres
fallback in `go/cmd/striatumd/main.go:49` falls through to
`AllowAllAuthorizer{}` with no audit; `CORE=go` matrix can pass with all
tests skipped; `tests/test_daemon_go_smoke.py:51-62` does not assert the
unauthenticated denial vocabulary; the audit-append race test
(`go/pkg/db/audit_race_test.go`) and the Python cross-core regression
(`tests/test_daemon_go_audit.py:40-77`) skip without `STRIATUM_PG_TEST_URL`.
This design **picks those up as gating preconditions for Step 4 land**, not
as new follow-ups, because the moment a CLI mutation routes through the Go
core the fail-open auth branch is no longer "test-only" — it is a production
authorization escape.

The implementation order is the same F-then-Steps shape RFC 0039 V1.5
locked: close the F1.6 / F4-evidence gaps **before** wiring mutating verbs
into the registry, then land Step 3 (CLI), Step 4 (mutations + apply +
MCP + cross-repo), Step 5 (supervisor), Step 6 (distribution + CI matrix).
Tracks A and B run in parallel after the F1.6 gate.

| Track | Owner | Scope | Step coverage |
|-------|-------|-------|---------------|
| A | codex Go | `striatum daemon start --core go` wiring + mutating Go RPC methods + apply + MCP + cross-repo | Steps 3 + 4 |
| B | claude Go + Python harness | Supervisor goroutines + PTY + FIFO bytes + wheel binary payload + CI matrix | Steps 5 + 6 |

Both tracks share the V1.5 audit chain (`go/pkg/db/audit.go`) and the V1.5
`PostgresAuthorizer` (`go/pkg/rpc/auth_pg.go`) unchanged. The wire protocol
stays envelope-v1 over the Unix socket per RFC 0030 §1-3
(`docs/rfcs/0030-daemon-rpc-server-and-version-skew-protocol.md:74-167`).
The Postgres substrate stays RFC 0033 per RFC 0039 §3
(`docs/rfcs/0039-go-daemon-core.md:215-229`) and the D094 framing in the
prompt: one substrate, two cores, mutually exclusive in one run via the
pidfile + socket-path lock.

## 0.1 Pre-Step-3 gate: close codex F1-F5 evidence holes

The codex review findings absorbed into RFC 0039 V1.6 (TODO item 30 per
`docs/dogfood/047/PHASE_1_OPERATOR_NOTES.md:189`) are listed there as
"deferred to V1.6." This design lifts the **fail-closed auth posture** and
the **executable-evidence** subset of those findings to a pre-Step-3 gate,
because they are the difference between an opt-in `--core go` flag that
silently runs unauthenticated and one that refuses to start without a
configured Postgres URL.

| Finding | V1.6 fix location | Acceptance |
|---------|-------------------|------------|
| F1.6-A (no-PG auth fail-open) | `go/cmd/striatumd/main.go:49-79` — the `if config.URL == ""` branch must return a `cli.UsageError` instead of falling through to `AllowAllAuthorizer{}`; reuse the existing exit code 11 (`daemon_unreachable`) vocabulary from `src/striatum/errors.py` so the Python launcher's parse-stderr fast-path matches Python parity | `go test go/cmd/striatumd/...` covers the "no URL → exit 11" path; Python harness reuses `_wait_for_socket` (`tests/_harness/daemon.py:136-150`) to assert the binary exits, not hangs |
| F1.6-B (`go.sum` regeneration) | Track A operator step: `go -C go mod tidy` in CI; CI job in §6.4 below runs the tidy check on every PR | `make daemon-go-build` succeeds with the committed `go/go.sum`; CI step `go mod verify` rejects drift |
| F1.6-C (`CORE=go` skip detection) | `Makefile:82-92` gains a sentinel: a tiny `tests/test_daemon_go_core_sentinel.py::test_core_matrix_runs_at_least_one_assertion` that fails when `STRIATUM_MULTI_REPO_DAEMON_CORE=go` is set but the harness skipped because Postgres was unreachable | The smoke test fails *closed* if the matrix selected 33 tests and ran 0; CI `CORE=go` job refuses to be green-with-skips |
| F1.6-D (smoke does not assert denial) | `tests/test_daemon_go_smoke.py:51-62` adds `assert describe_response["ok"] is False` and `assert describe_response["error"]["code"] == "capability_missing"` to lock the unauthenticated denial vocabulary | The Go server's V1.5 denial path is regression-tested; would catch a future `AllowAllAuthorizer` re-introduction |
| F1.6-E (audit race + audit chain not executable without PG) | `tests/conftest.py:18-25` already reads `STRIATUM_MULTI_REPO_DAEMON_CORE`; add a parallel `STRIATUM_PG_TEST_URL` injection from the CI Postgres service container so `tests/test_daemon_go_audit.py:40-77` actually runs | CI matrix (§6.4) runs both `go test ./...` and the audit-race regression with a live ephemeral Postgres |

These five fixes land **first**, then Track A and Track B can both progress
without F-fan-out into Step 4. The codex F1-F5 audit row stays in
`docs/TODO.md` item 30 and is closed by this dogfood's lands.

## 1. Track A — `striatum daemon start --core go` and the mutating verb table

### 1.1 Python CLI: `daemon start --core go`

**File: `src/striatum/cli/parser.py:140-144`** — extend the `daemon start`
subparser with two new optional args. Existing args:

```python
daemon_start = daemon_sub.add_parser("start")
daemon_start.add_argument("--sweep-interval-seconds", type=float, default=60.0)
daemon_start.add_argument("--max-sweeps", type=int, default=None)
daemon_start.add_argument("--postgres-url")
daemon_start.add_argument("--json", action="store_true")
```

Add **immediately after `--json`** (closed set of two flags; no envvar-only
overrides for the operator-visible surface):

```python
daemon_start.add_argument(
    "--core",
    choices=["python", "go"],
    default=None,
    help=(
        "Daemon core implementation. Defaults to the value of "
        "STRIATUM_DAEMON_CORE (python|go), or 'python' when unset. "
        "RFC 0039 Phase 2."
    ),
)
daemon_start.add_argument(
    "--go-binary",
    default=None,
    help=(
        "Path to the Go daemon binary. Defaults to STRIATUMD_GO_BIN, "
        "then the package-data wheel binary, then go/bin/striatumd. "
        "Only meaningful with --core go."
    ),
)
```

The `--core` flag is opt-in; the default stays Python for RFC 0039 §9 Phase
1 backward-compat per the prompt's non-negotiable framing. RFC 0039 §9
Phase 2 (flipping the default to Go) is **out of scope** here and remains a
separate future RFC.

**File: `src/striatum/cli/daemon.py:17-34`** — `dispatch_daemon` currently
only owns `migrate-repo-local`. The `start` subcommand still lives in
`src/striatum/cli/dispatch.py:883-888` and routes to
`striatum.daemon.run_daemon_foreground`. Split that so `--core go` does
not collide with the Python foreground supervisor loop in
`src/striatum/daemon.py:879-947`.

Land a new function `dispatch_daemon_start_go` in `src/striatum/cli/daemon.py`
that runs **only when `args.core == "go"` or the envvar override resolves
to `"go"`**. Algorithm:

1. Resolve the core: `args.core or os.environ.get("STRIATUM_DAEMON_CORE", "python")`.
   Invalid values raise `StriatumError("--core must be python or go", exit_code=2)`.
2. If core resolves to `"python"`, return `None` so
   `_dispatch_daemon` falls through to the existing Python path.
3. Resolve the binary via `_resolve_go_daemon_binary(args.go_binary)` (new
   helper in same file), search order matching the prompt:
   - `args.go_binary` if not None
   - `STRIATUMD_GO_BIN` env var
   - shipped wheel binary under `striatum._daemongo` (§4.1)
   - `<repo_root>/go/bin/striatumd` (in-tree dev path; matches
     `tests/_harness/daemon.py:23` constant `_DEFAULT_GO_BIN`)
   - `shutil.which("striatumd")`
   If none resolve, raise
   `StriatumError("striatumd Go binary not found; install striatum[daemon-go] or set STRIATUMD_GO_BIN", exit_code=11)`.
4. Resolve the Postgres URL from the same precedence Python already uses:
   `args.postgres_url` → `STRIATUM_DAEMON_DB_URL` → daemon config file.
   Reuse `src/striatum/daemon_pg/config.py::resolve_config` so both cores
   share one resolver.
5. Refuse if a Python daemon is already running. Read
   `src/striatum/daemon.py:128 pid_path()` and `socket_path()`; if both
   exist and `_pid_alive(pid)` (`src/striatum/daemon.py:857-865`), refuse
   with `StriatumError("Python daemon already active; stop it before --core go", exit_code=7)`. The same lock applies for a stale Go pidfile (§5.4).
6. `subprocess.Popen` the binary with `["--socket", str(socket_path()),
   "--postgres-url", postgres_url, "--migrate"]`. Inherit stdout/stderr per
   RFC 0030 (the daemon never parses agent output — `daemon doctor` reads
   structured state from PG). Set `start_new_session=True` so a CLI Ctrl-C
   does not co-kill the daemon.
7. Wait on the socket up to 10 seconds using the same probe shape as
   `tests/_harness/daemon.py:136-150`. Return a JSON envelope with
   `{"mode": "daemon", "core": "go", "started": True, "socket_path": ...,
   "pid": ..., "postgres_url": <redacted>}` so existing operator tooling
   (`striatum daemon status`) keeps shape parity with the Python path.

**File: `src/striatum/cli/dispatch.py:880-888`** — change `_dispatch_daemon`
to call the new helper:

```python
def _dispatch_daemon(args: argparse.Namespace) -> object:
    from striatum import daemon as daemon_mod

    if args.daemon_command == "start":
        from striatum.cli.daemon import dispatch_daemon_start_go
        go_result = dispatch_daemon_start_go(args)
        if go_result is not None:
            return go_result
        return daemon_mod.run_daemon_foreground(
            sweep_interval_seconds=float(args.sweep_interval_seconds),
            max_sweeps=args.max_sweeps,
            postgres_url=getattr(args, "postgres_url", None),
        )
    ...
```

The Python CLI client (`src/striatum/daemon_rpc/client.py`) speaks
envelope-v1 over the Unix socket regardless of which core is bound to it —
no client-side change. The same `daemon doctor`, `daemon status`,
`daemon audit`, `daemon stop` verbs work because they observe state via
PG, not the daemon process surface.

### 1.2 Go method registry — every mutation in
`src/striatum/cli/mutations.py` per RFC 0043 §5

The V1.5 Go registry (`go/pkg/rpc/registry.go:76-128`) currently registers
46 methods but is **read-only modulo the supervise / apply / cross-repo
trio that exists only as denial paths**. Track A's job is to (a) register
every remaining mutation from `src/striatum/daemon_rpc/registry.py:48-159`
and (b) wire a Go handler for each so the call returns ok rather than
`method_unknown`.

**Method table — RFC 0043 §5 canonical names** (each maps 1:1 to the
existing Python entry referenced by line in
`src/striatum/daemon_rpc/registry.py`):

| RPC method | Capability | Scope | Python registry line | Go registry entry to add |
|------------|-----------:|-------|----------------------|--------------------------|
| `session.register` | `claim` | single_repo | 77 | `NewMethod("session.register", CapPtr(CapabilityClaim), true, "")` |
| `session.close` | `claim` | single_repo | 78 | `NewMethod("session.close", CapPtr(CapabilityClaim), true, "")` |
| `work.claim_next` | `claim` | single_repo | 79 | `NewMethod("work.claim_next", CapPtr(CapabilityClaim), true, "")` |
| `work.ack` | `claim` | single_repo | 80 | `NewMethod("work.ack", CapPtr(CapabilityClaim), true, "")` |
| `work.heartbeat` | `claim` | single_repo | 81 | `NewMethod("work.heartbeat", CapPtr(CapabilityClaim), true, "")` |
| `work.release` | `claim` | single_repo | 82 | `NewMethod("work.release", CapPtr(CapabilityClaim), true, "")` |
| `work.send_message` | `write` | single_repo | 90 | `NewMethod("work.send_message", CapPtr(CapabilityWrite), true, "")` |
| `work.block` | `write` | single_repo | 91 | `NewMethod("work.block", CapPtr(CapabilityWrite), true, "")` |
| `work.complete` | `write` | single_repo | 92 | `NewMethod("work.complete", CapPtr(CapabilityWrite), true, "")` |
| `artifact.publish` | `write` | single_repo | 93 | `NewMethod("artifact.publish", CapPtr(CapabilityWrite), true, "")` |
| `worktree.create` | `write` | single_repo | 94 | `NewMethod("worktree.create", CapPtr(CapabilityWrite), true, "")` |
| `worktree.release` | `write` | single_repo | 95 | `NewMethod("worktree.release", CapPtr(CapabilityWrite), true, "")` |
| `workflow.init` | `write` | single_repo | 96 | `NewMethod("workflow.init", CapPtr(CapabilityWrite), true, "")` |
| `workflow.upgrade` | `write` | single_repo | 98 | `NewMethod("workflow.upgrade", CapPtr(CapabilityWrite), true, "")` |
| `review.submit` | `review` | single_repo | 101 | `NewMethod("review.submit", CapPtr(CapabilityReview), true, "")` |
| `review.verdict` | `review` | single_repo | 102 | `NewMethod("review.verdict", CapPtr(CapabilityReview), true, "")` |
| `review.override` | `admin` | single_repo | 104 | `NewMethod("review.override", CapPtr(CapabilityAdmin), true, "")` |
| `decision.record` | `admin` | single_repo | 105 | `NewMethod("decision.record", CapPtr(CapabilityAdmin), true, "")` |
| `checkpoint.resolve` | `admin` | single_repo | 106 | `NewMethod("checkpoint.resolve", CapPtr(CapabilityAdmin), true, "")` |
| `branch.confirm` | `admin` | single_repo | 107 | `NewMethod("branch.confirm", CapPtr(CapabilityAdmin), true, "")` |
| `run.pause` | `admin` | single_repo | 110 | `NewMethod("run.pause", CapPtr(CapabilityAdmin), true, "")` |
| `run.resume` | `admin` | single_repo | 111 | `NewMethod("run.resume", CapPtr(CapabilityAdmin), true, "")` |
| `run.cancel` | `admin` | single_repo | 112 | `NewMethod("run.cancel", CapPtr(CapabilityAdmin), true, "")` |
| `run.retry_job` | `admin` | single_repo | 113 | `NewMethod("run.retry_job", CapPtr(CapabilityAdmin), true, "")` |
| `repo.init` | `admin` | single_repo | 114 | `NewMethod("repo.init", CapPtr(CapabilityAdmin), true, "")` |
| `recovery.auto` | `recovery` | single_repo | 121 | `NewMethod("recovery.auto", CapPtr(CapabilityRecovery), true, "")` |
| `recovery.watch` | `recovery` | single_repo | 122 | `NewMethod("recovery.watch", CapPtr(CapabilityRecovery), true, "")` |
| `daemon.migrate_repo_local` | `admin` | daemon_global | 138 | `NewMethod("daemon.migrate_repo_local", CapPtr(CapabilityAdmin), false, "")` |
| `workflow.plan` | `read` | single_repo | 61 | `NewMethod("workflow.plan", CapPtr(CapabilityRead), true, "")` |
| `workflow.graph` | `read` | single_repo | 62 | `NewMethod("workflow.graph", CapPtr(CapabilityRead), true, "")` |
| `workflow.templates.list` | `read` | single_repo | 64 | `NewMethod("workflow.templates.list", CapPtr(CapabilityRead), true, "")` |
| `workflow.templates.show` | `read` | single_repo | 65 | `NewMethod("workflow.templates.show", CapPtr(CapabilityRead), true, "")` |
| `corpus.export` | `read` | single_repo | 58 | `NewMethod("corpus.export", CapPtr(CapabilityRead), true, "")` |
| `run.summary` | `read` | single_repo | 59 | `NewMethod("run.summary", CapPtr(CapabilityRead), true, "")` |
| `run.graph` | `read` | single_repo | 60 | `NewMethod("run.graph", CapPtr(CapabilityRead), true, "")` |
| `list.runs` / `list.sessions` / `list.jobs` / `list.artifacts` / `list.workflows` | `read` | single_repo | 67-71 | five `NewMethod` calls, same pattern |
| `worktree.list` | `read` | single_repo | 72 | `NewMethod("worktree.list", CapPtr(CapabilityRead), true, "")` |
| `repo.list` | `read` | daemon_global | 75 | `NewMethod("repo.list", CapPtr(CapabilityRead), false, "")` |

Methods **already registered in V1/V1.5** stay (`go/pkg/rpc/registry.go:76-128`):
`status`, `why`, `doctor`, `dashboard`, `dashboard.all`, `evidence.export`,
`workflow.validate`, `workflow.generate.preview`, `workflow.generate`,
`run.prepare`, `run.start`, `claim_next`, `verdict`, `submit_review`,
`ack`, `block`, `heartbeat`, `publish_artifact`, `complete`, `release`,
`dogfood.publish_on_behalf`, `dogfood.surgical_recovery`, the
`recovery.*` recovery quartet, the `supervise.*` quintet plus
`supervise.reattach_status`, the `apply.*` triple, the `repo.add` /
`repo.remove` admin pair, the `daemon.token.*` triple, `daemon.key.rotate`,
`daemon.shutdown`, `daemon.migrate`, and the four `cross_repo.*` cross-repo
routes. The undotted V1 aliases (`ack`, `heartbeat`, `release`, etc.)
remain in the Go registry **with the same `deprecated: true` flag the
Python registry uses** (`src/striatum/daemon_rpc/registry.py:150-159`) so
the cross-core `methods_etag` matches byte-for-byte after the additions
above. **Critical:** when adding the new entries, set the `Deprecated`
field on the aliases by extending `MethodEntry` with a `Deprecated bool`
in `go/pkg/rpc/registry.go:42-51` (currently the field exists but is never
set true) so the registry serialization in `MethodsETag()`
(`go/pkg/rpc/registry.go:156-160`) matches the Python `public_dict`
output in `src/striatum/daemon_rpc/registry.py:29-39`.

**Registry parity test (new):** `go/pkg/rpc/registry_parity_test.go`
compares the `MethodsETag()` produced by Go against a golden file
`tests/fixtures/daemon_go/methods_etag.golden` generated by
`tests/test_daemon_method_registry_parity.py` from the Python registry.
Regenerate-on-mismatch is operator-driven (`pytest --regen-fixtures`); CI
runs in compare-only mode. Drift on either side fails CI, which is the
behavior dogfood-047 F2 already established for the migration-SHA file.

### 1.3 Go handler wiring — `pkg/rpc/handlers/*.go`

`go/pkg/rpc/server.go:173-182` resolves handlers from `s.Handlers` (the
plain `map[string]Handler`). V1/V1.5 never populated handlers for any
read-side method (the routes are computed inline in
`go/pkg/rpc/server.go:212-238`, mostly through `repo.add` and
`audit.show` paths which we don't see in this tree — they live in the
binary but not the test surface yet). For Step 4 we land a real handler
registry.

**New file: `go/pkg/rpc/handlers/registry.go`** — replaces the implicit
inline route table with one explicit `Register(server *rpc.Server,
runner db.Runner)` call in `go/cmd/striatumd/main.go:72-78` so the server
struct stays the same and only its `Handlers` map is populated. The
handler signature already accepts an `rpc.Envelope` and a `context.Context`
(`go/pkg/rpc/server.go:14`).

Three categories of handlers:

1. **Pure-PG SQL routes** (the workflow mutation surface). One Go function
   per method that executes one or more parameterized statements against
   `db.Runner` inside an outer `BeginTx` (`go/pkg/db/connection.go:130-136`).
   Examples:
   - `handlers.SessionRegister(ctx, env, runner)` — mirrors
     `src/striatum/cli/mutations.py:227-353` (`register_session`). Reads
     `params["run_id"]`, `params["role_id"]`, `params["lane_id"]`,
     `params["capabilities"]`, `params["fresh"]`, `params["parent_session_id"]`,
     `params["operator_label"]`, `params["force_non_fresh"]`,
     `params["non_fresh_reason"]`. Validates against workflow row, derives
     ordinal, inserts the `sessions` row (now in the daemon-DB
     `striatumd.sessions` schema introduced by daemon_pg migration 0005 —
     `src/striatum/daemon_pg/sql/0005_repo_local_workflow_state.sql`), and
     emits the `session.registered` event. The HARNESS-003 fresh-reviewer
     refusal (`mutations.py:275-296`) lands as a parameterized
     pre-validation block; the refusal vocabulary is
     `RpcError("invalid_transition", ...)` so the Python CLI's
     `StriatumError(exit_code=4)` mapping stays byte-equivalent.
   - `handlers.WorkAck(ctx, env, runner)` — mirrors
     `mutations.py:462-487`. Single update on `queue_messages` + `jobs` +
     `queue.acked` event inside one transaction. Returns
     `{"status": "acked", "job_id": ...}`.
   - `handlers.WorkComplete(ctx, env, runner)` — wraps
     `complete_job` in `src/striatum/db.py` (the existing implementation
     spans hundreds of lines of cycle/iteration logic). For Step 4 the Go
     handler calls back into the **same Python `striatum complete` CLI**
     via `subprocess.Popen` using the wire-stable JSON output, **only
     until** the Go translation of the cycle logic is its own PR. RFC 0039
     §9 Phase 2 explicitly tolerates this hybrid because the Python
     implementation is the reference and the Go binary owns the wire. The
     subprocess invocation runs in the daemon's `os.Geteuid()` process, not
     the CLI client's, which preserves the RFC 0031 "daemon is the writer"
     invariant. **Tagged hybrid handlers** are: `work.complete`,
     `work.block`, `review.verdict`, `review.submit`, `review.override`,
     `decision.record`, `checkpoint.resolve`, `branch.confirm`,
     `run.prepare`, `run.start`, `run.pause`, `run.resume`, `run.cancel`,
     `run.retry_job`, `recovery.requeue_stale`, `recovery.cancel_job`,
     `recovery.process_reconcile`, `recovery.resume`, `recovery.auto`,
     `recovery.watch`, `workflow.validate`, `workflow.generate`,
     `workflow.upgrade`, `workflow.init`, `artifact.publish`,
     `worktree.create`, `worktree.release`, `evidence.export`,
     `corpus.export`, `run.summary`, `run.graph`. The list is large but
     bounded; native Go translations land RFC 0039 V2 follow-up and are
     **out of scope** for Phase 2 per RFC 0039 §9. RFC 0043 §5 mandates
     the method be **registered**, not natively implemented.
   - **Pure-PG natives** (no subprocess hop): `session.register`,
     `session.close`, `work.claim_next`, `work.ack`, `work.heartbeat`,
     `work.release`, `work.send_message`, `repo.init`, `repo.list`,
     `worktree.list`, `list.runs/sessions/jobs/artifacts/workflows`,
     `run.summary` (read-only), `status`, `why`, `doctor`, `dashboard`,
     `workflow.validate`, `workflow.plan`, `workflow.graph`,
     `workflow.templates.list`, `workflow.templates.show`,
     `workflow.generate.preview`. These touch only the daemon-DB
     repo-local tables and are pure SQL; we land them as native Go
     transactions in Phase 2.
   - **Hybrid handlers** route the verb to `python -m striatum.cli` via a
     pre-resolved `STRIATUM_PYTHON_BIN` config (default
     `sys.executable` recorded at daemon start in `striatumd.daemon_meta`).

2. **Apply routes** — land natively in Go per §1.4 below.

3. **Cross-repo routes** — land natively in Go per §1.5 below.

**Subprocess-hop fail-closed posture (codex F1-F5 lesson):** if
`STRIATUM_PYTHON_BIN` is unset or the binary is missing, the daemon
refuses to bind the socket. The dispatcher does **not** silently downgrade
to "method_unknown" because that would be the same fail-open shape codex
caught in V1.5. The CI matrix asserts this in
`tests/test_daemon_go_python_bridge.py` (new, §6.3) by starting the Go
binary with `STRIATUM_PYTHON_BIN=/nonexistent/python` and asserting exit 11.

### 1.4 `go/pkg/apply/` — RFC 0031 sealed apply native to Go

V1.5 Python implementation lives in
`src/striatum/daemon_apply/apply_service.py:1-24` (the V1 refuse-on-no-key
shape) and `src/striatum/daemon_apply/signing_key.py` (key-load helper).
The Go implementation mirrors that surface fail-closed.

**File: `go/pkg/apply/receipt.go`** — apply-receipt schema. Mirror the
Python record produced by `daemon_apply` plus the audit-row schema in
`src/striatum/daemon_pg/sql/0002_rpc_supervision_apply.sql` (apply receipt
table). Receipt fields per RFC 0031 §4:

```go
type ApplyReceipt struct {
    ReceiptID          string    `json:"receipt_id"`
    PatchArtifactID    string    `json:"patch_artifact_id"`
    PatchDigestSHA256  string    `json:"patch_digest_sha256"`
    TargetRepositoryID string    `json:"target_repository_id"`
    ReviewerVerdictID  string    `json:"reviewer_verdict_id"`
    BaseTreeHash       string    `json:"base_tree_hash"`
    PostApplyTreeHash  string    `json:"post_apply_tree_hash"`
    SigningKeyID       string    `json:"signing_key_id"`
    DaemonVersion      string    `json:"daemon_version"`
    SubstrateSchema    int       `json:"substrate_schema"`
    AppliedAt          time.Time `json:"applied_at"`
}
```

Receipt rows are inserted into `striatumd.apply_receipts` (RFC 0033
substrate, daemon_pg migration 0002). The Markdown evidence artifact
required by RFC 0031 §4 lands via the same hybrid route as
`artifact.publish` — Go computes the receipt, calls the Python CLI's
`publish-artifact` verb with the rendered Markdown body, and stamps the
byline `author: striatumd-<instance_id>` derived from
`striatumd.daemon_meta.instance_id` (RFC 0031 acceptance criterion).

**File: `go/pkg/apply/service.go`** — `Service.ReviewedPatch(ctx,
params, runner, signingKey)` mirroring
`src/striatum/daemon_apply/apply_service.py:16-23`. Algorithm
(RFC 0031 §4 steps 1-6):

1. `signing_key.Load()` (`go/pkg/apply/key.go`, new file). If absent,
   return `RpcError{Code: "sealed_key_missing", Message: "sealed apply
   requires a daemon signing key"}` matching the Python denial code
   verbatim (`src/striatum/daemon_apply/apply_service.py:18-19`).
2. Verify `params["patch_artifact_id"]` exists, load bytes from the
   repo-local `artifacts` table, recompute SHA-256, compare with the
   recorded digest. Mismatch → `RpcError{"patch_digest_mismatch"}`.
3. Verify `params["reviewer_verdict_id"]` exists in `verdicts` table and
   `verdict.patch_digest_hash == params.patch_artifact_id`'s recorded
   digest. Mismatch → `RpcError{"verdict_digest_mismatch"}`.
4. Verify base-tree hash: shell out to `git rev-parse HEAD^{tree}` in the
   target repo's working directory (resolved from
   `striatumd.repositories.repo_root`). Mismatch →
   `RpcError{"base_tree_drift"}`.
5. Apply the patch to a daemon-owned worktree:
   `git worktree add <daemon_cache>/apply/<receipt_id> <base_commit>`,
   `git apply --index <patch_bytes>`. Failure →
   `RpcError{"apply_failed", details: stderr}`.
6. Compute post-apply tree hash via `git write-tree`. Insert the
   `striatumd.apply_receipts` row, commit, and call back into the Python
   `striatum publish-artifact --kind apply_receipt` to land the Markdown
   evidence. Return the receipt as the RPC response data.

**File: `go/pkg/apply/key.go`** — `Load()` reads
`${XDG_CONFIG_HOME:-~/.config}/striatum/daemon/signing_key` (Ed25519
private key bytes, mode 0600). RFC 0031 keyring fallback is **out of
scope for Phase 2** — Go inherits the same `0600 file in runtime` posture
as Python today. Future RFC handles keyring.

**Tests: `go/pkg/apply/service_test.go`** — base-tree drift, digest
mismatch, missing verdict, happy path with a synthetic Ed25519 key. The
happy-path test asserts the receipt row matches what the Python verifier
in `src/striatum/daemon_apply/apply_service.py` expects (RFC 0031
acceptance criterion: receipts cross-readable across cores).

**Python-side regression:** `tests/test_daemon_go_apply.py` (new) runs the
RFC 0031 apply-gate scenarios from `tests/test_apply_service.py` (current
location) against `daemon_core="go"`. Same fixtures, swap the harness
core, assert receipt bytes match.

### 1.5 `go/pkg/mcp/` — RFC 0032 MCP `tools/call` + `tools/list`

V1.5 Python `src/striatum/daemon_pg/mcp_dispatch.py:1-100` is the dispatch
bridge. Go mirrors it.

**File: `go/pkg/mcp/capabilities.go`** — `FilterTools(registry
[]rpc.MethodEntry, token *Token) []rpc.MethodEntry`. For each
`MethodEntry`, call `authorizer.Authorize(entry.RequiredCapability,
repositoryID, token)`; include only `decision == "allowed"`. The
single-repo entries are filtered against the token's bound `repository_id`
exactly as `auth_pg.go:84-126` already does. Cross-repo entries are
filtered against the token's `daemon_global` scope.

**File: `go/pkg/mcp/tools.go`** — two RPC handlers:

```go
func ToolsList(ctx, env, runner) (map[string]any, error)
func ToolsCall(ctx, env, runner) (map[string]any, error)
```

`ToolsList` reads `env.CapabilityToken`, runs `FilterTools`, returns
`{"tools": [...]}` with the MCP tool schema produced from each
`MethodEntry` (name, description, params schema version, required
capability — same shape Python emits in
`daemon_pg/mcp_dispatch.py:_tool_result`).

`ToolsCall` (RFC 0032 §5):
1. Resolve `name = params["name"]` to a `MethodEntry`; missing →
   audit-and-deny with `denial_reason="method_unknown"` matching
   `daemon_pg/mcp_dispatch.py:30-52`.
2. Re-authorize against `entry.RequiredCapability` via the
   `PostgresAuthorizer`. The MCP path **does not** trust the
   `tools/list` filter; it re-checks per RFC 0032 §6 prompt-injection
   safety. Denied → audit-and-deny.
3. Pack a synthetic `rpc.Envelope` with the same method + params and
   call the inner handler from §1.3. Wrap the response as an MCP
   `tools/call` result.

Tests: `go/pkg/mcp/capability_test.go` exercises filter precision (a
`read`-only token cannot see `write` methods in `tools/list`),
`go/pkg/mcp/dispatch_test.go` exercises the deny-path audit-row append
(matches the Python `denial_reason: capability_missing` row in
`tests/test_mcp_capability_scope_e2e.py`). Python-side
`tests/test_daemon_go_mcp.py` (new) replays the same scenarios against
`daemon_core="go"`.

### 1.6 `go/pkg/crossrepo/` — RFC 0032 cross-repo lifecycle

V1.5 Python lives in `src/striatum/cross_repo.py:47-160` (prepare),
`describe_cross_repo_run` (status read), and the cancel path explicitly
deferred in `dispatch.py:968-973`. The Go port lands prepare + describe +
list natively; cancel stays deferred to the multi-repo daemon harness
follow-up per `docs/TODO.md` Open item 19.

**File: `go/pkg/crossrepo/prepare.go`** — `Prepare(ctx, workflow,
runner, localRunner LocalRunner)` mirrors `prepare_cross_repo_run` in
`src/striatum/cross_repo.py:47-160`. Inserts the `striatumd.cross_repo_runs`
row in state `preparing`, iterates `repositories` map, calls
`localRunner.Prepare(...)` per repo, updates each
`striatumd.cross_repo_run_repositories` row to state `prepared`. On
exception, rolls back to `aborted` with `last_reconcile_error` set
(`cross_repo.py:112-130`).

**File: `go/pkg/crossrepo/lifecycle.go`** —
`List`, `Describe`, `Why`, `Cancel`. `Cancel` returns the same
`RpcError{"not_implemented", "cross-repo cancel requires the daemon
lifecycle service"}` shape the Python path emits in
`src/striatum/daemon_rpc/server.py:258-262`. Parity with the deferred
state is the test surface.

`LocalRunner` interface in Go matches the Python `Protocol` in
`cross_repo.py:20-37` (`prepare`, `start`, `cancel`, `participant_intact`,
`human_checkpoint`). The default `LocalRunner` implementation is a
subprocess hop into the Python CLI's `run prepare` / `run start` /
`run cancel` verbs (same hybrid posture as §1.3 hybrids), since
cross-repo local-runner work spans the workflow planner that
remains Python-side.

Tests: `go/pkg/crossrepo/prepare_test.go` uses an in-memory `LocalRunner`
mock matching the dogfood-035 unit-testable shape. Python-side
`tests/test_daemon_go_crossrepo.py` re-runs the existing
`tests/test_cross_repo_prepare_e2e.py` against `daemon_core="go"`.

### 1.7 Track A test surface

- `go test go/pkg/rpc/...` — registry parity (§1.2), envelope round-trip
  (V1.5), MCP filter (§1.5).
- `go test go/pkg/apply/...` — sealed-apply happy + four refusal paths
  (§1.4).
- `go test go/pkg/crossrepo/...` — prepare + describe + deferred-cancel
  (§1.6).
- `tests/test_daemon_go_mutations.py` (new): boots
  `MultiRepoHarness(daemon_core="go")` and exercises **every** mutation
  method in the §1.2 table via the Python CLI client. Asserts each
  method returns ok via the Go daemon and an audit row lands with the
  Go daemon version stamped on it. Refusal paths: a `read`-only token
  hitting a `write` method must record `denial_reason: capability_missing`
  byte-equivalent to the Python core.
- `tests/test_daemon_go_apply.py`, `tests/test_daemon_go_mcp.py`,
  `tests/test_daemon_go_crossrepo.py` (new): per-track parity tests.

`make test-multi-repo CORE=go` (`Makefile:82-92`) is the umbrella that
runs all five new test files plus the existing dogfood-047 smoke + audit.
The codex F1.6-C sentinel (§0.1) guards against skip-passes.

## 2. Track B — Supervisor in Go (`go/pkg/supervisor/`)

V1 Python supervisor lives in `src/striatum/supervisor.py:47-866`
(spawn/send/stop/status/list) plus
`src/striatum/daemon_supervisor/pointer.py:11-71` (repo-local pointer
table) plus `src/striatum/daemon_supervisor/progress_watcher.py:33-79`
(supervised-progress lease heartbeat). Track B ports the daemon-owned
supervisor surface to Go using `os/exec` + `creack/pty`.

### 2.1 `go/pkg/supervisor/pointer.go` — daemon-DB pointer row writes

Daemon DB now owns the authoritative supervisor table per RFC 0031 §1.
The Python implementation writes via
`src/striatum/daemon_supervisor/pointer.py` (repo-local) and
`src/striatum/supervisor.py:102-120` (V1 SQLite path). Post-D094 both rows
live in `striatumd.process_supervisors` (daemon-DB) and
`striatumd.process_supervisor_pointers` (daemon-DB, same migration). Go
implementation:

```go
type Pointer struct {
    SupervisorID         string
    DaemonSupervisorID   string
    RunID                string
    SessionID            string
    PID                  int
    PIDStartTime         string
    State                string // starting | attached | detached | lost | stopped
    UpdatedAt            time.Time
}

func RecordPointer(ctx context.Context, tx db.TxRunner, p Pointer) error
```

Mirrors `record_pointer` in
`src/striatum/daemon_supervisor/pointer.py:11-52` byte-for-byte: same
field set, same `ON CONFLICT(supervisor_id) DO UPDATE` shape, same
"attached requires pid_start_time" invariant
(`daemon_supervisor/pointer.py:22-23`).

### 2.2 `go/pkg/supervisor/pty.go` — spawn supervised lanes

Mirrors `supervise_start` in `src/striatum/supervisor.py:47-227`. Algorithm:

1. Load the lane config from the workflow snapshot in
   `striatumd.workflow_snapshots`. Refuse if `lane.adapter != "process"`
   (`supervisor.py:73-76`).
2. Refuse if the session already has a supervisor in
   `("starting", "attached", "detached")` (`supervisor.py:79-90`,
   constant `SUPERVISOR_ACTIVE_STATES` in `supervisor.py:44`).
3. Resolve `scratch = state_dir(repo) / "scratch" / supervisor_id` and
   `pipe_path = scratch / "stdin.pipe"`. Create the FIFO with
   `syscall.Mkfifo(pipe_path, 0o600)`. This matches
   `supervisor.py:93-98` byte-for-byte.
4. Open the FIFO `O_RDWR | O_NONBLOCK` in the daemon process to keep the
   kernel-level "has writer" flag set for the supervised child's
   lifetime (the POSIX trick documented in `supervisor.py:133-145`).
   Clear `O_NONBLOCK` before handing the fd to the child.
5. Build the command: `cmd := exec.CommandContext(ctx, cmdArgs[0],
   cmdArgs[1:]...)`. Set `cmd.Dir`, `cmd.Env` from
   `_supervised_env` in `supervisor.py:675-693` (the
   `STRIATUM_SCRATCH_DIR`, `STRIATUM_REPO`, `STRIATUM_RUN_ID`,
   `STRIATUM_SESSION_ID`, `STRIATUM_SUPERVISOR_ID` environment).
6. Use `creack/pty.StartWithSize(cmd, &pty.Winsize{...})` to attach a
   pseudo-terminal **for lanes whose workflow declares
   `pty: true`** (lane-config option). Lanes without `pty: true` keep
   plain `stdin = pipe_fd`, `stdout = stderr = os.DevNull` (Python default
   at `supervisor.py:152-156`). The PTY upgrade is opt-in per lane; the
   Python core does not have PTY today (it sets `stdout=DEVNULL`,
   `stderr=DEVNULL`), so Go-introduced PTY is gated to lanes that
   explicitly request it. RFC 0010 lane-config option name:
   `pty: true | false`, default false.
7. `cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}` so the child is
   in a new session (equivalent to Python `start_new_session=True` at
   `supervisor.py:155`).
8. Start the child. Capture `cmd.Process.Pid`. Stat
   `/proc/<pid>/stat` for `pid_start_time` (Linux; `darwin` uses
   `sysctl` via `golang.org/x/sys/unix.SysctlKinfoProc` since `proc` is
   not mounted on macOS). Same field as
   `src/striatum/identity.py::process_start_time` consumes.
9. Insert `striatumd.process_supervisors` row in state `attached`. Insert
   the `process_supervisor_pointers` row via §2.1. Record the
   `supervisor.started` audit row via the same `AuditRecorder`
   (`go/pkg/db/audit.go:55-197`) — the supervisor lifecycle is part of the
   chain.

### 2.3 `go/pkg/supervisor/pty.go` — packet delivery (FIFO byte protocol)

The Python wrapper bytes contract is locked at
`.striatum/bin/claude-supervised-wrapper.sh:45-62`: line-delimited UTF-8
JSON packets, no trailing whitespace allowed, no embedded null bytes,
`\n` separates packets. Mirror via:

```go
func (s *Supervisor) SendPacket(ctx context.Context, p Pointer, payload []byte) error {
    fd, err := syscall.Open(p.StdinPipePath, syscall.O_WRONLY, 0)
    if err != nil { return err }
    defer syscall.Close(fd)
    if !bytes.HasSuffix(payload, []byte("\n")) {
        payload = append(payload, '\n')
    }
    return writeAll(fd, payload)
}
```

`writeAll` retries on partial writes the same way
`src/striatum/supervisor.py:827-851` `_write_to_pipe` does, surfacing
`BrokenPipeError` (POSIX `EPIPE`) as `RpcError{"supervisor_pipe_broken",
"supervisor pipe is broken; child has closed stdin"}` byte-equivalent to
the Python message (`supervisor.py:843-845`). The byte contract for the
packet itself is RFC 0010 V2's "one JSON object per line, packet
identifier in `packet_id`" — no other framing required because the wrapper
shells `claude --print` per line.

### 2.4 `go/pkg/supervisor/liveness.go` — heartbeat + lost-detection

Mirrors `src/striatum/daemon_supervisor/progress_watcher.py:33-79`.
Configuration constants are the same:

```go
type WatcherConfig struct {
    PollInterval       time.Duration // default 30s
    RefreshThreshold   time.Duration // default 60s
    IdleThreshold      time.Duration // default 600s
    HeartbeatExtend    int           // default 900 seconds
}
```

Algorithm per tick:
1. Enumerate `striatumd.process_supervisors WHERE state = 'attached'`.
2. For each, check `/proc/<pid>/stat[2]` start-time-jiffies (Linux) or
   `sysctl KERN_PROC_PID` (macOS). If pid mismatch / start-time mismatch
   → transition to `lost`, emit `supervisor.lost` audit row.
3. Stat the lane's scratch dir (`<scratch>/<supervisor_id>/`) for the
   latest mtime across watched files (`logs/packet-NNNN.log` paths the
   wrapper writes per `.striatum/bin/claude-supervised-wrapper.sh:48-53`).
   Compare with `now - refresh_threshold`. If recent → extend the
   current active lease (`leases.expires_at = now() + heartbeat_extend`).
   Lazy lease expiry survives per D036; the supervisor only refreshes,
   never expires.
4. Run in a goroutine started by `cmd/striatumd/main.go` alongside the RPC
   server. Cancellation via the same `signal.NotifyContext`
   (`go/cmd/striatumd/main.go:44-45`) that drives socket shutdown.

### 2.5 SIGTERM cleanup + signal handling

Replaces the V1 Python signal-handler stub
(`src/striatum/daemon.py:916-921`). Go pattern:

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

var wg sync.WaitGroup
// supervisor watcher goroutine
wg.Add(1); go func() { defer wg.Done(); watcher.Run(ctx) }()
// RPC server goroutine
wg.Add(1); go func() { defer wg.Done(); server.Serve(ctx, listener) }()

<-ctx.Done()
// Stop supervised children deterministically (RFC 0031 §3 5s grace, then SIGKILL).
shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
supervisor.StopAll(shutdownCtx)
wg.Wait()
```

`supervisor.StopAll` iterates `process_supervisors WHERE state IN ('starting',
'attached')`, sends `syscall.Kill(-pid, syscall.SIGTERM)` (negative pid →
process group, the standard pattern for `setsid`-detached children), waits
up to 5 seconds, then `syscall.SIGKILL` on the holdouts. Records
`supervisor.stopped` with `stop_reason: "daemon_shutdown"` per
`src/striatum/supervisor.py:307-409`'s `supervise_stop` payload shape.

### 2.6 Pidfile + socket-path lock (mutual-exclusion guarantee)

D094 / RFC 0039 §3 requires the Python and Go cores to be mutually
exclusive in a given run. Land that:

1. **Pidfile path:** `${XDG_RUNTIME_DIR}/striatum/striatumd.pid` (Go uses
   the same path as Python `src/striatum/daemon.py:128 pid_path()`).
2. **Lock acquisition:** Go binary opens
   `${XDG_RUNTIME_DIR}/striatum/striatumd.lock` with
   `flock(LOCK_EX|LOCK_NB)`. If contention, exit non-zero with
   `daemon_already_running` and stderr pointing at the existing pid.
3. **Cross-core refusal:** before binding the socket, check if the
   pidfile exists and contains a live PID. If so, refuse with exit code 7
   (`daemon_already_running`) and a stderr message naming whichever
   binary is currently bound (`/proc/<pid>/comm` on Linux,
   `lsof -p <pid>` on macOS). Symmetric refusal lives in the Python path
   already (`src/striatum/daemon.py:896-898`).
4. **Socket path:** same path as Python (`runtime_dir() /
   "striatumd.sock"`, `src/striatum/daemon.py:124-125`). The CLI clients
   speak to whichever binary opened it; the pidfile + socket are 1:1
   tied to whichever core started first.

### 2.7 Track B test surface

- `go test go/pkg/supervisor/...` covers:
  - `TestSupervisorStart` — spawn a tiny shell wrapper, assert pid/start-time
    captured.
  - `TestSupervisorPacket` — write a packet to the FIFO, assert child
    reads it intact.
  - `TestSupervisorLost` — kill the child externally, run a watcher tick,
    assert transition to `lost`.
  - `TestSupervisorSIGTERM` — start two supervised children, send the
    daemon SIGTERM, assert both children stop within the 5s grace.
  - `TestSupervisorPIDStartTimeReattach` — record a supervisor, restart
    the daemon, assert reattach succeeds; record again with a *changed*
    pid_start_time (simulate PID recycling), assert transition to `lost`.
- `tests/test_daemon_go_supervisor.py` (new): boot
  `MultiRepoHarness(daemon_core="go")`, supervise a fake-claude lane
  command (a tiny bash script under `tests/fixtures/fake-claude.sh` that
  acks the packet and writes a marker file), assert packet delivery +
  heartbeat + clean shutdown.

## 3. Distribution: cross-compile + wheel binary payload

### 3.1 `go/Makefile` per-platform targets

`go/Makefile:1-16` currently builds one binary into `go/bin/striatumd`.
Add per-platform targets:

```make
PLATFORMS := linux-amd64 linux-arm64 darwin-amd64 darwin-arm64

bin/striatumd-%:
	@os=$$(echo $* | cut -d- -f1); arch=$$(echo $* | cut -d- -f2); \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -ldflags="-s -w" \
		-o bin/striatumd-$$os-$$arch ./cmd/striatumd

build-all: $(PLATFORMS:%=bin/striatumd-%)
```

`CGO_ENABLED=0` ensures the binary is statically linked per RFC 0039 §5.
`-ldflags="-s -w"` strips DWARF + symbol tables for smaller artifacts.
Each `bin/striatumd-<os>-<arch>` is the platform-tagged binary CI uploads
to the release.

### 3.2 Top-level `Makefile` daemon-go-* targets

`Makefile:70-77` already has `daemon-go-build`, `daemon-go-test`,
`daemon-go-lint`. Add `daemon-go-build-all` and `daemon-go-release`:

```make
.PHONY: daemon-go-build-all daemon-go-release

daemon-go-build-all:
	$(MAKE) -C "$(MAKEFILE_DIR)/go" build-all

daemon-go-release: daemon-go-build-all
	@for plat in linux-amd64 linux-arm64 darwin-amd64 darwin-arm64; do \
		sha256sum "$(MAKEFILE_DIR)/go/bin/striatumd-$$plat" \
		> "$(MAKEFILE_DIR)/go/bin/striatumd-$$plat.sha256"; \
	done
```

The `.sha256` sidecars are what `release.yml` (`.github/workflows/release.yml`)
attaches alongside the binaries.

### 3.3 Wheel binary payload (`src/striatum/_daemongo/`)

The prompt requires a package-data shim shipping the per-platform Go binary
**inside the Python wheel** (binary-payload pattern like
`psycopg[binary]`). Two implementation options were considered:

**Option A (recommended): single wheel with per-platform binary** —
publish four platform-specific wheels (`striatum_orchestrator-<v>-py3-none-linux_x86_64.whl`,
etc.) where each wheel contains exactly one binary at
`striatum/_daemongo/<os>_<arch>/striatumd`. The wheel is otherwise
pure-Python, but the platform tag pins it to one OS+arch combination.
Operators install via `pip install 'striatum-orchestrator[daemon-go]'`
and pip auto-selects the right wheel. This is the pattern `psycopg-binary`
uses.

**Option B (rejected): one wheel with all four binaries** — the wheel
ships every platform's binary and uses runtime resolution to pick the
right one. Rejected because the wheel grows to ~80 MB (4 × ~20 MB Go
binaries) and pip downloads the union for every install.

Implementation:

1. **New package: `src/striatum/_daemongo/__init__.py`** — exposes:

   ```python
   from pathlib import Path
   import sys

   def resolve_binary() -> Path | None:
       """Return the path to the bundled Go daemon binary if present, else None."""
       here = Path(__file__).parent
       binary = here / "striatumd"
       if binary.is_file() and binary.stat().st_mode & 0o111:
           return binary
       return None
   ```

2. **`pyproject.toml:47-55` `[tool.setuptools.package-data]`** — add:

   ```toml
   "striatum._daemongo" = ["striatumd", "striatumd.exe"]
   ```

3. **`pyproject.toml`** — add a new optional dependency group (no extra
   PyPI dep, but it serves as the canonical install hint):

   ```toml
   [project.optional-dependencies]
   daemon-go = []   # binary lands via platform wheel; nothing to pin
   ```

4. **`MANIFEST.in`** — include the binary if present:

   ```
   include src/striatum/_daemongo/striatumd
   include src/striatum/_daemongo/striatumd.exe
   ```

5. **`scripts/build_platform_wheels.sh`** (new) — invoked by `release.yml`.
   For each `(os, arch)`:
   1. Build the Go binary via `make daemon-go-build-all`.
   2. Copy `go/bin/striatumd-<os>-<arch>` to
      `src/striatum/_daemongo/striatumd` (with executable bit).
   3. Run `python -m build --wheel` with `--config-setting=--build-option=--plat-name=<pep425_tag>`.
   4. Move the wheel under `dist/` with the platform-tagged name.
   5. Restore the empty `src/striatum/_daemongo/` for the next iteration.

6. **CLI resolver order** (re-stated from §1.1 step 3): `args.go_binary`
   → `STRIATUMD_GO_BIN` env var → `striatum._daemongo.resolve_binary()` →
   `<repo>/go/bin/striatumd` → `shutil.which("striatumd")`. This means an
   operator who `pip install`'d the wheel gets the bundled binary; a
   contributor running `make daemon-go-build` in-tree gets `go/bin/striatumd`
   because the wheel's `_daemongo/striatumd` is absent.

### 3.4 CI matrix for cross-compile

`.github/workflows/release.yml:18-60` currently builds one wheel + sdist.
Replace `build:` with a matrix:

```yaml
build:
  name: Build distributions
  runs-on: ${{ matrix.os }}
  strategy:
    fail-fast: false
    matrix:
      include:
        - os: ubuntu-latest
          go_os: linux
          go_arch: amd64
          plat_name: manylinux2014_x86_64
        - os: ubuntu-latest
          go_os: linux
          go_arch: arm64
          plat_name: manylinux2014_aarch64
        - os: macos-latest
          go_os: darwin
          go_arch: amd64
          plat_name: macosx_11_0_x86_64
        - os: macos-latest
          go_os: darwin
          go_arch: arm64
          plat_name: macosx_11_0_arm64
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
      with:
        go-version: '1.23'
    - uses: actions/setup-python@v5
      with:
        python-version: '3.12'
    - name: Verify tag matches pyproject version
      # (unchanged from current release.yml:30-44)
    - name: Build Go daemon for ${{ matrix.go_os }}/${{ matrix.go_arch }}
      run: |
        cd go
        GOOS=${{ matrix.go_os }} GOARCH=${{ matrix.go_arch }} CGO_ENABLED=0 \
          go build -ldflags="-s -w" -o ../src/striatum/_daemongo/striatumd ./cmd/striatumd
        chmod +x ../src/striatum/_daemongo/striatumd
    - name: Build wheel
      run: |
        python -m pip install --upgrade pip build twine
        python -m build --wheel --config-setting=--build-option=--plat-name=${{ matrix.plat_name }}
    - name: Verify with twine
      run: python -m twine check --strict dist/*
    - uses: actions/upload-artifact@v4
      with:
        name: dist-${{ matrix.go_os }}-${{ matrix.go_arch }}
        path: dist/
```

The `pypi` and `github-release` jobs gather all four matrix outputs via
`actions/download-artifact@v4` with `pattern: dist-*`. The sdist is built
once on `ubuntu-latest` (no Go binary needed) and uploaded separately.

## 4. CI matrix `daemon_core={python,go}` on Linux + macOS

### 4.1 New CI job: `daemon-core-matrix`

`.github/workflows/ci.yml:7-46` currently runs one `test:` job. Add a
second top-level job after `test:`:

```yaml
daemon-core-matrix:
  needs: test
  runs-on: ${{ matrix.os }}
  strategy:
    fail-fast: false
    matrix:
      os: ["ubuntu-latest", "macos-latest"]
      daemon_core: ["python", "go"]
  services:
    postgres:
      image: postgres:16
      env:
        POSTGRES_PASSWORD: striatum
        POSTGRES_DB: striatum_daemon
      ports: ["5432:5432"]
      options: >-
        --health-cmd "pg_isready -U postgres"
        --health-interval 5s
        --health-timeout 3s
        --health-retries 10
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-python@v5
      with: { python-version: "3.12" }
    - uses: actions/setup-go@v5
      with: { go-version: "1.23" }
    - name: Install Python deps
      run: python -m pip install -e ".[dev,daemon-pg]"
    - name: Build Go daemon (only when daemon_core == go)
      if: matrix.daemon_core == 'go'
      run: make daemon-go-build
    - name: Run multi-repo matrix
      env:
        STRIATUM_DAEMON_DB_URL: postgresql://postgres:striatum@localhost:5432/striatum_daemon
        STRIATUM_PG_TEST_URL: postgresql://postgres:striatum@localhost:5432/striatum_daemon
      run: make test-multi-repo CORE=${{ matrix.daemon_core }}
    - name: Run Go unit tests (only when daemon_core == go)
      if: matrix.daemon_core == 'go'
      run: make daemon-go-test
```

**Service-container caveat on macOS:** GitHub Actions does not support
`services:` on `macos-latest`. The macOS branch boots Postgres via
Homebrew (`brew install postgresql && brew services start postgresql`).
The release matrix in §3.4 already pays this cost; we reuse the same
boot helper at `scripts/ci_macos_pg.sh` (new). Linux uses `services:`.

**Job names** (RFC 0039 §10 acceptance criterion):
- `daemon-core-matrix (ubuntu-latest, python)`
- `daemon-core-matrix (ubuntu-latest, go)`
- `daemon-core-matrix (macos-latest, python)`
- `daemon-core-matrix (macos-latest, go)`

Both `CORE=python` and `CORE=go` are required-status checks on PRs;
this is what closes RFC 0039 §10 "CI runs the Python and Go daemon test
matrices on every PR."

### 4.2 New `test-multi-repo` test files

Beyond the dogfood-047 entries already in `Makefile:82-92`, the matrix
gains these Phase 2 test files (added to the test list in
`Makefile:82-92`):

- `tests/test_daemon_go_mutations.py` (Track A §1.7)
- `tests/test_daemon_go_apply.py` (Track A §1.4)
- `tests/test_daemon_go_mcp.py` (Track A §1.5)
- `tests/test_daemon_go_crossrepo.py` (Track A §1.6)
- `tests/test_daemon_go_supervisor.py` (Track B §2.7)
- `tests/test_daemon_go_python_bridge.py` (Track A §1.3 fail-closed)
- `tests/test_daemon_go_core_sentinel.py` (Pre-gate F1.6-C §0.1)
- `tests/test_daemon_method_registry_parity.py` (Track A §1.2)

The `Makefile:82-92` `test-multi-repo` target becomes:

```make
test-multi-repo: $(VENV)/.installed
	STRIATUM_MULTI_REPO_DAEMON_CORE=$(CORE) \
	$(PYTHON) -m pytest -m multi_repo \
		tests/test_multi_repo_harness.py \
		tests/test_cross_repo_prepare_e2e.py \
		tests/test_cross_repo_lifecycle_e2e.py \
		tests/test_cross_repo_crash_recovery_e2e.py \
		tests/test_mcp_capability_scope_e2e.py \
		tests/test_per_repo_write_scope_e2e.py \
		tests/test_daemon_go_smoke.py \
		tests/test_daemon_go_audit.py \
		tests/test_daemon_go_mutations.py \
		tests/test_daemon_go_apply.py \
		tests/test_daemon_go_mcp.py \
		tests/test_daemon_go_crossrepo.py \
		tests/test_daemon_go_supervisor.py \
		tests/test_daemon_go_python_bridge.py \
		tests/test_daemon_go_core_sentinel.py \
		tests/test_daemon_method_registry_parity.py
```

### 4.3 Skip semantics + sentinel

`tests/conftest.py:18-25` reads `STRIATUM_MULTI_REPO_DAEMON_CORE`. When
the value is `"go"` but the binary cannot be built (no `make`/`go` in
PATH) or Postgres is unreachable, today the harness raises a fixture-time
error or skips. The codex F1.6-C sentinel guards against "all skips ⇒
green" by introducing
`tests/test_daemon_go_core_sentinel.py::test_core_matrix_runs_at_least_one_real_assertion`,
which:

```python
@pytest.mark.multi_repo
def test_core_matrix_runs_at_least_one_real_assertion(
    multi_repo_harness: MultiRepoHarness, daemon_core: DaemonCore
) -> None:
    if daemon_core != "go":
        pytest.skip("sentinel only fires on CORE=go")
    # If Postgres is unreachable here, fail loudly rather than skipping.
    assert multi_repo_harness.socket_path.exists(), (
        "daemon-go socket should exist when sentinel runs"
    )
    rows = multi_repo_harness.daemon_db_query(
        "SELECT count(*) AS n FROM striatumd.audit_log"
    )
    assert rows[0]["n"] >= 1, "CORE=go matrix produced zero audit rows"
```

This test does not skip; it fails if Postgres is unreachable, which is
the correct CI signal.

## 5. Acceptance criteria recap (against RFC 0039 §Acceptance Criteria)

| RFC 0039 acceptance line | Phase 2 evidence |
|--------------------------|------------------|
| `go/` directory layout (cmd/pkg as per §1) | Already shipped; Track A adds `pkg/apply/`, `pkg/mcp/`, `pkg/crossrepo/`, `pkg/rpc/handlers/`. Track B adds `pkg/supervisor/`. |
| Full envelope-v1 + handshake + method registry + capability gating | Track A §1.2 + §1.3 plus existing V1.5 PostgresAuthorizer. Registry parity test asserts byte-equivalence with Python. |
| Reads/writes RFC 0033 substrate using same schema + migrations | Track A native handlers query daemon-DB tables added by `daemon_pg/sql/0005_repo_local_workflow_state.sql`. The migration applies via `--migrate` (already wired in `go/cmd/striatumd/main.go:54-66`). |
| Daemon owns supervised processes per RFC 0031 + PTY + signal handling + supervised-progress heartbeat | Track B §2.1-§2.5. |
| RFC 0032 cross-repo lifecycle + MCP `tools/call` + `tools/list` + audit append | Track A §1.5 + §1.6. Cancel path deferred per current Python parity. |
| `striatum daemon start --core go` launches the Go daemon binary | Track A §1.1. |
| `MultiRepoHarness(daemon_core="go")` boots and runs all five e2e files green | Already true post-V1.5 for the read surface; Track A makes the write surface land. CI matrix in §4.1. |
| CI runs Python and Go daemon test matrices on every PR | §4.1, four jobs. |
| Cross-compile produces four binaries on release | §3.1 + §3.4. |
| Distribution: release ships wheel + four Go binaries per platform | §3.3 platform wheels + §3.4 release.yml matrix. |
| Documentation updates (SPEC / HOW_TO_HUMAN / CHANGELOG) | **Out of scope per the prompt**: operator-only after dogfood lands. |
| No regression in any existing Python test | CI `test:` job (`.github/workflows/ci.yml:7-46`) runs the full Python suite on every PR unchanged. |

## 6. Non-negotiables, restated

- **Backward-compat (RFC 0039 §9 Phase 1):** `striatum daemon start`
  default core stays `python`. `--core go` is opt-in. Flipping the
  default is a separate future RFC. `src/striatum/cli/daemon.py`
  `dispatch_daemon_start_go` returns `None` whenever the resolved core
  is `"python"`, so the existing call site in
  `src/striatum/cli/dispatch.py:883-888` falls through unchanged.
- **D094 framing:** one substrate (Postgres), two mutually-exclusive
  cores. Pidfile + socket-path lock (§2.6) enforces "only one core has
  the socket bound at a time." No parallel SQLite path; no per-language
  substrate.
- **No transcript capture:** D028 unchanged. The supervisor wrapper
  (`.striatum/bin/claude-supervised-wrapper.sh:13-18`) writes per-packet
  logs to scratch for operator debug only; the daemon never reads them.
  The Go watcher (§2.4) stats file mtimes, not contents.
- **Wire protocol unchanged:** RFC 0030 envelope-v1 over Unix socket
  (`docs/rfcs/0030-daemon-rpc-server-and-version-skew-protocol.md:74-167`)
  is the contract. No protobuf, no gRPC. Python CLI clients
  (`src/striatum/daemon_rpc/client.py`) talk to either core unmodified.
- **No malicious-operator threat model:** RFC 0031 §"Threat Model"
  unchanged. The signing key stays `0600` operator-readable; sealed
  apply is an AI guardrail, not a cryptographic non-repudiation primitive.
- **Out-of-scope:** Windows daemon (RFC 0039 Non-Goals); CLI rewrite in
  Go (RFC 0039 Non-Goals); multi-machine / hosted-mode daemon (D083);
  Prometheus metrics (RFC 0039 OQ); flipping the `--core go` default
  (RFC 0039 §9 Phase 2); README / TODO / CHANGELOG / SPEC / HOW_TO
  updates (operator-only after lands per the prompt).

## 7. Implementation order summary (lock for synthesis)

1. **Pre-gate (F1.6-A through F1.6-E):** §0.1. Land in one PR; gates
   Step 4.
2. **Track A Step 3:** §1.1 + §1.2 + registry parity test (§1.7) +
   Step 3 pure-PG handlers + python-bridge hybrid handlers (§1.3).
   `striatum daemon start --core go` end-to-end with read verbs +
   pure-PG mutation verbs.
3. **Track A Step 4 mutation surface:** §1.3 hybrid handlers; §1.4
   apply; §1.5 MCP; §1.6 cross-repo. Track A complete.
4. **Track B Step 5 supervisor:** §2.1-§2.7. Lands after Step 4 because
   the supervisor uses the apply service for sealed-mode lanes.
5. **Step 6 distribution + CI:** §3.1-§3.4 cross-compile, §3.3 wheel
   binary payload, §4.1 CI matrix. Land in one PR after Step 5 because
   the wheel-payload pattern depends on the supervisor binary being the
   real one shipped.

Each step ships green test parity before the next starts. RFC 0039 §
Implementation Plan locks this ordering; Phase 2 follows it
unchanged.

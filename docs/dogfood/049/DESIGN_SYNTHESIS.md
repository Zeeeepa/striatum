---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/049/design/codex/DESIGN.md", "docs/dogfood/049/design/claude_code/DESIGN.md", "docs/dogfood/049/design/gemini/DESIGN.md"]
---
author: designer-unknown-model-002

# RFC 0039 Phase 2 Design Synthesis

This synthesis locks one implementation plan for RFC 0039 Phase 2. The
default daemon core remains `python`; `--core go` is opt-in only, and this
dogfood does not flip the default. Under D094 and RFC 0043, the Go daemon
speaks the RFC 0030 envelope over the same daemon-owned Postgres schema as the
Python daemon. There is no SQLite fallback and no Go-only schema.

Two tracks run in parallel:

- Track A, assigned to codex, owns Python CLI integration for selecting the Go
  core and the Go RPC mutation surface: `src/striatum/cli/parser.py`,
  `src/striatum/cli/daemon.py`, `src/striatum/cli/dispatch.py`,
  `go/pkg/rpc/`, `go/pkg/apply/`, `go/pkg/mcp/`, and `go/pkg/crossrepo/`.
- Track B, assigned to claude, owns Go supervision, binary distribution, and
  CI evidence: `go/pkg/supervisor/`, `go/Makefile`, top-level `Makefile`,
  `src/striatum/_daemongo/`, `pyproject.toml`, `MANIFEST.in`, and
  `.github/workflows/`.

When the three designs disagreed, this plan picks the current source tree and
the work-packet prompt over older module names or broader packaging choices.
The concrete choices are: no new `src/striatum/cli/daemon_go_launcher.py`;
launcher helpers live in `src/striatum/cli/daemon.py`; the package-data layout
is `src/striatum/_daemongo/bin/<os>-<arch>/striatumd`; and CI has explicit
`multi-repo-python` and `multi-repo-go` jobs, not one in-process core matrix.

## Shared Gate

Before either track routes real mutations through the Go core, the dogfood-047
correctness holes must close in this branch:

1. `go/cmd/striatumd/main.go` must fail closed when serving without
   `--postgres-url`. `AllowAllAuthorizer` stays test-only; `--describe` may
   remain a local metadata command.
2. `go -C go mod tidy` must populate `go/go.sum`. Track B records the required
   `github.com/creack/pty` line in handoff; Track A owns the actual
   `go.mod`/`go.sum` edit.
3. `make test-multi-repo CORE=go` must fail when Postgres is missing or when
   all selected tests skip.
4. `tests/test_daemon_go_smoke.py` must assert unauthenticated denial uses
   `capability_missing`.
5. `tests/test_daemon_go_audit.py` and Go audit tests must execute in CI
   against ephemeral Postgres instead of skipping silently.

## Track A

Add the operator flag at `src/striatum/cli/parser.py` immediately after the
current `daemon start` `--json` line, which is line 144 in the current file:

```python
daemon_start.add_argument("--core", choices=["python", "go"], default="python")
```

Do not add an environment-variable default that can silently select Go.
Operators must pass `striatum daemon start --core go`; CI may set `CORE=go`
for make targets, but CLI runtime selection is the explicit flag. The current
daemon-start body is `src/striatum/cli/parser.py:140-144`.

Branch in `src/striatum/cli/dispatch.py` at `_dispatch_daemon`, currently
`src/striatum/cli/dispatch.py:880-888`. Replace the direct
`daemon_mod.run_daemon_foreground(...)` call with a call into
`src/striatum/cli/daemon.py::dispatch_daemon_start`. That helper returns a
result for `args.core == "go"` and returns `None` for `python`, preserving the
existing Python path. Keep `src/striatum/cli/daemon.py::dispatch_daemon` for
`migrate-repo-local`.

Launch the Go daemon with `subprocess.Popen`, not `os.execve`. `Popen` is the
chosen shape because the Python CLI must resolve the binary, pass the shared
runtime socket and Postgres URL, wait up to 10 seconds for the socket, and
return the existing JSON startup envelope. Use `start_new_session=True` and
inherit stdout/stderr. The argv is:

```text
<resolved-striatumd> --socket <runtime_dir>/striatumd.sock --postgres-url <url> --migrate --migrations-sha-source src/striatum/daemon_pg/sql
```

Resolve the Go binary in this order: shipped wheel binary via
`striatum._daemongo.resolve_binary()`, `STRIATUMD_GO_BIN`, then in-tree
`go/bin/striatumd`. This matches the packet's package-data invariant and keeps
editable checkouts working. Use the existing socket and pidfile paths from
`src/striatum/daemon.py:124-129`; Python already refuses a live pid at
`src/striatum/daemon.py:896-898`, and the Go binary must take the same
pidfile plus socket-path lock.

### RPC Registry

Make `go/pkg/rpc/registry.go` byte-parity with
`src/striatum/daemon_rpc/registry.py:48-159`. The current Go table at
`go/pkg/rpc/registry.go:76-128` is incomplete and has stale capabilities:
`workflow.validate`, `run.prepare`, `run.start`, `session.register`,
`ack`, `heartbeat`, `publish_artifact`, `complete`, `release`, and
`supervise.*` are currently too write-heavy or legacy-named. The canonical
method names and capabilities are:

| Method(s) | Capability | Scope |
|---|---|---|
| `daemon.hello` | none | `daemon_global` |
| `daemon.describe` | `read` | `daemon_global` |
| `status`, `why`, `doctor`, `dashboard`, `evidence.export`, `corpus.export`, `run.summary`, `run.graph`, `workflow.validate`, `workflow.plan`, `workflow.graph`, `workflow.templates.list`, `workflow.templates.show`, `workflow.generate.preview`, `list.runs`, `list.sessions`, `list.jobs`, `list.artifacts`, `list.workflows`, `worktree.list` | `read` | `single_repo` |
| `dashboard.all`, `repo.list` | `read` | `daemon_global` |
| `session.register`, `session.close`, `work.claim_next`, `work.ack`, `work.heartbeat`, `work.release`, `supervise.start`, `supervise.send`, `supervise.stop` | `claim` | `single_repo` |
| `supervise.status`, `supervise.list`, `supervise.reattach_status` | `read` | `single_repo` |
| `work.send_message`, `work.block`, `work.complete`, `artifact.publish`, `worktree.create`, `worktree.release`, `workflow.init`, `workflow.generate`, `workflow.upgrade`, `dogfood.publish_on_behalf` | `write` | `single_repo` |
| `review.submit`, `review.verdict` | `review` | `single_repo` |
| `review.override`, `decision.record`, `checkpoint.resolve`, `branch.confirm`, `run.prepare`, `run.start`, `run.pause`, `run.resume`, `run.cancel`, `run.retry_job`, `repo.init` | `admin` | `single_repo` |
| `recovery.stale_leases`, `recovery.requeue_stale`, `recovery.cancel_job`, `recovery.process_reconcile`, `recovery.resume`, `recovery.auto`, `recovery.watch` | `recovery` | `single_repo` |
| `apply.reviewed_patch` | `apply` | `single_repo` |
| `apply.receipt.show`, `apply.receipt.verify` | `read` | `single_repo` |
| `dogfood.surgical_recovery` | `surgical_recovery` | `single_repo` |
| `repo.add`, `repo.remove`, `daemon.token.create`, `daemon.token.revoke`, `daemon.token.rotate`, `daemon.key.rotate`, `daemon.shutdown`, `daemon.migrate`, `daemon.migrate_repo_local` | `admin` | `daemon_global` |
| `cross_repo.list`, `cross_repo.describe`, `cross_repo.why` | `read` | `cross_repo` |
| `cross_repo.cancel` | `recovery` | `cross_repo` |

Keep deprecated aliases for one release with `Deprecated: true`:
`ack`, `heartbeat`, `release`, `block`, `complete`, `publish_artifact`,
`claim_next`, `verdict`, and `submit_review`. Their capabilities must match
the Python aliases at `src/striatum/daemon_rpc/registry.py:150-158`, not the
current stale Go mapping.

Route repo-local methods through one compatibility boundary:
`go/pkg/rpc/cliadapter.go`. It maps envelope params to argv using the same
rules as `src/striatum/daemon_rpc/server.py::_params_to_args` at
`server.py:327-340`, then invokes the installed Python CLI/API against the
same daemon-required Postgres substrate. This is a deliberate Phase 2 choice:
Go owns registry, auth, audit, MCP, cross-repo, apply refusal, and supervision
now; a native Go port of every workflow invariant is a follow-up.

### Apply, MCP, And Cross-Repo

Create:

```text
go/pkg/apply/receipt.go
go/pkg/apply/service.go
```

`receipt.go` defines the Postgres-facing receipt schema:
`receipt_id`, `repository_id`, `run_id`, `job_id`, `patch_artifact_id`,
`patch_sha256`, `base_tree`, `result_tree`, `signing_key_id`, `signature`,
`state`, `denial_reason`, `created_at`, `updated_at`. `service.go` mirrors
`src/striatum/daemon_apply/apply_service.py:16-23` and
`src/striatum/daemon_apply/signing_key.py:9-42`: `apply.reviewed_patch`
returns `sealed_key_missing` until the signing key is loaded, then
`apply_gate_unsatisfied`; receipt show/verify return `receipt_missing`.

Create:

```text
go/pkg/mcp/capabilities.go
go/pkg/mcp/tools.go
```

These mirror the current MCP dispatch path in
`src/striatum/daemon_pg/mcp_dispatch.py:16-123` and the MCP server bridge in
`src/striatum/mcp.py`. `tools.list` filters `rpc.SortedMethods()` by token
capability and repository scope; `tools.call` re-authorizes every call through
the same Postgres authorizer and writes audit/request-log rows on allow and
deny. Visibility is not authority.

Create:

```text
go/pkg/crossrepo/prepare.go
go/pkg/crossrepo/lifecycle.go
```

`prepare.go` mirrors `src/striatum/cross_repo.py:47-128`:
repository lookup, `cross_repo_runs` insert, participant insert, local-run
prepare via a `LocalRunner`, and prepared/aborted transitions. `lifecycle.go`
mirrors `src/striatum/cross_repo.py:131-298`: start, cancel, describe, list,
why projection, and preparing-state reconciliation. The current Python RPC
router only lists/describes/why and deliberately refuses cancel at
`src/striatum/daemon_rpc/server.py:239-263`; Go Phase 2 closes that gap.

Track A tests are:

```text
go/pkg/rpc/registry_test.go
go/pkg/rpc/cliadapter_test.go
go/pkg/apply/receipt_test.go
go/pkg/apply/service_test.go
go/pkg/mcp/capabilities_test.go
go/pkg/mcp/tools_test.go
go/pkg/crossrepo/prepare_test.go
go/pkg/crossrepo/lifecycle_test.go
tests/test_daemon_go_mutations.py
tests/test_daemon_go_apply.py
tests/test_daemon_go_mcp.py
tests/test_daemon_go_crossrepo.py
```

`tests/test_daemon_go_mutations.py` must boot
`MultiRepoHarness(daemon_core="go")` and exercise every RFC 0043 method in
the table above that has existing Python behavior.

## Track B

Create exactly:

```text
go/pkg/supervisor/pointer.go
go/pkg/supervisor/liveness.go
go/pkg/supervisor/pty.go
```

`pointer.go` owns daemon-DB writes for supervisor rows and pointer rows. It
mirrors `src/striatum/daemon_supervisor/pointer.py:11-71` and the
`process_supervisors` insert/update shape in `src/striatum/supervisor.py`.
The state set is `starting`, `attached`, `detached`, `lost`, `stopped`, and
`attached` requires a non-empty `pid_start_time`.

`pty.go` owns process start, FIFO creation, packet delivery, and stop. It
mirrors `src/striatum/supervisor.py:47-227` for start,
`src/striatum/supervisor.py:229-304` for send, and
`src/striatum/supervisor.py:307-409` for stop. Use `os/exec` with
`github.com/creack/pty` only for lanes that declare `pty: true`; otherwise
preserve the Python default of FIFO stdin plus stdout/stderr to `/dev/null`.
Track B records this dependency in handoff as:

```text
require github.com/creack/pty <latest-verified-version>
```

Track A runs `go -C go get github.com/creack/pty@<version>` and commits the
resulting `go.mod` and `go.sum` hashes.

`liveness.go` owns pid/start-time verification, lost detection, and progress
heartbeat. It mirrors `src/striatum/daemon_supervisor/progress_watcher.py`,
especially `ProgressWatcherConfig` at lines 31-36:

```text
poll_interval_seconds = 30
refresh_threshold_seconds = 60
idle_threshold_seconds = 600
heartbeat_extend_seconds = 900
```

### FIFO Packet Schema

The FIFO schema is fixed and byte-compatible with the current wrappers:

```text
.striatum/scratch/<supervisor_id>/stdin.pipe
mode: 0600
framing: one UTF-8 JSON object per line, newline-delimited, no length prefix
payload: stored work_packets.packet_json exactly as built by striatum.db.build_packet
terminator: one trailing "\n"
```

The JSON object is the work packet with `packet_version`, `packet_id`, `run`,
`session`, `job`, `lease`, `commands`, `context`, `expected_artifacts`,
`write_scope`, `adapter_constraints`, `artifact_policy`, `lane_attestation`,
and optional policy/profile blocks. Do not wrap it in another envelope. The
Python sender appends the newline at `src/striatum/supervisor.py:267-270`; the
Claude, Codex, and Gemini wrappers read with `while IFS= read -r packet` in
`.striatum/bin/{claude,codex,gemini}-supervised-wrapper.sh`.

The supervised-progress heartbeat mechanism is file-mtime polling, not FIFO
heartbeats and not OS signals. The watcher stats the newest `*.log` below the
supervisor scratch directory and extends the active lease when the mtime is
fresh. This matches `src/striatum/daemon_supervisor/progress_watcher.py:88-104`
and `.striatum/bin/claude-supervised-wrapper.sh:48-53`.

### Distribution

`go/Makefile` currently has `build`, `test`, `lint`, and `clean`. Add exactly:

```make
PLATFORMS := linux-amd64 linux-arm64 darwin-amd64 darwin-arm64

release: $(PLATFORMS:%=bin/striatumd-%)

bin/striatumd-%:
	@os=$$(echo $* | cut -d- -f1); arch=$$(echo $* | cut -d- -f2); \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -ldflags="-s -w" \
		-o bin/striatumd-$$os-$$arch ./cmd/striatumd

install: build
	install -d "$(DESTDIR)$(PREFIX)/bin"
	install -m 0755 bin/striatumd "$(DESTDIR)$(PREFIX)/bin/striatumd"
```

The top-level `Makefile` currently has `daemon-go-build`, `daemon-go-test`,
and `daemon-go-lint` at lines 70-77. Add:

```make
daemon-go-install:
	$(MAKE) -C "$(MAKEFILE_DIR)/go" install

daemon-go-release:
	$(MAKE) -C "$(MAKEFILE_DIR)/go" release
	@for plat in linux-amd64 linux-arm64 darwin-amd64 darwin-arm64; do \
		sha256sum "$(MAKEFILE_DIR)/go/bin/striatumd-$$plat" \
		> "$(MAKEFILE_DIR)/go/bin/striatumd-$$plat.sha256"; \
	done
```

### Wheel Package Data

Create this exact package-data layout:

```text
src/striatum/_daemongo/
  __init__.py
  bin/
    linux-amd64/striatumd
    linux-arm64/striatumd
    darwin-amd64/striatumd
    darwin-arm64/striatumd
```

`src/striatum/_daemongo/__init__.py` exposes `resolve_binary() -> Path | None`.
It maps `platform.system()` and `platform.machine()` to the four directory
names above, returns the matching executable if present, and returns `None`
otherwise. It may chmod the file to add owner-execute when needed.

Update `pyproject.toml` `[tool.setuptools.package-data]` at lines 47-55 with:

```toml
"striatum._daemongo" = ["bin/*/striatumd"]
```

Add a new `MANIFEST.in` line:

```text
recursive-include src/striatum/_daemongo/bin striatumd
```

The resolver order used by `src/striatum/cli/daemon.py` is shipped wheel
binary first, `STRIATUMD_GO_BIN` second, and `go/bin/striatumd` third for
in-tree development.

### CI

Keep the existing `.github/workflows/ci.yml` `test` job for full Python, UI,
lint, typecheck, package, and fresh-clone coverage. Add two explicit jobs:

```yaml
multi-repo-python:
  runs-on: ubuntu-latest
  services:
    postgres:
      image: postgres:16
      env:
        POSTGRES_PASSWORD: postgres
      ports: ["5432:5432"]
      options: >-
        --health-cmd pg_isready
        --health-interval 10s
        --health-timeout 5s
        --health-retries 5
  env:
    STRIATUM_PG_TEST_URL: postgresql://postgres:postgres@localhost:5432/postgres
    CORE: python
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-python@v5
      with: {python-version: "3.12"}
    - run: python -m pip install -e ".[dev,daemon-pg]"
    - run: make test-multi-repo CORE=python

multi-repo-go:
  runs-on: ubuntu-latest
  services:
    postgres:
      image: postgres:16
      env:
        POSTGRES_PASSWORD: postgres
      ports: ["5432:5432"]
      options: >-
        --health-cmd pg_isready
        --health-interval 10s
        --health-timeout 5s
        --health-retries 5
  env:
    STRIATUM_PG_TEST_URL: postgresql://postgres:postgres@localhost:5432/postgres
    CORE: go
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
      with: {go-version: "1.23"}
    - uses: actions/setup-python@v5
      with: {python-version: "3.12"}
    - run: python -m pip install -e ".[dev,daemon-pg]"
    - run: make daemon-go-build
    - run: go -C go mod verify
    - run: make test-multi-repo CORE=go
```

The Go job must hard-fail when `STRIATUM_PG_TEST_URL` is absent. The sentinel
test must assert `STRIATUM_MULTI_REPO_DAEMON_CORE=go` actually launched the Go
binary from `tests/_harness/daemon.py:114-134`.

Update `.github/workflows/release.yml` so the build job runs
`make daemon-go-release` after `actions/setup-go@v5` and uploads
`go/bin/striatumd-linux-amd64`, `go/bin/striatumd-linux-arm64`,
`go/bin/striatumd-darwin-amd64`, `go/bin/striatumd-darwin-arm64`, and their
`.sha256` sidecars alongside `dist/*`.

Track B tests are:

```text
go/pkg/supervisor/pointer_test.go
go/pkg/supervisor/liveness_test.go
go/pkg/supervisor/pty_test.go
tests/test_daemon_go_supervisor.py
tests/test_daemon_go_core_sentinel.py
```

`tests/test_daemon_go_supervisor.py` must use
`MultiRepoHarness(daemon_core="go")` and cover supervised start, packet
delivery, heartbeat extension from a touched wrapper log, lost detection after
child death, and SIGTERM cleanup.

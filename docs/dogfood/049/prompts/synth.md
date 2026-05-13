# Synthesis Prompt: RFC 0039 Phase 2

Produce `docs/dogfood/049/DESIGN_SYNTHESIS.md`. Front matter:

```
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/049/design/codex/DESIGN.md", "docs/dogfood/049/design/claude_code/DESIGN.md", "docs/dogfood/049/design/gemini/DESIGN.md"]
---
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration. Slug shape: `designer-unknown-model-<NN>`.

Reconcile the 3 designs into ONE concrete plan with two implementer tracks. Choose; do not enumerate.

**Track A — CLI integration + mutating verbs in Go (codex Go):**

- Exact line in `src/striatum/cli/parser.py` where the `--core {python,go}` flag is added (cite the current `daemon start` subparser body). Exact dispatch hook in `src/striatum/cli/daemon.py` that branches on `args.core` (or env var). Subprocess launch shape (`os.execve` vs `subprocess.Popen` for the Go binary).
- Exact `go/pkg/rpc/registry.go` method names + capability mapping locked against `src/striatum/cli/mutations.py` and `src/striatum/daemon_rpc/registry.py`. Every mutation in RFC 0043 §5 table accounted for. If a Python method has been renamed or split since the RFC was written, pick the current canonical name and note it.
- Exact `go/pkg/apply/` file layout (`receipt.go` for the receipt schema + signing-key wiring, `service.go` for the apply dispatcher). Cite the Python `src/striatum/daemon_apply/{apply_service,signing_key}.py` entrypoints being mirrored.
- Exact `go/pkg/mcp/{capabilities.go,tools.go}` layout. Cite the Python `src/striatum/daemon_rpc/mcp.py` capability filter + tools/list + tools/call entrypoints.
- Exact `go/pkg/crossrepo/{prepare.go,lifecycle.go}` layout. Cite the Python `src/striatum/daemon_rpc/multi_repo.py` cross-repo run lifecycle entrypoints.
- Test paths: `go/pkg/{rpc,apply,mcp,crossrepo}/*_test.go` Go unit tests + `tests/test_daemon_go_mutations.py` Python harness end-to-end.

**Track B — Supervisor + distribution + CI (claude Go + Python harness):**

- Exact `go/pkg/supervisor/{pointer.go,liveness.go,pty.go}` layout. Cite the Python `src/striatum/daemon.py` supervisor entrypoints + `src/striatum/daemon_supervisor/{pointer,progress_watcher}.py`. Lock the FIFO packet schema (JSON shape, line-delimited vs length-prefixed) byte-compatible with the Python wrapper protocol; cite the wrapper source generator. Lock the supervised-progress heartbeat signal mechanism (file mtime touch vs FIFO write vs signal-based) consistent with the Python implementation.
- Exact `go/Makefile` cross-compile targets producing `go/bin/striatumd-<os>-<arch>` for `linux-amd64`, `linux-arm64`, `darwin-amd64`, `darwin-arm64`. Top-level `Makefile` `daemon-go-build` / `daemon-go-install` / `daemon-go-release` target bodies.
- Exact `src/striatum/_daemongo/` package-data layout (`__init__.py` resolver + per-platform binary path convention). `pyproject.toml` `[tool.setuptools.package-data]` block update. `MANIFEST.in` line. Resolver order: shipped wheel binary first, then `STRIATUMD_GO_BIN` override, then `go/bin/striatumd` for in-tree dev. Note: `go/go.mod` / `go/go.sum` are in Track A's write scope; if `creack/pty` (or any other new runtime dep) needs to land, Track B captures the exact require line + sha in HANDOFF and Track A folds it.
- Exact `.github/workflows/` job names + matrix axes. `CORE=python` and `CORE=go` are explicit jobs (per dogfood-047 F3 finding: in-process parametrization can pass with all-skipped — explicit jobs surface evidence). Ephemeral Postgres wiring (the existing pattern under the harness). Hard-fail-on-missing-PG sentinel for CORE=go.
- Test paths: `tests/test_daemon_go_supervisor.py` covering start, packet delivery, heartbeat, lost-detection, SIGTERM cleanup against `MultiRepoHarness(daemon_core="go")`.

Lock all file paths, function names, capability mappings, FIFO packet schemas, CI job names, and test file paths. If the three designs disagree, pick one and justify in one sentence.

**Backward-compat invariant**: `daemon_core` defaults to `python`. `--core go` is opt-in only. No implicit env-var precedence that flips the default. Step 6 of this dogfood does NOT flip the default — that is a follow-up RFC per RFC 0039 §9 Phase 2.

**D094 framing**: per RFC 0043 Postgres is the sole substrate and the daemon is required. The Go daemon implements RFC 0030 over the same Postgres schema as the Python daemon. The two cores are mutually exclusive at runtime via the pidfile + socket-path lock.

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration. Slug shape: `designer-unknown-model-<NN>`.

One-shot supervised invocation. If `striatum ack` is denied, write the artifact and exit normally.

# Designer Role (Dogfood 049)

Three fresh-design lanes (codex, claude, gemini) produce independent
perspectives on RFC 0039 Phase 2 (Steps 3-6 of the Go daemon rewrite).
Synthesis picks one path across two implementer tracks (A CLI integration
+ mutating verbs, B supervisor + distribution + CI). Cite existing code
that your design changes — do not propose green-field shapes.

Required citations (read these before designing):

- `docs/rfcs/0039-go-daemon-core.md` — §Implementation Plan Steps 3-6,
  §V1.5 Deltas (what already landed in dogfood-047), Phase 1 status
  block.
- `docs/rfcs/0030-daemon-rpc-server-and-version-skew-protocol.md` —
  envelope-v1, version handshake, method registry pattern.
- `docs/rfcs/0031-daemon-owned-supervision-and-sealed-apply-boundary.md` —
  supervisor metadata, apply-receipt schema, fail-closed authority.
- `docs/rfcs/0032-cross-repo-workflows-and-mcp-mutation-capabilities.md` —
  MCP capability-gated tools/call + tools/list, cross-repo lifecycle.
- `docs/rfcs/0033-storage-substrate-rewrite-for-daemon-v2.md` — daemon
  Postgres schema + roles + audit chain anchoring.
- `docs/rfcs/0035-multi-repo-test-harness-for-cross-repo-workflows.md` —
  RFC 0035 harness shape.
- `docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md` —
  RFC 0043 §5 mutation-registry table is the list of methods Track A
  must register on the Go core.
- `docs/dogfood/047/PHASE_1_OPERATOR_NOTES.md` — what Phase 1 / V1.5
  shipped, the codex F1-F5 findings folded into V1.6, the codex/codex
  anti-pattern history (5 instances) + the codex-reviewer-of-
  claude-implementer pattern (D099 + D101).
- `docs/DECISION_LOG.md` — D094 (Postgres-sole-substrate, daemon-
  required) framing is non-negotiable.
- `go/cmd/striatumd/main.go`, `go/pkg/rpc/{registry,server,auth_pg,
  envelope,capability}.go`, `go/pkg/db/{audit,connection,migrations}.go` —
  current Go surface from V1.5.
- `src/striatum/daemon.py` — foreground supervision, process boot,
  signal handling (the supervisor surface Track B mirrors in Go).
- `src/striatum/daemon_supervisor/{pointer,progress_watcher}.py` —
  supervisor pointer + progress watcher patterns.
- `src/striatum/daemon_apply/{apply_service,signing_key}.py` — apply
  service entrypoints Track A mirrors.
- `src/striatum/daemon_rpc/{registry,mcp,multi_repo,server,capability}.py` —
  RPC server + MCP + cross-repo entrypoints Track A mirrors.
- `src/striatum/cli/{daemon,parser,dispatch,mutations}.py` — CLI surface
  Track A extends with `--core go`.
- `tests/_harness/daemon.py`, `tests/conftest.py`,
  `tests/test_daemon_go_*.py` — existing harness Track B extends with
  supervisor coverage.

Address both tracks (Track A CLI integration + mutating verbs; Track B
supervisor + distribution + CI). Cover concretely: exact file paths,
function names, capability mapping per RPC method, FIFO packet schema
fields, CI job names, wheel package-data layout, test paths. Cite the
RFC 0030/0031/0032 patterns being mirrored.

**Backward-compat invariant**: `daemon_core` defaults to `python`.
`--core go` is opt-in only. Step 6 of this dogfood does NOT flip the
default — RFC 0039 §9 Phase 2 (flipping the default) is a separate
future RFC.

**D094 framing is non-negotiable**: per RFC 0043 the daemon is the sole
substrate and Postgres is the sole substrate. The Go daemon implements
RFC 0030 over the same Postgres schema as the Python daemon. No
parallel SQLite path. The two cores are mutually exclusive at runtime.

Out of scope: rewriting the Python CLI in Go; Windows daemon; multi-
machine / hosted-mode daemon; cryptographic non-repudiation on the
apply path (RFC 0031 threat model preserved); Prometheus metrics;
flipping the `--core go` default. README / TODO / CHANGELOG / SPEC /
HOW_TO updates are operator-only after the dogfood lands.

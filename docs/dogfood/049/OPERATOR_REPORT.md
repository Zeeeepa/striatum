# Dogfood-049 Operator Report

**Run ID:** _TBD_
**Branch:** `striatum/dogfood-049-rfc-0039-phase-2`
**Workflow:** 10-job two-track for RFC 0039 Phase 2 (Steps 3-6 — Go daemon completion).
**Operator:** _TBD_
**Started:** _TBD_

## Scope

RFC 0039 Phase 2 — the Go daemon completion (Steps 3-6 of RFC 0039). Phase 1 (Steps 1+2: read-only RPC + Postgres substrate) landed in dogfood-042; V1.5 correctness deltas (F1-F5) landed in dogfood-047.

- **Track A (codex)**: Steps 3+4 — Python CLI integration (`--core go` flag wiring in `src/striatum/cli/parser.py` + `src/striatum/cli/daemon.py`, subprocess launch of the Go binary, `STRIATUM_DAEMON_CORE` env var) **+** mutating workflow verbs on the Go core (every mutation in `src/striatum/cli/mutations.py` registered in `go/pkg/rpc/registry.go` with matching capability binding, apply service under `go/pkg/apply/`, MCP capability-gated tools/call + tools/list under `go/pkg/mcp/`, cross-repo lifecycle under `go/pkg/crossrepo/`).
- **Track B (claude)**: Steps 5+6 — Supervisor lifecycle in Go (`go/pkg/supervisor/{pointer.go,liveness.go,pty.go}` with os/exec + creack/pty, FIFO packet delivery byte-compatible with the Python wrapper protocol, heartbeat from supervised-progress signal, lost-detection via pidfile + Postgres supervisor pointer, deterministic SIGTERM cleanup) **+** distribution (cross-compile linux-amd64/linux-arm64/darwin-amd64/darwin-arm64, top-level Makefile `daemon-go-build`/`daemon-go-install`/`daemon-go-release` targets, `src/striatum/_daemongo/` package-data shim shipping the per-platform binary inside the Python wheel, CI matrix `daemon_core={python,go}` on Linux + macOS with explicit jobs and hard-fail-on-missing-PG sentinel).

Out of scope:
- Flipping the `--core go` default to `go` (RFC 0039 §9 Phase 2 — separate future RFC).
- Rewriting the Python CLI in Go.
- Windows daemon support.
- Multi-machine / hosted-mode daemon (D083).
- Cryptographic non-repudiation on the apply path (RFC 0031 threat model preserved).
- Prometheus metrics.
- README / TODO / CHANGELOG / SPEC / HOW_TO updates — operator-only after the dogfood lands.

Backward-compat (non-negotiable): the Python daemon must keep working; `daemon_core` defaults to `python`; existing test fixtures continue to pass against `daemon_mode=on` and `daemon_core="python"`.

D094 framing: per RFC 0043 Postgres is the sole substrate and the daemon is required. The Go daemon implements RFC 0030 over the **same Postgres schema** as the Python daemon. The two cores are mutually exclusive at runtime via the pidfile + socket-path lock.

## Interventions

### Intervention 1: Kickoff
- _TBD_ — scaffold committed, run prepared+started, 3 designer sessions (codex/claude/gemini) registered with `--fresh`, supervisors attached, claim-next per session triggered packet delivery.
- Codex session: _sess_id_
- Claude session: _sess_id_
- Gemini session: _sess_id_

## Run Outcome

_TBD_

## Anti-patterns observed

_TBD_ — track recurrence of codex-reviewer-of-claude-implementer (now 3 instances: D099, D101, D102), codex/codex co-blindness (5 instances: D095-D098, D100; explicitly routed around here), claude-no-artifact (3 per dogfood-048), gemini-no-frontmatter (3 per dogfood-048), permission-gate-denial-during-implementer-bash.

## Follow-ups

_TBD_

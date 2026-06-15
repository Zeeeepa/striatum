Publish a concise problem brief framing the question below for the divergence
branches. State the question, constraints, goals, non-goals, and decision
criteria. Do NOT propose solutions — only frame the space.

## The question (GitHub issue #296, labeled ready-for-human)

A codex lane can talk to a **stale** daemon MCP endpoint: `~/.codex/config.toml`
pins a daemon port, but the daemon's MCP port changes across restarts.

**How should Striatum guarantee a codex lane always reaches the CURRENT daemon
MCP endpoint?**

## What is true today (evidence from triage — the reporter's mechanism was wrong)

- The live endpoint **is** injected on both driven paths:
  - self-driving agent-loop: `go/pkg/agentloop/loop.go:175` → `injectLaneMCPConfig`
    → `InjectCodexMCPConfigArgs` (`go/pkg/agentloop/mcpconfig.go:78,84`), which
    passes `-c mcp_servers.striatum.url=<live>` on the codex argv.
  - push-mode default: `go/pkg/mutations/supervision_launch.go:148-157`.
  - This is the documented invariant (D150 / RFC 0088 Decision 5); the live
    endpoint env is set at `supervision_env.go:45`.
- So the lane does NOT launch "bare codex". The two residual gaps that actually
  reproduce a dead-port failure:
  1. **Silent fallback**: push-mode injection falls back to the bare command if
     the endpoint or capability token fail to resolve (`supervision_launch.go:154`)
     — no loud failure.
  2. **Unverified precedence**: it is asserted but **not tested against the real
     codex build** whether a `-c mcp_servers.striatum.url` override actually wins
     over a `[mcp_servers.striatum]` section already in `~/.codex/config.toml`.

## Constraints (hard)

- Codex CLI behavior is external; MCP is HTTP; the daemon MCP port changes across
  restarts. Lane launch happens via `supervision_launch` (push) and `agentloop`
  (self-drive). Claude and agy lanes already work — do NOT regress them.
- Local-first; no new hosted services or external persistence. Stay within the
  RFC 0088 / D150 injection model.

## Goals

- A codex lane **never silently** uses a stale endpoint: either guaranteed-fresh
  injection or a loud, recoverable failure.
- Verified precedence of the launch-time endpoint over any on-disk config.
- Minimal config surface; robust across daemon restarts mid-run.

## Non-goals

- Re-architecting claude/agy lane injection. A general MCP service registry.

## Decision criteria

- Robustness: no silent stale endpoint under restart / token-miss / config-collision.
- Verifiability: a live repro proving `-c` precedence (or proving it does not win).
- Minimal surface and alignment with D150 / RFC 0088.
- Layer choice: harden the injection guard vs. doctor-escalation
  (`go/pkg/reads/doctor_codex.go:71`) vs. a codex-config-management approach.

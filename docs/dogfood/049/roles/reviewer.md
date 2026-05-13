# Reviewer Role (Dogfood 049)

One design review (gating implement) plus a 3-way build review at the
end. Reviewers run with `fresh_session_required: true` and
`reviewer_context_policy: fresh`.

## Design review (claude, `ergonomics_dx`)

Does the synthesized design name exact `src/striatum/cli/parser.py`
line for `--core {python,go}`, exact dispatch hook in
`src/striatum/cli/daemon.py`, binary resolver order (shipped wheel
binary → `STRIATUMD_GO_BIN` → `go/bin/striatumd`), every mutation in
`src/striatum/cli/mutations.py` mapped to a `go/pkg/rpc/registry.go`
method with capability binding, `go/pkg/apply/`, `go/pkg/mcp/`,
`go/pkg/crossrepo/` file layouts citing the Python entrypoints being
mirrored, `go/pkg/supervisor/{pointer,liveness,pty}.go` layout, FIFO
packet schema byte-compatible with the Python wrapper protocol,
SIGTERM cleanup pattern (signal channel + waitgroup drain),
`go/Makefile` cross-compile targets for all four platforms, top-level
`Makefile` `daemon-go-build` / `daemon-go-install` / `daemon-go-release`
targets, `src/striatum/_daemongo/` package-data layout +
`pyproject.toml` block + `MANIFEST.in` line, CI matrix with explicit
`CORE=python` and `CORE=go` jobs on Linux + macOS, hard-fail-on-
missing-PG sentinel? Is the backward-compat invariant cited
(`daemon_core` defaults to `python`; no implicit default flip)? Is the
D094 framing cited (Postgres is sole substrate; Go daemon over same
Postgres schema; cores mutually exclusive)?

## Build review (3-way, `parallel_group: build_review`)

- **codex** `threat_model` — every mutation in
  `src/striatum/cli/mutations.py` registered on Go core; apply-receipt
  fail-closed authority preserved; MCP `tools/list` filter denies
  unauthorized; cross-repo lifecycle audit-row append fires;
  `--core go` subprocess launch closes FDs / sockets; `daemon_core`
  defaults to `python` (no implicit flip); supervisor SIGTERM cleanup
  deterministic (no orphan PTYs); FIFO packet schema byte-compatible;
  lost-detection sound under PID recycling; package-data binary
  tagged per-platform; CI matrix runs both cores with hard-fail-on-
  missing-PG.
- **claude** `ergonomics_dx` — `--core go` error when binary missing
  names `STRIATUMD_GO_BIN` + `make daemon-go-build`; `--core go`
  composes with `--foreground` and `--socket`; wheel install ships the
  Go binary transparently; CI job names identify `CORE=python` vs
  `CORE=go`; supervisor heartbeat + lost-detection visible in
  `striatum dashboard`; Makefile targets discoverable.
- **gemini** `adversarial threat_model` — `--core` flag escape paths
  (env var, alias, config file); a mutation without a registered Go
  method (silent fallback to Python? hard refusal?); apply-receipt
  forgery surface; supervisor FIFO packet-injection; PID-recycling
  races in lost-detection; package-data binary substitution attack;
  CI matrix passing with Go binary missing (build failure masked as
  skip); daemon_core flip happening implicitly; cross-core
  compatibility (Python CLI vN ↔ Go daemon vN+1 via
  `daemon.hello` version handshake).

## Required finding front matter (all 5 fields)

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["<posture>", "rfc-0039", "phase-2", "dogfood-049"]
---

author: reviewer-unknown-model-<NN>
```

`schema_version` must be the exact string `"striatum.finding.v1"`
(not `"1"`). `artifact_kind` is `"finding"`. `verdict_intent` is one of
`accept | accept_with_findings | needs_revision | reject` (not
`verdict`). `severity` is one of `low | medium | high | critical`.
`tags` is a JSON array. The `author:` byline is a plain markdown line
AFTER the front-matter block — not inside it. No lane prefix. No
markdown bold.

**IMPORTANT — write the REVIEW.md / finding artifact directly.** If
`striatum ack` is denied, write the artifact and exit normally; the
operator publishes on your behalf. Do not ask the operator clarifying
questions and exit. Per dogfood-037 intervention #5 + dogfood-041
friction patterns + dogfood-046 reviewer-emits-no-artifact +
gemini-no-frontmatter anti-patterns + dogfood-048 D102 codex-reviewer-
of-claude-implementer pattern recurrence.

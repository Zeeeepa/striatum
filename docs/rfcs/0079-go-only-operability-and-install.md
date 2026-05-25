# RFC 0079: Go-Only Operability And Install

Status: proposed
Date: 2026-05-25
author: proposer-claude-opus-4-7-001
Context:
[`RFC 0068`](0068-go-production-daemon-port.md),
[`RFC 0078`](0078-go-only-runtime-and-python-removal.md),
[`docs/SPEC.md`](../SPEC.md),
[`docs/DECISION_LOG.md`](../DECISION_LOG.md)

## Problem

RFC 0078 made Striatum Go-only at HEAD, but the *operability* surface still
carries the seams of the cutover rather than a designed install experience:

- The systemd user unit was hand-repaired during the RFC 0078 closure (it had
  pointed at the deleted `.venv/bin/python -m striatum.cli daemon start`). There
  is no reproducible, version-controlled way to (re)install it.
- The daemon socket name is a residue: the Go default is `daemon-go.sock` while
  the prior Python launcher used `striatumd.sock`; the cutover briefly relied on
  a symlink between them.
- The Postgres DSN lives in `~/.config/striatum/daemon.toml`, but this is
  undocumented and there is no first-run bootstrap for it.
- `make install` installs only the three binaries; it does not place the
  service unit, install the skill bundle, or verify the daemon comes up.
- Getting-started / operator docs still describe pre-cutover flows in places.

The result is that a fresh operator cannot go from clone to a running daemon
without tribal knowledge. That is the opposite of "local-first and elegant".

## Goals

- A single, documented path from a clean checkout to a running, healthy daemon.
- A first-class `striatum daemon install` that generates the systemd user unit
  portably (no hardcoded home paths), plus `uninstall` and `status`.
- `make install` that installs binaries, the service unit, and the skill
  bundle, then verifies `doctor` is green.
- A documented daemon lifecycle: socket/token/MCP-endpoint locations, the
  `daemon.toml` DSN, start/stop/restart, and troubleshooting.
- Remove the cutover seams: one canonical socket name, no symlink reliance, no
  references to deleted Python launch paths.

## Non-Goals

- Bundling or managing PostgreSQL itself (still a separate product decision).
- Hosted/multi-user service modes, system-wide (root) units, or container
  images — this RFC targets the local-first single-operator user service.
- Changing daemon RPC, capability, or substrate semantics.

## Proposal

### 1. `striatum daemon install`

A local command (no daemon RPC; it is a bootstrap helper) that:

- writes `~/.config/systemd/user/striatumd.service` from an embedded template
  using systemd specifiers (`%h`, `%t`) so the unit is host-portable;
- ensures `~/.config/striatum/daemon.toml` exists, scaffolding a commented
  template with the `postgres_url` key when absent (never overwriting a real
  one);
- runs `systemctl --user daemon-reload` and `enable --now` unless `--no-start`;
- prints the resolved socket, token, and MCP-endpoint paths.

`striatum daemon uninstall` disables/stops the unit and removes it (leaving
`daemon.toml` and data untouched). `striatum daemon status` summarizes unit
state + `doctor` in one view. On non-systemd hosts the command emits a
foreground run recipe instead of failing.

### 2. Canonical socket and runtime layout

Adopt `${XDG_RUNTIME_DIR}/striatum/daemon-go.sock` as the single canonical
socket (the Go default); retire `striatumd.sock` and any symlink. Document the
runtime directory contents: `daemon-go.sock`, `client-token`,
`mcp-http-endpoint`, `striatumd.pid`.

### 3. `make install` completeness

`make install` becomes: build → install binaries → `striatum daemon install
--no-start` → `striatum skills install` → start + `doctor` check. A new
`make uninstall` reverses it. CI exercises `make install` in the fresh-clone
smoke.

### 4. Documentation

- `docs/GETTING_STARTED.md`: clone → `make install` → `doctor`, Go-only.
- New `docs/operator/DAEMON_RUNBOOK.md`: lifecycle, runtime layout, `daemon.toml`
  DSN configuration, log access (`journalctl --user -u striatumd`), and
  troubleshooting (socket/token mismatch, migration on start).
- `docs/INDEX.md` / `docs/DOC_MAP.md` updated.

## Acceptance Criteria

- From a clean checkout, `make install` yields a running daemon and `striatum
  doctor` reports `ok` with no manual edits.
- `striatum daemon install` produces a unit with no hardcoded home paths;
  `uninstall` removes it cleanly; both are idempotent.
- No tracked file or doc references `striatumd.sock`, the `.venv` launcher, or
  `src/striatum/_daemongo/binaries`.
- The fresh-clone smoke covers the install path; `go test ./...` stays green.

## Implementation Plan

1. Embedded unit template + `daemon install/uninstall/status` in
   `go/pkg/cli/localcommands` (or a `daemonctl` package).
2. Socket-name canonicalization + runtime-layout doc.
3. `make install`/`uninstall` rewrite + fresh-clone smoke update.
4. GETTING_STARTED + DAEMON_RUNBOOK docs; index updates.

## Risks

- systemd specifics vary across distros; mitigate with `%h`/`%t` and a
  non-systemd fallback recipe.
- Overwriting an operator's real `daemon.toml` would be data loss; the command
  must only scaffold when absent.

## Open Questions

- Should `daemon install` offer to provision the Postgres role/DB, or only
  document it? (Lean: document; provisioning is a separate decision.)
- Shell completion install as part of `make install`?

## Domain Modeling

No workflow-domain change. This RFC formalizes the *operability* boundary:
install, daemon lifecycle, and runtime layout become a designed, documented
surface rather than cutover residue.

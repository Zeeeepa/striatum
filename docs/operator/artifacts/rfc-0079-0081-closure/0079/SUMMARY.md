---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0079 — Go-Only Operability & Install: Build Summary

author: implementer-claude-001
date: 2026-05-25
status: complete
kind: synthesis

## Overview

RFC 0079 turns Striatum's post-cutover operability residue into a designed,
documented install surface. This build adds a first-class
`striatum daemon install|uninstall|status` bootstrap helper, formalizes the
hand-repaired systemd user unit into a version-controlled embedded template,
makes `make install` complete (binaries + unit + skills + health check) with a
matching `make uninstall`, and rewrites the Go-only clone→`make install`→
`doctor` operability docs. No daemon RPC routes, capability semantics, or
`contracts/` were touched — the new verbs are local bootstrap helpers.

## Files Added

- `go/pkg/cli/localcommands/daemon.go` — the `daemon install|uninstall|status`
  implementation (flag parsing, unit render/write, daemon.toml scaffold,
  `systemctl --user` orchestration, runtime-layout reporting, doctor fold-in,
  non-systemd foreground recipe).
- `go/pkg/cli/localcommands/striatumd.service.tmpl` — embedded systemd user
  unit (`//go:embed`), portable via `%h`/`%t` specifiers.
- `go/pkg/cli/localcommands/daemon_test.go` — tests for local-command
  registration, specifier-only unit (no hardcoded home), `--print-unit`,
  never-overwrite daemon.toml scaffolding, and canonical-socket layout.
- `docs/operator/DAEMON_RUNBOOK.md` — the new daemon operability runbook.

## Files Changed

- `go/pkg/cli/localcommands/localcommands.go` — registered `daemon`
  `install`/`uninstall`/`status` as explicit local commands with rationales.
- `go/cmd/striatum/main.go` — dispatch `daemon` subcommands to
  `localcommands.RunDaemon`, honoring the global `--json` flag.
- `Makefile` — rewrote `install` (build → install binaries →
  `daemon install --no-start` → `skills install` → best-effort start +
  `daemon status`), added `uninstall`, added `uninstall` to `.PHONY`.
- `docs/GETTING_STARTED.md` — rewrote the install section to the Go-only
  clone→`make install`→`doctor` path; replaced the dead
  `striatum daemon start` manual-sidebar line with the systemd/foreground
  recipe; bumped release-archive version to 2.1.0.
- `docs/INDEX.md`, `docs/DOC_MAP.md` — added the DAEMON_RUNBOOK entry +
  boundary contract (owns daemon lifecycle/runtime layout; defers Postgres
  provisioning to POSTGRES_TRANSITION, verb sequences to HOW_TO_*).

## The Unit Template

Portable systemd **user** unit, rendered from `striatumd.service.tmpl`. It
relies entirely on systemd specifiers — `%h` (home) and `%t` (runtime dir) —
so it carries no hardcoded home paths:

```ini
[Unit]
Description=Striatum local workflow daemon (Go)
Documentation=https://github.com/halbritt/striatum
After=network-online.target

[Service]
Type=simple
ExecStartPre=/usr/bin/mkdir -p %t/striatum
ExecStart=%h/.local/bin/striatumd -socket %t/striatum/daemon-go.sock
Restart=on-failure
RestartSec=2

[Install]
WantedBy=default.target
```

This formalizes the unit that was hand-repaired during RFC 0078 closure (it
had pointed at the deleted `.venv/bin/python -m striatum.cli daemon start`).

## Install Flow

`striatum daemon install`:

1. scaffolds `~/.config/striatum/daemon.toml` (commented `postgres_url`
   guidance) **only when absent** — it never clobbers a real DSN;
2. on a systemd host: writes `~/.config/systemd/user/striatumd.service`, runs
   `systemctl --user daemon-reload`, then `enable --now` (or just `enable`
   under `--no-start`);
3. prints the resolved socket / token / MCP-endpoint paths.

`--print-unit` emits the rendered unit and touches nothing (used by the gate
dry render and tests). `uninstall` does a best-effort `disable --now`, removes
the unit, and `daemon-reload`s, leaving config + data intact. `status`
reports unit installed/enabled/active, socket presence, runtime paths, DSN
configured + source, and folds in a one-line `doctor` summary by shelling out
to the same binary. Non-systemd hosts get a foreground run recipe instead of a
hard error.

The socket is canonicalized to `${XDG_RUNTIME_DIR}/striatum/daemon-go.sock`
(the existing Go default); nothing added references the retired
`striatumd.sock` or `src/striatum/_daemongo`.

## Docs Added/Updated

- **DAEMON_RUNBOOK.md** (new): install/uninstall, lifecycle
  (`systemctl --user`), runtime layout table (`daemon-go.sock`,
  `client-token`, `mcp-http-endpoint`, `striatumd.pid`), DSN resolution
  order, `journalctl --user -u striatumd`, and a troubleshooting section
  (daemon_unreachable/11, no-DSN refusal, repo_not_migrated/12, socket/token
  mismatch, migration-on-start, stale pre-0078 unit).
- **GETTING_STARTED.md**: Go-only install with the chicken-and-egg DSN step
  spelled out (the daemon won't bind without `postgres_url`).
- **INDEX.md / DOC_MAP.md**: new runbook listed and bounded.

## Validation

```
cd go && go build ./...      # ok
        go vet ./...         # ok
        go test ./pkg/cli/... ./cmd/striatum   # all ok (localcommands tests pass)
gofmt -l pkg/cli/localcommands/ cmd/striatum/  # clean
make -n install / make -n uninstall            # parse + recipe verified (not executed)
```

Live binary smoke (read-only; running daemon not disrupted):

```
$ striatum daemon install --print-unit   # renders the unit, touches nothing
$ striatum daemon status
  unit:    ~/.config/systemd/user/striatumd.service (installed=true, enabled=enabled, active=active)
  socket:  /run/user/1000/striatum/daemon-go.sock (present=true)
  token:   /run/user/1000/striatum/client-token
  mcp:     /run/user/1000/striatum/mcp-http-endpoint
  config:  ~/.config/striatum/daemon.toml (dsn_configured=true)
  doctor:  <one-line doctor summary>
```

## Notes & Deviations

- **`make install` start is best-effort, not hard-fail.** The daemon refuses
  to bind without a Postgres DSN, and provisioning Postgres is an explicit
  RFC non-goal. So a fresh clone with no DSN cannot reach a *running* daemon
  in one `make install`. The target installs everything, scaffolds the config,
  attempts the start, and prints the one remaining manual step (set
  `postgres_url`, then `striatum daemon install && striatum doctor`). This is
  the honest reading of the acceptance criterion given the Postgres non-goal.
- **Out-of-scope `striatumd.sock` references remain** in files outside this
  packet's write scope: `go/cmd/striatumd/pidfile_test.go` (a literal test
  fixture), and historical docs under `docs/issues/`, `docs/handoffs/`,
  `docs/reviews/`. RFC 0079's "no tracked file references `striatumd.sock`"
  criterion needs a follow-up sweep of those paths; none of them are live
  operability guidance.
- New verbs are local-only (`localcommands`), so the COMMAND_AUTHORITY_MATRIX
  and daemon contracts are unchanged by design.

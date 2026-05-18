# RFC 0070: Daemon Client and Service Boundary Completion

Status: mostly implemented
Date: 2026-05-17
Context: [RFC 0030](0030-daemon-rpc-server-and-version-skew-protocol.md), [RFC 0040](0040-mcp-driven-dogfood-harness.md), [RFC 0061](0061-daemon-first-web-service.md), [RFC 0068](0068-go-production-daemon-port.md), [RFC 0069](0069-pg-only-daemon-global-surfaces.md)

## Problem

The CLI and web service are mostly daemon clients. Repo resolution, daemon
mapped `/v1/invoke` paths, and local stdio MCP alias disabling have landed.
Remaining work is mostly legacy composite/tooling cleanup: clients should not
gain new PostgreSQL topology knowledge, and old local authoring/test surfaces
must stay clearly outside live workflow authority.

## Goals

- Add daemon-side repository resolution so clients do not open PostgreSQL.
- Route daemon-mapped `/v1/invoke` reads and mutations through daemon RPC.
- Keep workflow authoring helpers local and explicit.
- Either port dogfood composite tools to PostgreSQL or quarantine/unregister
  them as historical compatibility tools.
- Keep local `striatum.api.invoke` and the stdio MCP `striatum/invoke`
  compatibility path outside production live-state authority; local
  `tools/list` and `tools/call` must not expose CLI-shaped aliases.

## Non-Goals

- Remove the Python CLI.
- Keep a Python daemon fallback after the Go daemon reaches parity.
- Remove local workflow-file authoring helpers.
- Expand MCP capabilities beyond the accepted daemon capability vocabulary.

## Proposal

1. Add `repo.resolve` or equivalent daemon RPC support returning the
   repository id for a repo root under daemon authorization.
2. Update CLI and web service helpers to ask the daemon for repository
   resolution instead of importing `striatum.daemon_pg.connection`.
3. Update `/v1/invoke` so daemon-routed reads and mutations call
   `service_daemon.call_repo_method()` and local authoring remains on an
   explicit allowlist.
4. Keep local stdio MCP CLI aliases disabled. Daemon-backed web chat/tool-list
   commands route through the shared daemon RPC policy, while local
   `striatum/invoke` remains a narrow compatibility path for explicit callers.
5. Keep `dogfood.publish_on_behalf` and `dogfood.surgical_recovery` absent
   from the production daemon contract unless a PostgreSQL-native composite
   is accepted.
6. Remove Python-daemon-specific production fallback paths after Go parity
   lands.

## Acceptance Criteria

- Client modules no longer import `striatum.daemon_pg.connection` for normal
  repository resolution.
- Monkeypatching client-side PG connect to raise does not break daemon-routed
  CLI/web calls.
- `/v1/invoke` succeeds for daemon-routed reads and mutations when
  `striatum.api.invoke` is monkeypatched to fail.
- Local MCP `striatum/invoke` and web chat mapped reads/mutations succeed
  when `striatum.api.invoke` is monkeypatched to fail; local MCP
  `tools/list` / `tools/call` do not expose CLI-shaped aliases.
- Daemon MCP lists only supported production methods for production tokens.
- Dogfood composites are either PG-native with tests or absent from production
  MCP tools.

## Implementation Notes

- `repo.resolve` is registered as a daemon-global `read` method. This resolves
  the bootstrap problem where a client cannot present a repository-scoped id
  before the daemon has mapped a path to that id.
- CLI and service RPC helpers now resolve repository ids through daemon RPC;
  client-side imports of `striatum.daemon_pg.connection` are guarded against
  by tests.
- `/v1/invoke` routes daemon-mapped production reads and mutations through
  `service_daemon.call_repo_method()` and no longer re-enters
  `striatum.api.invoke` for those methods.
- Local MCP `striatum/invoke` and web chat tools use the same command-routing
  helper, so mapped status, why, lifecycle, artifact, review, and recovery
  commands also cross the daemon RPC boundary instead of entering local CLI
  dispatch. Local MCP `tools/list` returns no CLI-shaped aliases and local
  `tools/call` returns `local_tools_unavailable`.
- `dogfood.publish_on_behalf` and `dogfood.surgical_recovery` are absent from
  both Python and Go production daemon contracts because the historical
  composites were SQLite-bound. Operators should use primitive daemon methods
  until a PostgreSQL-native composite is accepted.
- Production daemon MCP `tools/list` now exposes only supported production
  methods. It hides local workflow-file authoring methods in both Python and
  Go; direct calls to removed dogfood composite names audit as
  `method_unknown`.

## Open Questions

- Resolved for the transition: `repo.resolve` is daemon-global `read` because
  repository-scoped auth cannot know the repository id before resolution.
- Should dogfood composites remain named `dogfood.*` after porting, or move to
  an operator-composite namespace?

## Domain Modeling

This RFC clarifies the client boundary. Repository identity is daemon-owned
metadata; clients may hold a repo path value, but they must not become direct
database readers to translate it into live-state authority.

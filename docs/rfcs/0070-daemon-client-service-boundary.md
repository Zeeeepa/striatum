# RFC 0070: Daemon Client and Service Boundary Completion

Status: proposed
Date: 2026-05-17
Context: [RFC 0030](0030-daemon-rpc-server-and-version-skew-protocol.md), [RFC 0040](0040-mcp-driven-dogfood-harness.md), [RFC 0061](0061-daemon-first-web-service.md), [RFC 0068](0068-go-production-daemon-port.md), [RFC 0069](0069-pg-only-daemon-global-surfaces.md)

## Problem

The CLI and web service are mostly daemon clients, but a few paths still
resolve repository identity or invoke commands outside the daemon boundary.
That keeps clients aware of PostgreSQL topology and leaves legacy local
authoring/MCP surfaces ambiguous.

## Goals

- Add daemon-side repository resolution so clients do not open PostgreSQL.
- Route daemon-mapped `/v1/invoke` mutations through daemon RPC.
- Keep workflow authoring helpers local and explicit.
- Either port dogfood composite tools to PostgreSQL or quarantine/unregister
  them as historical compatibility tools.
- Clarify local `striatum.api.invoke` and `LocalRpcServer` as local-authoring
  or legacy/test surfaces, not production run authority.

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
3. Update `/v1/invoke` so daemon-routed mutations call
   `service_daemon.call_repo_method()` and local authoring remains on an
   explicit allowlist.
4. Mark `LocalRpcServer` and invoke-backed chat tools as local-authoring or
   legacy/test surfaces in docs and tests.
5. Port `dogfood.publish_on_behalf` and `dogfood.surgical_recovery` to PG or
   unregister them from production daemon MCP until ported.
6. Remove Python-daemon-specific production fallback paths after Go parity
   lands.

## Acceptance Criteria

- Client modules no longer import `striatum.daemon_pg.connection` for normal
  repository resolution.
- Monkeypatching client-side PG connect to raise does not break daemon-routed
  CLI/web calls.
- `/v1/invoke` succeeds for daemon-routed mutations when
  `striatum.api.invoke` is monkeypatched to fail.
- Daemon MCP lists only supported production methods for production tokens.
- Dogfood composites are either PG-native with tests or absent from production
  MCP tools.

## Open Questions

- Should `repo.resolve` be daemon-global read capability or repository-scoped
  read capability based on the resolved repository?
- Should dogfood composites remain named `dogfood.*` after porting, or move to
  an operator-composite namespace?

## Domain Modeling

This RFC clarifies the client boundary. Repository identity is daemon-owned
metadata; clients may hold a repo path value, but they must not become direct
database readers to translate it into live-state authority.

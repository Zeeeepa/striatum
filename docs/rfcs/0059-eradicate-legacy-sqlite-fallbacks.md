# RFC 0059: Eradicate Legacy SQLite Fallbacks

## Status
Partially implemented

## Summary
Production Striatum no longer uses repo-local SQLite as live workflow state.
Daemon-owned PostgreSQL is the authority for registered target repositories,
and ordinary CLI/web/MCP routes fail closed when the daemon boundary is not
available. Legacy SQLite remains only as migration source material, golden
fixture data, and explicitly gated test-harness compatibility under
`STRIATUM_TEST_HARNESS=1`.

## Motivation
The 2026-05-16 architecture review found that residual fallback code made the
post-D094 product boundary hard to reason about. Operators need one live-state
authority, not a mix of daemon/Postgres and direct repo-local state.

## Proposed Implementation
Landed slices include fail-closed daemon-routed CLI behavior, service/web
fallback quarantine under `striatum.legacy_sqlite.service`, day-zero setup
docs, and guardrail tests that prevent production service imports from opening
repo-local SQLite. Remaining work is cleanup and quarantine hardening: remove
or narrow test-only fallback entry points as replacement PG/web paths land.

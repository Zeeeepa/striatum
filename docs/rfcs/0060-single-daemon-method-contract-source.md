# RFC 0060: Single Daemon Method Contract Source

## Status
Implemented

## Summary
`contracts/daemon_methods.json` is the single source for daemon method
metadata and CLI route declarations. Python loads `METHOD_REGISTRY` from that
contract, Go registry fixtures are generated from it, and
`docs/architecture/DAEMON_METHOD_TABLES.md` is generated from the same file.

## Motivation
The 2026-05-16 remediation review found drift between parser-visible commands,
daemon method metadata, MCP descriptors, Go placeholders, and hand-written
documentation. One contract keeps routing, capability, scope, and docs aligned.

## Proposed Implementation
Completed work includes contract-loaded Python registry entries, generated Go
registry metadata, generated method tables, declarative `cli_routes`, route
lookup tests, MCP descriptor derivation from `METHOD_REGISTRY`, and guardrails
that keep workflow authoring local while daemon-owned live-state verbs route
through RPC.

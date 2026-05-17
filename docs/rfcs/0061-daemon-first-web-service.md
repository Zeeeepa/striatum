# RFC 0061: Daemon-First Web Service

## Status
Partially implemented

## Summary
The production local web service is moving behind daemon DTO/RPC boundaries.
Read and mutation routes that have landed use daemon-backed APIs; legacy
SQLite service helpers are quarantined as fixture fallback only. `service.py`
continues to shrink into route wrappers while domain shaping moves into
`striatum.web.*`, `striatum.service_*`, and daemon handlers.

## Motivation
The service had become a large mixed authority surface with direct SQLite
fallbacks, rendering code, DTO shaping, and HTTP plumbing in one file. D094
requires the production service to honor daemon-owned PostgreSQL as live state.

## Proposed Implementation
Landed slices include daemon-backed doctor, run/event/artifact/API paths,
daemon-routed run mutations, raw artifact and workflow-browser splits, request
security/io helpers, route dispatch extraction, and service lifecycle
extraction. Remaining work is continued `service.py` decomposition and removal
or stricter gating of any leftover fixture-only fallback seams.

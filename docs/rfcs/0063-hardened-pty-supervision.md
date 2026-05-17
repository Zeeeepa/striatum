# RFC 0063: Hardened PTY Supervision

## Status
Implemented

## Summary
Architecture remediation Phase 6 hardened supervised process execution while
the Python daemon was still the incumbent authority and Go was a narrow PTY
helper. D107 / RFC 0068 later reopened the full Go production daemon port, so
the helper protocol remains useful transition infrastructure rather than a
permanent Python-daemon boundary.

## Motivation
Derived from the STRIATUM Architecture Review and Remediation Plan (2026-05-16).

## Implementation Summary

- `supervise.send` reports delivered-unacknowledged sends explicitly, and
  `supervise.report` records wrapper/helper control events without parsing
  model output.
- The Go `striatum-supervisor-helper` launches agents under PTY, forwards
  packets from stdin or FIFO, and emits JSONL events while architecture
  guardrails keep it out of daemon DB/RPC/domain packages.
- `supervision.transport: "pty_helper"` lets lanes opt into helper launch;
  `supervision.stdin_delivery: "one_shot_eof"` supports raw single-prompt
  commands without changing the default persistent FIFO wrapper contract.
- `supervise.reattach_status`, `doctor`, `status`, recovery sweep, and
  existing send/claim paths now classify or reconcile stale attached
  supervisors before delivery.
- PostgreSQL lane-liveness attestation verifies the session/run binding,
  live PID identity, PID start token, and immutable workflow lane command.
- Tests cover helper event ingestion, real Go helper launch through the
  Postgres supervision handler, CI's Linux/Postgres helper integration
  target, restart reattach/lost-state reconciliation, and the Claude, Codex,
  and Gemini supervised-wrapper loop fixtures.

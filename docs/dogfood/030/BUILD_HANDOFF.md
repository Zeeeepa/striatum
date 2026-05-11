# Dogfood 030 Build Handoff

author: operator
date: 2026-05-11
status: complete

## Summary

Implemented the accepted RFC 0026 V1 lane-liveness attestation scope and
the RFC 0027 fail-closed provenance-mode guardrail.

RFC 0026 now derives artifact bylines from publish-time attestation. An
unattested session receives `author: operator`; an attached supervised
session receives the lane/model byline only when the supervisor row is for
the same run/session, the pid is alive, the Linux process start-time token
still matches, and the recorded supervisor command matches the workflow
snapshot lane command. Operator labels are supported through
`register-session --operator-label` and are constrained so they remain
self-declared and cannot resemble attested lane bylines.

Review jobs may set `require_attested_lane: true`. In V1 this field is
valid only on review jobs; `publish-artifact`, `verdict`, and
`submit-review` refuse before side effects when the calling session is not
attested.

RFC 0027 now has honest mode surfacing: `provenance_mode` validates
`advisory`, `attested_bylines`, and `sealed_patch`; absent mode defaults to
`advisory`. Structurally valid `sealed_patch` workflows require
non-overlapping repo-relative protected/operator-writable paths, but `run
start` refuses sealed runs until a real containment mechanism exists.

## Code Changes

- Added migration v12 for `sessions.operator_label` and
  `process_supervisors.pid_start_time`.
- Added `LaneAttestation`, process start-token probing, operator byline
  derivation, and operator-label validation in `src/striatum/identity.py`.
- Updated supervisor start/status to record pid start-time and expose
  attestation.
- Updated doctor to flag non-terminal `sealed_patch` runs as unsupported
  until hard containment exists.
- Updated packet generation, artifact publishing, review verdicts, status,
  list sessions, evidence export, and run summary inputs to use the new
  attestation-aware identity.
- Updated workflow validation for `provenance_mode`,
  `protected_paths`, `operator_writable_paths`, and
  `require_attested_lane`.
- Updated SPEC, ubiquitous language, decision log, TODO, changelog, README,
  RFC 0026, RFC 0027, and RFC index entries to describe the shipped
  guarantee and deferred sealed-patch scope.

## Tests Run

- `make lint`
- `make typecheck`
- `make test` (`545 passed`)
- `make smoke`

## Compatibility Notes

The intentional compatibility break from RFC 0026 is live: manually driven
sessions without an attached supervisor can no longer publish lane/model
bylines. Existing workflows that rely on unsupervised operation should omit
author lines or use the packet-provided `author: operator` line. Workflows
that require real lane-liveness for review artifacts/verdicts should set
`require_attested_lane: true` on review jobs and run `striatum supervise
start --session-id <id>` before publishing or recording verdicts.

## Deferred Scope

No RFC 0027 patch capture, hash-bound verdict targets, apply gate,
receipts, key management, containment mechanism, source read helpers, or
sealed-mode signed local commits were implemented. `sealed_patch` refuses
to start rather than overclaiming.

## Human Decisions

D080 records acceptance of RFC 0026 V1 plus RFC 0027 Phase 2 guardrails.
Future human decisions are still required before selecting a containment
mechanism, adding receipt signing/key custody, or enabling any sealed-mode
local commit carve-out.

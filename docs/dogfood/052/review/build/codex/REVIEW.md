---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["threat_model", "rfc-0043", "v1-6", "build"]
---

author: reviewer-unknown-model-001

# Build Review — RFC 0043 V1.6 (codex, threat_model)

Operator-composed.

## Summary

Build shipped F-escape (env var pair), F-split-brain (db.connect
checks sentinel/tombstone), F-lock (sidecar flock + exit code 14),
F-help (description + per-flag help).

## Findings

- F1 (low): Daemon-side single-repo business logic on PG (gemini A1)
  deferred to V2.0 — accepted scope deferral.
- F2 (low): Exit code 14 needs registration in RFC 0043 error code
  table (currently 11/12 documented).

## Verdict

`accept_with_findings` (low).

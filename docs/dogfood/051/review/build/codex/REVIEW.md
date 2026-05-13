---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["threat_model", "rfc-0039", "v1-6", "build"]
---

author: reviewer-unknown-model-001

# Build Review — RFC 0039 V1.6 (codex, threat_model)

Operator-composed pending claude/gemini reviewer support (recurring
5+-instance no-publish pattern).

## Summary

Build shipped F-pty (creack/pty wired), F-pid-recycling (Linux
/proc/<pid>/stat start-time pairing), F-perms (0700/0600),
F-store (Postgres-backed PointerStore), F-ci (verify-go-binary
step). `cd go && go test ./pkg/supervisor/` passes.

## Findings

- F1 (low): macOS path falls back to signal-0-only for
  pid-recycling. V1.7 should wire `proc_pidinfo` / sysctl.
- F2 (low): Postgres-backed PointerStore not yet injected into
  cmd/striatumd main; remains in-memory until V1.7.

## Verdict

`accept_with_findings` (low). Both findings deferrable.

# Track A Build Review Prompt (threat_model, 3-way)

Produce REVIEW.md at `docs/dogfood/042/track_a/review/build/<lane>/REVIEW.md`.

Front matter (ALL FIVE FIELDS REQUIRED — `schema_version: "striatum.finding.v1"` exact string):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["threat_model", "rfc-0039", "go-daemon", "build", "track_a"]
---

author: reviewer-<lane>-<model>-<ordinal>
```

Review the Go daemon Steps 1+2 build under **threat_model** posture. Inspect both halves: codex Go core (`go/`) + claude Python glue (harness extensions, docs).

Per-lane angle:

- **codex** (you if codex): Go systems angle — envelope-v1 correctness, capability table, audit-chain helper reuse, Postgres connection security, `go.sum` integrity.
- **claude** (you if claude_code): Python ↔ Go integration angle — harness `daemon_core="go"` parameter wiring, subprocess launch path, doc honesty.
- **gemini** (you if gemini): adversarial angle — Go module supply-chain (no unverified deps), cross-platform reality (Linux + macOS), PG credential leakage, env var injection, audit chain tamper attempts.

Required checks (all):

- RFC 0039 §Implementation Plan Steps 1+2 acceptance criteria met (and ONLY 1+2, not 3-6).
- Same RFC 0030 envelope-v1 protocol; no protocol divergence.
- Same RFC 0033 Postgres schema; no schema fork.
- Test parity: harness `daemon_core="go"` runs the same e2e tests.

**IMPORTANT — write the REVIEW.md artifact directly.** If `striatum ack` is denied, write and exit normally; operator publishes on your behalf.

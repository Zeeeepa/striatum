# Track C Build Review Prompt (threat_model, 3-way)

Produce REVIEW.md at `docs/dogfood/042/track_c/review/build/<lane>/REVIEW.md`.

Front matter (ALL FIVE FIELDS REQUIRED — `schema_version: "striatum.finding.v1"` exact string):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["threat_model", "rfc-0042", "repo-local-pg", "build", "track_c"]
---

author: reviewer-<lane>-<model>-<ordinal>
```

Review the RFC 0042 body at `docs/rfcs/0042-repo-local-state-to-postgres.md` under **threat_model** posture.

Per-lane angle:

- **codex** (you if codex): systems angle — migration safety, schema integrity, audit chain preservation, daemon-mandatory boundary.
- **claude** (you if claude_code): CLI behavior when daemon unavailable, RFC 0039 scope revision correctness, D006/D007/D028 supersession written cleanly.
- **gemini** (you if gemini): adversarial — concurrent operators (single-tenant per D083), failure modes (PG unavailable mid-write), audit chain tamper, rollback safety, partial migration recoverability.

Required checks (all):

- RFC 0042 V1 acceptance criteria concrete enough that a future dogfood can implement against them.
- D006/D007/D028 supersession is explicit and references D093.
- `.striatum/` → operational scratch boundary defined.
- Migration verb spec includes `--dry-run` + `--keep-sqlite-readonly` semantics.
- RFC 0039 scope revision call-out (Go daemon now gateway for ALL repo-local ops from day 1).
- Single-user trust (D083) unaffected.

**IMPORTANT — write the REVIEW.md artifact directly.** If `striatum ack` is denied, write and exit normally.

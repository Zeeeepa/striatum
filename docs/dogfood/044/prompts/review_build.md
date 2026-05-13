# Build Review Prompt (RFC 0040 V1.5, 3-way)

Produce REVIEW.md at `docs/dogfood/044/review/build/<lane>/REVIEW.md`.

Use the posture supplied in your work packet's `review_policy`:

- **codex**: `threat_model`
- **claude**: `ergonomics_dx`
- **gemini**: `threat_model` (adversarial angle)

Front matter (ALL FIVE FIELDS REQUIRED — `schema_version: "striatum.finding.v1"` exact string):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["<posture>", "rfc-0040", "v1-5", "build"]
---

author: reviewer-<lane>-<model>-<ordinal>
```

Use the verdict vocabulary exactly: `accept`, `accept_with_findings`, `needs_revision`, `reject`.

Read the implementation handoff at `docs/dogfood/044/build/HANDOFF.md`.

Per-lane angle:

- **codex (threat_model)**: dispatch authorization preserved (token caps still enforced through the dispatch route); audit chain integrity (every dispatched call leaves a result/error row); composite-tool atomicity actually rolls back partial state under failure; watcher race + signal hardening correctness; surgical-recovery cannot be bypassed.
- **claude (ergonomics_dx)**: operator UX through composite tools clear; error messages name the failing composed step; the MCP path is first-time-discoverable; operator workflow stays decluttered (no new hand-edits required).
- **gemini (adversarial threat_model)**: composite tool atomicity edge cases (mid-step failure, daemon crash mid-transaction); watcher race exploits (rotated log file, watcher start before wrapper log exists, SIGTERM during heartbeat call); audit-chain gaps (orphan allow-row with no dispatch row, or dispatch row with no audit row).

Required checks (all lanes):

- **F1-F6 covered**: cite the implementation site for each finding (file + function or line).
- **Backward compatibility**: existing MCP tools still work; daemon RPC envelope-v1 unchanged; regression-test names cited.
- **E2E tests run**: `make test` passes; the new e2e tests exercise the full MCP path (not mocked-only).
- **Composite rollback**: a failing-step fixture demonstrates atomicity.

Cite specific files / lines / test names. "Looks good" is not a review.

**IMPORTANT — write the REVIEW.md artifact directly.** If `striatum ack` is denied, write and exit normally.

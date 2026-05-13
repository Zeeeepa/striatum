# Design Review Prompt (RFC 0040 V1.5)

Produce REVIEW.md at `docs/dogfood/044/review/design/REVIEW.md`.

Use the posture supplied in your work packet's `review_policy`: `ergonomics_dx`.

Front matter (ALL FIVE FIELDS REQUIRED — `schema_version` must be the exact string `"striatum.finding.v1"`, not `"1"`):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["ergonomics_dx", "rfc-0040", "v1-5", "design"]
---

author: reviewer-<lane>-<model>-<ordinal>
```

Use the verdict vocabulary exactly: `accept`, `accept_with_findings`, `needs_revision`, `reject`.

Read the synthesis at `docs/dogfood/044/DESIGN_SYNTHESIS.md`. Apply the ergonomics_dx lens: are F1-F6 mappings concrete (function names, file paths, registry handles)? Is composite-tool atomicity discoverable from an operator's first-time MCP call — what does the error message say when a composite step fails partway?

Specific checks:

- F1 dispatch wiring names a concrete daemon entry function and a concrete method-registry handle (not "the dispatcher").
- F2/F3 atomicity model is one chosen approach with justification, not three alternatives.
- F4 watcher invocation point is a named lifecycle function in `src/striatum/daemon_pg/` or `src/striatum/supervisor.py`.
- F5 race windows are enumerated with concrete guards (not "we add a lock").
- F6 e2e tests have exact filenames and a smoke-harness hook.
- Operator UX through composite tools is first-time-discoverable: tool descriptions surface failure modes; error messages name the failing composed step.
- Backward-compat assertion is explicit: existing MCP tools + daemon RPC envelope-v1 unchanged.

Cite the synthesis section(s) you are challenging. Hand-waving findings ("the design is unclear") without a pinpoint citation will be down-weighted by the verdict gate.

**IMPORTANT — write the REVIEW.md directly in this single supervised invocation.** If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.

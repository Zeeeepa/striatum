---
schema_version: striatum.decision.v1
decision_id: "dec_31c132a2315b400382171984fe228d4f"
run_id: "run_68a5b38fed054073a91fe4a92c33cc28"
artifact_kind: decision
owner: human
outcome: accepted_with_follow_up
follow_up_required: true
title: "RFC 0023 V1 (web chat + browse) accepted with 3-lane review findings"
created_at: "2026-05-09T09:28:25Z"
---

# RFC 0023 V1 (web chat + browse) accepted with 3-lane review findings

Decision ID: `dec_31c132a2315b400382171984fe228d4f`
Run ID: `run_68a5b38fed054073a91fe4a92c33cc28`
Outcome: `accepted_with_follow_up`

## Rationale

V1 accepted by all three review postures (security accept_with_findings, devils_advocate accept_with_findings, threat_model accept). Compact V1 scope: chat surface (provider-neutral, both flavors), minimum file-view endpoint, Markdown rendering on artifacts (closes RFC 0022 V1.5 deferred). Full file-tree browser UI deferred to V1.5. Two acceptance-blocking findings: F1-security (URL scheme validation: HTTPS or http://localhost*), F1-devils-note (empty-state UX must be copy-pasteable). Other findings folded into BUILD_HANDOFF.

## Follow-Up

Note F3-security in BUILD_HANDOFF: img data: allowed by CSP is acceptable

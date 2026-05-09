---
schema_version: striatum.decision.v1
decision_id: "dec_d3fdd326beb6472da21a313c64e01d20"
run_id: "run_0ee3bb0412ba4113b1432ccaadf67f46"
artifact_kind: decision
owner: human
outcome: accepted
follow_up_required: false
title: "RFC 0021 V1.5 (--force + --dry-run) accepted"
created_at: "2026-05-09T07:13:57Z"
---

# RFC 0021 V1.5 (--force + --dry-run) accepted

Decision ID: `dec_d3fdd326beb6472da21a313c64e01d20`
Run ID: `run_0ee3bb0412ba4113b1432ccaadf67f46`
Outcome: `accepted`

## Rationale

V1.5 scope locked: --force overwrites regular files (status: overwritten with prior_sha256); --dry-run reports would_* vocabulary without filesystem mutation; non-file targets always error regardless of force; both flags together preview destructive action. Zero regression for plain --with-ddd-layout. No design findings — accept clean.

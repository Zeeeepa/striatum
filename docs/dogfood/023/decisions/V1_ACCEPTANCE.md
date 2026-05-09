---
schema_version: striatum.decision.v1
decision_id: "dec_159fd895bef24250a596b2350554b51a"
run_id: "run_2ed0a979e5934ace8a031f76f1d273eb"
artifact_kind: decision
owner: human
outcome: accepted
follow_up_required: false
title: "RFC 0024 V1 (workflow browser, browse-only) accepted"
created_at: "2026-05-09T16:47:18Z"
---

# RFC 0024 V1 (workflow browser, browse-only) accepted

Decision ID: `dec_159fd895bef24250a596b2350554b51a`
Run ID: `run_2ed0a979e5934ace8a031f76f1d273eb`
Outcome: `accepted`

## Rationale

V1 ships read-only browse: discover walk + workflows index + workflow detail + list_workflows chat tool. No new runtime deps, reuses RFC 0022 V1's SVG renderer, path-safe like /view/<path>. Visual builder deferred to V1.5.

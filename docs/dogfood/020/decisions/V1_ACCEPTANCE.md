---
schema_version: striatum.decision.v1
decision_id: "dec_b2a291eb8c55435c8b15931d2d1c2325"
run_id: "run_258219d9e1ad4b9fa9d9672999e0ff5f"
artifact_kind: decision
owner: human
outcome: accepted_with_follow_up
follow_up_required: true
title: "RFC 0022 V1 (web UI redesign) accepted with two notes"
created_at: "2026-05-09T07:41:14Z"
---

# RFC 0022 V1 (web UI redesign) accepted with two notes

Decision ID: `dec_b2a291eb8c55435c8b15931d2d1c2325`
Run ID: `run_258219d9e1ad4b9fa9d9672999e0ff5f`
Outcome: `accepted_with_follow_up`

## Rationale

V1 ships RFC 0022's three steps: Jinja2 SSR + multi-page routing, refreshed CSS palette + dark mode, layered SVG dependency graph with click-navigate. Two design-review findings (both notes): clarify cycles are not rendered as graph edges (only forward DAG), and add a legacy /static/index.html test. CSP unchanged; JSON API + SSE unchanged; mutation gating preserved.

## Follow-Up

Add Finding 2 legacy-static-index test case

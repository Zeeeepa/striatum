---
schema_version: striatum.decision.v1
decision_id: "todo-60-git-pr-boundary"
run_id: "run_1c3dc3dbfb0959d3c33538be2418f0da"
artifact_kind: decision
owner: human
outcome: accepted_with_follow_up
follow_up_required: true
title: "TODO 60 optional Git and PR integration boundary"
created_at: "2026-05-21T04:32:29Z"
---

# TODO 60 optional Git and PR integration boundary

Decision ID: `todo-60-git-pr-boundary`
Run ID: `run_1c3dc3dbfb0959d3c33538be2418f0da`
Outcome: `accepted_with_follow_up`

## Rationale

Human principal accepts the recommendation: Striatum core does not autonomously commit, push, call hosted providers, or import provider SDKs; read-only local Git snapshots come first; durable commit-request and PR-request artifacts may be added; local git commit-apply may create a local commit only after explicit operator confirmation; hosted provider actions stay out of core and require human-principal confirmation if later accepted as optional plugin behavior.

## Follow-Up

Sequence RFC 0067 read-only snapshots, commit-request and PR-request artifact contracts, optional local commit-apply behind explicit confirmation, and only then any optional provider plugin decision.

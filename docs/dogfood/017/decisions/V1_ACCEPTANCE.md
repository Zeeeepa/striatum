---
schema_version: striatum.decision.v1
decision_id: "dec_67ab03c61113470e8d0e8faffec94463"
run_id: "run_8f0bf6f3e6954f67a88e8db5a1e2920e"
artifact_kind: decision
owner: human
outcome: accepted_with_follow_up
follow_up_required: true
title: "RFC 0021 V1 (DDD layout scaffold) accepted with two implementation findings"
created_at: "2026-05-09T02:17:26Z"
---

# RFC 0021 V1 (DDD layout scaffold) accepted with two implementation findings

Decision ID: `dec_67ab03c61113470e8d0e8faffec94463`
Run ID: `run_8f0bf6f3e6954f67a88e8db5a1e2920e`
Outcome: `accepted_with_follow_up`

## Rationale

V1 ships RFC 0021 in two steps (template tree + CLI wiring). The devil's-advocate review surfaced two acceptance-blocking findings the implementation must address: (1) use Path.is_file() not Path.exists() for the per-file existence check so a target that's a directory or broken symlink reports status:error rather than silently being treated as 'skipped'; (2) drop the '__' directory-separator naming convention — the skill-bundle precedent shows real subdirectories work fine with setuptools package-data patterns like **/*.md.tmpl. Two non-blocking findings folded into the test plan: assert exactly seven files via importlib.resources, and add a filesystem-error per-file test.

## Follow-Up

Note Finding 1 (composability with --with-skills) in BUILD_HANDOFF

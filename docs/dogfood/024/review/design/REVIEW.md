# Design review (devils_advocate): RFC 0024 V1.5

author: reviewer-claude-opus-001
date: 2026-05-09
verdict: accept_with_findings

## Verdict

**accept_with_findings** — V1.5 is implementable. Two findings (one acceptance-blocking).

## Sweep

### F1 (acceptance-blocking) — JSON POST size + content-type policing

The synthesis has the JS island POST `Content-Type: application/json` with the full workflow JSON. Striatum's existing `_dispatch_post` only handles `/v1/invoke` (JSON via `_read_json_body`) and chat (form bodies). The new endpoint needs:

1. Max-body-size cap to prevent OOM (synthesis doesn't pin one).
2. Content-Type validation: refuse non-`application/json` bodies with 415.

Recommend: 1 MB body cap (workflows are tiny — even a 100-job workflow is ~50 KB).

### F2 (note) — Empty-state UX for non-existent paths

Synthesis says GET on a non-existent path returns a scaffold. Two concerns:

1. `workflow_id` derived from path stem (`examples/foo/workflow.json` → `foo`) — usually fine.
2. The page should signal "Editing **new** workflow" vs "Editing **existing** workflow" so the operator knows which mode.

### Other concerns

- Concurrency (last-writer-wins): acceptable for single-operator local model.
- localStorage backup: good defensive UX.
- Field-level error highlighting deferred: V1.5 floor is the top-of-form banner.
- Mutation gate + redirect-on-save: standard pattern.

## Findings summary

| # | Severity | Action |
| --- | --- | --- |
| F1 | acceptance-blocking | 1 MB body cap; reject non-`application/json` with 415 |
| F2 | note | Page header signals new vs existing workflow |

## Decision

Accept V1.5 with F1 implemented and F2 noted in BUILD_HANDOFF.

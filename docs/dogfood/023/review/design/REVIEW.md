# Design review (devils_advocate): RFC 0024 V1

author: reviewer-claude-opus-001
date: 2026-05-09
verdict: accept

V1 is read-only browse — small, well-scoped, reuses existing primitives.

## Counterarguments

### "Discovery walk performance for big repos"

`rglob("workflow.json")` walks the entire tree. For a 100k-file repo this could be slow. **Survives?** The skip-dir filter excludes the most common big subtrees (`.git`, `node_modules`, `.venv`). For repos that hit pathological cases (a `dogfood/` with thousands of run dirs), V1.5 can add caching. **Accept.**

### "list_workflows tool budget could explode"

A repo with 1000 workflow.json files would produce a 100K-byte tool result. **Survives** — synthesis caps at 100 entries with truncation marker. **Accept.**

### "Path traversal — same risks as /view/<path>"

Same path-safety logic; that pattern is well-tested in RFC 0023 V1. **Accept.**

### "Invalid workflow rendering — does it actually 200 not 500?"

The synthesis catches `WorkflowError` + `json.JSONDecodeError` in `discover` AND in the detail handler. Implementer should write `test_workflow_detail_renders_invalid` to cover this. Already in test plan. **Accept.**

### "Why no thumbnail-specific render path?"

CSS scaling of the full-size SVG is the right answer for V1. The full SVG is at most ~5 KB; rendering it 200×150 visually is fine. **Accept.**

### "RFC 0024 V1 isn't dogfooded against itself"

The browser surface lists workflow.json files including `docs/dogfood/023/workflow.json` itself. Operators visiting `/workflows` after V1 lands will see it in the list. Self-evident dogfood. **Accept.**

## Decision

Accept clean. Implementer can proceed.

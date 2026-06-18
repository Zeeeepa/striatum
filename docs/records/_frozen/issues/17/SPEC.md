
    # GH #17 -- Update Striatum document consistency for Engram memory integration

    Source: <https://github.com/halbritt/striatum/issues/17> (filed 2026-05-14).
    Labels: none.
    Captured here verbatim so the runner's `context.docs` is self-contained
    and reviewers do not need GitHub API access mid-run.

    ---

    ## Summary

Striatum documentation should be made internally consistent around the Engram memory-system integration path.

Engram is being reprioritized around the Striatum use case first, with personal-memory functionality deferred. Striatum docs should reflect that direction consistently so operators, workflow agents, and future implementors all receive the same guidance.

## Context

Recent Engram planning has established Striatum as the primary near-term memory-system use case:

- Engram should ingest and index Striatum operator logs, workflow-agent logs, RFCs, designs, reviews, operator reports, changelogs, git history, issues, blockers, and generated artifacts.
- Striatum should treat Engram as an optional local memory backend with graceful degradation when unavailable.
- Runtime boundaries should remain explicit: Striatum should not import Engram client code directly or depend on an always-running Engram daemon unless that is intentionally designed later.
- Personal-memory functionality is deferred behind the Striatum application-memory path.

## Requested Work

Audit Striatum documentation for consistency around:

- Operator initialization guidance.
- Workflow-agent handoff and logging expectations.
- Memory/export bundle expectations for Engram ingestion.
- PostgreSQL transition guidance, where relevant to memory/history storage.
- References to Engram as optional/local-only infrastructure.
- Graceful-degradation behavior when Engram is unavailable.
- Any stale docs that imply a different persistence, memory, or operator context model.

## Acceptance Criteria

- A single coherent Striatum documentation path exists for operator initialization and memory integration.
- Docs describe what Striatum should export for Engram ingestion.
- Docs avoid implying cloud persistence or required external services.
- Docs make the Striatum/Engram boundary clear.
- Stale or conflicting guidance is updated or explicitly marked deferred.

## Related

This complements the existing issues for PostgreSQL transition guidance and a complete operator initialization prompt.

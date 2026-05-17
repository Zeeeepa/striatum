# RFC 0067: Optional Git and PR Integration

## Status
Blocked on product decision

## Summary
Optional Git/PR integration is intentionally deferred until Striatum has an
accepted policy for local commit authority and hosted-provider boundaries.

## Motivation
Derived from the STRIATUM Architecture Review and Remediation Plan (2026-05-16).

## Boundary

Striatum is local-first. Any Git/PR integration must preserve the product
boundary: no hosted services, cloud APIs, external persistence, or telemetry
without an explicit product decision.

## Safe First Slice

The only implementation slice that can proceed without a new product decision
is read-only local git snapshotting, for example:

- current branch and HEAD SHA,
- dirty-tree summary,
- file-level changed-path list,
- local commit ancestry metadata.

These methods must not create commits, push branches, call hosted providers, or
persist external identifiers.

## Blocked Decisions

Implementation beyond read-only local snapshots is blocked until an accepted
decision or RFC answers:

- Who has authority to create a commit?
- Is commit creation represented as an artifact/request first, with explicit
  operator confirmation before apply?
- Which confirmation model is required for commit apply?
- Are hosted-provider APIs allowed at all?
- If hosted providers are allowed, are they implemented only as optional
  plugins/connectors?
- Where are provider credentials configured, and how are they kept outside
  durable runner state?
- Are PR creation/update operations in scope, or should Striatum stop at a
  local patch/commit handoff?

Until those questions are decided, TODO 60 remains blocked.

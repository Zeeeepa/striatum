---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0081: Conversation Trajectories — Implementation Synthesis

author: implementer-gemini-001
Date: 2026-05-25

## Summary

Implemented real-time observable conversation trajectories, canonical ordering, and the conversation workflow type as defined in RFC 0081.

### 1. Canonical Ordering: `run_event_seq`
Introduced a monotonic per-run sequence assigned at daemon ingest. This sequence provides the primary ordering authority for trajectories across messages, events, artifacts, verdicts, and blockers.
- Added `run_event_seq` column to core database tables via migration `0015_conversation_trajectories.sql`.
- Established `striatumd.run_event_sequence` as the global source of truth for trajectory ordering.

### 2. Read-Model Projection
Implemented `trajectory.export` and `trajectory.watch` daemon RPC methods in `go/pkg/reads/trajectory.go`.
- Supported `dialogue` and `provenance` profiles.
- **Strict D028 Compliance:** Ensured that trajectories never retrieve raw provider transcripts (stdout/stderr) or uncurated adapter output.
- Both methods return JSONL-ready record sets ordered by `run_event_seq`.

### 3. CLI & tmux Integration
Extended `striatum trajectory` to support real-time observation.
- `striatum trajectory export --run-id <id> [--profile <p>]`: ordered JSONL export.
- `striatum trajectory watch --run-id <id> [--profile <p>]`: continuous tailing watch loop.
- Added documentation for tmux integration and `jq` filtering in `docs/operator/DAEMON_RUNBOOK.md`.

### 4. Conversation Workflow Type
Added a new `conversation` workflow shape to the generator and template catalog.
- Supports N-turn, M-model alternating speaker dialogue over the message bus.
- Updated `go/pkg/workflowgenerate/`, `go/pkg/workflowtemplates/`, and `docs/WORKFLOW_TYPES.md`.

## Verification Results

### Automated Tests
- **Go Tests:** `go test ./...` passed, covering migrations, RPC registry, read model curation, and template catalog.
- **D028 Guard:** `TestHandleTrajectoryExportD028` verified that raw transcript fields are curated out of the read model projection.
- **Parity Tests:** Kept Python-source migrations and catalogs in sync to maintain compatibility test integrity.

### Implementation Status
| Feature | Status | Note |
|---|---|---|
| `run_event_seq` | Done | Migration 0015 applied. |
| `trajectory.export` | Done | RPC + CLI JSONL. |
| `trajectory.watch` | Done | RPC + CLI watch loop. |
| `trajectory_segments` | Done | Metadata table created. |
| `conversation` shape | Done | Generator + Catalog updated. |
| D028 Boundary | Done | Enforced in `reads/trajectory.go`. |

This implementation completes the RFC 0081 requirements for local-first, observable conversation trajectories.

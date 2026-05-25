# RFC 0081: Conversation Trajectories

Status: accepted
Date: 2026-05-25
Accepted: 2026-05-25 (D137; implemented in the rfc-0079-0081-closure run.
Final design uses a read-derived per-run sequence — ROW_NUMBER over
created_at + source class + primary key — rather than a stored run_event_seq
column, since existing tables are owner-restricted and the converged design
called for no new authority. Verified: `striatum trajectory export` reproduces
the recorded two-model conversation.)
author: proposer-claude-opus-4-7-001
Context:
[`RFC 0030`](0030-daemon-rpc-server-and-version-skew-protocol.md),
[`RFC 0043`](0043-postgres-as-sole-substrate-and-daemon-required-runtime.md),
[`docs/SPEC.md`](../SPEC.md),
[`docs/DECISION_LOG.md`](../DECISION_LOG.md) (D028 no-transcript-capture),
`docs/operator/artifacts/two-model-conversation/` (the design dialogue that
produced this RFC)

## Problem

Agents already communicate over a run-scoped message bus
(`work.send_message` → a `messages` row + a `message.sent` event), and the
daemon records a rich lifecycle event stream (claim, ack, complete, verdict,
artifact.published, blocker transitions). Together these *are* the trajectory
of a run — including multi-model conversations. But there is no way to read,
export, or watch that trajectory: `inbox` is escalation-scoped, and the events
are only reachable by ad-hoc inspection. A two-model conversation spike
confirmed the bus works end-to-end yet surfaced exactly this gap.

We want first-class trajectory observability **without** crossing the D028
boundary: Striatum must not capture or persist raw provider transcripts
(stdout/stderr of the model CLIs).

## Goals

- Read-only **trajectory projection** over existing daemon-owned records — no
  new authoritative state, no second write target to drift.
- Two projection profiles: `dialogue` (chat messages + artifact/publication
  references) and `provenance` (the fuller lifecycle: claim/ack/complete/
  verdict/blocker, plus dialogue).
- `striatum trajectory export` (replayable/diffable JSONL) and `striatum
  trajectory watch` (live tail) with a tmux-friendly renderer.
- A formal **conversation workflow type** so multi-model dialogues are a
  declared, generatable shape rather than a hand-built fixture.
- Strict D028 compliance: projected rows carry curated message bodies, state
  labels, references, and content hashes — never scraped model output.

This design is the consensus of the recorded claude↔codex conversation: keep
one ordering model, project over existing tables, add no new authority.

## Non-Goals

- Capturing provider transcripts, stdout/stderr, or token streams (D028).
- A new authoritative `trajectory_events` table (rejected in the dialogue in
  favor of a read model over `messages`/`events`).
- Hosted/streaming export, external sinks, or telemetry.

## Proposal

### 1. Canonical ordering: `run_event_seq`

Adopt a monotonic per-run sequence assigned at daemon ingest as the canonical
ordering primitive across `messages`, `events`, `artifacts`, and `verdicts`.
If not already present, add it as a daemon-assigned column/derivation; it is
the only ordering authority a trajectory needs.

### 2. Read-model projection (no new authority)

Trajectory reads are daemon RPC methods that project existing records ordered by
`run_event_seq`:

- `trajectory.export` — returns the ordered, profile-filtered records for a run.
- `trajectory.watch` — a cursor (`since_seq`) tailing new records.

Projected rows are constrained to: `seq`, `ts`, `session_id`, `role_id`,
`lane_id`, `kind`, `parent_message_id`, curated `body` (for `chat`), state
labels (for lifecycle), references (artifact paths), and stable content hashes.
No provider stdout/stderr is ever read or stored.

### 3. `trajectory_segments` (export/checkpoint metadata only)

A narrow table holding export manifests and watch checkpoints (run_id, profile,
from_seq, to_seq, content hash, created_at). It is **not** trajectory authority;
it records what was exported/checkpointed for reproducibility and resumable
watch. Deleting it loses no trajectory data.

### 4. CLI + tmux

- `striatum trajectory export --run-id <id> --profile dialogue|provenance
  --format jsonl` → ordered JSONL with typed summaries + hashes.
- `striatum trajectory watch --run-id <id> [--profile ...] [--since <seq>]` →
  live tail; render-friendly for a tmux pane beside `striatum dashboard`.
- tmux tailing is disposable UI with no workflow-state side effects.

### 5. Conversation workflow type

A declared workflow shape (and generator support) for N-turn, M-model
conversations over the bus: alternating speaker lanes, a topic/seed, and per-turn
"read inbox → reply via `send` → complete" packets. The two-model-conversation
fixture becomes the canonical example.

## Acceptance Criteria

- `trajectory.export`/`trajectory.watch` reproduce the recorded
  two-model-conversation run for both profiles, ordered by `run_event_seq`.
- No projected row contains provider stdout/stderr; a guard test asserts the
  read model only touches curated fields (D028).
- `trajectory_segments` holds only export/checkpoint metadata; dropping it loses
  no conversation content.
- `striatum trajectory watch` tails a live run; a tmux recipe is documented.
- A `conversation` workflow type validates and generates; the example runs.
- `go test ./...` green (with live-PG harness from RFC 0080); guardrail strict.

## Implementation Plan

1. `run_event_seq` ordering (derive/assign at ingest) + tests.
2. `trajectory.export`/`trajectory.watch` read handlers + registry + routes;
   `trajectory_segments` migration.
3. `striatum trajectory export/watch` CLI + JSONL renderer + tmux recipe doc.
4. `conversation` workflow type + generator + example; doc in
   `docs/WORKFLOW_TYPES.md`.
5. D028 guard test; operator doc in the daemon runbook.

## Risks

- Profile leakage could expose more than intended; the projection allowlist
  (not denylist) keeps fields curated.
- `run_event_seq` retrofitting across existing tables must preserve current
  ordering semantics; cover with migration invariant tests (RFC 0080 harness).

## Open Questions

- Is `run_event_seq` already derivable from existing event ids/timestamps, or
  does it need a dedicated daemon-assigned column? (Implementation decides.)
- Should `export` support a redaction tier beyond field-allowlisting for
  shareable artifacts?

## Domain Modeling

Introduces *trajectory* as a read-model projection of the existing run
aggregate — a derived view, not new authority. `dialogue` and `provenance` are
profiles over the same ordered event stream. This extends observability without
adding workflow state, consistent with the daemon-as-single-writer boundary and
the D028 no-transcript rule.

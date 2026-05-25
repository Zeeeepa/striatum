---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["run_f3dfcf2dfe7244d2b237bdba0d51e509 chat bus (turn 3)", "msg_fba84ee65388a5891f994a5252404d96"]
---

# Two-Model Conversation — Turn 3 (Claude refines)

author: speaker-claude-001

Accepted Codex's turn-2 push to drop my authoritative `trajectory_events`
table in favor of a read model over the existing `messages` and `events`
tables, agreeing that a second write target only invites divergence and that
no-new-authority is the cleaner expression of the D028 boundary. I accepted
`run_event_seq` as the canonical ordering primitive but tightened its
semantics: it must be allocated inside the same transaction that inserts each
message/event row, with a `UNIQUE(run_id, run_event_seq)` constraint backed by
a per-run cursor row taken `FOR UPDATE`, so concurrently-supervised lanes
cannot race into a non-reproducible interleaving. I pinned `trajectory_segments`
to an export-manifest/checkpoint role only —
`(run_id, segment_id, from_seq, to_seq, content_hash, created_at)` — where
`content_hash` chains the prior segment hash plus per-event digests to make
JSONL export idempotent, resumable, and tamper-evident, with
`trajectory.watch` resuming a tail purely from `to_seq`. I reaffirmed that each
JSONL row is a projected event (seq, role/lane, parent_message_id, typed
`summary` = curated body or artifact path+digest) and never raw provider
output, and that `trajectory.watch` stays strictly observational — a read-only
cursor RPC with no acks or lease side effects so a tmux pane can render it
without becoming workflow state. I handed Codex one open question: whether
lifecycle transitions (claim/ack/complete/verdict) project into the same stream
as `chat`/`artifact.published`, or whether we keep a narrow "dialogue" view and
a full "provenance" view selectable via `--include lifecycle`.

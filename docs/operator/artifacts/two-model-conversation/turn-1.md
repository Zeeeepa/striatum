---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["run_f3dfcf2dfe7244d2b237bdba0d51e509 chat bus (turn 1)"]
---

# Two-Model Conversation — Turn 1 (Claude opens)

author: speaker-claude-001

Opened the trajectory-logging design conversation on the run-scoped chat bus.
My opening proposal defines a *trajectory* as the run-scoped, totally-ordered
sequence of structured bus events — `chat` messages, `message.sent`, lifecycle
state transitions (claim/ack/complete/verdict), and `artifact.published` —
explicitly excluding raw provider transcripts, which D028 forbids. I proposed
persisting it daemon-side as an append-only
`trajectory_events(run_id, seq, ts, session_id, role_id, lane_id, kind, parent_message_id, payload jsonb)`
table whose `payload` holds curated agent-authored content plus references
(artifact paths, content digests) rather than scraped model output — the line
that keeps the design inside the no-transcript-capture boundary. For tooling I
suggested `striatum trajectory export --run-id <id> --format jsonl` for
replayable/diffable export and `striatum trajectory watch --run-id <id>` (a
`trajectory.tail` cursor over `seq`) rendered in a tmux pane beside
`striatum dashboard`. I closed by handing the next model two open questions:
whether `trajectory_events` should be a new table or a materialized view over
existing `messages`/`events`, and how to anchor cross-lane ordering (monotonic
per-run `seq` at daemon ingest vs. a hybrid logical clock).

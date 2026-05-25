# Two-Model Conversation — one turn

You are one model in a structured two-model conversation held entirely over
the Striatum message bus (`work.send_message` / `inbox`). The other turns are
taken by a different model. The conversation is the trajectory; it lives as
run-scoped `chat` messages and `message.sent` events, not as files.

## Topic

**How should Striatum log conversation trajectories between agents while
respecting the D028 no-transcript-capture boundary?** Propose and refine a
concrete, elegant design: what a "trajectory" is, what is persisted (structured
message/event provenance, not raw model transcripts), where it lives
(daemon-owned PostgreSQL), how it is exported, and how an operator watches it
live (e.g., tmux). Build on the other model's last message — agree, refine, or
push back with specifics.

## Your turn (do exactly this, then complete)

1. Read the conversation so far:
   `striatum inbox --run-id <RUN_ID from packet.run.run_id> --json`
   (messages have `kind: "chat"`; read the most recent ones).
2. Compose your next turn: 3–6 sentences that advance the design, explicitly
   responding to the other model's most recent point. Be concrete (name
   tables/events/commands), not generic.
3. Post it to the bus with the `send` command from your packet's `commands`
   block, e.g.:
   `striatum send --session-id <your session_id> --kind chat --body "<your turn>"`
4. Publish a one-paragraph `progress_note` artifact summarizing your turn to
   `docs/operator/artifacts/two-model-conversation/turn-<N>.md` if your packet
   lists an expected artifact, then run the `complete` command from your packet.

Do not write conversation content to files — the bus is the channel. Keep the
exchange substantive; this dialogue is the design input for the trajectory RFC.

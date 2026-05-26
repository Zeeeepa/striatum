# Coordinator

The human-facing operator role. You do not run inside an agent lane; you observe
the runner, drive headless lanes, and make accept/override decisions at
human-checkpoint gates.

Responsibilities:

- Launch headless lanes pinned to explicit models (`claude --model opus`,
  `codex exec -m gpt-5.5`). Do NOT use the PTY launcher for TUI agents.
- Watch `striatum dashboard --run-id <id>` for stuck jobs, stale leases, failing
  reviews, and interrogation threads that never close.
- Confirm the `interrogable` synthesizer and implementer sessions reach
  `awaiting_interrogation` and stay live until the panel closes interrogation.
- Decide whether `needs_revision` cycles are productive or whether to abort.
- Apply recovery verbs when adapters or lanes wedge.

The coordinator never edits artifacts inside
`docs/operator/workflows/f42-conversation-turn-driver/artifacts/`.

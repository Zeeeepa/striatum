# Coordinator

You are the human-facing operator role for this fixture. You do not run
inside an agent lane; you observe the runner and make accept/override
decisions at human-checkpoint gates.

Responsibilities:

- Supply the concrete task (replace `{{TASK}}` in the prompts) before
  `striatum run start`.
- Watch `striatum dashboard --run-id <id>` for stuck jobs, stale leases,
  failing reviews, and interrogation threads that never close.
- Decide whether `needs_revision` cycles are productive or whether to
  abort the run and rewrite the synthesis or implementation.
- Confirm that the reviewed (`interrogable`) synthesizer and implementer
  sessions reach `awaiting_interrogation` and stay live until the panel
  closes its interrogation threads.
- Apply the `striatum recovery` verbs when adapters or lanes wedge.

The coordinator never edits artifacts inside
`examples/iterated-interrogating-panel/artifacts/`.

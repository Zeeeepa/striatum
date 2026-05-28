# Design review - adversarial interrogation

Read `TASK.md`, RFC 0089, and
`docs/operator/workflows/rfc-0089-tmux-helper-redesign-rerun/artifacts/DESIGN_SYNTHESIS.md`.
Your posture is `devils_advocate`.

You must interrogate the live synthesizer before verdict:

1. Open an interrogation against the synthesizer session.
2. Ask up to 3 rounds targeting load-bearing assumptions.
3. Poll `interrogation.show` for answers.
4. Close the interrogation.

Write two artifacts in your review directory:

- `REVIEW.md`: finding front matter (`schema_version: striatum.finding.v1`,
  `artifact_kind: finding`, `verdict_intent`, `severity`, `tags`), your author
  byline, findings, verdict, round count, and stop reason.
- `INTERROGATION_CHAT.md`: curated interrogation log with interrogation id,
  target session id, turn order, questions, and answers from
  `interrogation.show`. Do not include raw terminal/tmux/pty output.

Finalize with one `review.submit` for `REVIEW.md`. Publish the chat log as the
declared handoff artifact before completing if the packet requires separate
artifact publication.

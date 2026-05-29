# Adjudicator Role

Use this role for RFC 0093 collaboration shapes where a live dialogue is gated
by a `collaboration_ledger` verdict.

Read only curated workflow state:

- the RFC 0081 `dialogue` trajectory, preferably through
  `trajectory export --profile dialogue`
- published participant artifacts named in the work packet
- explicit work-packet commands and expected artifact contracts

Do not use raw PTY logs, provider stdout/stderr, transcript exports, or private
diagnostics as evidence. The ledger must reference dialogue turn ids such as
`dialogue:3`; it must not copy raw terminal output into front matter.

Publish a Markdown `collaboration_ledger` artifact at the exact logical name
and path the work packet's `expected_artifacts` give you. Revision cycles are
cycle-scoped: each `needs_revision` re-run hands you a fresh
`cycle_<N>` logical name and path, so a re-adjudication never collides with the
prior cycle's ledger. Always publish to the packet-provided target rather than a
hard-coded filename.

The artifact carries `striatum.collaboration_ledger.v1` front matter. A clearing verdict
(`accept` or `accept_with_findings`) requires, at minimum, one `claim`, one
`challenge`, and one `rebuttal` entry with dialogue refs. Use
`needs_revision` when a challenge is material and unrebutted, or when the
dialogue is theatrical but lacks a concrete falsifying challenge and direct
answer.

Apply this rubric:

- `claim`: a concrete participant assertion that downstream work depends on
- `challenge`: a material objection, falsifying question, or counterexample
- `rebuttal`: a direct answer that resolves or narrows the challenge
- `constraint`: a boundary that limits the acceptable downstream action
- `nomination`: a proposed claim to retain, retire, or carry forward

You may demote a fluent exchange to `needs_revision` if the substance is absent.
Do not promote a weak exchange merely because every participant completed a turn.

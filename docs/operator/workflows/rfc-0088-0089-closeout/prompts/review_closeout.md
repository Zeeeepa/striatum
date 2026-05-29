# Phase 2 review — RFC 0088 closeout, interrogating panel

Read:

- `docs/operator/workflows/rfc-0088-0089-closeout/TASK.md`
- `docs/rfcs/0088-deprecate-print-interactive-pty-lanes-agy-migration.md`
- `docs/operator/workflows/rfc-0088-0089-closeout/artifacts/build_0088/HANDOFF.md`
- the current diff (note: this includes the Phase 1 RFC 0089 changes plus the
  Phase 2 deletions).

Your posture comes from the work packet.

You must interrogate the live builder before verdict:

1. Identify the live implementer session for this run. Prefer the latest active
   `role_id=implementer`, `lane_id=codex_builder` session.
2. Open an interrogation against that implementer session.
3. Ask up to 3 rounds focused on your posture.
4. Poll `interrogation.show` for answers.
5. Close the interrogation.

If `interrogation.open` fails, do not publish a review artifact and do not
record a verdict. Use `work.block` with a concise reason instead.

Posture focus:

- `threat_model`: was any live authoring/attestation path deleted before its
  replacement was proven? Do D148-D151 cite REAL evidence (a session id + run
  id that actually exist for this run), or are they backdated assertions? Does
  removing the `--print` wrapper silently downgrade any artifact byline to
  `author: operator`? Does owned-PTY attestation still hold after the deletion?
- `ergonomics_dx`: after deletion, is the lane model coherent and discoverable?
  Any dangling references to turn-driver / single_shot / --print in help text,
  docs, spec, or the command-authority matrix? Does the retired-vocabulary gate
  give a clear, actionable failure message?
- `devils_advocate`: hunt for dangling call sites that compile but are dead;
  tests that pass while a real lane path is broken; a grep gate that does not
  actually fail on a planted retired token; glossary/spec claims that disagree
  with the deleted code surface.

Run relevant verification. At minimum:

```bash
cd go && go vet ./... && go test ./...
```

Also run the new retired-vocabulary gate and confirm it actually fails on a
planted token (then revert the plant) — a gate that never fails is not a gate.

Write two artifacts in your review directory:

- `REVIEW.md`: finding front matter, author byline, verdict, verification run,
  interrogation id, interrogation round count, and stop reason.
- `INTERROGATION_CHAT.md`: curated `interrogation.show` question/answer log.
  Do not include raw terminal/tmux/pty output.

Verdict policy: `accept` if no material residual risk; `accept_with_findings`
for non-blocking follow-ups; `needs_revision` only for a real blocker (e.g. a
dangling reference, a fake-looking decision-log evidence citation, or a gate
that does not fire) that must be fixed before RFC 0088 can be declared closed.

Finalize with one `review.submit` for `REVIEW.md` using your declared logical
name, and publish `INTERROGATION_CHAT.md` as the declared handoff artifact. The
job is complete only after the files exist, the verdict is recorded through
Striatum commands or MCP tools, and the review transition succeeds.

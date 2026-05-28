# Final build review - interrogating panel

Read:

- `docs/operator/workflows/rfc-0089-tmux-final-build-review/TASK.md`
- `docs/rfcs/0089-tmux-backed-lane-monitoring.md`
- `docs/operator/workflows/rfc-0089-tmux-final-build-review/artifacts/build/HANDOFF.md`
- the current diff.

Your posture comes from the work packet.

You must interrogate the live Codex builder before verdict:

1. Identify the live implementer session for this run. Prefer the latest active
   `role_id=implementer`, `lane_id=codex_builder` session.
2. Open an interrogation against that implementer session.
3. Ask up to 3 rounds focused on your posture.
4. Poll `interrogation.show` for answers.
5. Close the interrogation.

If `interrogation.open` fails, do not publish a review artifact and do not
record a verdict. Use `work.block` with a concise reason instead.

Posture focus:

- `threat_model`: byline/provenance safety, no tmux-text authority, pid reuse,
  stale pane/session spoofing, delivery-degraded false-health risk, stop and
  recovery safety.
- `ergonomics_dx`: attach command visibility, delivery-vs-pane health clarity,
  failure classes operators can act on, fallback behavior, status/dashboard
  discoverability.
- `devils_advocate`: race conditions, stale tmux server state, platform gaps,
  tests that can pass while the real lane is still misreported.

Run relevant verification. At minimum inspect the tests and run:

```bash
cd go && go test ./pkg/supervisor ./pkg/mutations ./pkg/reads
```

Write two artifacts in your review directory:

- `REVIEW.md`: finding front matter, author byline, verdict, verification run,
  interrogation id, interrogation round count, and stop reason.
- `INTERROGATION_CHAT.md`: curated `interrogation.show` question/answer log.
  Do not include raw terminal/tmux/pty output.

Finalize with one `review.submit` for `REVIEW.md`. Publish the chat log as the
declared handoff artifact before completing if the packet requires separate
artifact publication.

Do not stop after printing a review in terminal prose. The job is only complete
after the files exist on disk, the declared artifacts/verdict have been recorded
through Striatum commands or MCP tools, and `work.complete` succeeds. This is
especially important for one-shot print lanes.

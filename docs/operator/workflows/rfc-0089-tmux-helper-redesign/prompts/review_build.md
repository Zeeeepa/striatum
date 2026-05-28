# Build review - interrogating panel

Read `TASK.md`, RFC 0089,
`docs/operator/workflows/rfc-0089-tmux-helper-redesign/artifacts/build/HANDOFF.md`,
and the diff. Your posture comes from the work packet.

You must interrogate the live codex builder before verdict:

1. Open an interrogation against the implementer session.
2. Ask up to 3 rounds focused on your posture.
3. Poll `interrogation.show` for answers.
4. Close the interrogation.

Posture focus:

- `threat_model`: byline/provenance safety, no tmux-text authority, pid reuse,
  stale pane/session spoofing, stop/recovery safety.
- `ergonomics_dx`: attach command visibility, failure classes operators can act
  on, fallback behavior, status/dashboard clarity.
- `devils_advocate`: race conditions, stale tmux server state, platform gaps,
  tests that can pass while the real lane is still misreported.

Run relevant verification, including `cd go && go vet ./... && go test ./...`
unless the handoff gives a narrower justified command.

Write two artifacts in your review directory:

- `REVIEW.md`: finding front matter, author byline, verdict, verification run,
  interrogation round count, and stop reason.
- `INTERROGATION_CHAT.md`: curated `interrogation.show` question/answer log. Do
  not include raw terminal/tmux/pty output.

Finalize with one `review.submit` for `REVIEW.md`. Publish the chat log as the
declared handoff artifact before completing if the packet requires separate
artifact publication.

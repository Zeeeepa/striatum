# Phase 2 review — AGY one-shot interrogating panel (RFC 0088 closeout)

You are the AGY reviewer for the `devils_advocate` Phase 2 review.

This lane is launched with `agy --print`, so you get exactly one model process.
Do not exit after opening or asking an interrogation question. The job is not
done until the interrogation is closed, both artifacts exist, the chat artifact
is published, and `submit-review` succeeds.

Read:

- `docs/operator/workflows/rfc-0088-0089-closeout/TASK.md`
- `docs/rfcs/0088-deprecate-print-interactive-pty-lanes-agy-migration.md`
- `docs/operator/workflows/rfc-0088-0089-closeout/artifacts/build_0088/HANDOFF.md`
- the current diff.

Use the packet's exact CLI fallback commands when they are supplied. Keep the
packet's `session_id`, `job_id`, `lease_id`, `message_id`, artifact logical
names, and paths unchanged.

Required sequence:

1. Acknowledge the packet if it is not already acknowledged.
2. Identify the live implementer session for this run. Prefer the latest active
   `role_id=implementer`, `lane_id=codex_builder` session.
3. Open exactly one interrogation against that implementer session.
4. Ask exactly one focused devil's-advocate question (e.g. "name a call site
   that referenced the deleted turn-driver / single_shot / --print wrapper and
   show me it is gone, not just unreferenced; and show that D148-D151 cite a
   session id and run id that actually exist for this run").
5. Poll `interrogation.show --json` until the builder answer is present. Wait
   and retry; do not stop after asking. If no answer arrives before the lease
   would expire, call the packet's block command instead of writing a verdict.
6. Close the interrogation after the answer is present.
7. Run:

```bash
cd go && go vet ./... && go test ./...
```

   Also run the new retired-vocabulary gate and confirm it fails on a planted
   token (then revert the plant).

8. Write both artifacts in:
   `docs/operator/workflows/rfc-0088-0089-closeout/artifacts/review/closeout/agy/`

Artifacts:

- `REVIEW.md`: finding front matter, author byline, verdict, verification run,
  interrogation id, interrogation round count, and stop reason.
- `INTERROGATION_CHAT.md`: curated `interrogation.show` question/answer log.
  Do not include raw terminal/tmux/pty output.

Verdict policy:

- `accept` if no material residual risk remains.
- `accept_with_findings` for non-blocking follow-up findings.
- `needs_revision` only for a real blocker (dangling reference, fake-looking
  decision-log evidence, or a gate that does not fire) that must be fixed before
  RFC 0088 can be declared closed.

Finalization sequence:

1. Publish `INTERROGATION_CHAT.md` as the declared handoff artifact
   `closeout_interrogation_chat_agy`.
2. Submit `REVIEW.md` with the packet's `submit-review` command or equivalent
   `review.submit` call, using logical name `closeout_review_agy`.
3. Do not call `work.complete` for the review job unless the packet explicitly
   tells you to; `submit-review` is the review job completion transition.

Do not stop after terminal prose. If any required daemon command fails, inspect
the error and either retry with the packet's exact command shape or call the
packet's block command with a concise reason.

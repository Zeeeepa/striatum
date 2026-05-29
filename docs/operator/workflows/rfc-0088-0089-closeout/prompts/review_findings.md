# Phase 1 review — RFC 0089 findings, interrogating panel

Read:

- `docs/operator/workflows/rfc-0088-0089-closeout/TASK.md`
- `docs/rfcs/0089-tmux-backed-lane-monitoring.md`
- `docs/operator/workflows/rfc-0088-0089-closeout/artifacts/build_0089/HANDOFF.md`
- the current diff.

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

- `threat_model`: can a `degraded` lane be reported `healthy` (false-health)?
  Can `rebridge` deliver to a dead pane and silently drop messages? Does the
  probe record leak raw pane text into daemon state? pid-reuse / stale-pane
  spoofing across the three-state transitions; attestation integrity when a
  lane goes degraded then recovers.
- `ergonomics_dx`: are delivery-state, pane-liveness, attestation, and the
  tmux-vs-PTY badge actually distinguishable at a glance on `dashboard --once`?
  Are the doctor hints specific enough that an operator knows whether to
  `rebridge` vs reclaim? Is the rebridge failure message actionable when the
  pane is dead?
- `devils_advocate`: race between a probe miss and a recovery; thresholds that
  misclassify fast TUI lanes vs slow headless lanes; tests that pass while the
  real lane is still misreported; platform gaps in the pane-pid liveness check.

Run relevant verification. At minimum:

```bash
cd go && go test ./pkg/supervisor ./pkg/mutations ./pkg/reads
```

Write two artifacts in your review directory:

- `REVIEW.md`: finding front matter, author byline, verdict, verification run,
  interrogation id, interrogation round count, and stop reason.
- `INTERROGATION_CHAT.md`: curated `interrogation.show` question/answer log.
  Do not include raw terminal/tmux/pty output.

Verdict policy: `accept` if no material residual risk; `accept_with_findings`
for non-blocking follow-ups; `needs_revision` only for a real blocker that must
be fixed before Phase 1 can land. Remember Phase 2 (the irreversible RFC 0088
deletions) is gated on your acceptance.

Finalize with one `review.submit` for `REVIEW.md` using your declared logical
name, and publish `INTERROGATION_CHAT.md` as the declared handoff artifact. The
job is complete only after the files exist, the verdict is recorded through
Striatum commands or MCP tools, and the review transition succeeds — not after
terminal prose.

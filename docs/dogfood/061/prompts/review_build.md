# Build review (3-way: codex `threat_model`, claude `ergonomics_dx`,
# gemini adversarial `threat_model`)

Read `docs/dogfood/061/build/HANDOFF.md` and the implementation in
`src/striatum/`. Produce a `finding.v1` review at
`docs/dogfood/061/review/build/<lane>/REVIEW.md` with your assigned
posture.

## Posture-specific checklists

### codex `threat_model`

- Auto-finalize never advances a job whose byline ≠
  `expected_author_line`. Test asserts this.
- YAML / JSON injection attempts in frontmatter parse-fail cleanly
  and fall through (no escalated exception).
- The audit-chain row records `decision='allowed'` AND the event
  row contains `lane_finalization=auto_from_artifact`.
- Feature flag check is at hook entry; no path bypasses it.
- Capability_token discipline preserved (auto-finalize uses an
  internal capability path that's still audit-chained).
- SQL parameterized — no string interpolation from frontmatter
  values into queries.

### claude `ergonomics_dx`

- On refusal (byline mismatch / malformed FM / flag off), the
  dashboard shows the lane-stall blocker exactly as today.
- The auto-finalize success path emits the documented event
  sequence in order; tests assert event order, not just count.
- Operator-readable error messages on test failures (not raw dict
  diffs); a parity diff helper is preferred.
- The audit log contains a greppable `lane_finalization=
  auto_from_artifact` marker so a reviewer can distinguish from
  agent-initiated finalize.

### gemini adversarial `threat_model`

- Forged byline that happens to match `expected_author_line`: does
  the runner detect intent or just byline? Document the gap if
  any.
- Frontmatter with `verdict_intent: needs_revision` on a review
  job: does the runner record `needs_revision`, not silently
  `accept`?
- Mid-write race: agent is still writing the file when the
  lease-tick fires (mtime < 10s). Does the anti-race guard
  actually protect? Add a test if missing.
- Flag-off run that somehow still triggers auto-finalize: any
  error path that re-enters the hook without re-checking the
  flag?
- Partial-finalize crash: publish lands but complete crashes —
  does the job state reconcile cleanly on the next tick?

## Verdict guidance

Use the four-verdict shape from
[`docs/UBIQUITOUS_LANGUAGE.md`](../../../UBIQUITOUS_LANGUAGE.md).
Reserve `reject` for cycle exhaustion (the implement→review cycle
has `max_iterations: 1`).

## Write scope

`docs/dogfood/061/review/build/<your lane>/REVIEW.md`.

---
schema_version: "striatum.harness_improvement_proposal.v1"
artifact_kind: "harness_improvement_proposal"
target: "spec"
expected_benefit: "Make reviewer-independence and byline policies enforceable in practice. Today the runner records what the workflow declared, and the operator can satisfy the CLI contract with a session that has already seen the author's reasoning — there is no actual independence guarantee, only a label."
risk: "Tightening enforcement may force operators to install and configure additional agent CLIs they do not have today (which is what blocked dogfood-001 in the first place). The mitigation is to document the contract loudly first, then layer enforcement on top."
rollback: "Each layer of enforcement (parent-pid check, lane-distinct check, byline anchor) can be reverted independently."
---

# HARNESS-003 — Reviewer-independence and byline are advisory, not enforced

Status: proposed
Run: dogfood-001
Reporter: reviewer-claude-opus-001 (note: not the byline declared by the
work packet; see "Observed friction")
Surface: spec

## Observed friction

The dogfood-001 workflow declares:

```
"review_change": {
  "lane_id": "codex",
  "fresh_session_required": true,
  "reviewer_access_scope": "artifact_augmented",
  "reviewer_context_policy": "fresh",
  ...
}
```

and the work packet's `expected_artifacts` carries
`author_line: "reviewer-codex-gpt-5.5-001"`. The intent is clear: the
reviewer must be a fresh codex session that has not seen the author's
context.

What the runner actually verifies, on this run:

1. The reviewer is registered as a session against the `codex` lane.
   (Verified by `register-session --role reviewer --lane codex`.)
2. `fresh_session_required: true` is recorded on the job row.
3. The work packet declares the expected byline.

What the runner does not verify:

1. That the actual process publishing the finding is a different agent
   from the author. The reviewer session id is independent, but the
   operator (the human-facing Claude Code session) is the same process
   that authored the change. Same context, same prompt history, same
   model.
2. That the published finding carries the declared byline. The artifact
   schema only enforces "if `author:` is present, it must match"; an
   empty author line is accepted, and a wrong-but-plausible author line
   is also accepted as long as it equals the packet-declared string.
3. That the operator is not driving both lanes from the same
   keyboard/process. There is no concept of "this session must not have
   seen the draft handoff."

In dogfood-001 specifically: HARNESS-001 made the configured `codex` lane
unable to actually run the review job (same supervised-CLI gap that
`claude_code` hit). The pragmatic path was for the operator to drive the
review themselves. The runner happily issued a packet, accepted a
finding, and is about to accept a verdict — even though the reviewer is
the author wearing a different session id.

## Supporting runner evidence

- run_id: `run_a04880660517480a95438fcc0368d2e0`
- job_id: `job_run_a04880660517480a95438fcc0368d2e0_review_change`
- packet_id: (in claim-next response — not stored separately)
- supervisor_id: not started for the codex lane
- relevant event types: `session.registered (lane=codex,role=reviewer)`,
  `queue.claimed`, soon to be followed by `artifact.published (kind=finding)`
  and `verdict.recorded`.

## Proposed change

Layer the enforcement bottom-up. Pick at minimum the first two:

1. **`docs/SPEC.md` Reviewer Independence subsection.** State plainly
   that `fresh_session_required` and `reviewer_context_policy: fresh`
   are advisory at the runner level. The runner enforces session-id
   distinctness and lane configuration; it cannot enforce that the
   process driving the session has not seen the author's context. List
   the threats to independence and the operator obligations.

2. **`striatum doctor` should warn when both author and reviewer
   sessions on a run have the same `pid` recorded by their supervisor
   adapters,** or when the reviewer was registered with no supervisor at
   all on a run that has any supervised author session. This is a
   conservative signal — false positives are fine; the warning's job is
   to prompt the operator to confirm independence.

3. **Optional: parent-pid policy in `register-session`.** When a job
   declares `reviewer_context_policy: fresh`, refuse to register a
   reviewer session whose parent process matches the parent of any
   active author session in the same run, unless `--force-non-fresh
   --reason "..."` is passed. This is more invasive but actually
   enforces the contract.

4. **Byline integrity.** When the work packet declares an
   `author_line`, `publish-artifact` should require an `author:` line
   on the published artifact (not just validate when present). And
   `submit-review` could require the lane id of the publisher's session
   to match the lane embedded in the declared byline. Today,
   `reviewer-codex-gpt-5.5-001` was published from a `codex`-lane
   session (correct lane), but the actual model was Claude Opus
   (wrong) and the runner has no way to know that.

## Risk

- Doctor warnings are low risk.
- Parent-pid policy can become annoying for legitimate single-host
  workflows where the operator drives both lanes deliberately. It
  needs a `--force-non-fresh` escape hatch and a doctor warning even
  with the escape hatch, so the breach is visible.
- Byline integrity requires solving "which model is actually behind
  this lane" — that's a lane-config problem (`display_model` field is
  already there but unverified). A weaker form is to require that the
  byline string match the lane's `display_model` in lowercase form.

## Rollback

- Each layer is independent. The doctor warning is the cheapest first
  step; if any layer ships and proves wrong, revert just that layer.

## Notes

This finding is filed by the operator-acting-as-reviewer, which is
exactly the situation it describes. That irony is part of the friction
report: a runner that is "checking" that I am a different reviewer than
the author cannot tell that I am the same human-facing session driving
both. The session-id distinctness check is necessary but not anywhere
near sufficient for the policy that `fresh` claims to enforce.

Cross-link: `docs/dogfood/001/findings/HARNESS-001.md` (the supervised
lane gap that forced the operator to drive both jobs).

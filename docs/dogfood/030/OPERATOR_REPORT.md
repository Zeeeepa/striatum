# Dogfood 030 Operator Report

author: operator
date: 2026-05-11
status: complete

## Run

Run ID: `run_13135619594c496ab28215d1d2a84e9a`
Workflow: `dogfood-030-rfc-0026-0027-provenance-two-lane`
Branch: `striatum/dogfood-030-rfc-0026-0027-provenance`
Final state: `completed`

## Control-Plane Outcome

The run completed after the owner instructed the operator to cancel the
remaining repeated devil's-advocate build review path. Striatum final
status reports 16 completed jobs, one canceled job, no open blockers, no
human checkpoints, and no stale, lost, timed-out, or running processes.

The canceled job was
`job_run_13135619594c496ab28215d1d2a84e9a_review_build_devils_a3`.
It was canceled with `recovery cancel-job --cascade` after repeated
`needs_revision` verdicts and checkpoint resolutions requeued the same
review job.

## Owner Decisions

- `dec_34587176cca340c1b979747bd00e5cab`: continue after the security
  design review cycle was exhausted, carrying risks forward.
- `dec_9de81e9958634e79bc9d3e1f7771de56`: treat the exhausted security
  design review as owner accepted with follow-up.
- `dec_bd869b7b016745a19afeb812f685f11c`: continue after the first
  exhausted devil's-advocate build review checkpoint.
- `dec_edb72c84426b499aac71998e655b4d2e`: continue after the second
  exhausted devil's-advocate build review checkpoint.
- Final owner instruction: cancel the remaining review path.

## Recorded Risks

The following are recorded reviewer findings and owner-accepted follow-up
risks, not operator-authored review findings:

- `attested_bylines` mode was challenged as overclaimed because the
  reviewer found no runtime distinction from `advisory` beyond validation
  and surfacing.
- `docs/DECISION_LOG.md` was challenged for a duplicate `D080` id.
- RFC 0026 end-to-end publish-time scenarios were challenged as
  insufficiently covered by tests.
- `supervise_send` was challenged as weaker than the full
  `session_lane_attestation` binding.
- run-summary and decision-artifact byline/attestation surfacing were
  challenged as incomplete relative to the printed docs.
- release hygiene remains open: version bump and changelog finalization.

## Verification Artifacts

The implementer reported passing:

- `make lint`
- `make typecheck`
- `make test` (`545 passed`)
- `make smoke`

The operator exported:

- `docs/dogfood/030/RUN_SUMMARY.md`
- `docs/dogfood/030/EVIDENCE.md`

## Deliberately Left Out

The operator did not implement or repair any reviewer findings after the
owner's cancel instruction. The unrelated untracked `foo` file at the
repository root was left untouched and is not part of the committed run
artifact set.

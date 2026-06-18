# GH #19 — Stale repo_write lease has no operator recovery path — design jobs block runs indefinitely after supervisor death

Source: https://github.com/halbritt/striatum/issues/19

## Summary

When a supervised lane process dies after writing its expected artifact but before its publish-artifact + complete callback fires, the lease expires and the job transitions to `stale_lease`. For `repo_write: true` jobs, **every documented operator-recovery verb refuses**, with no force flag and no documented "I have inspected" path. The run is blocked indefinitely — observed waiting >4 hours on dogfood-060 before operator gave up and `striatum run cancel`ed the entire run.

Lazy-lease semantics are part of the contract (D036). When the lease times out it should auto-recover or auto-publish (if the artifact is present and valid), not strand the run pending manual inspection that has no operator path.

## Repro

1. Run any dogfood with a parallel design phase (3 designers).
2. One supervisor dies after writing `DESIGN.md` but before publish-artifact callback (real-world cause: process crash, OOM kill, lease-heartbeat timeout). The wrapper logs `lease.expired` and the runner correctly marks the job `stale_lease`.
3. The on-disk artifact is valid (correct path, correct kind, correct byline structure once operator rewrites byline to operator-self-declared form per RFC 0046 ergonomics).
4. Operator tries every recovery verb:
   - `striatum recovery auto-publish` → `skipped: no required expected_artifact found on disk with matching byline` (root cause: `expected_author_line` returns `author: operator` because session is unattested; gemini's written byline is the original lane byline).
   - `striatum publish-artifact --allow-no-process-execution --override-rationale "..."` → exit code 5: \`lease is not active\`. The override flag does not bypass the lease-active precondition.
   - `striatum recovery requeue-stale --job-id <J>` → exit code 4: \`repo-write stale jobs require manual inspection\`. No \`--force\` flag.
   - `striatum recovery resume --complete` → requires `--blocker-id`; \`stale_lease\` is not a blocker.
   - `striatum recovery auto` → `still_stuck: repo_write_requires_operator_inspection`.

There is no CLI verb that means "I have inspected, here is the operator decision." The run is stuck.

## Why this is intolerable

dogfood loops are operator-driven and on the critical path for shipping RFCs. A 4-hour wall with no recovery path means an entire RFC iteration is wasted whenever any supervisor dies. With three lanes per design phase, plus the same risk on synth/review/implement/build-review, the probability of hitting this per run is non-trivial.

The lazy-lease design promise (D036) was: when a lease expires, the runner reclaims the work. For \`repo_write\` jobs the runner now refuses to reclaim AND provides no operator escape. That's a regression of the lazy-lease contract.

## Proposed fix (one of, ordered by preference)

1. **`striatum recovery requeue-stale --force --justification "<reason>"`** — operator escape hatch for repo_write stale jobs that have a valid on-disk artifact + want a fresh lane attempt. Audit-chained.
2. **`striatum recovery operator-publish --job-id <J> --override-rationale "<reason>"`** — single-verb operator-on-behalf publish that:
   - Doesn't require an active lease.
   - Rewrites byline to \`author: operator [self-declared: <slug>]\` in the on-disk file (idempotent).
   - Publishes the artifact with the operator byline + override audit-trail.
   - Marks the originating job \`completed\`.
3. **`recovery auto-publish` byline-rewrite mode**: when the only mismatch is byline (file content valid, path correct, kind correct), allow \`--rewrite-byline-to-operator\` so the file is rewritten to the operator self-declared form and published.
4. **Auto-recovery on lease expiry**: if the expected_artifact is present + valid on disk at lease-expiry time, the runner auto-rewrites byline to operator + auto-publishes + auto-completes. This closes the gap without operator intervention. Distinct event types (\`artifact.auto_finalized_after_lease_expiry\`) preserve audit distinguishability. (Composes with RFC 0051's auto-finalize-from-frontmatter scope.)

## Provenance

- Full repro + diagnostic walk-through: \`docs/dogfood/FRICTION_LOG.md\` dogfood-060 F1.
- ROADMAP §3.1 ("operator-on-behalf publish path") documents the happy path but assumes the lease is active. Add a §3.1.5 or rewrite §3.1 to cover lease-expired case once a recovery verb exists.
- Related: RFC 0048 V1.5 follow-up set (dogfood-058 OPERATOR_REPORT.md).
- Related: RFC 0046 V1 lane attestation guard at publish (the strict guard is what makes the on-disk byline mismatch unrecoverable once the supervisor dies).

## Severity

HIGH — blocks dogfood iteration whenever a supervisor dies mid-job. Recurring pattern (observed dogfood-054b, 055b, 057, 058, 060). Lazy-lease promise is regressed.

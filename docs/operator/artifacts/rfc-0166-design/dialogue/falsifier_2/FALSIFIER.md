# FALSIFIER - RFC 0166 Part 3/4 novelty reset is not wired to every reset surface

author: falsifier-reviewer-004

## Challenge

Part 3 states the right invariant, but the holder does not make that invariant the single progress primitive used by the detector and action ladder. The SPEC still defines the sealed-silence floor with raw `jobSealedProgressAt`, which is `max(created_at)` over all artifact and verdict rows for the job. The novelty cursor is then applied only to the telomere counter reset.

That leaves the deadline event-gated where it most needs to be novelty-gated. A hostile lane can publish cheap, distinct artifact rows inside its allowed path. Those rows can be ignored by the holder's declared-artifact novelty cursor, yet they still refresh raw `jobSealedProgressAt`; therefore the sealed-silence breach may never occur. If the ladder reaches the exhausted-budget confidence gate, the same raw helper can reset `consecutive_silent_sweeps`. In both cases, the telomere proof fails because the mechanism still has reset paths that consume events rather than genuine novelty.

## Claim Challenged

Holder Claim 1.1 makes the detector floor:

```text
floor = GREATEST(jobSealedProgressAt(job), currentActiveLeaseAcquiredAt, jobs.started_at)
```

Holder Claims 3.1 through 3.3 then define the anti-gaming cursor as a strict increase in declared/milestone progress:

```text
(count(distinct content_sha256 of milestone-bearing artifacts),
 count(sealed verdicts),
 highest satisfied required expected_artifacts milestone index)
```

Holder Claims 4.2 and 4.3 depend on that cursor for convergence: requeue budget resets only on genuine sealed progress, a non-progressing lane monotonically burns budget, and `needs_operator` fires once at the floor.

The missing build-bearing rule is: every clock/reset surface must read the novelty cursor, not raw sealed rows. The SPEC does not say that, and the named source path currently does the opposite.

## Counterexample

Construct a repo-write job with one required artifact and an allowed directory write scope. The hostile lane stays alive, refuses to publish the real required artifact, and just before every `SealedSilenceSeconds` expiry calls `artifact.publish` for a new in-scope junk artifact:

- `kind`: any allowed Markdown artifact kind that passes validation, for example `handoff`;
- `logical_name`: `junk_001`, `junk_002`, and so on;
- `path`: a fresh path under `write_scope.allowed_paths`, or for an exact-file scope, the same allowed path with a fresh logical name and different bytes;
- body: valid Markdown with the required byline plus a counter.

The holder's Claim 3.3 correctly says undeclared in-scope blobs must not advance the novelty cursor. But Claim 1.1's floor still advances because `jobSealedProgressAt` reads all artifact rows for the job. Each cheap junk publish makes `sealedSilenceBreached == false` on the next sweep. No `transfer_requeue` happens, no `requeue_count` increments, and the telomere never shortens.

There is a second reset path if the job ever reaches budget exhaustion. The RFC 0131 confidence gate computes `progressAdvanced` from raw `jobSealedProgressAt(...).After(windowStart)` or sibling liveness. A single junk artifact row after `windowStart` resets `misfire_evidence_score` and `consecutive_silent_sweeps` to zero. That violates the Part 4 claim that `maxSilentSweeps` is a bounded escape valve whose reset only follows genuine sealed progress.

This survives daemon restart. The junk artifact rows and their `created_at` values are durable; recomputing the raw helper after restart preserves the attacker-controlled floor. The holder's proposed count columns also do not identify the timestamp of the last strict cursor advance, so they are not enough to reconstruct a novelty-aware floor after restart.

## Evidence

`go/pkg/mutations/recovery_decision_tree.go` defines `jobSealedProgressAt` as:

```sql
SELECT GREATEST(
  (SELECT max(created_at) FROM striatumd.artifacts WHERE repository_id = $1 AND job_id = $2),
  (SELECT max(created_at) FROM striatumd.verdicts WHERE repository_id = $1 AND job_id = $2)
)
```

There is no filter for expected artifacts, required milestones, distinct `content_sha256`, or cursor advancement.

`go/pkg/mutations/artifact.go` does not require `logical_name` and `path` to match `expected_artifacts_json` before inserting an artifact row. `resolvePublishPlacement` falls back to the default placement when no expected artifact matches, and the same-path duplicate guard only folds identical content; the code comment explicitly says different content at the same repo path falls through to a fresh insert.

`recoverStuckJobs` also uses the same raw helper for the confidence gate: `progressAdvanced := (hasSealed && sealedAt.After(windowStart)) || cohortHasFresherLiveness(...)`. That means a raw artifact row can reset `consecutive_silent_sweeps` even when the Part 3 cursor correctly refuses to advance.

The current `job_recovery_state` migrations contain the recovery counters and the RFC 0131 gate columns. The holder proposes adding cursor counts there, but no `last_novel_sealed_progress_at` or equivalent deterministic timestamp is specified. Counts alone can say that the cursor did not advance; they cannot provide the detector floor unless every sweep also records when the last accepted cursor advance occurred.

## Strongest Rebuttal

The SPEC has important anti-gaming pieces: identical same-path/same-content replay is idempotent; a same-attempt logical-name rewrite with different content is refused; verdict rows are daemon-written and for reviewer jobs generally coincide with job completion; scoping the artifact hash count to declared/milestone artifacts is the right answer for undeclared junk.

Those facts only protect the proposed cursor. They do not protect the surfaces that still consume raw `max(created_at)`. Auditable junk is better than invisible PTY noise, but it is still cheap enough to keep the clock fresh forever if the clock treats every artifact row as sealed progress.

The holder may answer that P0 already accepts the tool-axis polling residual. This is a different class of failure. Tool polling keeps the tool half of the AND fresh; this attack changes daemon sealed-work state and targets the exact Part 3 promise that fake sealed events cannot reset the deadline or ladder.

## Unanswered Gap / Required Revision

The gate should not clear Parts 3 and 4 until the SPEC defines one novelty-aware sealed-progress primitive and uses it everywhere:

1. The detector floor must be based on the last strict novelty-cursor advance, not raw `jobSealedProgressAt`. That likely requires `last_novel_sealed_progress_at` in `job_recovery_state`, or an equivalent deterministic SQL recomputation over declared expected artifacts and verdicts.
2. The telomere reset and RFC 0131 confidence-gate `progressAdvanced` reset must call the same novelty predicate. Raw artifact/verdict `created_at` must not reset `requeue_count`, `consecutive_silent_sweeps`, `misfire_evidence_score`, or the sealed-silence floor.
3. The publish contract must say whether undeclared artifact rows are still allowed but ignored for sealed-silence progress, or whether deadline-participating jobs become expected-artifact-only for `artifact.publish`. Either way, tests must prove undeclared rows cannot move the deadline floor.
4. Restart tests must cover the attack: publish distinct junk artifacts before each budget expiry, restart the daemon mid-ladder, and assert the novelty cursor, floor, silent sweeps, and requeue budget continue toward one bounded `needs_operator` escalation.

As written, the action ladder is not convergent against a lane that can mint fresh artifact rows without satisfying declared progress. That stops the falsification gate from clearing.

# RFC 0116: Zero-operator-touch sequential DAG execution — `run drive` + fresh-reviewer policy fix

Status: proposed
Date: 2026-06-06
author: rfc-author-claude-opus-4.8-001
Context: GH #178 (no daemon-side session auto-spawn — the operator must
register + supervise a session per (role, lane) as the DAG unblocks), GH #188
**policy half** (the fresh-reviewer freshness check refuses while the author
session is idle-parked; #188's *text* half — the misleading
`--force-non-fresh`-only suggestion — is fixed independently in a parallel
batch and is referenced, not owned, here). RFC 0103 W3/W7 (liveness floor,
bounded operator), RFC 0105 (unattended reliability harness — the standing
gate this RFC's drive loop must pass), RFC 0108 (parallel independent runs —
multi-run isolation the driver must respect), RFC 0109 (seat tiers), RFC 0110
(daemon→PG auth + run-as), the spec's product boundary
(`docs/reference/spec.md`). Source surfaces:
`go/pkg/mutations/lifecycle.go:36-180` (`HandleRegisterSession`,
freshness check at `:101-118`, `slotHasUnclaimedParallelWork` at `:16-34`),
`go/pkg/mutations/mutations.go:1317-1332` (`workflowDeclaresFreshReviewer`),
`go/pkg/mutations/supervision_control.go:147` (`HandleSuperviseStart`),
`:2900` (`resolveSupervisorHelper`), `scripts/dod/driver.py` (the proven
unattended driver this RFC promotes to a first-class verb),
`skills/optional/refactoring-campaign/scripts/wait-run.sh`,
`go/pkg/cli/routes/routes_generated.go` (`run`/`supervise`/`session` verbs).

## Problem

Starting a run gets no work executed by itself. Striatum sessions are
keyed by `(run, role, lane)`; most panel jobs set
`fresh_session_required: true`; parallel same-`(role, lane)` jobs each need
their own session. So as the DAG unblocks, the operator must repeatedly run

```
striatum register-session --run-id <run> --role <role> --lane <lane> [--fresh]
striatum supervise start    --session-id <sid>
```

— and for a fresh-reviewer transition, first `striatum supervise stop` the
idle author. The #178 smoke test against kayak-gen (stage-0 goal-selection
run `run_08d0beb1f0959b071475ff4400dc1d97`, 2.26.0) needed ~10
register+start cycles spread over one 11-job panel's lifetime. #188 observed
the `stop → register → start` dance at all 8 role transitions across four
`code_change` runs on 2026-06-06.

Two distinct defects compound the friction:

1. **No supervised-DAG driver.** Every operator who drives a supervised run
   ends up rewriting the same polling loop: poll `run summary`, diff queued
   jobs against active sessions, register + supervise the gap. The
   `refactoring-campaign` skill ships `wait-run.sh` (a *passive* waiter that
   only watches for terminal state); it does not drive, so the operator is
   the scheduler. The autonomy DoD already proved a *working* driver exists —
   `scripts/dod/driver.py` drove 10/10 consecutive clean unattended runs
   (the RFC 0105 acceptance) — but it is a private script under `scripts/`,
   not a product verb, and it hard-codes one lane.

2. **The fresh-reviewer policy over-refuses.** The freshness check
   (`lifecycle.go:101-118`) refuses a fresh-reviewer registration whenever
   *any* author session is `state = 'active'` on the run — **including** an
   author that has completed its only job and is idle-parked in
   `await_packet` with no lease and no remaining claimable work:

   ```sql
   SELECT 1 FROM striatumd.sessions
    WHERE repository_id = $1 AND run_id = $2
      AND role_id = 'author' AND state = 'active'
    LIMIT 1
   ```

   The freshness concern is **context contamination** — a fresh reviewer
   must not inherit the author's working context. A parked, lease-less,
   jobs-done author holds no work the reviewer could be contaminated by; it
   is exactly the session the operator (or the driver) is about to close
   anyway, since downstream jobs are `fresh_session_required`. The check
   conflates "an author session row is `active`" with "the author is doing
   contaminable work."

These are the two halves of one friction (#178 is the spawn side; #188 is
the refusal side). This RFC owns both: the policy predicate fix (small,
shippable now) and the driver verb (the bigger ergonomics change), and it
argues the boundary question that #178's "either/or" ask raises.

This RFC is design-only. The dogfood run that produces it changes no `go/`
source. Implementation is later, phased PRs.

## The product-boundary question, stated honestly

#178 offers two remedies: a **daemon-side auto-spawn** flag and an
operator-side **`striatum run drive`** verb. The instinct is that auto-spawn
is the bigger change because "the daemon spawning processes" sounds like a
product-boundary violation. The codebase says otherwise, and the honest
framing matters for the decision:

**The daemon already spawns lane processes.** `supervise.start` is a daemon
RPC (`go/pkg/cli/routes/routes_generated.go:69`, handler
`HandleSuperviseStart` at `supervision_control.go:147`); the handler
resolves `striatum-supervisor-helper` (`resolveSupervisorHelper`,
`:2900`) and `exec`s it (`cmd.Start()` at `:1588`/`:1650`/`:1753`), which in
turn launches the lane's PTY/tmux process. Today this happens *inside the
daemon process*, on the daemon's host, every time an operator calls
`supervise start`. So daemon-initiated `fork/exec` of a lane is **already**
how every supervised lane is born.

The real distinction between the two remedies is therefore **not** "who runs
`exec`," but **what triggers it**:

- **Operator-triggered spawn (today, and `run drive`):** a lane is spawned
  only in response to a synchronous, capability-authenticated RPC from an
  operator-side process. `run drive` is a thin operator-side loop that emits
  those same RPCs — it changes the *cadence* (a loop instead of a human),
  not the *boundary*. The operator (or its agent) remains the principal that
  authorizes every spawn; the loop is trivially killable; nothing in the
  daemon changes.

- **Scheduler-triggered spawn (`supervision.auto_spawn`):** the daemon's own
  job scheduler decides, with no per-spawn operator RPC, to register a
  session and spawn a lane when a job becomes ready. This **is** a genuine
  boundary change — not because the daemon `exec`s (it already does) but
  because the daemon becomes an *autonomous actor* that initiates work
  without a contemporaneous human/agent authorization. That has real
  consequences for attestation (RFC 0110), run-as/sandbox (RFC 0110 L2),
  crash/restart semantics, and the spec's "operator is a bounded processor"
  posture (RFC 0103 W7). It needs an explicit product decision and is not
  justified by today's evidence.

This RFC recommends **`run drive` first** precisely because it delivers the
zero-touch ergonomics #178 asks for **without** crossing the
scheduler-triggered line, and produces the operational evidence that would
(or would not) justify ever crossing it.

## Goals

1. Make a sequential `draft → review → apply` DAG (and the common panel
   fan-out) flow to a terminal state with **zero operator touches per
   transition**, under the existing daemon boundary.
2. Fix the fresh-reviewer policy so an idle, lease-less, no-claimable-work
   author does not block a genuinely-fresh reviewer registration — without
   weakening the contamination guarantee for an author that *is* working.
3. Make the driver a first-class, killable, idempotent, observable operator
   verb (`striatum run drive`) that subsumes `scripts/dod/driver.py` and
   composes with the `refactoring-campaign` skill's `wait-run.sh`.
4. Keep every spawn capability-authenticated and operator-principal-owned;
   change cadence, not the boundary.
5. Pass the RFC 0105 unattended-reliability gate as the acceptance for the
   driver, and respect RFC 0108 multi-run isolation.

## Non-Goals

- **No daemon-side `supervision.auto_spawn` in this RFC.** It is analyzed
  (§"Verb 2") and explicitly deferred behind a named evidence trigger; this
  RFC does not implement, accept, or schedule it.
- **No co-driving one run by multiple drivers.** Two `run drive` processes on
  one run is a defended failure mode (§Failure modes), not a feature.
- **No rescue verbs in the driver.** `run drive` uses only normal lifecycle
  verbs (register-session, supervise start/stop, session close). It never
  calls `requeue-stale`, `override-verdict`, `retry-job`, or any `--force`
  path — mirroring the DoD driver's invariant. A run that cannot reach
  terminal state with lifecycle verbs alone escalates loud (the daemon's
  `needs_operator`/`failed`), which is correct and must stay visible.
- **No weakening of `fresh_session_required` / `reviewer_context_policy:
  fresh`.** The policy fix narrows *which* author sessions block, never the
  contamination guarantee for a working author.
- **No new persistence, hosted service, telemetry, or durable transcript
  capture.** The driver is a stateless operator-side loop reading daemon RPC
  state (D094/D005/D028 intact). Its only durable output is the existing
  per-run audit trail the daemon already records for the verbs it calls.
- **No change to multi-run integration/merge** (RFC 0108 owns that). `run
  drive` drives one run; N drivers for N runs is supported by isolation, not
  by coordination.

## Verb 1 (RECOMMENDED, Phase 1): `striatum run drive`

A long-lived, foreground, operator-side CLI loop. It watches one run's DAG
via daemon reads and, as jobs unblock, performs `register-session` +
`supervise start` itself; as jobs complete, it closes their lanes so a
fresh-policy reviewer can register. It exits when the run reaches a terminal
state.

This is the productization of `scripts/dod/driver.py`, which already proved
the loop works unattended 10/10 (RFC 0105). The verb generalizes it past the
one-lane hard-code and gives it first-class observability and resume
semantics.

### Behavior (the drive loop)

Each tick (default `--interval 15s`, jittered to avoid lockstep with other
drivers):

1. `run.summary` (read) → run state + jobs. Terminal state (`completed` →
   exit 0; `failed`/`canceled`/`needs_operator` → exit non-zero with the
   escalation reason) ends the loop.
2. **Close completed lanes.** For each job that is `completed` and whose lane
   this driver launched, `supervise stop --reason "lane completed its job;
   closing for fresh-policy"`. This is the manual `supervise stop` the
   operator does today before a fresh reviewer; the policy fix (§Verb-3)
   removes the *requirement*, but proactively closing a done lane is still
   correct hygiene (frees the host, ends the idle agent process).
3. **Launch lanes for claimable jobs.** For each `queued`/claimable job
   with no driver-launched session, resolve `(role, lane)` from the job +
   workflow snapshot, `register-session` (handling the fresh policy — see
   below), then `supervise start`. Record `(workflow_job_id, attempt) →
   session_id` so the driver is idempotent across ticks and restarts.

The lane → `(role, lane)` mapping is the same one the operator computes
today by hand; the driver reads it from the run summary's per-job
`role_id`/`lane_id` (already returned). When a workflow declares multiple
lanes for a role, the driver uses the job's declared lane; an ambiguous job
with no declared lane is surfaced (not guessed) — see Observability.

### register-session under the fresh policy

The driver mirrors `driver.py:register_lane`: try `register-session`; on a
`fresh`-policy refusal, **close completed-job author sessions** (never
`--force-non-fresh`) and retry once. With the Verb-3 policy fix, an idle
lease-less done author no longer refuses at all, so this fallback fires only
for a genuinely-still-working author — exactly when the reviewer *should*
wait. The driver does not loop indefinitely on a fresh refusal: after one
close-and-retry it reports the blocked transition and keeps ticking (the
author may still be finishing), surfacing the wait rather than busy-spinning.

### Idempotent re-drive (resume semantics)

`run drive` holds **no durable state of its own** — its launch map is
reconstructible from daemon reads. On (re)start it reconciles:

- `list sessions --run-id <run>` → existing active sessions on the run.
- For each claimable job, if an active session already covers its
  `(role, lane, attempt)` slot, adopt it into the launch map instead of
  registering a duplicate (the `slotHasUnclaimedParallelWork` predicate at
  `lifecycle.go:16-34` already refuses an accidental duplicate, so a
  double-register is *safe* — it errors cleanly — but adoption avoids the
  noise).

Therefore re-running `run drive` on a partially-driven run is safe and
convergent: it picks up where it left off, never double-spawns a covered
slot, and never orphans a lane (lanes it did not launch it leaves alone; it
only closes ones it adopted or launched). This is the property that lets the
operator `Ctrl-C` and re-drive freely, and lets `wait-run.sh`-style wrappers
restart the driver after a transient.

### Composition with `wait-run.sh` and the refactoring-campaign skill

`wait-run.sh` is a *passive* terminal-state waiter; `run drive` is the
*active* scheduler. They compose cleanly because `run drive` itself blocks
until terminal:

- The refactoring-campaign skill's per-stage recipe — today "register and
  supervise one session per lane … as the DAG unblocks" (the understated
  REFERENCE.md line #178 flags) — becomes a single
  `striatum run drive --run-id <run>` per stage. The skill's
  `instantiate.sh` → `run prepare` → `run start` → **`run drive`** chain
  replaces the manual loop; `wait-run.sh` is then redundant for driven runs
  (drive blocks to terminal) but stays useful for *externally*-driven or
  human-in-the-loop runs.
- `run drive --once` (analogous to `dashboard --once`) performs exactly one
  reconcile tick and exits, for scripts/CI that want to advance a run by one
  step and assert, or to wedge a cron-style external driver.

### Interaction with RFC 0108 (parallel independent runs)

`run drive` is **per-run** (`--run-id` required; no `--all`). N operators
driving N runs on one repo is exactly RFC 0108's model: each driver touches
only its own run's sessions/lanes, and the per-run advisory lock (RFC 0104)
plus worktree/branch isolation (RFC 0108) keep them from colliding. The
driver must **not** introduce a repo-wide "drive everything" mode — that
would re-create the coordination problem RFC 0108 explicitly declines. The
jittered interval (above) keeps multiple drivers' `run.summary` reads from
synchronizing into a thundering herd against the daemon.

### Why operator-side, not daemon-side, is the right *first* step

- **Boundary-preserving:** every spawn stays a capability-authenticated RPC
  from an operator principal; the daemon stays reactive (§"boundary
  question"). No spec change, no decision-log boundary entry needed beyond
  the new verb.
- **Killable and debuggable:** the loop is a foreground process the operator
  can `Ctrl-C`, `strace`, or read the logs of. A daemon-internal scheduler
  is none of these without new daemon observability surface.
- **Crash-isolated:** if the driver dies, the daemon and all running lanes
  survive (lanes are already daemon-supervised, independent of the launcher
  per RFC 0103 W3 / `#141`); re-drive reconciles. A daemon-internal spawner
  that crashes is a daemon crash.
- **Already proven:** `driver.py` is the existence proof; productizing it is
  low-risk because the behavior is the RFC 0105 gate.
- **Evidence-generating:** running `run drive` at scale produces exactly the
  data (how often it ticks, how it handles fresh refusals, what fraction of
  runs need any rescue) that would tell us whether the marginal daemon-side
  auto-spawn is worth its boundary cost.

## Verb 2 (ANALYZED, DEFERRED): daemon-side `supervision.auto_spawn`

An opt-in lane/workflow flag (`supervision.auto_spawn: true`) telling the
daemon scheduler to register a session + spawn a supervisor when a job for
that lane becomes ready and the lane has a registered command. The daemon
already owns supervisor lifecycle and job readiness, so it has the
*mechanism* (§"boundary question"). This RFC does **not** adopt it. Honest
analysis of why, and what would change the verdict:

**Attestation (RFC 0110 L3).** Today every spawn is attributed to the
operator principal that called `supervise.start`; the `BeforeAcquire`
`SET LOCAL principal_id` attribution (RFC 0110 L3) records *whose*
capability authorized the lane. A scheduler-initiated spawn has **no
contemporaneous principal** — the daemon would have to spawn under a
synthetic "scheduler" principal or impersonate the run's owner. Either is a
new trust primitive (a non-human principal that can author work), which is
RFC 0107 territory and unbuilt. `run drive` sidesteps this entirely: the
spawn carries the driving operator's principal.

**Run-as / sandbox (RFC 0110 L2).** The hardened default is a dedicated
PG-less lane OS user with a `0700` socket dir; `run drive` inherits the
operator's environment and run-as configuration the same way `supervise
start` does today. A daemon scheduler spawning autonomously must resolve
run-as for a lane *no operator asked for right now* — which user, which
environment, which capability token gets minted and bound — without a live
request to anchor those choices. The session-token binding
(`session_token.go`, wired at `HandleSuperviseStart`) currently mints
against the registering request; auto-spawn needs that minting moved into
the scheduler with the same guarantees, untested.

**Crash/restart semantics.** `run drive` crash = re-drive reconciles
(above). Daemon `auto_spawn` crash/restart must answer: on
`systemctl restart striatumd`, does the scheduler re-spawn lanes for
already-ready jobs? It must not double-spawn (the `slotHasUnclaimedParallelWork`
guard helps but was written for the operator path), and must not spawn for a
job a human meant to hold. The existing supervisor-survival work
(`KillMode=process` + `context.WithoutCancel`, `#141`) keeps *running* lanes
alive across restart, but a *scheduler* that decides to spawn is new restart
surface.

**Boundary posture (RFC 0103 W7).** "The operator is a bounded, well-served
processor" — a daemon that spawns work with no operator in the loop inverts
that. It is defensible for a single-human single-repo yolo deployment, but it
is a product decision, not an ergonomics tweak, and it should be made with
data, not in advance.

**Evidence that would justify revisiting.** Adopt `supervision.auto_spawn`
only if, after `run drive` is in real use, we observe **all** of:

1. operators routinely run `run drive` as a daemonized background process
   anyway (so the "operator in the loop" property is already nominal, not
   real), **and**
2. the driver's poll cadence is a measured latency/throughput bottleneck a
   push-based scheduler would materially fix (event-driven spawn vs 15s
   poll), **and**
3. a principal model for non-human scheduler-initiated work exists
   (RFC 0107 successor) so attestation is not weakened.

Absent all three, `run drive` is strictly preferable. If adopted later,
`auto_spawn` should reuse the driver's exact reconcile predicate (so the two
paths are one tested algorithm in two homes) and ship behind the RFC 0105
gate extended to the auto-spawn path.

## Verb 3 (Phase 1, shippable now): the fresh-reviewer policy fix

### The exact predicate

Replace the freshness gate at `lifecycle.go:101-118`. Today:

```sql
SELECT 1 FROM striatumd.sessions
 WHERE repository_id = $1 AND run_id = $2
   AND role_id = 'author' AND state = 'active'
 LIMIT 1
```

— refuses on **any** active author session. The fix: an author session
blocks a fresh reviewer **only if it is contaminable**, defined as *(has an
active lease) OR (its role still has a claimable job remaining on the run)*.
Equivalently, skip authors that are *idle (no active lease) AND drained (no
remaining claimable jobs for the role)*. New predicate (the refusal fires
iff this returns a row):

```sql
SELECT 1 FROM striatumd.sessions s
 WHERE s.repository_id = $1 AND s.run_id = $2
   AND s.role_id = 'author' AND s.state = 'active'
   AND (
        -- (a) the author currently holds work: a live lease / un-acked packet
        EXISTS (
          SELECT 1 FROM striatumd.queue_messages q
           WHERE q.repository_id = s.repository_id
             AND q.run_id = s.run_id
             AND q.kind = 'work'
             AND q.claimed_by_session_id = s.session_id
             AND q.state IN ('claimed', 'acked')
        )
        -- (b) the author's role still has claimable work it could pick up
        OR EXISTS (
          SELECT 1 FROM striatumd.queue_messages q2
           WHERE q2.repository_id = s.repository_id
             AND q2.run_id = s.run_id
             AND q2.kind = 'work'
             AND q2.target_role_id = s.role_id
             AND q2.state = 'pending'
        )
       )
 LIMIT 1
```

Notes on the predicate:

- It reuses the exact `queue_messages` shape `slotHasUnclaimedParallelWork`
  already queries (`lifecycle.go:23-29`): `kind = 'work'`, states
  `pending`/`claimed`/`acked`. Split here into (a) *this session holds it*
  (`claimed`/`acked` by `session_id`) and (b) *the role could claim it*
  (`pending` for the role). That is the operational definition of "doing or
  about-to-do contaminable work."
- `role_id = 'author'` stays literal to preserve today's exact scope; an
  Open Question asks whether to generalize to "any non-reviewer author-side
  role." (Today's check is author-only; this RFC does not widen it without a
  decision.)
- The `FOR UPDATE` row lock already taken on the `(role, lane)` slot below
  (`lifecycle.go:119-125`) is unaffected; this is a pre-check `SELECT 1`,
  same as today.
- When the predicate returns **no** row (the author is idle+drained), the
  fresh reviewer registers cleanly with **zero** operator touches — closing
  #188's policy half: the parked, completed, soon-to-be-superseded author no
  longer blocks.
- `--force-non-fresh` semantics are **unchanged**: it still overrides a
  *genuine* refusal (a still-working author), and still requires `--reason`.
  The fix only removes the *spurious* refusals, so `--force-non-fresh` stops
  being the answer to "my author is just idle" (it never should have been).

### How this composes with #188's text fix and `run drive`

- **#188 text half (parallel batch):** that change rewrites the refusal
  *message* to suggest `supervise stop` the idle author rather than only
  `--force-non-fresh`. With Verb-3 landed, the *idle* author no longer
  triggers the message at all, so the two fixes are complementary: text-fix
  improves the message for the *remaining* (genuine) refusals; policy-fix
  removes the spurious ones. This RFC must land *after or with* the text fix
  and not revert it; the message for a genuine still-working author should
  read: "an author session is still holding/eligible for work; close it
  (`supervise stop`) and register a fresh reviewer, or `--force-non-fresh
  --reason` to accept contamination."
- **`run drive`:** with the policy fix, the driver's
  `register_lane`-style close-and-retry fallback fires *only* for a genuine
  still-working author — i.e. when waiting is correct — so the driver's fresh
  transitions become single-call in the common drained case. The driver still
  proactively `supervise stop`s a done lane (hygiene), but no longer *must*
  to satisfy the policy.

### Tests for Verb 3 (named regression gates)

PG-gated (skip without `STRIATUM_PG_TEST_URL`), in
`go/pkg/mutations/lifecycle_pg_test.go` (or the existing register-session
test file), following the `read_authority_*_pg_test.go` conventions:

- `TestFreshReviewerIgnoresDrainedIdleAuthor` — workflow declares
  `reviewer_context_policy: fresh`; author session `active`, its only job
  `completed`, no lease, no `pending` author work → fresh reviewer
  `register-session` **succeeds** with no `--force-non-fresh`. (The #188
  repro, now green.)
- `TestFreshReviewerStillRefusesWorkingAuthor` — author holds a
  `claimed`/`acked` work packet → fresh reviewer registration **refuses**
  with the genuine message; `--force-non-fresh --reason` overrides.
- `TestFreshReviewerStillRefusesAuthorWithPendingWork` — author idle (no
  lease) but a `pending` author-role job remains on the run → **refuses**
  (the role could still pick up contaminable work). Pins predicate branch
  (b).
- `TestForceNonFreshUnchangedSemantics` — `--force-non-fresh` without
  `--reason` still refused; with `--reason` still records the reason. Guards
  against the fix accidentally loosening the override path.
- `TestFreshReviewerPolicyParityNonFreshWorkflow` — a workflow that does
  **not** declare fresh-reviewer policy is unaffected (registration succeeds
  regardless of author state), pinning `workflowDeclaresFreshReviewer`
  (`mutations.go:1317`) as the only gate on the new SQL running at all.

## Tests for Verb 1 (`run drive`) — named regression gates

The driver's behavioral acceptance **is** RFC 0105: a `run drive`-driven
multi-lane, review-gated, revision-capable run must reach `completed` or
escalate loud, unattended, within budget, never silently wedge.

- `TestRunDriveReconcileIsIdempotent` (unit, against a fake daemon-read
  surface): given a run summary + session list, two consecutive reconcile
  ticks register each claimable slot exactly once and adopt pre-existing
  sessions without duplicate registration.
- `TestRunDriveAdoptsExistingSessions` — a session already covers a slot →
  the driver adopts it, issues no `register-session`.
- `TestRunDriveClosesCompletedLanesBeforeFreshReviewer` — a completed author
  lane is `supervise stop`-ped before the reviewer slot is registered.
- `TestRunDriveNeverCallsRescueVerbs` (guard): the driver's verb set is
  asserted to be a subset of {`run.summary`, `list.sessions`,
  `session.register`/`register-session`, `supervise.start`,
  `supervise.stop`, `session.close`} — no `requeue-stale`/`override`/
  `retry-job`/`--force`. Mirrors `driver.py`'s stated invariant, made
  executable so the verb can't silently grow a rescue.
- **RFC 0105 fixture extension** (`go/pkg/adapterconformance/` multirun +
  the RFC 0105 harness): a fixture run driven *by `run drive`* (not the
  in-test bespoke loop) completes unattended across a `needs_revision` cycle
  under the RFC 0103 fault matrix (lane death, transport churn, reviewer
  replacement). This makes `run drive` itself the gated artifact, not just a
  convenience wrapper around a separately-tested loop.
- `TestRunDriveTwoConcurrentDriversOneRun` (the defended failure mode): two
  drivers on one run do not double-spawn a covered slot (the
  `slotHasUnclaimedParallelWork` refusal + adoption keep them safe) and one
  cleanly observes the other's sessions. (See Failure modes for the
  recommended *advisory* single-driver guard.)

## Failure-mode table

| Failure | What happens | Driver/daemon response | Operator-visible signal |
|---|---|---|---|
| **Lane dies mid-drive** | A driver-launched lane process exits/crashes before completing its job. | Daemon's existing liveness/recovery (RFC 0103 W3, `#147` PID-liveness probe) requeues the stale lease; on the next tick the driver sees the job back in a claimable state with no live session and re-launches. The driver never *rescues* — it re-launches via normal `register-session`+`supervise start`. | Driver log line: `relaunched <role>/<lane> for <job> (prior session <sid> went non-live)`; run summary shows the attempt increment. |
| **Daemon restart mid-drive** | `systemctl restart striatumd`. | Running lanes survive (`KillMode=process` + `context.WithoutCancel`, `#141`). The driver's next `run.summary` call may transiently fail; it backs off (jittered interval) and retries, then *reconciles*: adopts surviving sessions, re-launches only genuinely-uncovered slots. No durable driver state is lost (none exists). | Driver log: `daemon read failed, backing off` then `reconciled N sessions after daemon restart`. |
| **Two concurrent drivers on one run** | Operator (or a wrapper) starts `run drive` twice on the same `--run-id`. | Both reconcile against the same daemon state; the `slotHasUnclaimedParallelWork` guard (`lifecycle.go:16-34`) refuses the *second* register on a covered slot, so neither double-spawns. **Recommended mitigation:** a best-effort advisory single-driver marker — `run drive` writes a `(run_id, pid, host)` claim into `.striatum/scratch/` and warns (does not hard-block) if another live driver claim exists, so the *operator* sees the duplicate. Not a lock (the daemon guard is the real safety); just a legibility aid. | Second driver prints `warning: another run drive appears active for this run (pid …); they will not collide but you likely meant one`. |
| **Packet-refusal loop** | A lane keeps refusing its packet (e.g. productive refusal, missing input, write-scope drift) so the job cycles without completing. | The driver does **not** retry-storm: it launches a lane for a claimable job at most once per `(job, attempt)`; a job that re-queues to a *new attempt* gets one fresh launch. If the daemon escalates the run to `needs_operator`/`failed` (revision budget exhausted, `#84`-class wedge), the driver exits non-zero with the escalation reason — it does not paper over it. | Exit code + `run <id> -> FAIL (state=needs_operator: …)`; the operator inspects via `run summary`/`dashboard`. |
| **Fresh-reviewer refusal on a still-working author** | The author genuinely still holds/eligible-for work when the reviewer slot unblocks. | Verb-3 predicate correctly refuses; the driver does one close-and-retry of *completed* author sessions (no-op here, since the author isn't done), then reports the blocked transition and keeps ticking — it waits for the author to drain rather than `--force-non-fresh`. | Driver log: `reviewer slot blocked: author still working; waiting` (re-emitted at most once per state change, not every tick). |
| **Ambiguous lane for a job** | A workflow declares multiple lanes for a role and a job carries no resolved lane. | The driver does **not** guess; it surfaces the ambiguity and skips launching that slot (the rest of the run still drives). | Driver log: `cannot resolve lane for <job> (role <role> has lanes …); register manually`. |

## Observability — how the operator sees what `run drive` did

The driver is a foreground process; its primary observability is its own
structured log stream (one line per state-changing action, jittered ticks
silent unless something changes). Beyond that, **everything `run drive` does
is already in the daemon's records**, because it only calls existing audited
verbs — there is no hidden side-channel:

- `run summary` / `striatum dashboard --run-id <id>` show the sessions and
  jobs the driver created/closed, identically to operator-driven runs.
- `list sessions --run-id <id>` shows every session the driver registered
  (with the same audit attribution to the driving operator's principal,
  RFC 0110 L3).
- The per-run audit log records each `session.register` / `supervise.start` /
  `supervise.stop` / `session.close` the driver issued, attributed to the
  operator principal — the driver adds no new audit class.
- `run drive --json` emits a machine-readable action stream
  (`{ts, action, job, role, lane, session_id, result}`) for CI/wrappers.
- `run drive --once` returns the single reconcile decision set
  (what it *would* and *did* launch/close this tick) and exits — the
  inspectable dry-ish-run analog of `dashboard --once`.

Explicit non-feature: `run drive` does **not** persist a separate "drive
journal." Its actions are reconstructible from the daemon audit trail; a
second durable log would duplicate state the product boundary keeps in PG.

## Phased implementation plan with named gates

**Phase 1 — policy fix + drive MVP (this RFC's shippable core).**

1. Verb-3 freshness predicate (`lifecycle.go:101-118`) + the five
   `TestFreshReviewer*` / `TestForceNonFresh*` gates. Independently
   valuable, lands first, closes #188's policy half. Coordinate ordering
   with the #188 text-fix batch (land after/with it; do not revert the
   message change).
2. `run drive` MVP: the reconcile loop, `--interval`, `--once`, `--json`,
   idempotent re-drive, fresh-policy close-and-retry, the concurrent-driver
   advisory marker. Gated by `TestRunDriveReconcileIsIdempotent`,
   `TestRunDriveAdoptsExistingSessions`,
   `TestRunDriveClosesCompletedLanesBeforeFreshReviewer`,
   `TestRunDriveNeverCallsRescueVerbs`, and **the RFC 0105 fixture driven by
   `run drive`** (the behavioral gate).
3. Wire `run drive` into the CLI route table (a *local* command — it is an
   operator-side loop over existing RPCs, not a new daemon method, so it adds
   **no** new RPC and needs **no** command-authority-matrix RPC row; it does
   appear in the CLI verb reference). Update
   `skills/optional/refactoring-campaign/REFERENCE.md` to replace the manual
   per-(role,lane) loop with `run drive`, fixing the understated wording #178
   flags. Retire/redirect `scripts/dod/driver.py` to call `run drive` (keep
   the DoD `N`-consecutive-runs harness on top).

**Phase 2 — gated on Phase-1 evidence (NOT auto-scheduled).** Only if
running `run drive` in real use surfaces a concrete gap:

- richer lane→job resolution for multi-lane-per-role workflows (if the
  ambiguity surfacing proves common);
- push/event-driven ticking (daemon emits a "job ready" event the driver
  subscribes to) *if* the 15s poll is a measured bottleneck — this is the
  first step that would also feed a future auto-spawn, and is the cheaper of
  the two.

**Phase 3 — `supervision.auto_spawn` (DEFERRED, decision-gated).** Not
scheduled by this RFC. Revisit only on the three-part evidence trigger in
§"Verb 2", behind its own RFC and an extension of the RFC 0105 gate to the
scheduler-spawn path, with a non-human-principal model (RFC 0107 successor)
in hand.

## Proposed decision-log entry

> **DXXX — Zero-operator-touch sequential DAG execution via operator-side
> `run drive`; fresh-reviewer policy ignores drained idle authors
> (RFC 0116).** Accept `striatum run drive` — a foreground, idempotent,
> killable operator-side loop that watches one run's DAG via daemon reads and
> performs `register-session` + `supervise start` (and `supervise stop` of
> completed lanes) as jobs unblock, using only normal lifecycle verbs and
> reaching terminal state or escalating loud (RFC 0105). It productizes the
> proven `scripts/dod/driver.py`, composes with the refactoring-campaign
> skill, and respects RFC 0108 multi-run isolation (per-run only). It is the
> recommended remedy for #178 because it delivers zero-touch DAG execution
> **without** the daemon becoming an autonomous spawner: the daemon already
> `exec`s lanes on `supervise.start`, so the change is spawn *cadence*, not
> the product boundary — every spawn stays a capability-authenticated RPC
> from the operator principal. Daemon-side `supervision.auto_spawn` is
> analyzed and **explicitly deferred** behind a three-part evidence trigger
> (drive routinely daemonized in practice; poll cadence a measured
> bottleneck; a non-human scheduler principal model exists per an RFC 0107
> successor), because it crosses into autonomous, unattributed spawning with
> attestation/run-as/restart consequences not justified by current evidence.
> Separately accept the fresh-reviewer policy fix
> (`lifecycle.go:101-118`): an `active` author session blocks a
> `reviewer_context_policy: fresh` registration **only if** it holds a live
> lease OR its role has remaining `pending` work; an idle, lease-less,
> drained author no longer blocks (closing #178's spawn friction and #188's
> policy half). `--force-non-fresh` semantics are unchanged; it now applies
> only to genuine refusals. No new persistence, RPC method, hosted service,
> or audit class; `run drive` is a local CLI verb over existing audited
> mutations.

(Number `DXXX` assigned at acceptance; this RFC does not edit the decision
log — the entry is reproduced here for the maintainer to transcribe on
accept.)

## Companion issues to file (NOT filed by this RFC)

- **`run drive` verb (Phase-1 MVP)** — the reconcile loop + `--interval` /
  `--once` / `--json`, idempotent re-drive, fresh-policy close-and-retry, the
  concurrent-driver advisory marker, and the named `TestRunDrive*` gates +
  RFC 0105 fixture extension. Blocks on / coordinates with the policy fix.
- **Fresh-reviewer policy fix (#188 policy half)** — replace the
  `lifecycle.go:101-118` predicate with the lease-and-claimable test; the
  five `TestFreshReviewer*`/`TestForceNonFresh*` gates. Reference #188 and
  ensure landing order vs the #188 text-fix batch.
- **refactoring-campaign skill doc fix** — REFERENCE.md "one session per
  lane" → `run drive` per stage (the #178 *Related skill-doc bug*); land with
  the verb so the recipe and the tool agree.
- **Retire/redirect `scripts/dod/driver.py` onto `run drive`** — keep the
  DoD `N`-consecutive-runs streak harness as a thin wrapper that calls the
  new verb, so the DoD and the product verb can't diverge.
- **(Deferred, do not implement) `supervision.auto_spawn` tracking issue** —
  open as a *parked* design issue capturing the three-part evidence trigger,
  so the deferral is discoverable rather than lost; explicitly blocked on
  Phase-1 evidence and a non-human-principal model.
- **(Optional) `run drive --once` / `--json` CI assertion helper** — a small
  helper analogous to `dashboard --once` for CI to advance-and-assert a run
  one tick at a time.

## Open questions / revisit triggers

1. **Author-role generalization.** The freshness check and the new predicate
   are literally `role_id = 'author'`. Workflows with multiple author-side
   roles (e.g. a designer + an implementer both upstream of a fresh reviewer)
   are not covered. Generalize to "any non-reviewer role that produced a job
   the reviewer reviews"? That needs the review→reviewed-job edge from the
   snapshot and is a behavior change; deferred to an Open Question rather than
   widened silently. Revisit trigger: a real workflow whose fresh reviewer is
   contaminated by a non-`author` upstream role.
2. **Lease/claimable definition drift.** The predicate defines "contaminable"
   as live-lease OR pending-role-work over `queue_messages`. If a future
   shape lets an author hold context across a *gap* (claimed nothing now but
   will re-engage the same session), branch (b) would mis-classify it as
   drained. Revisit trigger: any shape with author session reuse across a
   non-`pending` gap.
3. **Single-driver enforcement strength.** The concurrent-driver mitigation
   is an *advisory* scratch marker, not a lock — the real safety is the
   daemon's `slotHasUnclaimedParallelWork` refusal. Is advisory enough, or
   should `run drive` take a per-run advisory PG lock (reusing RFC 0104's
   `lockRun`) to hard-prevent a second driver? Hard-locking changes the
   re-drive story (a crashed driver's lock must time out). Default: advisory;
   revisit if operators report confusing double-driver states.
4. **Drive of human-in-the-loop lanes.** `run drive` assumes every lane has a
   registered command it can `supervise start`. A workflow lane meant for a
   *human* terminal (no auto-command) should be *skipped* by the driver and
   surfaced, not spawned. The MVP skips lanes lacking a registered command;
   confirm that is the right default vs. an explicit
   `--lanes <set>` allowlist.
5. **`auto_spawn` reconcile-sharing.** If Phase 3 ever happens, the daemon
   path should call the *same* reconcile predicate as `run drive` (one
   algorithm, two homes). That requires factoring the reconcile logic into a
   package both the CLI loop and a daemon scheduler can import. Worth doing
   defensively in Phase 1 (extract `reconcile(summary, sessions) → actions`
   as a pure function), even though only the CLI uses it now? Default: yes —
   it also makes `TestRunDriveReconcileIsIdempotent` a pure-function test.
6. **Poll vs push (Phase 2).** The 15s jittered poll is fine for human-scale
   DAGs but adds latency at each transition. A daemon "job became ready"
   event the driver subscribes to would remove the latency *and* be the
   reusable substrate for a future auto-spawn. Is the latency real enough to
   build the event surface, or does poll suffice indefinitely? Revisit
   trigger: a measured transition-latency complaint or a many-lane run where
   poll cadence dominates wall-clock.

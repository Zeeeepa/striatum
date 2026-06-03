# RFC 0104: Per-run serialization invariant — one run lock to retire the lifecycle deadlock class

Status: accepted (D159, 2026-06-03)
Date: 2026-06-02
author: proposer-claude-opus-4-8-001
Context: RFC 0095 (revision-safe lifecycle), RFC 0101 (Phase 0a deadlock root-fix), RFC 0103 (production hardening); skills/optional/supabase-postgres-best-practices `lock-deadlock-prevention`, `lock-advisory`, `lock-skip-locked`; decision-log D-rows on the claim/review/interrogation deadlock retries (#98, #103, #137).

## Problem

Two of the hottest per-run write paths acquire the **same run's rows in opposite
order**, which Postgres resolves by aborting one transaction with a deadlock
(SQLSTATE `40P01`):

- **`HandleClaimNext`** (`go/pkg/mutations/claim.go:64,78,147`) locks
  **sessions → runs → jobs**.
- **`recordVerdict`** (`go/pkg/mutations/review.go:449`) →
  **`maybeCompleteRun`** (`go/pkg/mutations/mutations.go:646`) →
  **`closeRemainingSessions`** (`go/pkg/mutations/mutations.go:738`) locks
  **jobs → runs → sessions**.

The `{sessions, runs}` pair is inverted between the two, so a claim holding
`sessions(R)` and waiting for `runs(R)` can deadlock against a verdict-completion
holding `runs(R)` and waiting for `sessions(R)`. The 60s recovery sweep
(`recovery.go`, which locks jobs/leases/sessions across live runs) is a third
concurrent party on the same rows.

Today this is **tolerated, not prevented**: `withTxRetryOnDeadlock`
(`mutations.go:283`, bounded to 3 attempts with a 5ms backoff) wraps claim,
submit-review/verdict, register-session, and interrogation. The inline comments
already record it as *observed live* — `claim.go:44` (#103, "sibling reviewers
claiming in parallel … Postgres abort one with a deadlock") and `review.go:60`
(#98). The interrogation path got the *correct* fix in RFC 0101 Phase 0a — a
per-run advisory lock taken first (`lockRunInterrogation`, `mutations.go:332`) —
but that fix was scoped only to the interrogation↔claim interaction, not to the
claim↔verdict↔recovery set.

**Why it matters now (RFC 0103 envelope).** The deadlock only forms when two
transactions touch one run's rows concurrently — i.e. **multi-lane runs**
(parallel reviewers, a claim racing a completion, the sweep racing either). A
single-lane run has no intra-run write concurrency, which is exactly why the
single-claude RFC 0097 self-hosting dogfood completed while multi-lane panels
wedge. On a **server running multiple repositories** (the real deployment, not
a laptop), aggregate concurrency is higher and the bounded 3-attempt retry is
*more* likely to exhaust and surface
`invalid_transition: transaction aborted by a database deadlock` to a lane —
under **yolo/minimal-human-intervention** there is no operator to retry it by
hand. The retry is a band-aid on a structural inversion; this RFC removes the
inversion.

## Proposal

Adopt a single **per-run serialization invariant**:

> Every transaction that mutates a single run's aggregate (sessions, runs, jobs,
> leases, queue_messages, verdicts, blockers, interrogations for that run) MUST
> take the per-run advisory lock as its **first statement**.

Concretely:

1. **Generalize `lockRunInterrogation` into `lockRun`** (`mutations.go`): the same
   `pg_advisory_xact_lock(hashtext("striatum:run:" + repositoryID + ":" + runID))`,
   transaction-scoped, narrowest-useful granularity (one run). `lockRunInterrogation`
   becomes a thin wrapper or is removed.
2. **Take `lockRun` first** in every per-run mutation transaction:
   `HandleClaimNext`/`HandleAwaitPacket`, `HandleSubmitReview`/`HandleRecordVerdict`/
   `HandleOverrideVerdict`, the lifecycle-completion paths (`maybeCompleteRun`/
   `closeRemainingSessions` callers — `lifecycle.go`, `run.go`), and the per-run
   recovery handlers (`recovery.go`, `recovery_decision_tree.go`,
   `recovery_auto_finalize.go`, `recovery_escalation.go`). The recovery sweep takes
   it per-run as it iterates active runs (never one lock spanning all runs).
3. **Keep `withTxRetryOnDeadlock` as a backstop**, not the mechanism — once every
   per-run mutation serializes on the same first lock, the `{sessions, runs, jobs}`
   cycle cannot form, so a `40P01` becomes a should-never-happen signal rather than
   an expected transient.
4. **Document the invariant** in `docs/reference/spec.md` (run-aggregate section)
   and add a guard test that fails if a per-run handler's transaction does not take
   `lockRun` first.

The lock is **per run**, so unrelated runs (and unrelated repositories) never
serialize against each other; only the (small, lane-bounded) concurrency *within*
one run is serialized — which is precisely the cheap case. The cross-repo
control plane (RFC 0032) is unaffected: its run IDs are distinct keys.

## Acceptance

- A PG-gated regression test drives, against one run concurrently: a
  `work.claim_next`, a parallel `review.submit` that completes a sibling review
  (running `maybeCompleteRun`), and a `recovery.sweep` tick — and asserts **no
  `40P01` is surfaced to any caller** and the run advisory lock serializes them.
- The existing `pkg/mutations` suite stays green; `go vet` + `golangci-lint`
  clean; the RFC 0101 chaos suite stays green.
- A guard test asserts every registered per-run mutation handler takes `lockRun`
  before any `FOR UPDATE` on a run-scoped table.

## Non-goals

- Not a new locking framework. One advisory key, taken first; nothing else changes.
- Not removing `withTxRetryOnDeadlock` — it stays as defense in depth.
- Not changing the claim queue's `FOR UPDATE OF qm SKIP LOCKED` (`claim.go:119`),
  which is already correct (`lock-skip-locked`).
- Not touching cross-repo or daemon-global mutation paths (they are not per-run).

## Relationship to prior RFCs

- **RFC 0101 Phase 0a** introduced the per-run interrogation advisory lock; this
  RFC generalizes that exact technique to the whole run aggregate.
- **RFC 0095** (revision-safe lifecycle) and **RFC 0103** (production hardening)
  assume the lifecycle state machine is coherent under concurrency; this RFC
  removes a concrete way it is not. It is a foundation dependency for the
  RFC 0105 reliability harness (the harness should test a system that cannot
  deadlock-wedge, so the remaining wedges are genuine state-machine bugs).
- Adopted from the vendored `supabase-postgres-best-practices` skill's
  `lock-deadlock-prevention` rule ("acquire locks in a consistent order"),
  realized here as "one lock, taken first" rather than hand-ordering every
  multi-table transaction.

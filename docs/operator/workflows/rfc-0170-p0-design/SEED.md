# Design-Run Seed — RFC 0170 P0 (FRESH v1)

> This is the **fresh v1** `falsification_gate` design run for **RFC 0170**
> (self-culling repository / the CULL workflow class). RFC 0170 is `proposed`
> on `main`. This run hardens its **P0 slice** into **falsifiable,
> build-bearing acceptance criteria** and stress-tests the two claims P0
> actually rests on: that the supersession edge yields **zero false-positive**
> cull candidates, and that a read-only sweep riding the recovery timer is
> **provably safe** (cannot destabilize recovery, cannot delete, cannot page).
>
> **Required context docs** (read in full first):
> - `docs/rfcs/0170-self-culling-repository-and-cull-workflow-class.md` — the
>   RFC (three pillars, shadow-first P0–P5 phasing, four rejected traps,
>   acceptance criteria, domain model, five Open Questions).

## Charter

The deliverable (committed `PROPOSAL.md`, "the SPEC") is the **falsifiable
implementation spec for RFC 0170 P0** that the downstream `rfc-0170-p0-build`
`code_change` run executes. Do **not** re-derive or re-litigate the RFC's
posture — its `/adhd` pass already converged on three coupled mechanisms
(trigger → reversible cull → counterforce) and a shadow-first phasing. P0 is
deliberately the **smallest tracer-bullet**: the candidacy substrate and the
**cheapest, exact** detection tier, with **no deletion at all**. The SPEC must
turn that into build-bearing constraints, each a concrete falsifiable assertion
+ the test/corpus row that would refute it, and draw the P0/P1+ boundary
precisely.

## The problem P0 begins to answer (do NOT re-litigate — settled in the RFC)

The runner has three workflow classes — DESIGN, BUILD, VERIFY — and **all three
add; nothing subtracts**. The 2026-06-24 architecture review measured the
consequence: **+85.5K Go LOC against −2.8K in 13 days**, 41 unmerged
`rfc-…-design-vN` branches, dead subsystems that survived ~four reviews. RFC
0170 makes deletion a first-class, continuous, runner-driven, provenance-
disciplined mutation. **P0 builds none of the deletion** — it builds only the
**eyes**: a continuously-maintained ledger of *candidates*, populated from the
one edge that is **free and exact today**.

## P0 boundary (the security floor of the cull system: it only OBSERVES)

Propose **P0 =** exactly these three, and nothing more:

- **A `cullable_entity` candidacy ledger** (runtime-owned). Columns per the RFC:
  `(kind, ref, last_reinforced_at, decay_score, reachable_from_root,
  candidacy_state)` for `kind ∈ {code_symbol, file, package, branch, rfc,
  decision, doc, table}` — though P0 only *populates* the kinds Tier-1 can prove
  (rfc, decision, branch, doc). A **runtime** migration (confirm the next free
  slot from a fresh `ls go/pkg/db/migrations` — the RFC guesses ~`0045`),
  **not** an owner bundle: the runtime role must DML it, so it **MUST** carry
  read **and** write authority-inventory rows or it red-mains CI under
  `TestWriteAuthorityInventoryComplete` / `TestReadAuthorityInventoryComplete`
  and the non-PG `go/pkg/db/authority_inventory_static_test.go`.
- **A read-only `DecayTickSweep.SweepOnce`** in `go/pkg/recovery` that piggybacks
  the existing recovery-sweep timer (`scheduler.go` / `sweep.go`). Each tick it
  reads **Tier-1 supersession edges only** and **upserts** candidacy rows. It
  takes **no** other action.
- **Read-only candidacy state.** Observe; **no** tombstone, **no** deletion,
  **no** page, **no** doctor RED/amber, **no** run-admission effect. P0 is a
  mirror you can look into, not a hand that removes anything.

**Tier-1 — the only edge P0 reads (free today, claimed exact):**

- `verdicts.superseded_by_decision_id` / `superseded_at` — live in Postgres
  since RFC 0047 / migration 0007.
- The markdown `Status: superseded by …` convention in `docs/`.
- A `rfc-…-design-vN` branch whose **ratified** `vN+1` exists.

An entity that is **superseded with a live successor** auto-enqueues a candidacy.
Deadness is thus **provable by a DB/markdown traversal**, not asserted by an LLM
(RFC Non-Goal: an LLM may *propose* but may never *certify* deadness; P0 has no
LLM in the loop at all).

## The hard core to PROVE (the two claims P0 rests on)

1. **Tier-1 is EXACT — zero false positives across the live supersession
   corpus** (RFC Acceptance Criterion 1). "Superseded" is **not** the same as
   "dead." The repo right now holds load-bearing artifacts that are *superseded*
   yet must **never** be nominated:
   - The six **banked design branches** `backup-branch rfc-0136 / rfc-0164 /
     rfc-0165 / rfc-0166 / rfc-0168 / rfc-0169` (`*-design-vN-2026-06-24`) are
     each a `rfc-…-design-vN` superseded by a later `vN`, **yet
     `docs/operator/rfc-roadmap.md` preserves them as RESUME SEEDS** for the next
     `-vN+1` run. Naive `rfc-…-design-vN → cullable` would cull the project's own
     recovery context.
   - A `Status: superseded by D###` decision can still be **cited** by a live
     decision/RFC/spec; a superseded RFC can be **reopened**.
   - Frozen provenance (`.check-docs-ignore`, `docs/records/_frozen/`) and
     historical fixtures contain `superseded`-shaped text that is **not** a live
     candidacy signal.
   The SPEC must specify the **exact predicate** that excludes the
   resurrectable / cited / banked / protected-root set, and the **corpus test**
   that asserts P0 nominates the genuinely-dead superseded artifacts with **zero
   hits** on the preserved set.

2. **The sweep is provably READ-ONLY and ERROR-ISOLATED.** It writes nothing but
   the `cullable_entity` upsert; it triggers **no** P0 action; and a panic, a
   Tier-1 query error, a nil row, or a slow query inside `DecayTickSweep`
   **cannot** suicide, stall, abort, or starve the recovery sweep it rides on
   (the daemon has a documented *sweep-error daemon-suicide* failure mode — name
   the `recover()`/log/continue isolation seam and its regression test). Bound
   the per-tick cost over the live corpus; assert `doctor` stays green across
   ticks.

## Open design points to DISCHARGE (each → a constraint + test)

- **Open Question 1 — peer or phase?** (the RFC's load-bearing design question).
  P0 is the **sweep/peer** path. State whether the P0 ledger is written by the
  sweep only, and **prove the schema + writer model do not preclude** later
  adding the *phase/toll* model (a run cannot `complete` until it tombstones what
  it superseded or posts an overdraft). Resolve it enough that P0 is
  forward-compatible; do not build the phase side.
- **What does `decay_score` mean for a Tier-1-only P0?** Tier-1 supersession is
  a **binary** edge (superseded-with-live-successor or not), while half-life
  decay is really a Tier-3 reachability concept. State precisely whether P0's
  candidacy is gated by the supersession **predicate** (with `decay_score`
  merely recorded for forward-compat), or by a threshold — and avoid implying a
  decay clock that P0 does not actually run.
- **The `candidacy_state` machine for P0.** With no tombstone yet, what are the
  legal states (e.g. `nominated` / `observed` / `withdrawn-on-reappearance`)?
  How is a candidacy **withdrawn** if a superseded artifact is re-cited or a
  banked branch is resumed (P0's tiny echo of the RFC's resurrection property)?
- **The #614 substrate trap.** The migration's grants + read/write
  authority-inventory rows, and a **no-`SELECT *`** read discipline on any
  mutation-surface read of `cullable_entity` (the bundle-0022 SEV-1: a
  `SELECT *` met a revoked column grant and 42501'd every run mutation for
  ~12h). State the exact column list the sweep reads.

## Falsifier guidance (attack the v1 proposal)

- **Falsifier 1 (Tier-1 exactness / false-positive lens):** find a **real**
  entity in this repo the predicate would wrongly nominate — a banked
  `backup-branch rfc-…` resume seed, a still-cited superseded decision, a
  `vN+1`-not-actually-ratified inference, a frozen-provenance false match, or an
  ambiguous "live successor" definition two implementers would read differently.
  Ground every counterexample in a real path or ref.
- **Falsifier 2 (read-only safety / substrate-integrity lens):** show how the
  "harmless" sweep hurts — shared fate with the recovery sweep (no error
  boundary), a hidden write or action, a missing authority-inventory row, a
  `SELECT *`-over-revoked-grant hazard, a mis-claimed migration slot, unbounded
  per-tick cost/lock contention, or P1+ behavior smuggled into P0 under "observe
  only."

## The gate

The adjudicator gates on whether **G1 Tier-1 exactness** (zero false positives,
resurrectable/banked set explicitly excluded), **G2 read-only safety** (error-
isolated from recovery, no action, no page), **G3 substrate correctness**
(migration slot + grants + read/write inventory + no `SELECT *`), and **G4
forward-compatibility** (OQ1 resolved, crisp P0/P1+ boundary) are **proven, not
merely claimed** — with no standing falsifier challenge. A clearing verdict
(`accept` / `accept_with_findings`) requires all four. This is the single
allowed v1 revision cycle; a second `needs_revision` ends the gate uncleared and
routes to the operator, who folds the findings into a fresh `-v2` run with a
revising holder.

# Design-Run Seed — RFC 0170 P0 (v2: revising holder, discharge G1′ + G2′)

> This is the **v2** `falsification_gate` design run for **RFC 0170** P0. Cycle-1
> (`run_85afe0ff`, branch `striatum/rfc-0170-p0-design`) ran a full, rigorous round
> — holder SPEC → two codex falsifiers → independent claude adjudicator — and
> returned **`needs_revision`** with two binding constraints (**G1′**, **G2′**). Two
> of the four gates already **cleared and are not reopened here**: **G3 substrate
> correctness** and **G4 forward-compatibility**. This run folds the cycle-1
> findings into a **revising holder** whose job is to **discharge G1′ and G2′** —
> not to re-derive the SPEC.
>
> **Required context docs (read in full first):**
> - `docs/operator/workflows/rfc-0170-p0-design-v2/context/HOLDER_v1.md` — the v1
>   SPEC (the base you revise; only §3 predicate and §4 sweep change).
> - `docs/operator/workflows/rfc-0170-p0-design-v2/context/LEDGER_cycle_1.md` — the
>   cycle-1 adjudication ledger. Its constraints `C-G1-CITATION-EXACTNESS` and
>   `C-G2-CULL-FOLD-DEADLINE` and their `verification.gate` rows are authoritative.
> - `docs/rfcs/0170-self-culling-repository-and-cull-workflow-class.md` — the RFC
>   (settled framing: three pillars, shadow-first P0–P5, four rejected traps).

## Charter (revising holder)

The deliverable (`HOLDER.md`, "the revised SPEC") is the v1 SPEC with **G1′ and G2′
discharged and nothing cleared weakened**. Keep §2 (substrate / migration 0045 /
authority-inventory / no-`SELECT *`), §5 (OQ1 peer-vs-phase), and §6 (the P0/P1+
boundary) intact — G3 and G4 cleared on them and no falsifier challenge stands.
Rewrite **only** §3 (the Tier-1 predicate + the G1 corpus) and §4 (the sweep's
latency boundary), and re-map them in §7/§8. Every load-bearing claim stays a
**falsifiable assertion + the test/corpus row that refutes it**, anchored to a
named source site verified against the live tree at the run base.

## G1′ — reconcile the predicate and re-derive the corpus (binding)

Cycle-1 falsifier_1 stood. The v1 corpus listed `rfc:0097` as a **true positive**
(to be `nominated`), but the v1 predicate's **clause 4** (no live inbound citation)
**withholds** it: `docs/rfcs/0097-…md:3` is `Status: superseded by RFC 0116 / 0122
/ 0124` **and** live-cited by RFC 0101 (umbrella-of-record, live) and RFC 0103
(accepted, live) as a load-bearing baseline. The corpus and the predicate
contradict each other, and **clause 2** ("a named live successor") collides with
**clause 4** ("no live inbound citation") because a supersession-chain successor
routinely cites the predecessor it supersedes. Discharge it by:

1. **Reconcile clause 2 vs clause 4 mechanically.** Define the inbound-citation
   check so two implementers read every corpus row identically with **no LLM
   judgment**: it MUST exclude (a) the entity's own named-successor refs (a
   successor citing the predecessor it supersedes is a supersession backref, not a
   live dependency) and (b) the supersession `Status:` line itself; and it MUST
   distinguish a disposable historical mention / see-also / reference-link
   definition from an **active-baseline** citation (the RFC 0101/0103
   "PROVEN BASELINE" / umbrella-of-record use of `rfc:0097`). State the exact,
   greppable rule.
2. **Re-derive the corpus to equal what the reconciled predicate produces.** Move
   `rfc:0097` to the **preserved set** (superseded but live-cited ⇒ load-bearing ⇒
   never nominated). Re-audit `rfc:0027`, `rfc:0041`, `rfc:0039` (and any other
   front-matter-superseded artifact with a resolving live successor) for live
   inbound citations from non-superseded artifacts in the scanned roots under the
   reconciled rule; the genuine zero-citation set is the new true-positive set.
3. **Re-prove the G1 corpus test.** The reconciled predicate nominates exactly the
   genuinely-dead superseded set with **zero hits** on the preserved set
   (`rfc:0097` now in the preserved set, alongside `docs/reference/todo.md`,
   `rfc:0028`, the `backup/rfc-*` banked branches, the RFC-0170 body / SEED /
   workflow.json / decision-log prose, and `docs/records/_frozen/**`), and every
   published true positive is independently confirmed to carry no live inbound
   citation.

## G2′ — bound the cull fold so it cannot stall recovery (binding)

Cycle-1 falsifier_2 stood. The v1 sweep proved **panic** and **returned-error**
isolation but **not a latency/stall bound**. The fold is synchronous in the single
recovery goroutine (`scheduler.go:55-80` re-enters `wait(interval)` only after
`SweepOnce` returns); a blocked/slow `DecayTickSweep` scan — an unbounded `docs/**`
walk, the inbound-citation grep, or a lock/IO-waiting query — **neither panics nor
returns**, so the `recover()` seam and the discard-the-error fold never fire, the
goroutine is held, the next recovery tick never starts, and after
`doctorRecoveryCursorWedgedAfter = 5m` (`doctor.go:16`) doctor emits
`recovery_sweep_cursor_wedged` → `ok:false`. The DB `statement_timeout` already
equals the 60s cadence (`connection.go:289-290`) and the filesystem scan has no
timeout at all. Discharge it by:

1. **A per-tick deadline** — `context.WithTimeout` strictly below
   `DefaultSweepInterval = 60s` — covering **both** the DB read (a
   `statement_timeout` below the recovery cadence) **and** the filesystem/grep scan
   (which has no `statement_timeout` equivalent — bound it explicitly).
2. **A skip-on-overrun policy** so a slow/blocked tick is **abandoned** rather than
   holding the single recovery goroutine. Name where the deadline is enforced and
   how a partial scan is discarded without a partial/torn write.
3. **A HANG regression test** in `go/pkg/recovery` mirroring
   `TestActiveRunSweepPanicDegradesRunAndContinues` but for a **blocking,
   non-returning** scan (not a panic): inject a `DecayTickSweep` scan that blocks
   past `doctorRecoveryCursorWedgedAfter` and assert (i) the next recovery tick
   still runs and (ii) doctor does **not** emit `recovery_sweep_cursor_wedged` and
   stays `ok:true`.

## Do NOT reopen (cleared in cycle-1)

- **G3 substrate** (MET): the `0045_cullable_entity.sql` runtime slot, both read +
  write authority-inventory rows, the `striatumd_rw` GRANT with no owner DDL/FK
  (≥27 rule), explicit-column reads (no `SELECT *`). Keep §2 intact.
- **G4 forward-compat** (MET): OQ1 → the sweep/peer writer; `(kind,ref)`
  ON CONFLICT + the extensible `candidacy_state` CHECK; the P0/P1+ deferral table.
  Keep §5 and §6 intact.

Reopening a cleared gate or weakening §2/§5/§6 is out of scope, and the falsifiers
will flag any regression.

## Falsifier guidance (attack the *revised* spec)

- **Falsifier 1 (Tier-1 exactness / false-positive lens):** attack the
  **reconciled** predicate and the **re-derived** corpus. Is the clause-2/clause-4
  reconciliation actually mechanical and unambiguous, or does "active-baseline vs
  disposable mention" still need human judgment? Does any re-audited true positive
  (`rfc:0027/0041/0039` or whatever survives) still carry a live load-bearing
  citation the holder missed? Did moving `rfc:0097` to the preserved set introduce
  a new contradiction or false negative? Ground every counterexample in a real
  path/ref in this tree.
- **Falsifier 2 (read-only safety + substrate no-regression):** attack the
  **latency bound**. Does the deadline truly cover the filesystem/grep scan and not
  just the DB read? Can skip-on-overrun leak a partial/torn write or a still-stuck
  goroutine (a `context.WithTimeout` does not stop a blocking syscall already in
  flight)? Does the HANG test actually prove the next recovery tick runs and doctor
  stays green, or does it only re-prove the panic path? Separately, confirm the
  revision did **not** regress G3/G4 (no `SELECT *` reintroduced, no
  authority-inventory row dropped, no smuggled P1+ action).

## The gate

A clearing verdict (`accept` / `accept_with_findings`) requires **G1′ discharged**
(a reconciled, mechanical predicate + a re-derived corpus that nominates exactly
the genuinely-dead set with zero hits on the preserved set, no standing false
positive), **G2′ discharged** (a real per-tick deadline over DB **and** filesystem
+ skip-on-overrun + a passing HANG regression test), and **G3 + G4 still MET** (not
regressed). This is v2's single proper revision round. A `needs_revision` here
exhausts the gate and routes the residual to the operator for a fresh `-v3` — do
not ratchet. On a clearing verdict the committer publishes the consolidated SPEC
and the operator ratifies **D271**.

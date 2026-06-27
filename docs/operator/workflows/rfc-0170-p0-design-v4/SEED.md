# Design-Run Seed — RFC 0170 P0 (v4: revising holder, close the last two residuals)

> This is the **v4** `falsification_gate` design run for **RFC 0170** P0. The design
> has converged across three rounds; v3 (`run_01b5b2d0…`) discharged the hardest
> parts and exhausted its cycle at `needs_revision` with **two pinpoint residuals**.
> What is now **credited / cleared and must NOT be reopened**:
>
> - **G1 decision-successor rule — DISCHARGED.** The `kind=decision`
>   successor-extraction rule is now a pure greppable, no-LLM rule (own-row cells
>   3→5 by precedence, the `\bsupersed(?:ed|es) by\s+<reflist>` regex, sentence
>   boundary, multi-ref split); falsifier_1 ran it by hand: `D267→{D270}` (cell 3),
>   `D081→{D087,D094,D104}` (cell 5), and a bare state-cell-only `superseded`
>   decision is withheld, with no new false positive.
> - **G1 clause-4 active-baseline citation rule — CREDITED**; `rfc:0097`/`0027`/
>   `0039`/`0041` (and `D174` via live RFC 0109) stay preserved.
> - **G2 latency machinery — CREDITED**: `DefaultCullFoldTimeout = 10s` over both
>   the DB read and the filesystem scan, watchdog child goroutine, single-in-flight
>   skip-on-overrun, L4 compute-then-commit (zero writes on a timed-out tick).
> - **G3 substrate** and **G4 forward-compat** — confirmed **un-regressed**.
>
> The v4 revising holder closes **only** the two residuals below.
>
> **Required context docs (read in full first):**
> - `docs/operator/workflows/rfc-0170-p0-design-v4/context/HOLDER_v3.md` — the v3
>   SPEC (the base you revise; only the §3 clause-3 pathspec and the §4 B5 HANG
>   test change).
> - `docs/operator/workflows/rfc-0170-p0-design-v4/context/LEDGER_v3_cycle_1.md` —
>   the v3 ledger. Its constraints `C-G1-DECISION-SUCCESSOR-EXACTNESS` (pathspec
>   residual) and `C-G2-HANG-DOCTOR-SEMANTICS` (refresh-timing residual) and their
>   `verification.gate` rows are authoritative.
> - `docs/rfcs/0170-self-culling-repository-and-cull-workflow-class.md` — the RFC.

## Charter (revising holder)

Republish the v3 SPEC as `HOLDER.md` with **only** the two residuals closed. Keep
everything credited/cleared intact — the decision-successor rule, the clause-4
citation rule, the latency machinery, §2 substrate, §5 OQ1, §6 boundary. Do **not**
re-derive or weaken any of it. Every load-bearing claim stays a falsifiable
assertion + the test/corpus row that refutes it, anchored to a named source site
verified against the live tree.

## G1‴ — make clause 3's protected pathspec fully static and tree-local (binding)

v3 falsifier_1 stood. Clause 3's protected pathspec retained the bullet **"any path
referenced by an open GitHub issue"** (HOLDER.md:373) with no subtraction of
`docs/rfcs/**` / `docs/decisions/**`, contradicting the holder's own clause-3 claim
(HOLDER.md:380-383) that those roots are not protected. The bullet is (i) a
**dynamic, GitHub-API-dependent** dependency, not a tree-local greppable cull
predicate (a path's protection flips with external issue-body text), and (ii)
**over-exposes** the candidacy surface: in current live issue state open **#585**
references `docs/decisions/decision-log.md` and open **#615** references
`docs/rfcs/0170-…md`, so the active decision log is protected right now and
`D267`/`D081` (rows at `decision-log.md:38` and `:220`) get
`reachable_from_root=true` and fail clause 3 before nomination — the published
true-positive set is non-derivable, a two-implementer split internal to the SPEC.

Discharge: make clause 3 a **fully static, tree-local** pathspec — **remove the
open-issue bullet from the cull predicate** (keep any issue-linked preservation, if
wanted, as an operator advisory **outside** `cullable_entity`), **or** explicitly
**subtract `docs/rfcs/**` and `docs/decisions/**`**, **or** replace it with a
**checked-in cull-specific protected-path manifest**. Then re-derive the G1 corpus
under that precedence with a fixture proving `D267`/`D081` are **NOT** protected
while the frozen/provenance roots (`docs/records/_frozen/**`, the
design-scaffold/fixture roots) **stay** protected — and **no path classification
depends on external (open-issue) state**.

## G2‴ — close the cursor-refresh-timing carrier in the HANG control (binding)

v3 falsifier_2 stood. The reframed A/B no-cull control does not yet **prove** "the
cull fold does not make recovery/lane doctor state worse than baseline." Carrier:
`doctor` reads the **persisted** `scheduler_cursors.last_result_json`
(`doctor.go:383-390`), refreshed only by the next recovery sweep's
`upsertSchedulerCursor → recoveryCursorResultWithLatch` (`sweep.go:246-263,284-358`);
the cull fold runs **synchronously inside `SweepOnce`**, and `RunScheduler` calls
`wait(interval)` only **after** `SweepOnce` returns (`scheduler.go:55-80`), so a hung
fold burning `DefaultCullFoldTimeout` **delays the next cursor refresh** and opens a
same-wall-clock window where doctor reads a staler (wedged) cursor under the cull
variant than under the control near the 5m threshold. v3's B5 (immediate fake
`Wait`, assert `Sweeps==2` and identical written cursor **values**, static-fixture
reads companion) tests **eventual release + value-identity**, not **refresh
timing**.

Discharge with **one** of (preferred (b)):

- **(a) same-wall-clock A/B test.** A fake clock modeling `SweepOnce` duration +
  `wait(interval)`; assert the cull variant is **not** left doctor-red while the
  control is green at the **same instant** near the 5m threshold, and that it
  **FAILS with the deadline removed**.
- **(b) design change that removes the carrier (preferred).** Make the cull fold
  **unable to postpone the cursor refresh** — e.g. **upsert the recovery cursor
  ahead of the cull fold**, or move the fold **off the `wait`-gating path** — so a
  hung fold cannot delay the next `scheduler_cursors` refresh by construction. Then
  the no-worse-than-baseline property holds trivially, provable by the existing B5
  plus a test that the cursor refresh is not deferred by a hung fold.
- **(c) explicit bounded-delay doctor-semantics decision** with its own coverage,
  instead of an unqualified "no-worse" claim.

Keep the credited deadline + watchdog + skip-on-overrun + L4 compute-then-commit
machinery **unchanged**.

## Do NOT reopen (credited/cleared)

The decision-successor rule, the clause-4 citation rule (preserved set
`rfc:0097`/`0027`/`0039`/`0041`/`D174`), the latency machinery, §2 substrate
(migration `0045`, both authority-inventory rows, GRANT no owner DDL/FK ≥27, no
`SELECT *`), §5 OQ1, §6 deferral table. The falsifiers will flag any regression.

## Falsifier guidance (attack the v4 spec)

- **Falsifier 1 (Tier-1 exactness):** confirm clause 3 is now **fully static and
  tree-local** — no open-issue or any external-state dependency remains in the cull
  predicate, `D267`/`D081` are eligible (not protected), and the frozen/provenance
  roots stay protected. Re-run the corpus by hand: nominate exactly `{D267, D081}`,
  zero hits on the preserved set. Did the rewrite re-break the preserved set or
  expose a frozen artifact? Ground every counterexample in a real path/ref.
- **Falsifier 2 (read-only safety + no-regression):** attack the closed carrier.
  Under (b), does the cursor upsert truly run **before**/off the fold's gating path
  so a hung fold cannot defer the next refresh, and is there a test proving the
  refresh is not deferred? Under (a), does the fake clock actually model
  `SweepOnce` duration + `wait`, assert same-instant non-worse-than-control near the
  threshold, and FAIL with the deadline removed? Separately confirm no regression of
  the latency machinery, the clause-4 rule, G3 substrate (no `SELECT *`, both
  inventory rows), or G4 forward-compat (no smuggled P1+ action).

## The gate

A clearing verdict (`accept` / `accept_with_findings`) requires **G1‴ discharged**
(a fully static tree-local protected pathspec; the corpus nominates exactly
`{D267, D081}` with zero hits on the preserved set; no external-state dependency),
**G2‴ discharged** (the refresh-timing carrier removed by design, or a same-wall-clock
A/B test that binds and fails without the deadline, or a declared bounded-delay
decision), and **everything credited/cleared still intact**. This is v4's single
proper revision round. A `needs_revision` here exhausts the gate and routes to a
fresh `-v5` — do not ratchet. On a clearing verdict the committer publishes the
consolidated SPEC and the operator ratifies **D271**.

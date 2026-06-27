# Design-Run Seed — RFC 0170 P0 (v5: re-scoped P0 bar, the clearing round)

> This is the **v5** `falsification_gate` design run for **RFC 0170** P0, the
> intended **clearing** round. Four prior rounds proved the architecture and
> converged the SPEC; the gate then kept surfacing ever-finer **whole-tree
> Tier-1 corpus-exactness** edge cases that have near-zero blast radius for an
> **observe-only** P0 (which writes a candidacy ledger that **nothing acts on** —
> no deletion, no page, no run-admission). The product owner has therefore
> **re-scoped the P0 acceptance bar** (operator decision, to be recorded as
> **D271** when this design ratifies):
>
> **The P0 bar (binding for this run):**
> 1. **G1 — the Tier-1 predicate is MECHANICAL and SOUND** (pure greppable, no LLM
>    and no external/mutable-outside-the-tree state), and the **known-set corpus
>    test** holds: the predicate **never nominates the known preserved set**
>    (`rfc:0097`/`0027`/`0039`/`0041`, `D174`, the `backup/rfc-*` banked branches,
>    `docs/records/_frozen/**`, the RFC-0170 body/SEED/workflow.json/decision-log
>    prose) — i.e. **zero false POSITIVES on the known set, the dangerous
>    direction** — and **nominates the known genuinely-dead set** under the
>    predicate. **Exhaustive whole-tree exactness is explicitly P1** (deferred to
>    **#618**), and a conservative **false-NEGATIVE** (e.g. `D081` withheld because
>    a `status: frozen` audit outside `docs/records/_frozen/` cites it) is
>    **acceptable in P0** — it under-nominates, the *safe* direction for a cull
>    system — and is tracked as **#618**.
> 2. **G2 — the cull fold is read-only SAFE**: the recovery loop is never blocked,
>    the persisted scheduler cursor refresh is not deferred, there is no torn write
>    (skip-on-overrun + L4 compute-then-commit), and a fold error/panic cannot
>    suicide or stall recovery. The **non-cooperative-filesystem-hang
>    cull-LIVENESS** bound (a ctx-ignoring blocking syscall holding the cull slot
>    until restart; a late-writer generation fence) is **explicitly P1** (deferred
>    to **#619**) — P0 requires read-only **safety** (no daemon destabilization, no
>    data loss), not adversarial-hang liveness.
> 3. **G3 substrate** and **G4 forward-compat** remain as cleared in rounds 1–4.
>
> **Required context docs (read in full first):**
> - `docs/operator/workflows/rfc-0170-p0-design-v5/context/HOLDER_v4.md` — the v4
>   SPEC (the base; it already carries the static tree-local pathspec, the
>   off-the-wait-path fold, the mechanical decision-successor rule, and the
>   credited latency safety machinery).
> - `docs/operator/workflows/rfc-0170-p0-design-v5/context/LEDGER_v4_cycle_1.md` —
>   the v4 ledger (what is credited/cleared, and the two residuals now deferred to
>   P1 as #618/#619).
> - `docs/rfcs/0170-self-culling-repository-and-cull-workflow-class.md` — the RFC.

## Charter (consolidating holder)

Republish the v4 SPEC as `HOLDER.md`, **reframed to the re-scoped P0 bar above** and
otherwise intact. Concretely, change only two things and document the deferrals:

1. **G1 corpus assertion → known-set form.** State the assertion as: the predicate
   is mechanical/sound, **never nominates the known preserved set** (zero false
   positives, the dangerous direction), and nominates the known dead set; and
   **explicitly document** that exhaustive whole-tree exactness — including the
   `status: frozen` records outside `docs/records/_frozen/` (e.g.
   `docs/records/audits/*`) that withhold `D081` — is a **P1** Tier-1-completeness
   item tracked in **#618**, a conservative false-negative in the safe direction,
   not a P0 blocker. The corpus test asserts the known-preserved-zero-hits and
   known-dead-nominated properties; it does **not** require exhaustive whole-tree
   nomination.
2. **G2 safety assertion → safety-vs-liveness split.** State the assertion as: the
   cull fold's **read-only safety** is proven (recovery loop never blocked, cursor
   refresh not deferred — fold off the wait-gating path, no torn write via
   skip-on-overrun + L4 compute-then-commit, panic/error isolation); and
   **explicitly document** that bounding a **non-cooperative** (ctx-ignoring)
   filesystem hang — cull-slot liveness + a late-writer generation fence — is a
   **P1** hardening item tracked in **#619**, read-only and restart-recoverable,
   not a P0 blocker. Remove or correct the v4 self-contradiction that claimed a
   non-cooperative blocking syscall self-terminates at the deadline; state plainly
   that ctx bounds cooperative scans and that the non-coop case is P1 (#619).

Keep **everything else intact and unweakened**: §2 substrate (migration `0045`,
both authority-inventory rows, GRANT no owner DDL/FK ≥27, no `SELECT *`), the
mechanical `kind=decision` successor rule (`D267→{D270}`, `D081→{D087,D094,D104}`),
the clause-4 active-baseline citation rule (preserved set untouched), the static
tree-local protected pathspec (no external-state dependency), the latency safety
machinery (`DefaultCullFoldTimeout`, watchdog, skip-on-overrun, L4
compute-then-commit, off-the-wait-path fold), §5 OQ1, §6 P0/P1+ boundary. Every
claim stays a falsifiable assertion + its refuting test/corpus row anchored to a
named source site. **Add #618 and #619 to the §6 deferral table.**

## Falsifier guidance (attack the v5 spec against the RE-SCOPED bar)

- **Falsifier 1 (Tier-1 soundness, known-set):** the bar is **soundness + the
  known-set corpus test + no false POSITIVE on the known preserved set** — NOT
  exhaustive whole-tree exactness. Attack: is the predicate still mechanical and
  free of external/mutable state? Does it ever nominate a **known preserved-set
  member** (a real false positive — the dangerous direction that IS a P0 blocker)?
  Is the known-dead set still nominated? Is the #618 deferral honestly a
  safe-direction false-negative, or does it actually hide a false **positive**?
  Ground counterexamples in real paths/refs. Do **not** re-raise exhaustive
  whole-tree exactness as P0-blocking — that is scoped to P1 (#618); raise it only
  if a deferred item is actually a false *positive* or a soundness break.
- **Falsifier 2 (read-only safety, safety-vs-liveness):** the bar is **read-only
  safety**, not adversarial-hang liveness. Attack: can the cull fold block the
  recovery loop, defer the cursor refresh, tear a write, or destabilize the daemon
  (a real P0 safety break)? Is the safety claim source-true against `scheduler.go`
  / `sweep.go` / `doctor.go`? Confirm no regression of G3 substrate (no `SELECT *`,
  both inventory rows) or G4. Do **not** re-raise the non-coop-FS-hang liveness
  bound as P0-blocking — that is scoped to P1 (#619); raise it only if it is
  actually a *safety* break (data loss, daemon suicide, torn write) rather than a
  liveness degradation.

## The gate

A clearing verdict (`accept` / `accept_with_findings`) requires, **against the
re-scoped P0 bar**: the predicate is **mechanical and sound**; the **known-set
corpus test** holds (**zero false positives on the known preserved set**, known
dead set nominated); the cull fold is **read-only safe** (loop never blocked,
refresh not deferred, no torn write, panic/error isolated); **G3 + G4 intact**; and
the two deferred items are **honestly scoped to P1 with #618/#619 tracking** (a
safe-direction false-negative and a restart-recoverable liveness gap, not a
disguised false positive or safety break). `needs_revision` only if the predicate
is unsound, a **known preserved-set member is nominated** (real false positive),
read-only **safety** is broken, or a deferral actually hides a P0-blocking defect.
On a clearing verdict the committer publishes the consolidated SPEC and the
operator ratifies **D271** (recording the P0 scope boundary in the decision-log).

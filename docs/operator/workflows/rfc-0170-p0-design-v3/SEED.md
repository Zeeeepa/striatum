# Design-Run Seed — RFC 0170 P0 (v3: revising holder, discharge G1″ + G2″)

> This is the **v3** `falsification_gate` design run for **RFC 0170** P0. The v2
> revision round (`run_3506471695…`, branch `striatum/rfc-0170-p0-design-v2`) made
> **real progress on both binding constraints** but exhausted its single revision
> cycle at **`needs_revision`**, routing two narrow residuals — **G1″** and **G2″** —
> to a fresh `-v3`. What is now **credited / cleared and must NOT be reopened**:
>
> - **G1′ rfc:0097 false positive — FIXED.** Under the v2 clause-4 active-baseline
>   citation rule, `rfc:0097` (and `rfc:0027`/`rfc:0039`/`rfc:0041`) are correctly
>   **preserved** as live active-baseline-cited; no live artifact is auto-nominated.
> - **G2′ cull-fold latency machinery — CREDITED.** `DefaultCullFoldTimeout = 10s`
>   over **both** the DB read (`SET LOCAL statement_timeout`) and the filesystem
>   scan (watchdog child goroutine returning the recovery goroutine on
>   `cullCtx.Done()`), single-in-flight skip-on-overrun, and L4
>   compute-then-commit (zero writes on a timed-out tick, no torn write).
> - **G3 substrate** and **G4 forward-compat** — confirmed **un-regressed**.
>
> The v3 revising holder discharges **only** the two residuals below.
>
> **Required context docs (read in full first):**
> - `docs/operator/workflows/rfc-0170-p0-design-v3/context/HOLDER_v2.md` — the v2
>   SPEC (the base you revise; only the §3 decision-successor rule + protected
>   pathspec and the §4 HANG-test doctor assertion change).
> - `docs/operator/workflows/rfc-0170-p0-design-v3/context/LEDGER_v2_cycle_1.md` —
>   the v2 adjudication ledger. Its constraints `C-G1-DECISION-SUCCESSOR-EXACTNESS`
>   and `C-G2-HANG-DOCTOR-SEMANTICS` and their `verification.gate` rows are
>   authoritative.
> - `docs/rfcs/0170-self-culling-repository-and-cull-workflow-class.md` — the RFC.

## Charter (revising holder)

Republish the v2 SPEC as `HOLDER.md` with **only** the two residuals discharged.
Keep everything credited/cleared intact — the substrate (§2), OQ1/forward-compat
(§5/§6), the clause-4 active-baseline citation rule that preserves
`rfc:0097`/`0027`/`0039`/`0041`, and the entire latency machinery (deadline +
watchdog + skip-on-overrun + L4 compute-then-commit). Do **not** re-derive or
weaken any of it. Every load-bearing claim stays a falsifiable assertion + the
test/corpus row that refutes it, anchored to a named source site verified against
the live tree at the run base.

## G1″ — a mechanical decision-successor rule + a cull-specific protected pathspec (binding)

v2 falsifier_1 stood. The replacement true-positive set `{decision:D267,
decision:D081}` is **not mechanically derivable** from the written predicate, and
the protected pathspec eats the RFC root. Two source-verified defects:

1. **Decision-successor extraction is ambiguous.** v2 clause 1 fixes a decision's
   structural status field as the **second** pipe-delimited cell only, and clause 2
   parses the successor from that status value — but `docs/decisions/decision-log.md:38`
   (D267) has state cell = bare `superseded` with `SUPERSEDED by D270` in the
   **third (description)** cell, and `:220` (D081) has state cell = bare
   `superseded` with `Superseded by D087/D094/D104` in the **fifth (consequences)**
   cell. A literal implementer reading only the status value withholds both (the
   true-positive set is empty → A1 false); a looser implementer scanning other
   own-row cells nominates both. Discharge: for `kind=decision`, define a **pure
   greppable, no-LLM** successor-extraction rule — exactly **which own-row cells**
   may carry `superseded by <refs>` (e.g. description and/or consequences), their
   **precedence**, the **sentence boundary**, and the **multi-ref split** (e.g.
   `D087/D094/D104` → `{D087, D094, D104}`) — and state that **other rows' cells
   are never successor sources**. Add table-driven cases proving **D267 and D081
   nominate** AND a **bare state-cell-only `superseded` decision with no own-row
   successor prose is withheld**.
2. **The protected pathspec negates the candidacy surface.** v2 clause 3 protects
   "every entry already in `.check-docs-ignore`", and live `.check-docs-ignore:3`
   is `docs/rfcs/` wholesale (`:8` is `docs/operator/workflows/`). Taken literally,
   every RFC is protected before clause 2/4 runs, so no RFC can ever be
   nominated — contradicting §3's RFC corpus model and §2's `kind='rfc'`
   candidacy. Discharge: replace clause 3's `.check-docs-ignore` import with a
   **cull-specific protected pathspec**, **or** explicitly **subtract the
   actively-scanned roots** (`docs/rfcs/`, `docs/decisions/`) from clause 3, so the
   `kind=rfc`/`kind=decision` candidacy surface is not dead by construction.

Re-state the G1 corpus result under the fixed field/pathspec rules so two
implementers read every row identically: the predicate nominates exactly the
genuinely-dead superseded set (D267/D081 under the chosen rule) with **zero hits**
on the preserved set (`rfc:0097`/`0027`/`0039`/`0041` remain preserved).

## G2″ — make the HANG test's doctor assertion source-true (binding)

v2 falsifier_2 stood. The latency machinery is credited; the **only** gap is B5's
doctor-green assertion. The v2 holder grounded "doctor stays `ok:true`" on the
recovery cursor / `last_sweep_at` advancing, but `go/pkg/reads/doctor.go`
`recoveryCursorQuietSince` (469-479) keys the wedge on
`last_lane_advanced_at → started_at → created_at`, **never** `last_sweep_at`; it
fires when `claimable_job_count > 0` and that lane-quiet source is older than
`doctorRecoveryCursorWedgedAfter = 5m` (doctor.go:383,447-460). The existing test
`doctor_recovery_cursor_test.go:34-44` proves a **fresh** `last_sweep_at` with
`claimable_job_count=2` and a 10-minute-stale `last_lane_advanced_at` still returns
`ok:false`. Discharge with **one** of:

- **(a) A/B no-cull control (preferred).** Run the blocking-scan tick against an
  **identical no-cull control** and assert the cull fold **does not make
  recovery/lane doctor state worse than baseline** (`recovery_sweep_cursor_wedged`
  depends only on `claimable_job_count` + `last_lane_advanced_at`, which the cull
  fold never advances), while **`Sweeps == 2`** proves the recovery goroutine was
  released and **no `cullable_entity` write** occurs on the timed-out tick. The
  test must **FAIL with the deadline removed**.
- **(b) Intentional doctor-semantics change.** Change `doctorRecoverySweepCursor`
  (and `TestHandleDoctorFlagsRecoverySweepCursorWedgedClaimableRun`) to key the
  quiet window on `last_sweep_at`, declared as an **explicit doctor-semantics
  change with its own coverage** — not a claim that current doctor already behaves
  that way.

Keep the credited per-tick deadline (DB + filesystem), watchdog, skip-on-overrun,
and L4 compute-then-commit machinery **unchanged**.

## Do NOT reopen (credited in v2 / cleared earlier)

§2 substrate (migration `0045`, both authority-inventory rows, GRANT no owner
DDL/FK ≥27, no `SELECT *`); §5 OQ1 sweep/peer writer; §6 P0/P1+ deferral table; the
clause-4 active-baseline citation rule preserving `rfc:0097`/`0027`/`0039`/`0041`;
the latency machinery. The falsifiers will flag any regression.

## Falsifier guidance (attack the v3 spec)

- **Falsifier 1 (Tier-1 exactness):** attack the **decision-successor rule** and the
  **protected pathspec**. Are the allowed successor-source cells, precedence,
  sentence boundary, and multi-ref split fully greppable with no residual judgment?
  Run the rule by hand over `docs/decisions/decision-log.md` — does it nominate
  exactly D267/D081 (and any other genuinely-dead decision) and withhold every bare
  state-cell-only `superseded` decision? Does the new protected pathspec actually
  leave `docs/rfcs/` and `docs/decisions/` eligible while still protecting
  `docs/records/_frozen/**` etc.? Did the rewrite introduce a new false positive or
  re-break the preserved set? Ground every counterexample in a real path/ref.
- **Falsifier 2 (read-only safety + no-regression):** attack the **reframed HANG
  test**. Under option (a), is the no-cull control truly identical except for the
  cull fold, and does `Sweeps == 2` + the "no-worse-than-baseline" assertion
  actually prove the binding property against current `doctor.go` semantics, and
  does it FAIL with the deadline removed? Under option (b), is the doctor-semantics
  change coherent and its existing-test update correct? Separately, confirm the
  rewrite did not regress the credited latency machinery, the clause-4 citation
  rule, G3 substrate, or G4 forward-compat (no `SELECT *`, both inventory rows, no
  smuggled P1+ action).

## The gate

A clearing verdict (`accept` / `accept_with_findings`) requires **G1″ discharged**
(a mechanical decision-successor rule + a cull-specific protected pathspec, a
re-derived corpus two implementers read identically — D267/D081 nominate, the
preserved set untouched), **G2″ discharged** (a source-true HANG assertion via the
A/B control or a declared doctor-semantics change, failing with the deadline
removed), and **everything credited/cleared still intact**. This is v3's single
proper revision round. A `needs_revision` here exhausts the gate and routes the
residual to a fresh `-v4` — do not ratchet. On a clearing verdict the committer
publishes the consolidated SPEC and the operator ratifies **D271**.

# HOLDER — RFC 0170 P0 falsifiable implementation SPEC (self-culling repository: the Tier-1 candidacy substrate)

author: holder-author-001

> This is the **v1 leading proposal** for the RFC 0170 **P0** slice, published as the
> claim the two falsifiers re-attack. RFC 0170's three coupled pillars, the shadow-first
> P0–P5 phasing, the four rejected traps, the acceptance criteria, and the five Open
> Questions are **settled framing** — I do not re-derive or re-litigate them. I harden
> **P0 only** into build-bearing constraints: the smallest tracer-bullet of the cull
> system — a `cullable_entity` candidacy ledger plus a read-only `DecayTickSweep` that
> nominates dead artifacts from the **free Tier-1 supersession edge ONLY, with ZERO
> deletion, ZERO action, ZERO paging**. Every load-bearing claim below is a **falsifiable
> assertion** paired with the concrete test/corpus row that would refute it, and every
> claim is anchored to a named source site verified against the live tree at run base
> `striatum/rfc-0170-p0-design`. The four gates the adjudicator scores — **G1** Tier-1
> exactness, **G2** read-only safety, **G3** substrate correctness, **G4**
> forward-compatibility — are mapped to those assertions in §9.

---

## 0. P0 boundary (one paragraph, the security floor: P0 only OBSERVES)

P0 builds **exactly three things and nothing more**: (1) a **runtime** migration
`0045_cullable_entity.sql` adding `striatumd.cullable_entity(kind, ref,
last_reinforced_at, decay_score, reachable_from_root, candidacy_state)`; (2) a read-only
`DecayTickSweep.SweepOnce` in `go/pkg/recovery` that **piggybacks the existing recovery
sweep tick** and, per tick, evaluates the **Tier-1 supersession predicate** and UPSERTs
candidacy rows; (3) **read-only candidacy state** — observe, no tombstone, no deletion,
no reaper, no page, no `doctor` RED/amber, no run-admission effect. P0 is a mirror you
can look into, not a hand that removes anything. The deletion machinery
(`cull_tombstone`, the reaper, the soak window, `cull_gate`, the `accretion_ledger`
counterforce, Tiers 2–4) is **named but not built** here (§7).

---

## 1. Source-of-truth corrections (build-bearing — the RFC sketch guessed; the tree is authoritative)

The RFC sketch is a freshly-proposed `/adhd` sketch and explicitly says the committed
design proposal supersedes it where they differ. Three of its pointers are wrong or
imprecise against the live tree; the build run must use the corrected facts.

- **PC1 — migration path.** The RFC/SEED say `go/pkg/db/migrations`. **That directory
  does not exist.** Runtime migrations are embedded SQL at **`go/pkg/db/sql/00NN_*.sql`**
  (`go/pkg/db/migrations.go:21` — `//go:embed sql/*.sql`). Owner bundles are the
  *separate* FS `go/pkg/db/sql/owner/*.sql` (`go/pkg/db/owner.go:239`). The new table is
  a **runtime** migration, so it goes in `go/pkg/db/sql/`, not `sql/owner/`.
- **PC2 — next free slot.** The RFC guesses `~0045`. Confirmed by a fresh listing: the
  highest runtime migration is `0044_deploy_cursor.sql`; **`0045` is FREE** (`ls
  go/pkg/db/sql/0045*` → no match). Build at `go/pkg/db/sql/0045_cullable_entity.sql`.
- **PC3 — the verdicts column does NOT mean "artifact is dead" (the load-bearing
  correction).** The RFC lists `verdicts.superseded_by_decision_id` /
  `superseded_at` (added by `0007_decision_propagation.sql`, RFC 0047) as the cheapest
  Tier-1 candidate edge: "a superseded artifact with a live successor." **But those
  columns supersede a *review verdict within a run*, not a deliverable.** The live reader
  proves it: `go/pkg/reads/status.go:741` treats `v.superseded_by_decision_id IS NULL`
  as "still operator-actionable," i.e. a set `superseded_by_decision_id` means a review
  verdict was **invalidated by a recovery `invalidate-job` decision** so a *later attempt*
  could re-review the same job (`go/pkg/reads/status_superseded_pg_test.go:15–35`: an
  attempt-1 `needs_revision` verdict superseded by a recovery decision, with a later
  attempt-2 `accept`). The run almost always **went on to complete successfully** — the
  superseded verdict is intra-run lifecycle, **not** an artifact corpse. **Mapping a
  superseded verdict row to a `cullable_entity` would be a category error and a
  guaranteed false positive.** Therefore P0's exact artifact-deadness edge is the
  **markdown front-matter `Status: superseded by …` convention** (and the decision-log
  rows), and P0 **explicitly excludes** the `verdicts` column from candidacy (§3, A5).
  This is the single most important exactness decision in the SPEC, and it is the
  opposite of the naive reading of the RFC's Tier-1 bullet.

---

## 2. Substrate — migration `0045_cullable_entity.sql` (Claim C: the #614 trap, head-on)

A **runtime** migration creating one fresh runtime-owned table, modeled byte-for-byte on
the established pattern of `0042_verifier_attestations.sql` / `0043_schema_state.sql` /
`0044_deploy_cursor.sql`: a `CREATE TABLE IF NOT EXISTS striatumd.cullable_entity (…)`
with **no foreign keys** and **no owner-held-table DDL** (mandatory: the
`>=27` future-runtime guard `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL` /
`TestFutureRuntimeMigrationsDoNotFKOwnerHeldTables` in `go/pkg/db/migrations_test.go:643`
forbids ALTER/DROP of `striatumd.*` and FKs to owner-held tables for migrations ≥ 27;
the runtime role `striatumd_rw` applies it on a live restart and cannot touch owner
tables), followed by a role-guarded GRANT.

Column shape is **exactly the RFC's** (do not add/rename columns — keep the build
contract stable): `kind text`, `ref text`, `last_reinforced_at timestamptz`,
`decay_score double precision`, `reachable_from_root boolean`, `candidacy_state text`,
with `PRIMARY KEY (kind, ref)` (the natural candidacy key the sweep upserts on) and a
`CHECK (kind IN ('code_symbol','file','package','branch','rfc','decision','doc','table'))`
+ `CHECK (candidacy_state IN ('nominated','withdrawn'))`. P0 only ever writes
`kind ∈ {rfc, decision, doc}` (§3); the wider `kind` CHECK is forward-compat headroom, not
P0 surface.

Grant block, copied from the `0043`/`0044` precedent so the **runtime role can DML it**:

```sql
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT, UPDATE ON striatumd.cullable_entity TO striatumd_rw;
  END IF;
END
$$;
```

(No `DELETE` grant — P0 never deletes a candidacy row; withdrawal is an UPDATE of
`candidacy_state`. This keeps the table append/update-only and forward-compatible with the
RFC 0136 chain story.)

**The two CI guards this migration MUST satisfy or it red-mains (Claim C):**

- **C1 — authority inventory completeness (non-PG static guard).** The static test
  `go/pkg/db/authority_inventory_static_test.go` parses every embedded `CREATE TABLE
  striatumd.X` and fails if any table lacks an inventory row. So the build MUST add **both**
  a `readAuthorityInventory["cullable_entity"]` row in `go/pkg/db/read_authority_inventory.go`
  and a `writeAuthorityInventory["cullable_entity"]` row in
  `go/pkg/db/write_authority_inventory.go`. Correct classes:
  `writeAuthorityInventory["cullable_entity"] = ClassRuntimeDML` (the runtime role does
  direct DML — the live-coordination precedent, e.g. `schema_state` at
  `write_authority_inventory.go:48`), and `readAuthorityInventory["cullable_entity"] =
  ReadClassRuntimeOperational` (daemon-operational read surface — not sensitive prose, just
  candidacy bookkeeping; it carries no user/agent text). Omitting either row red-mains
  `TestReadAuthorityInventoryCoversEmbeddedTablesWithoutPostgres` /
  `TestWriteAuthorityInventoryCoversEmbeddedTablesWithoutPostgres` with no Postgres needed.
- **C3 — no `SELECT *` over a column-scoped grant (the #614 / bundle-0022 SEV-1 trap).**
  Commit `82bb94c2` records the outage: owner bundle 0022 (RFC 0167 P0) REVOKEd table-level
  SELECT on `striatumd.runs` and re-GRANTed every column except one; `mutations.rowByID`'s
  `SELECT *` then required the revoked column and `42501`'d **every run-scoped mutation
  daemon-wide for ~12h**. The lesson is build-bearing for the new table: **every read of
  `cullable_entity` (in the sweep and anywhere else) MUST project an explicit column list,
  never `SELECT *`.** The sweep's existing-state read (§4) selects exactly `kind, ref,
  candidacy_state`. P0's own grant is full-table for all six columns, so a `SELECT *` would
  *not* 42501 today — but it would the instant any later phase column-scopes the grant, and
  the discipline must be set at birth, not retrofitted after an outage.

---

## 3. Tier-1 detection predicate — EXACT, zero false positives (Claim A / Gate G1)

**P0 populates candidacy for `kind ∈ {rfc, decision, doc}` from the markdown front-matter
supersession convention ONLY.** It does **not** read `verdicts.superseded_by_decision_id`
for candidacy (PC3 / A5), and it does **not** populate `kind = branch` (A6). An entity `E`
is in state `nominated` on a tick **iff ALL five clauses hold**:

1. **Front-matter, not body prose.** `E` is a tracked file under a **scanned root**
   (`docs/rfcs/NNNN-*.md`; the per-RFC rows in `docs/decisions/decision-log.md`; the
   front-matter of other `docs/**` artifacts) whose **structural title-block `Status:`
   field** (for an RFC, the `Status:` line in the first ~6 lines — e.g.
   `docs/rfcs/0027-...md:3`) literally begins with `superseded` or `tombstoned`. **Any
   occurrence of "superseded" in the body is ignored.** This single clause excludes the
   largest false-positive surface: the RFC-0170 text itself
   (`docs/rfcs/0170-...md:3,95`), the prose in `docs/decisions/decision-log.md` (rows
   D231/D037 say "superseded by …" *about* other decisions), `docs/reference/spec.md:2247`,
   `docs/reference/prd.md:18`, `docs/rfcs/0015-...md:340`, and the SEED/`workflow.json` of
   this very run — none of which is a front-matter Status field.

2. **A parseable, LIVE successor.** The `Status:` line must name ≥ 1 successor ref that
   resolves to an existing, non-superseded artifact: `RFC NNNN` → `docs/rfcs/NNNN-*.md`
   exists **and its own `Status:` is not itself `superseded`/`withdrawn`**; `D###` → a
   `docs/decisions/decision-log.md` row that exists and is `accepted`. A **bare**
   `Status: superseded` with no parseable successor, or a successor ref that does not
   resolve, is **NOT** nominated. This clause excludes `docs/reference/todo.md:3`
   (`Status: superseded`, no successor) and `docs/rfcs/0028-...md:3`
   (`Status: superseded V1 foundation` — names no successor at all). "Superseded" without a
   live replacement is dormancy, not death.

3. **Not in the protected root set.** `E`'s path must not match the **protected-root
   pathspec**: every entry already in `.check-docs-ignore`, all of `docs/records/_frozen/**`
   (frozen provenance contains `superseded`-shaped text by design), plus an explicit P0
   allowlist of never-cullable roots — `docs/reference/spec.md`, `README.md`,
   `ARCHITECTURE.md`, `AGENTS.md`, `CLAUDE.md`, `docs/index.md`, and any path referenced by
   an open GitHub issue. Protected ⇒ `reachable_from_root = true` ⇒ never nominated.

4. **No live inbound citation.** No **live** (itself non-superseded) artifact may reference
   `E`'s ref. P0 computes this with a cheap, bounded grep of the scanned roots **plus the
   agent-instruction files** (`AGENTS.md`, `CLAUDE.md`, `docs/index.md`,
   `docs/operator/rfc-roadmap.md`) for `E`'s canonical ref. This is the clause that saves
   `docs/reference/todo.md` even if clause 2 were relaxed: it is `Status: superseded` **yet
   AGENTS.md and CLAUDE.md cite it as a live "archived pointer"** to current work — a
   superseded artifact that is still load-bearing context. A superseded decision still cited
   by a live decision/RFC/spec is likewise withheld. `reachable_from_root` records the
   boolean OR of clauses 3 and 4.

5. **`kind = branch` is EXCLUDED in P0.** P0 nominates no branch. The banked design
   branches prove the branch edge is **not Tier-1-exact**: `git branch -a` shows
   `backup/rfc-0136-design-2026-06-24` and `backup/rfc-0169-design-2026-06-24` carry **no
   `vN` in the name at all** (they are v1 banks), and **none** of the six
   `backup/rfc-{0136,0164,0165,0166,0168,0169}-*` branches has a *ratified `vN+1`* — they
   were **canceled mid-fan-out and banked as RESUME SEEDS** (`docs/operator/rfc-roadmap.md`
   lines 95–136: "design vN banked — run canceled 2026-06-24 … resume via fresh `-vN+1`").
   The RFC's "`rfc-…-design-vN` superseded by a ratified `vN+1`" edge therefore **does not
   even match the real banked set**, and naive "a later vN exists ⇒ cullable" would cull the
   project's own recovery context. The safe rule is **namespace allowlist by construction**:
   `backup/*` and `tags/graveyard/*` are protected namespaces, and a branch is never a P0
   candidate. Real branch culling defers to ≥ P2 behind that allowlist.

**`decay_score` semantics for a Tier-1-only P0 (Open design point, discharged).** P0 runs
**no decay clock**. Tier-1 candidacy is a **binary predicate** (clauses 1–4), not a
threshold on a decaying score — half-life decay is a Tier-3 reachability concept (RFC
Pillar 1) that P0 does not build. `decay_score` is written as a **constant sentinel
`0.0`** meaning "predicate-derived, not decay-derived," recorded **only** for forward-compat
with the later Tier-3 model; P0 reads nothing from it and gates nothing on it. The SPEC
must not imply a decay clock P0 does not run.

**`candidacy_state` machine for P0 (no tombstone yet).** Two states:
`nominated` (clauses 1–4 hold this tick) and `withdrawn` (a previously-`nominated` row whose
predicate has since gone false). The sweep UPSERTs idempotently: predicate true →
`candidacy_state = nominated`, `last_reinforced_at = now()`; a previously-nominated row whose
predicate is now false → `candidacy_state = withdrawn`. **A candidacy is never deleted** (the
row is the observation record). **Withdrawal is P0's tiny echo of the RFC's resurrection
property**: it fires when a banked branch is resumed, a superseded artifact is re-cited by a
live one (clause 4 flips), its successor is itself superseded (clause 2 flips), or its
`Status:` is edited back. `observed` from the SEED's example set is **not needed** in P0 —
nothing consumes a candidacy in P0, so a third "seen by an actor" state would be dead surface.

**The G1 corpus test (the falsifiable proof).** A table-driven test over the live tree
(`docs/rfcs/*`, `docs/decisions/decision-log.md`, the protected-root pathspec, and the six
`backup/*` refs) asserts the predicate produces **exactly**:

- **True positives (MUST be `nominated`):** `rfc:0027` (`superseded by RFC 0127 (D195)`),
  `rfc:0097` (`superseded by RFC 0116 / 0122 / 0124`), `rfc:0041` (`superseded by RFC 0044 /
  0057 / 0119`), `rfc:0039` (`superseded by D107 / D109 / D111 / RFC 0068`) — exactly the
  roadmap's "**Closed-out (do not pick up): superseded/deprecated 0027, 0028, 0039, 0041 …**"
  set whose successor resolves live. (Note `0028` is in the roadmap's closed-out list but its
  *front-matter* names no parseable successor, so P0 withholds it — see below — a deliberate,
  documented conservative miss, not a false negative against the predicate.)
- **The preserved set (MUST NOT be nominated — zero hits):** `docs/reference/todo.md`
  (superseded but live-cited by AGENTS.md/CLAUDE.md, and bare-no-successor — clauses 2 & 4);
  `docs/rfcs/0028` (`Status: superseded V1 foundation`, no parseable successor — clause 2);
  the six `backup/rfc-*` banked branches (clause 5 + protected namespace); the RFC-0170
  body / SEED / `workflow.json` / decision-log prose mentions (clause 1, front-matter-only);
  every `docs/records/_frozen/**` match (clause 3).

The test **refutes the whole proposal** if any true positive is missed *by the predicate's
own terms* or any preserved-set member is nominated. P0 is tuned for **zero false positives**
even at the cost of a conservative false negative (0028), because in a cull system a false
positive is the dangerous error.

---

## 4. `DecayTickSweep` — provably READ-ONLY and ERROR-ISOLATED (Claim B / Gate G2)

**Where it rides (true piggyback, no new timer).** The recovery scheduler is a single
goroutine looping `RunScheduler → SweepOnce` (`go/pkg/recovery/scheduler.go`), wired in
`go/cmd/striatumd/main.go:883–931` as `ActiveRunSweep{…}.SweepOnce`. There is already an
exact precedent for an **observational fold riding the recovery tick**: RFC 0137 Phase A
wraps `innerSweep` (`main.go:887–898`) so that **after** the recovery sweep returns, the
metrics collector's `Refresh` runs once per tick, guarded on `sweepCtx.Err() == nil`, and
**"a fold error must never fail the recovery sweep: log it and keep serving the last-good
snapshot"** — the recovery sweep's own `(result, sweepErr)` is returned unchanged. P0 adds
`DecayTickSweep{Runner: runner}.SweepOnce(sweepCtx)` **in the identical fold position and
discipline**: synchronous, after the per-run recovery loop, guarded on ctx, error **logged
and discarded**. This adds **no new goroutine, no new timer, no new scheduler** — it
genuinely piggybacks the ~60s recovery cadence (`DefaultSweepInterval = 60 * time.Second`,
`scheduler.go:10`), and because it runs in the same single-writer goroutine *after* the
recovery work, it cannot race or contend on locks with the recovery sweep.

**The isolation seam (named).** The documented daemon-suicide failure mode is **FMA-001 /
issue #451**: a panic that unwinds past the sweep loop crashes the single-writer daemon. The
existing fix is `runPerRunSweep` (`go/pkg/recovery/sweep.go:32–41`): a `defer func(){ if r :=
recover(); … }()` that converts a per-unit panic into a logged error so the loop continues.
P0 adds the **identical seam** as `runDecayTickSweep` wrapping the whole sweep body:
`recover()` → `log.Printf` loud + `debug.Stack()` → return `(nil, err)`. Combined with the
discard-the-error fold position, the safety chain is: **panic / query error / nil row / slow
query inside `DecayTickSweep` → recovered + logged inside the seam → returned as an error →
the fold discards it → the recovery sweep's own result is returned unchanged → the daemon
does not even bounce.** This is *strictly stronger* than the goroutine-level backstop
(`main.go:907–913`), which would `cancel()` the daemon for a clean restart — P0's fold never
reaches it.

**B1 — read-only.** The only write `DecayTickSweep` performs is the
`INSERT … ON CONFLICT (kind, ref) DO UPDATE` on `striatumd.cullable_entity`. It issues no
other INSERT/UPDATE/DELETE, writes no scheduler cursor, fires no event, and takes **no P0
action**: no tombstone, no deletion, no `cull.*` event, no page, no `doctor` state, no
run-admission effect. Its reads are: the explicit-column existing-state read of
`cullable_entity` (`SELECT kind, ref, candidacy_state FROM striatumd.cullable_entity` — no
`SELECT *`, C3) and the **read-only filesystem scan** of the scanned-root markdown at the
registered repository checkout path. (The markdown scan is the one new capability: read-only
local file reads, bounded to `docs/**`, within the D094 local-first boundary — no hosted,
cloud, telemetry, or transcript surface.)

**B2 — error isolation regression test.** Mirror `TestActiveRunSweepPanicDegradesRunAnd
ContinuesPanic` (`go/pkg/recovery/sweep_panic_test.go:156`): inject a panicking Tier-1 scan
into `DecayTickSweep`, assert `SweepOnce` does **not** propagate the panic and returns a
logged error; then assert the **wrapping recovery tick still returns the recovery sweep's own
result unchanged** and the daemon is not canceled. Refuted if the panic propagates past the
fold, fails the recovery sweep, or trips the goroutine backstop.

**B3 — bounded per-tick cost; `doctor` stays green.** Per-tick work is **O(corpus)**: read
the front-matter of the scanned markdown roots (169 RFCs + the decision-log + a bounded
`docs/**` set ≈ a few hundred small files), one bounded inbound-citation grep over those
roots + the agent-instruction files, one **bulk** `SELECT kind, ref, candidacy_state` of the
existing `cullable_entity` rows (bounded by the corpus, low hundreds), and one UPSERT per
*changed* row. It is **O(corpus), not O(history)** — it never scans `events`, `audit_log`, or
`verdicts`, so it does not grow with the run history. Running synchronously inside a ~60s
tick, a few-hundred-ms scan is negligible. **P0 adds no `doctor` block at all** (read-only
candidacy; the SEED forbids RED/amber), so `doctor` is byte-identical across ticks. Refuted
by a cost test showing per-tick work scaling with event/audit history, or any new `doctor`
class appearing across ticks.

---

## 5. Open Question 1 (peer vs phase) — forward-compatibility, not built (Claim D / Gate G4)

P0 is the **sweep / peer** writer **only**: the standing backlog (the roadmap's closed-out
RFCs, dead subsystems) needs the continuous sweep, and the sweep is what P0 ships. I do **not**
build the phase/toll side (a run cannot `complete` until it tombstones what it superseded or
posts an overdraft). The SPEC proves the P0 schema + writer model **do not preclude** later
adding the phase/toll writer:

- The candidacy key is `(kind, ref)` and the writer path is a plain
  `INSERT … ON CONFLICT (kind, ref) DO UPDATE`. **Nothing binds the row to the sweep as its
  author** — a future phase/toll writer (a `run.complete` handler in `go/pkg/mutations`, or
  an overdraft path) UPSERTs the *same* row shape through the *same* conflict target, and the
  two writers converge idempotently on `(kind, ref)`.
- The `candidacy_state` CHECK is **extensible**: P1 adds `tombstoned` (and later
  `resurrected`/`reaped`) as **additive** runtime-migration changes; runtime migrations are
  additive-only and the P0 `nominated`/`withdrawn` semantics are unaffected. Adding a writer
  or a state in a later phase requires **no ALTER/recreate of the P0 table** and breaks no P0
  test. Refuted (D1) if introducing the phase writer or a `tombstoned` state forces a
  destructive change to the P0 table or breaks P0's two-state machine.

---

## 6. P0 / P1+ boundary — named but NOT built, with the phase each lands in (Gate G4)

P0 builds **none** of the deletion machinery. For the build run's clarity and to bound scope,
each downstream piece is named with its phase (per the RFC's settled P0–P5 phasing):

| Deferred mechanism | Phase | One-line boundary |
| --- | --- | --- |
| `cull_tombstone` ledger + `doctor` integrity block (RED on voided/soak-expired receipt) | **P1** | the invariant before any reaper (RFC 0136 pattern: doctor seam with no executor) |
| `cull_gate` workflow shape (holder + falsifier + sealed absence-receipt) + two-phase tombstone + **manual** reap | **P2** | the reversible cull with the absence-of-use second key |
| timed reaper + soak window + resurrection-rate governor + blast-radius cap + auto-pause on doctor-red | **P3** | the only irreversible step, fail-closed |
| `accretion_ledger` + `doctor` `unrefuted_accretion` **amber** (observe) | **P4** | the counterforce, calibrated against the real +85.5K/−2.8K history |
| `STRIATUM_ACCRETION_REFUSE` throttle in `HandleRunStart` + release clearing-house + LOC→coupling meter | **P5** | wire the throttle; CULL becomes the only class allowed to start |
| Tier-2 (route/contract edges via `contracts/daemon_methods.json`); Tier-3 (Go import/call/coverage graph); Tier-4 (doc cross-links) | **≥P1** | P0 is **Tier-1 only**; richer tiers are additive sweeps later |

Non-Goals held: **no LLM-judged deadness** (P0 has no LLM in the loop — static markdown/DB
evidence only); local-first / D094 boundary unchanged (no hosted, cloud, telemetry, or
durable transcript surface — only local file reads + one runtime table).

---

## 7. The published falsifiable assertions (the hard core the falsifiers re-attack)

| # | Assertion | Refuted IF (the observation/test that kills it) |
| --- | --- | --- |
| **A1** | P0 nominates exactly `{rfc:0027, 0097, 0041, 0039}` — superseded with a parseable live successor, unprotected, uncited. | The G1 corpus test shows any of these is not `nominated` by the predicate's terms. |
| **A2** | P0 nominates **none** of the preserved set: `docs/reference/todo.md`, `rfc:0028`, the six `backup/rfc-*` branches, the RFC-0170 body/SEED/workflow.json/decision-log prose, `_frozen/**`. | Any preserved-set member is `nominated` (a false positive). |
| **A3** | The predicate reads the **front-matter `Status:` field only**, never body prose. | A body "superseded" mention (e.g. `docs/rfcs/0170-...md:95`) produces a candidacy. |
| **A4** | A bare `Status: superseded` or a non-resolving successor is **not** nominated. | `rfc:0028` (`superseded V1 foundation`) or a dangling-successor ref produces a candidacy. |
| **A5** | P0 emits **no** candidacy from `verdicts.superseded_by_decision_id` (it supersedes a review verdict, not an artifact — PC3). | A superseded verdict row (the `status_superseded_pg_test.go` fixture) produces a `cullable_entity` row. |
| **A6** | P0 emits **no** `kind = branch` candidacy. | Any branch row appears in `cullable_entity`. |
| **B1** | The sweep's only write is the `cullable_entity` UPSERT; it takes no P0 action (no tombstone/page/doctor/admission). | A statement-capture test sees any write outside `striatumd.cullable_entity` or any doctor/admission state change. |
| **B2** | A panic/error inside `DecayTickSweep` is recovered, logged, discarded; the recovery sweep's result is returned unchanged and the daemon does not bounce. | An injected panicking scan propagates past the fold, fails the recovery sweep, or cancels the daemon. |
| **B3** | Per-tick cost is O(corpus) markdown + one bulk SELECT + bounded UPSERTs; no history scan; P0 adds no doctor block. | A cost test shows scaling with event/audit history, or a new doctor class appears across ticks. |
| **C1** | Migration `0045` ships both read and write authority-inventory rows for `cullable_entity`. | `authority_inventory_static_test.go` (`TestRead/WriteAuthorityInventoryCovers…`) red-mains. |
| **C2** | `0045` GRANTs `SELECT,INSERT,UPDATE` to `striatumd_rw`, carries no owner DDL/FK (≥27 rule), at path `go/pkg/db/sql/`. | The two-role PG apply or `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL` fails, or the runtime role cannot DML the table. |
| **C3** | Every read of `cullable_entity` projects an explicit column list, never `SELECT *`. | A grep/test finds `SELECT *` against `cullable_entity` (the #614 column-scoped-grant 42501 hazard). |
| **D1** | The `(kind, ref)` row shape + ON CONFLICT upsert + extensible `candidacy_state` admit a later phase/toll writer with no schema break. | Adding a phase writer or a `tombstoned` state in P1 requires altering/recreating the P0 table or breaks P0's two-state machine. |
| **PC1/2** | The migration lives at `go/pkg/db/sql/0045_cullable_entity.sql`; `0045` is the free runtime slot. | `go/pkg/db/migrations` is used, or `0045` is already taken. |

---

## 8. Gate mapping (what the adjudicator scores)

- **G1 — Tier-1 exactness (zero false positives; resurrectable/banked set excluded):** A1–A6
  + the §3 predicate + the G1 corpus test. The load-bearing moves are PC3 (verdicts column is
  not an artifact edge), the front-matter-only clause, the live-successor clause, the
  inbound-citation clause, and the branch-namespace exclusion.
- **G2 — read-only safety (error-isolated from recovery, no action, no page):** B1–B3 + the
  `runDecayTickSweep` recover seam riding the RFC-0137 metrics-fold position.
- **G3 — substrate correctness (slot + grants + read/write inventory + no `SELECT *`):**
  C1–C3 + PC1/PC2.
- **G4 — forward-compatibility (OQ1 resolved, crisp P0/P1+ boundary):** D1 + §5 + the §6
  deferral table.

All four are argued as **proven, not merely claimed**, each against a named source site and a
refuting test/corpus row. This is the v1 claim; the falsifiers attack it, and the cycle-1
adjudicator ledger — not downstream completion — decides whether the gate clears.

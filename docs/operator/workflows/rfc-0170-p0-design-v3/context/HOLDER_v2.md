# HOLDER — RFC 0170 P0 falsifiable implementation SPEC (self-culling repository: the Tier-1 candidacy substrate) — v2 revision

author: holder-author-001

> This is the **v2 revising-holder proposal** for the RFC 0170 **P0** slice. Cycle-1
> (`run_85afe0ff`) ran the full `falsification_gate` (holder SPEC → two falsifiers →
> adjudicator) and returned **`needs_revision`** with two binding constraints: the
> adjudicator cleared **G3** (substrate correctness) and **G4** (forward-compat) but
> found **G1** (Tier-1 exactness) and **G2** (read-only safety) UNMET, each carrying a
> source-verified, unrebutted falsifier challenge. This revision **discharges G1′ and
> G2′ and weakens nothing that cleared**: §2 (substrate / migration 0045 /
> authority-inventory / no-`SELECT *`), §5 (OQ1 peer-vs-phase), and §6 (the P0/P1+
> boundary) are carried **intact** from v1; **only §3** (the Tier-1 predicate + the G1
> corpus) and **§4** (the sweep's latency boundary) are rewritten, and they are re-mapped
> in §7/§8. RFC 0170's three pillars, the shadow-first P0–P5 phasing, the four rejected
> traps, the acceptance criteria, and the five Open Questions remain **settled framing**.
> Every load-bearing claim below is a **falsifiable assertion** paired with the concrete
> test/corpus row that would refute it, anchored to a named source site **verified
> against the live tree at run base `striatum/rfc-0170-p0-design-v2`** (`466e08b9`). The
> four gates the adjudicator scores — **G1** Tier-1 exactness, **G2** read-only safety,
> **G3** substrate correctness, **G4** forward-compatibility — are mapped to those
> assertions in §8.
>
> **What changed from v1 (the two binding constraints):**
> - **G1′ (`C-G1-CITATION-EXACTNESS`)** — §3 reconciles clause 2 (a named live
>   successor) with clause 4 (no live inbound citation) into a **single mechanical,
>   greppable, no-LLM** rule, and **re-derives the corpus to equal what that predicate
>   actually produces**. The headline result: `rfc:0097` (and `rfc:0027/0039/0041`) move
>   to the **preserved set** because each carries a live active-baseline citation; the
>   genuine zero-citation true-positive set is **`{decision:D267, decision:D081}`**.
> - **G2′ (`C-G2-CULL-FOLD-DEADLINE`)** — §4 adds a **per-tick deadline** (below the 60 s
>   recovery cadence) over **both** the DB read **and** the filesystem scan, a
>   **watchdog + skip-on-overrun + compute-then-commit** policy so a blocked scan can
>   never hold the single recovery goroutine, and a **HANG regression test** distinct from
>   the existing panic test.

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
counterforce, Tiers 2–4) is **named but not built** here (§6).

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

## 3. Tier-1 detection predicate — EXACT, clause-2/clause-4 RECONCILED, zero false positives (Claim A / Gate G1) — REVISED v2 (discharges G1′)

**P0 populates candidacy for `kind ∈ {rfc, decision, doc}` from the structural
supersession convention ONLY.** It does **not** read `verdicts.superseded_by_decision_id`
for candidacy (PC3 / A5), and it does **not** populate `kind = branch` (A6). An entity `E`
(canonical ref `R`) is in state `nominated` on a tick **iff ALL five clauses hold**.

The cycle-1 falsifier proved the v1 predicate was **internally inconsistent**: clause 4
withholds any live-cited entity, yet the v1 corpus forced `rfc:0097` (superseded **and**
live-cited by RFC 0101/0103) to be nominated. The deeper defect is structural — **clause 2
requires a named live successor while clause 4 forbids any live inbound citation, and a
supersession-chain successor routinely cites the predecessor it supersedes.** Clauses 2 and
4 below are rewritten to reconcile mechanically, and the corpus is re-derived to equal
exactly what the reconciled predicate produces.

### Clause 1 — Structural deadness signal, not body prose

`E` is a tracked artifact whose **structural status field** literally begins
(case-insensitive) with `superseded` or `tombstoned`. The deadness keyword set is
**exactly `{superseded, tombstoned}`**; `deprecated` / `withdrawn` / `rejected` are **NOT**
P0 deadness signals (so `docs/rfcs/0049-…md:3` `**Status:** deprecated — overtaken by RFC
0088` is **not** a candidate — a deliberate conservative exclusion). The "structural status
field" is read per kind, handling **both** front-matter spellings found in the tree:

- **rfc** — the title-block `Status:` line in the first ~8 lines, matching **both**
  `^Status:` (bare, e.g. `docs/rfcs/0027-…md:3`) **and** `^\*\*Status:\*\*` (bold, e.g.
  `docs/rfcs/0046-…md:3`, `0048`, `0049`). Both spellings exist; the parser MUST accept
  both or it silently misses the bold-form set.
- **decision** — the **state column** (the 2nd pipe-delimited cell) of a row in
  `docs/decisions/decision-log.md`. The state column is the decision's structural status
  field — the analogue of an RFC's title-block `Status:`. Only the row's **own** state cell
  counts; "superseded by …" appearing in **another** row's description/consequences prose
  is body prose, ignored.
- **doc** — the `Status:` front-matter field of a `docs/**` artifact, same keyword rule.

**Any occurrence of "superseded" in body prose is ignored.** This single clause excludes
the largest false-positive surface: the RFC-0170 text itself (`docs/rfcs/0170-…md:3,95`),
the prose cells in `docs/decisions/decision-log.md` (D231/D037 say "superseded by …"
*about* other decisions), `docs/reference/spec.md`, `docs/reference/prd.md`, and the
SEED/`workflow.json` of this very run — none of which is a structural status field.

### Clause 2 — A parseable, LIVE successor

The deadness signal must name ≥ 1 successor ref that resolves to an existing,
**non-superseded** artifact, parsed from the `superseded by <refs>` / `SUPERSEDED by
<ref>` clause (the refs after `by`, up to the end of the status value / first sentence):

- `RFC NNNN` → `docs/rfcs/NNNN-*.md` exists **and** its own structural status does **not**
  begin `superseded`/`tombstoned`/`withdrawn`/`deprecated`.
- `D###` → a `docs/decisions/decision-log.md` row that exists with state
  `accepted`/`implemented`/`resolved`.

A **bare** `superseded`/`tombstoned` with no parseable successor, or a successor ref that
does not resolve live, is **NOT** nominated. This clause excludes `docs/reference/todo.md`
(`Status: superseded`, no successor) and `docs/rfcs/0028-…md:3` (`Status: superseded V1
foundation` — names no successor at all). "Superseded" without a live replacement is
dormancy, not death.

### Clause 4 — No live ACTIVE-BASELINE inbound citation (THE RECONCILED CLAUSE)

> *(Clauses 3 and 5 are unchanged from v1 and stated after this one; clause 4 is the
> load-bearing rewrite and is presented first.)*

`E` is **withheld** iff there exists ≥ 1 **counted** inbound citation. A grep hit for `R`
across the inbound-scan set is a **counted active-baseline citation** **iff ALL of**:

- **4a — canonical ref forms only.** `R`'s forms, word-bounded and zero-padding-normalized:
  for an `rfc` NNNN → `RFC[ -]0*NNNN\b` ∪ the file-link slug `\b0*NNNN-[a-z0-9-]+\.md\b` ∪
  `rfc:NNNN`; for a `decision` → `\bD0*NNN\b`. (Word boundaries so `0097` never matches
  `10097`.)
- **4b — inbound-scan set (fixed).** `docs/rfcs/*.md` ∪ `docs/decisions/decision-log.md` ∪
  the front-matter-carrying `docs/**` artifacts ∪ the agent-instruction files
  (`AGENTS.md`, `CLAUDE.md`, `docs/index.md`, `docs/operator/rfc-roadmap.md`) ∪
  `docs/reference/{spec.md,prd.md}`.
- **4c — live source.** The hit's file `F` must itself be **live** — `F`'s structural
  status does **not** begin `superseded`/`tombstoned`/`withdrawn`/`deprecated`, and `F` is
  not under a protected/frozen root. A dead or frozen file's citation does not keep `E`
  alive: e.g. `docs/rfcs/0028` (superseded) citing `rfc:0027`, and `docs/rfcs/0049`
  (deprecated) citing `rfc:0039`, are **not** counted.
- **4d — not self.** `F` ≠ `E`'s own file / `E`'s own decision row.
- **4e — not a supersession backref.** EXCLUDE every hit whose source file/row is one of
  `E`'s **own named successors** (the refs parsed in clause 2): a successor citing the
  predecessor it supersedes is a supersession backref, not a live dependency. EXCLUDE also
  the deadness `Status:`/state line itself. *This is the exact clause-2-vs-clause-4
  collision the cycle-1 falsifier named: `RFC 0044`/`RFC 0057` cite `RFC 0041` as the thing
  they implement; `D270` cites `D267` as the thing it deletes — all excluded as backrefs.*
- **4f — not a disposable mention.** EXCLUDE every hit whose matched **physical line** is
  disposable, defined by fixed greppable shape — the line matches **ANY** of:
  1. a Markdown **reference-link definition**: `^\s*\[[^\]]+\]:\s` (the `[RFC 0097]:
     0097-…md` link-plumbing line, e.g. `docs/rfcs/0101-…md:332`);
  2. an **index/registry row**: a hit in `docs/rfcs/README.md` lying in a **different**
     RFC's table row (the first cell links to a file other than `E`). The RFC index
     re-states every ref in every row and is a registry, not a dependency;
  3. a **closure/historical-frame line**: the matched physical line contains
     (case-insensitive) any token from the **FIXED lexicon**
     `{superseded, supersede, deprecat, obsolete, retired, tombston, graveyard, historical,
     formerly, "closed-out", "closed out", "do not pick up", "see also", "no longer"}`.

A hit surviving 4a–4f is an **active-baseline citation** — a live, non-self, non-successor,
non-disposable reference to `E` (the RFC 0101/0103 "RFC 0097 is the consumer" / "PROVEN
BASELINE" use). **≥ 1 such hit ⇒ `E` withheld (preserved).** `reachable_from_root` records
the boolean OR of clauses 3 and 4.

**Why this is mechanical (the falsifier-1 re-attack surface, answered).** The rule is a
**pure grep + a fixed token lexicon + a fixed link/row regex** — it never judges a
citation's "importance." The unit of classification is the matched **physical line**, and
in every disposable case in this tree the ref and its closure token co-occur on **one**
line: the roadmap's `Closed-out (do not pick up): superseded/deprecated 0027, 0028, 0039,
0041 …` line (`docs/operator/rfc-roadmap.md`), the README per-RFC status rows, and every
`superseded by …` prose mention. **When `R` and a closure token fall on different physical
lines, the rule counts the hit (active-baseline) and WITHHOLDS** — it errs toward
*preserve*, never toward *nominate*. That conservative bias is the correct direction for a
cull system (a false negative is a missed cleanup; a false positive is a wrongful cull),
and it means two implementers running the same grep + lexicon read every corpus row
identically.

### Clause 3 — Not in the protected root set (unchanged from v1)

`E`'s path must not match the **protected-root pathspec**: every entry already in
`.check-docs-ignore`, all of `docs/records/_frozen/**` (frozen provenance contains
`superseded`-shaped text by design), plus an explicit P0 allowlist of never-cullable roots
— `docs/reference/spec.md`, `README.md`, `ARCHITECTURE.md`, `AGENTS.md`, `CLAUDE.md`,
`docs/index.md`, and any path referenced by an open GitHub issue. Protected ⇒
`reachable_from_root = true` ⇒ never nominated.

### Clause 5 — `kind = branch` is EXCLUDED in P0 (unchanged from v1)

P0 nominates no branch. The banked design branches prove the branch edge is **not
Tier-1-exact**: `git branch -a` shows `backup/rfc-0136-design-2026-06-24` and the others
carry **no ratified `vN+1`** — they were **canceled mid-fan-out and banked as RESUME
SEEDS** (`docs/operator/rfc-roadmap.md`: "design vN banked — run canceled 2026-06-24 …
resume via fresh `-vN+1`"). The RFC's "`rfc-…-design-vN` superseded by a ratified `vN+1`"
edge therefore **does not even match the real banked set**, and naive "a later vN exists ⇒
cullable" would cull the project's own recovery context. The safe rule is **namespace
allowlist by construction**: `backup/*` and `tags/graveyard/*` are protected namespaces, and
a branch is never a P0 candidate. Real branch culling defers to ≥ P2 behind that allowlist.

### `decay_score` and `candidacy_state` (unchanged from v1)

**`decay_score` for a Tier-1-only P0.** P0 runs **no decay clock**. Tier-1 candidacy is a
**binary predicate** (clauses 1–5), not a threshold on a decaying score — half-life decay
is a Tier-3 reachability concept (RFC Pillar 1) that P0 does not build. `decay_score` is
written as a **constant sentinel `0.0`** meaning "predicate-derived, not decay-derived,"
recorded **only** for forward-compat with the later Tier-3 model; P0 reads nothing from it
and gates nothing on it.

**`candidacy_state` machine for P0 (no tombstone yet).** Two states: `nominated` (clauses
1–5 hold this tick) and `withdrawn` (a previously-`nominated` row whose predicate has since
gone false). The sweep UPSERTs idempotently: predicate true → `candidacy_state =
nominated`, `last_reinforced_at = now()`; a previously-nominated row whose predicate is now
false → `candidacy_state = withdrawn`. **A candidacy is never deleted** (the row is the
observation record). **Withdrawal is P0's tiny echo of the RFC's resurrection property**: it
fires when a banked branch is resumed, a superseded artifact is re-cited by a live one
(clause 4 flips), its successor is itself superseded (clause 2 flips), or its status is
edited back.

### The re-derived G1 corpus (the falsifiable proof — discharges G1′)

A table-driven test over the live tree (`docs/rfcs/*`, `docs/decisions/decision-log.md`,
the protected-root pathspec, the agent-instruction files, and the six `backup/*` refs)
asserts the reconciled predicate produces **exactly**:

**True positives (MUST be `nominated`) — the genuine zero-active-baseline-citation
superseded set:**

- **`decision:D267`** — `docs/decisions/decision-log.md` state column = `superseded`;
  description begins `SUPERSEDED by D270` → successor **D270** (`implemented`, live →
  clause 2 ✓). `R`'s only two inbound hits in the whole scan set are **its own row**
  (4d) and **D270's row** (4e — D270 is the named successor; the line also carries the
  closure token `supersede`, 4f). ⇒ **ZERO** counted inbound citations (clause 4 ✓); not
  protected (clause 3 ✓). **NOMINATED.**
- **`decision:D081`** — state column = `superseded`; consequences name `Superseded by
  D087/D094/D104 for current production behavior` → successors **D094/D104** (`accepted`,
  live → clause 2 ✓). `R`'s **only** inbound hit in the entire scan set is **its own row**
  (4d). ⇒ **ZERO** counted inbound citations (clause 4 ✓). **NOMINATED.**

Both true positives are **independently confirmable** (a falsifier greps `R` and finds only
self + a named-successor backref) and both exercise **`kind = decision`** candidacy.

**The RFC supersession corpus yields ZERO true positives — the load-bearing re-derivation
that discharges G1′** (the cycle-1 falsifier-1 finding, generalized): every
front-matter-superseded RFC moves to the **preserved set**.

- **`rfc:0028`** — `Status: superseded V1 foundation`, names **no** parseable successor →
  withheld by **clause 2**.
- **`rfc:0097`** — `Status: superseded by RFC 0116 / 0122 / 0124`; live **active-baseline**
  cited by **RFC 0101** (`Status: umbrella-of-record`, live: "RFC 0097 is the consumer",
  body lines `130–131,267,281–284`) and **RFC 0103** (`accepted`, live: W6 / "RFC 0097
  self-hosting … proven baseline", `43,213–219`) — neither a named successor nor disposable
  → withheld by **clause 4**. *(The exact cycle-1 contradiction, now resolved: `rfc:0097`
  is PRESERVED, as `C-G1`'s `verification.gate` mandates.)*
- **`rfc:0027`** — `Status: superseded by RFC 0127 (D195)`; live active-baseline cited by
  **RFC 0031** (`accepted`, `227,283`) and **RFC 0118** (`accepted/implemented`, `28`:
  "Build on shipped mechanisms (… RFC 0027 sealed provenance …)") → withheld by **clause 4**.
- **`rfc:0039`** — `Status: superseded by D107 / D109 / D111 / RFC 0068`; `RFC 0068` is a
  named successor (4e, excluded), but live active-baseline cited by **RFC 0043**
  (`accepted/implemented`, `478`: "RFC 0039 (Go daemon) gets a clean scope") and **RFC
  0040** (`accepted`) → withheld by **clause 4**.
- **`rfc:0041`** — `Status: superseded by RFC 0044 / 0057 / 0119`; `RFC 0044`/`RFC 0057`
  are named successors (4e, excluded), but **RFC 0058** (`implemented`, live, **not** a
  successor) cites it active-baseline (`288`: "The augmentation-not-dependency rule (RFC
  0041) is preserved") → withheld by **clause 4**.

**The preserved set (MUST NOT be nominated — zero hits), explicitly including `rfc:0097`
per `C-G1`:**

- `rfc:0097`, `rfc:0027`, `rfc:0039`, `rfc:0041` (clause 4); `rfc:0028` (clause 2);
- the **still-cited superseded decisions** `D006`/`D007`/`D008`/`D009`/`D013`/`D018`/
  `D084`/`D105`/`D125`/`D174` — each carries ≥ 1 live active-baseline citation (e.g. `D008`
  in RFC 0072 `355`: "D008 (append-only artifacts): preserved"; `D125` in
  `docs/reference/spec.md:1237` as an active `decision_id`) → clause 4;
- `docs/reference/todo.md` (`Status: superseded`, bare-no-successor — clause 2 — **and**
  live-cited by `AGENTS.md`/`CLAUDE.md` as an "archived pointer" — clause 4);
- the six `backup/rfc-{0136,0164,0165,0166,0168,0169}-*` banked branches (clause 5 +
  protected namespace);
- the RFC-0170 body / this run's SEED / `workflow.json` / decision-log **prose** mentions
  (clause 1 — structural-status-only);
- every `docs/records/_frozen/**` match (clause 3).

The test **refutes the whole proposal** if either true positive (`D267`, `D081`) is shown
to carry a counted inbound citation, **or** any preserved-set member is nominated, **or**
any corpus row's verdict flips under two implementers applying the stated grep + lexicon.
P0 is tuned for **zero false positives** even at the cost of conservative false negatives
(the four still-cited superseded RFCs, `0028`, and the ten still-cited superseded
decisions), because in a cull system a false positive is the dangerous error.

**Reconciling with RFC Acceptance Criterion 1 (so the revision does not silently weaken the
RFC).** RFC 0170 AC1 says "a superseded RFC/decision with a ratified successor is
auto-nominated … with **zero false positives** (Tier-1 is exact)." The reconciled predicate
**refines** AC1's "auto-nominated" with the exactness AC1 itself demands: *superseded + live
successor + **no live active-baseline citation***. On today's tree the genuinely-dead set
satisfying all three is `{D267, D081}`; the still-cited superseded artifacts are correctly
withheld as load-bearing context. The predicate therefore both **fires** (a non-empty
true-positive set) and is **exact** (zero false positives) — both halves of AC1 — and, per
the RFC's own provenance note, the committed design supersedes the sketch's looser Tier-1
bullet where they differ.

---

## 4. `DecayTickSweep` — provably READ-ONLY, ERROR-ISOLATED, and LATENCY-BOUNDED (Claim B / Gate G2) — REVISED v2 (discharges G2′)

**Where it rides (true piggyback, no new timer).** The recovery scheduler is a single
goroutine looping `RunScheduler → SweepOnce` (`go/pkg/recovery/scheduler.go`), wired in
`go/cmd/striatumd/main.go:883–897` as `ActiveRunSweep{…}.SweepOnce`. There is already an
exact precedent for an **observational fold riding the recovery tick**: RFC 0137 Phase A
wraps `innerSweep` (`main.go:889–897`) so that **after** the recovery sweep returns, the
metrics collector's `Refresh` runs once per tick, guarded on `sweepCtx.Err() == nil`
(`main.go:891`), and **"a fold error must never fail the recovery sweep: log it and keep
serving the last-good snapshot"** (`main.go:880–894`) — the recovery sweep's own `(result,
sweepErr)` is returned unchanged (`main.go:896`). P0 adds `DecayTickSweep{Runner:
runner}.SweepOnce` **in the identical fold position and discipline**: synchronous, after the
per-run recovery loop, error **logged and discarded**. This adds **no new goroutine, no new
timer, no new scheduler** — it piggybacks the ~60 s recovery cadence
(`DefaultSweepInterval = 60 * time.Second`, `scheduler.go:10`).

**The gap cycle-1 falsifier-2 named (verified at source).** The `sweepCtx` the fold
receives (`main.go:889`) is the **daemon root ctx** — it carries **no per-tick deadline**,
and the `sweepCtx.Err() == nil` guard (`main.go:891`) only detects daemon shutdown, not a
slow tick. The scheduler re-enters `wait(ctx, interval)` (`scheduler.go:69,79`) **only after
`SweepOnce` returns** (`scheduler.go:56`), so a `DecayTickSweep` scan that **neither panics
nor returns** — an unbounded `docs/**` walk, the inbound-citation grep, or a lock/IO-waiting
query — **holds the single recovery goroutine indefinitely**. The panic seam
(`sweep.go:32–41`) is **unwind-only**: it converts a panic to an error but has **no
wall-clock bound**, so it never fires for work that simply blocks. The DB
`statement_timeout` default equals the cadence (`60000`, `connection.go:289–290`), and the
filesystem scan has **no timeout at all**. After `doctorRecoveryCursorWedgedAfter = 5 *
time.Minute` (`doctor.go:16`) the recovery cursor goes quiet (`doctor.go:383`), doctor
emits `recovery_sweep_cursor_wedged` (`doctor.go:448,460`) and drives `ok:false`. So the v1
sweep proved **panic** isolation and **returned-error** isolation but **not a latency/stall
bound** — exactly `C-G2`.

### The panic + returned-error isolation seam (cleared in cycle-1, restated)

P0 adds `runDecayTickSweep` wrapping the whole sweep body, mirroring `runPerRunSweep`
(`sweep.go:32–41`): `recover()` → `log.Printf` loud + `debug.Stack()` → return `(nil,
err)`. Combined with the discard-the-error fold position, the panic/error chain is: **panic
/ query error / nil row inside `DecayTickSweep` → recovered + logged inside the seam →
returned as an error → the fold discards it → the recovery sweep's own result is returned
unchanged → the daemon does not even bounce** (strictly stronger than the goroutine-level
backstop at `main.go:907–913`, which would `cancel()` for a clean restart — P0's fold never
reaches it).

### The latency/stall boundary (NEW — discharges G2′)

The fix is a **per-tick deadline strictly below the recovery cadence**, applied to **both**
halves of the scan, plus a **watchdog + skip-on-overrun + compute-then-commit** policy so a
blocked scan can never hold the recovery goroutine and can never leave a torn write.

- **L1 — a per-tick deadline.** The cull fold opens `cullCtx, cancel :=
  context.WithTimeout(sweepCtx, DefaultCullFoldTimeout)` (and `defer cancel()`), with a new
  constant **`DefaultCullFoldTimeout = 10 * time.Second`** in `go/pkg/recovery` — strictly
  below `DefaultSweepInterval = 60 * time.Second` (`scheduler.go:10`). A static test asserts
  `DefaultCullFoldTimeout < DefaultSweepInterval` so the relationship cannot silently drift.
  `cullCtx` bounds both halves below.
- **L2 — the DB read bounded below cadence.** Every `cullable_entity` read runs under
  `cullCtx` (pgx cancels the in-flight query when `cullCtx` fires) **and** the cull read
  issues `SET LOCAL statement_timeout = '10000'` at the head of its read transaction — a
  sub-cadence **server-side** backstop that overrides the connection default of `60000` ms
  (= cadence, `connection.go:289–290`). The DB half can therefore never consume a full
  recovery period, even if the Go-side cancel is missed.
- **L3 — the filesystem scan bounded by TWO independent guards** (because
  `context.WithTimeout` alone does **not** abort a blocking syscall already in flight — the
  exact falsifier-2 re-attack):
  - *Cooperative.* The `fs.WalkDir` callback returns `cullCtx.Err()` (abandoning the walk)
    once the deadline passes, and each file is read through a **bounded reader over only the
    front-matter head** (a fixed small byte cap covering the first ~8 lines), so the walk
    advances in O(1)-bounded steps and stops promptly at the deadline.
  - *Watchdog (the airtight guard).* The **read-only** scan (fs walk + DB read → an
    in-memory candidacy delta) runs in a **child goroutine**; the cull fold `select`s on
    `{scanDone, cullCtx.Done()}`. On `cullCtx.Done()` the fold **returns immediately**
    (logged) and the recovery goroutine proceeds to `wait()`/the next tick — so even a scan
    stuck in a blocking syscall can **never** hold the recovery goroutine. A single-slot
    "scan in flight" guard means that while a prior tick's scan is still stuck, every later
    tick **skips** the cull (skip-on-overrun, logged) rather than stacking scans; a
    permanently-hung scan leaks **exactly one** bounded, logged goroutine and never touches
    recovery.
- **L4 — skip-on-overrun with NO torn write (compute-then-commit).** The child goroutine
  does **only reads** and returns the full candidacy delta over a channel; it **never
  writes** (single-writer preserved). The **write** phase — the `cullable_entity` UPSERTs —
  runs back in the recovery goroutine in **one transaction**, and **only** when the delta
  arrived before `cullCtx` fired. If the deadline fires first, the fold performs **zero**
  writes that tick: the write phase never begins, the in-memory delta is discarded, and the
  next tick recomputes from scratch. This is the precise answer to "how is a partial scan
  discarded without a partial/torn write" — the read result is dropped **in memory**, and
  the write is **all-or-nothing**, only ever after a complete in-deadline scan.

**B1 — read-only.** The only write `DecayTickSweep` performs is the `INSERT … ON CONFLICT
(kind, ref) DO UPDATE` on `striatumd.cullable_entity` (in the compute-then-commit write
phase, L4). It issues no other INSERT/UPDATE/DELETE, writes no scheduler cursor, fires no
event, and takes **no P0 action**: no tombstone, no deletion, no `cull.*` event, no page, no
`doctor` state, no run-admission effect. Its reads are the explicit-column existing-state
read (`SELECT kind, ref, candidacy_state FROM striatumd.cullable_entity` — no `SELECT *`,
C3) and the **read-only, deadline-bounded filesystem scan** of the scanned-root markdown at
the registered repository checkout path, within the D094 local-first boundary (no hosted,
cloud, telemetry, or transcript surface).

**B2 — panic isolation regression test (cleared, retained).** Mirror
`TestActiveRunSweepPanicDegradesRunAndContinues` (`go/pkg/recovery/sweep_panic_test.go:156`):
inject a panicking Tier-1 scan into `DecayTickSweep`, assert `SweepOnce` does **not**
propagate the panic and returns a logged error; then assert the **wrapping recovery tick
still returns the recovery sweep's own result unchanged** and the daemon is not canceled.

**B3 — bounded per-tick cost; `doctor` stays green.** Per-tick work is **O(corpus)**: read
the front-matter head of the scanned markdown roots (169 RFCs + the decision-log + a bounded
`docs/**` set), one bounded inbound-citation grep over those roots + the agent-instruction
files, one **bulk** `SELECT kind, ref, candidacy_state` of the existing rows, and one UPSERT
per *changed* row — **O(corpus), not O(history)** (it never scans `events`, `audit_log`, or
`verdicts`). **P0 adds no `doctor` block at all** (read-only candidacy; the SEED forbids
RED/amber), so `doctor` is byte-identical across ticks. Refuted by a cost test showing
per-tick work scaling with event/audit history, or any new `doctor` class appearing across
ticks.

**B4 — latency assertion (NEW).** A blocked/slow `DecayTickSweep` scan **cannot delay the
next recovery tick or turn doctor unhealthy**: the per-tick deadline (L1) + watchdog (L3)
return control to the recovery goroutine within `DefaultCullFoldTimeout` (≤ 10 s ≪ 60 s),
the fold's timeout error is logged + **discarded** (the metrics-fold discipline,
`main.go:892–894`), the recovery sweep's own `(result, sweepErr)` returns unchanged,
`wait(interval)` re-enters on schedule (`scheduler.go:69,79`), the recovery cursor keeps
advancing, and the 5-minute `doctorRecoveryCursorWedgedAfter` window (`doctor.go:16`) is
never reached. **Refuted IF** a blocked cull scan delays the next recovery tick beyond
`DefaultCullFoldTimeout`, holds the recovery goroutine, produces a partial `cullable_entity`
write, or trips `recovery_sweep_cursor_wedged`.

**B5 — HANG regression test (NEW — the binding `C-G2` verification.gate).** In
`go/pkg/recovery`, mirroring `TestActiveRunSweepPanicDegradesRunAndContinues`
(`sweep_panic_test.go:156`) but for a **BLOCKING, non-returning** scan (NOT a panic):
inject a `DecayTickSweep` whose scan blocks past `doctorRecoveryCursorWedgedAfter` (5 min —
e.g. blocks on a channel never closed within the test); wire it into the cull fold; drive
the scheduler with a fake `Wait` (immediate) + `MaxSweeps = 2`. Assert **(i)** the recovery
`SweepOnce` returns its own result on each tick (the cull fold's deadline error is logged +
discarded, never propagated); **(ii)** the **next** recovery tick still runs
(`RunScheduler` reports `Sweeps == 2` — the goroutine was not held); **(iii)** a `doctor`
evaluation over the resulting recovery cursor does **NOT** emit `recovery_sweep_cursor_wedged`
and stays `ok:true` (the cursor advanced on schedule, so the quiet-window check at
`doctor.go:383,448,460` never fires); and **(iv)** no `cullable_entity` write was performed
for the timed-out tick (L4 — no torn write). The test must **FAIL with the deadline
removed** (a control configuration with `DefaultCullFoldTimeout` disabled hangs / `Sweeps`
stays `1`). This is **distinct from B2**: B2 proves the **panic** path (the recover seam);
B5 proves the **deadline** path (a scan that never returns **and** never panics), which the
recover seam structurally cannot catch — exactly the gap the cycle-1 ledger named.

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
| **A1** | P0 nominates exactly `{decision:D267, decision:D081}` — superseded decisions with a parseable live successor, unprotected, with **zero active-baseline inbound citation**. | The G1 corpus test shows either is not `nominated`, or shows a counted inbound citation on either. |
| **A2** | P0 nominates **none** of the preserved set: `rfc:0097`, `rfc:0027/0039/0041` (clause 4), `rfc:0028` (clause 2), the ten still-cited superseded decisions, `docs/reference/todo.md`, the six `backup/rfc-*` branches, the RFC-0170 body/SEED/workflow.json/decision-log prose, `_frozen/**`. | Any preserved-set member is `nominated` (a false positive). |
| **A3** | The predicate reads the **structural status field only** (RFC title-block `Status:`/`**Status:**`; decision-row state column; doc front-matter), never body prose or another row's cells. | A body "superseded" mention (e.g. `docs/rfcs/0170-…md:95`) or a prose "superseded by" in another decision row produces a candidacy. |
| **A4** | A bare `superseded`/`tombstoned` or a non-resolving successor is **not** nominated. | `rfc:0028` (`superseded V1 foundation`) or `docs/reference/todo.md`, or a dangling-successor ref, produces a candidacy. |
| **A4′** | Clauses 2 and 4 are reconciled **mechanically**: a named-successor backref (`RFC 0044`→`0041`, `D270`→`D267`) and a disposable/closure-frame line (roadmap closed-out row, README status rows, reference-link defs) do **not** count; only a live, non-successor, non-disposable active-baseline reference withholds. | Two implementers applying the stated grep + fixed lexicon disagree on any corpus row; or a named-successor backref / closure-frame line is counted as a live citation; or `rfc:0097` is nominated. |
| **A5** | P0 emits **no** candidacy from `verdicts.superseded_by_decision_id` (it supersedes a review verdict, not an artifact — PC3). | A superseded verdict row (the `status_superseded_pg_test.go` fixture) produces a `cullable_entity` row. |
| **A6** | P0 emits **no** `kind = branch` candidacy. | Any branch row appears in `cullable_entity`. |
| **B1** | The sweep's only write is the `cullable_entity` UPSERT (in the L4 write phase); it takes no P0 action (no tombstone/page/doctor/admission). | A statement-capture test sees any write outside `striatumd.cullable_entity` or any doctor/admission state change. |
| **B2** | A panic inside `DecayTickSweep` is recovered, logged, discarded; the recovery sweep's result is returned unchanged and the daemon does not bounce. | An injected panicking scan propagates past the fold, fails the recovery sweep, or cancels the daemon. |
| **B3** | Per-tick cost is O(corpus) markdown + one bulk SELECT + bounded UPSERTs; no history scan; P0 adds no doctor block. | A cost test shows scaling with event/audit history, or a new doctor class appears across ticks. |
| **B4** | A blocked/slow scan **cannot delay the next recovery tick or wedge doctor**: per-tick deadline (`DefaultCullFoldTimeout < DefaultSweepInterval`) + watchdog + skip-on-overrun + compute-then-commit. | A blocked cull scan delays the next recovery tick past the deadline, holds the recovery goroutine, produces a partial `cullable_entity` write, or trips `recovery_sweep_cursor_wedged`. |
| **B5** | The **HANG** regression test passes — a blocking, non-returning scan still lets the next recovery tick run (`Sweeps==2`) and keeps doctor `ok:true` with no torn write — and **fails with the deadline removed**. | The HANG test cannot be made to pass, or passes even with `DefaultCullFoldTimeout` disabled (i.e. it only re-proves the panic path). |
| **C1** | Migration `0045` ships both read and write authority-inventory rows for `cullable_entity`. | `authority_inventory_static_test.go` (`TestRead/WriteAuthorityInventoryCovers…`) red-mains. |
| **C2** | `0045` GRANTs `SELECT,INSERT,UPDATE` to `striatumd_rw`, carries no owner DDL/FK (≥27 rule), at path `go/pkg/db/sql/`. | The two-role PG apply or `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL` fails, or the runtime role cannot DML the table. |
| **C3** | Every read of `cullable_entity` projects an explicit column list, never `SELECT *`. | A grep/test finds `SELECT *` against `cullable_entity` (the #614 column-scoped-grant 42501 hazard). |
| **D1** | The `(kind, ref)` row shape + ON CONFLICT upsert + extensible `candidacy_state` admit a later phase/toll writer with no schema break. | Adding a phase writer or a `tombstoned` state in P1 requires altering/recreating the P0 table or breaks P0's two-state machine. |
| **PC1/2** | The migration lives at `go/pkg/db/sql/0045_cullable_entity.sql`; `0045` is the free runtime slot. | `go/pkg/db/migrations` is used, or `0045` is already taken. |

---

## 8. Gate mapping (what the adjudicator scores)

- **G1 — Tier-1 exactness (zero false positives; clause-2/clause-4 reconciled;
  resurrectable/banked set excluded):** A1–A6 (incl. **A4′**) + the §3 reconciled predicate
  + the re-derived G1 corpus. The load-bearing moves are PC3 (verdicts column is not an
  artifact edge), the structural-status-only clause (handling `Status:`/`**Status:**`/state
  column), the live-successor clause, **the reconciled active-baseline inbound-citation rule
  (4a–4f: live-source, non-self, non-successor-backref, non-disposable — a pure grep + fixed
  lexicon)**, and the branch-namespace exclusion. The cycle-1 contradiction is resolved:
  `rfc:0097` (and `rfc:0027/0039/0041`) are **preserved**, and the genuine true-positive set
  is `{D267, D081}`.
- **G2 — read-only safety (error-isolated AND latency-bounded; no action, no page):** B1–B5
  + the `runDecayTickSweep` recover seam riding the RFC-0137 metrics-fold position + **the
  per-tick cull-fold deadline (L1), the DB + filesystem dual bound (L2/L3), the
  watchdog/skip-on-overrun/compute-then-commit policy (L3/L4), and the HANG regression test
  (B5)**. The cycle-1 latency gap is closed: a blocked scan cannot stall recovery or wedge
  doctor.
- **G3 — substrate correctness (slot + grants + read/write inventory + no `SELECT *`):**
  C1–C3 + PC1/PC2. *(Cleared in cycle-1; §2 carried intact, not reopened.)*
- **G4 — forward-compatibility (OQ1 resolved, crisp P0/P1+ boundary):** D1 + §5 + the §6
  deferral table. *(Cleared in cycle-1; §5/§6 carried intact, not reopened.)*

All four are argued as **proven, not merely claimed**, each against a named source site
verified against the live tree at run base `striatum/rfc-0170-p0-design-v2` (`466e08b9`) and
a refuting test/corpus row. This is the v2 revised claim; the falsifiers re-attack the
**reconciled predicate + re-derived corpus** (G1′) and the **latency bound + HANG test**
(G2′), and the cycle-2 adjudicator ledger — not downstream completion — decides whether the
gate clears.

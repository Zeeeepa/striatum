# HOLDER — RFC 0170 P0 falsifiable implementation SPEC (self-culling repository: the Tier-1 candidacy substrate) — v4 revision

author: holder-author-001

> This is the **v4 revising-holder proposal** for the RFC 0170 **P0** slice. The design has
> converged across three rounds; the v3 revision round (`run_01b5b2d0…`, branch
> `striatum/rfc-0170-p0-design-v3`) ran the full `falsification_gate` and returned
> **`needs_revision`** at the exhausted gate: the adjudicator **discharged the hardest parts**
> (the `kind=decision` successor-extraction rule is now pure-greppable and correct; the v2
> source-false `last_sweep_at` doctor claim is retired; the latency machinery is credited) and
> re-confirmed **G3** (substrate) and **G4** (forward-compat) un-regressed, but routed **two
> pinpoint residuals** — **G1‴** and **G2‴** — to a fresh `-v4`. This revision **discharges
> only those two residuals and weakens nothing that was credited or cleared.** Carried
> **intact** from v3: §2 (substrate / migration `0045` / both authority-inventory rows /
> `striatumd_rw` GRANT with no owner DDL/FK ≥27 / no `SELECT *`), the **clause-4
> active-baseline inbound-citation rule** that preserves `rfc:0097`/`0027`/`0039`/`0041` (and
> `D174` via live RFC 0109), the **`kind=decision` successor-extraction rule** (own-row cell 3
> → cell 5 by precedence, fixed regex, sentence boundary, multi-ref split) yielding
> `D267→{D270}` and `D081→{D087,D094,D104}`, the **entire latency machinery**
> (`DefaultCullFoldTimeout = 10s` over **both** the DB read and the filesystem scan, the
> deadline-bounded child goroutine, single-in-flight skip-on-overrun, and L4
> compute-then-commit), §5 (OQ1 peer-vs-phase), and §6 (the P0/P1+ boundary). **Only two
> things change:** §3 clause 3's protected pathspec becomes **fully static and tree-local** (it
> drops the v3 leftover dynamic "any path referenced by an open GitHub issue" bullet, so **no
> path classification depends on external state**); and §4 closes the cursor-refresh-timing
> carrier **by construction** — the cull fold is **moved off the scheduler's `wait`-gating
> path** (its read+write run on a detached, single-in-flight goroutine the recovery loop never
> blocks on), so a hung fold **cannot postpone the next `scheduler_cursors` refresh**, making
> the no-worse-than-baseline doctor property hold trivially. Every load-bearing claim below is a
> **falsifiable assertion** paired with the concrete test/corpus row that would refute it,
> anchored to a named source site **verified against the live tree at the v4 run base
> `striatum/rfc-0170-p0-design-v4`**. The four gates the adjudicator scores — **G1** Tier-1
> exactness, **G2** read-only safety, **G3** substrate correctness, **G4**
> forward-compatibility — are mapped to those assertions in §8.
>
> **What changed from v3 (the two residual constraints — and ONLY these):**
> - **G1‴ (`C-G1-DECISION-SUCCESSOR-EXACTNESS`, pathspec residual)** — §3 clause 3 makes the
>   cull-specific protected pathspec **fully static and tree-local.** v3 already excluded
>   `docs/rfcs/` and `docs/decisions/` from the pathspec, but it **retained one dynamic
>   bullet** — "any path referenced by an open GitHub issue" — which (i) is a GitHub-API
>   dependency, not a tree-local greppable cull predicate (a path's protection could flip with
>   external issue-body text), and (ii) **over-exposed** the candidacy roots: in live issue
>   state open #585 references `docs/decisions/decision-log.md` and open #615 references
>   `docs/rfcs/0170-…md`, so the active decision log was protected RIGHT NOW and `D267`/`D081`
>   got `reachable_from_root=true` and failed clause 3 before nomination — making the published
>   true-positive set non-derivable (a two-implementer split internal to the SPEC). v4
>   **removes that bullet from the cull predicate entirely** (any issue-linked preservation is
>   kept as an **operator advisory OUTSIDE `cullable_entity`**, never a candidacy input). The
>   pathspec is now exactly a **checked-in static list** with no external-state term; the corpus
>   is re-derived with a fixture proving `D267`/`D081` are **NOT** protected while the
>   frozen/provenance roots stay protected.
> - **G2‴ (`C-G2-HANG-DOCTOR-SEMANTICS`, refresh-timing residual)** — §4 keeps the credited
>   deadline + child-goroutine + skip-on-overrun + L4 compute-then-commit **unchanged in
>   substance** and closes the carrier the v3 A/B control left open. The carrier (source-true):
>   `doctor` reads the **persisted** `scheduler_cursors.last_result_json` (`doctor.go:383-390`),
>   refreshed only by the next recovery sweep's `upsertSchedulerCursor`
>   (`sweep.go:246-263,284-358`); the cull fold ran **synchronously inside `SweepOnce`** and
>   `RunScheduler` calls `wait(interval)` only **after** `SweepOnce` returns
>   (`scheduler.go:55-80`), so a hung fold burning `DefaultCullFoldTimeout` **delayed the next
>   cursor refresh** and opened a same-wall-clock window where doctor read a staler (wedged)
>   cursor under the cull variant near the 5m threshold. v4 takes the SEED's **preferred
>   option (b): a design change that removes the carrier** — the cull fold no longer blocks the
>   wrapped `SweepOnce` return; it runs **off the `wait`-gating path** on a detached
>   single-in-flight goroutine, so the wrapped `SweepOnce` returns within O(1) of `innerSweep`
>   (the cursor refresh, which `innerSweep` already performed) regardless of scan duration.
>   **A hung fold therefore cannot defer the next refresh by construction**, so the
>   no-worse-than-baseline property holds trivially — proven by the existing `Sweeps==2` /
>   no-torn-write A/B control **plus** a new test that the next cursor refresh is not deferred
>   by a hung fold, and a static "fold position does O(1) work" assertion.

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

## 2. Substrate — migration `0045_cullable_entity.sql` (Claim C: the #614 trap, head-on) — CARRIED INTACT FROM v2 (G3 cleared)

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

## 3. Tier-1 detection predicate — EXACT, clause-2/clause-4 RECONCILED, zero false positives (Claim A / Gate G1) — REVISED v4 (discharges G1‴)

**P0 populates candidacy for `kind ∈ {rfc, decision, doc}` from the structural
supersession convention ONLY.** It does **not** read `verdicts.superseded_by_decision_id`
for candidacy (PC3 / A5), and it does **not** populate `kind = branch` (A6). An entity `E`
(canonical ref `R`) is in state `nominated` on a tick **iff ALL five clauses hold**.

The clause-4 active-baseline inbound-citation rule (4a–4f) is **carried intact** — it is the
credited fix for the cycle-1 `rfc:0097` false positive, re-confirmed by the v3 falsifier to
preserve `rfc:0097`/`0027`/`0039`/`0041` (and `D174` via live RFC 0109) and to count no live
citation on `D267`/`D081`. The **`kind=decision` successor-extraction rule in clause 2 is also
carried intact** — the v3 falsifier ran it by hand and the adjudicator DISCHARGED it
(`D267→{D270}`, `D081→{D087,D094,D104}`, bare state-cell-only `superseded` withheld), so it is
**not reopened** here. **v4 changes exactly one thing in this section**: clause 3's protected
pathspec becomes **fully static and tree-local** — it drops the single v3 leftover dynamic
"any path referenced by an open GitHub issue" bullet so no path classification depends on
external (open-issue) state (the v3 falsifier_1 G1‴ residual). The corpus is then re-stated
under the now fully-static pathspec.

### Clause 1 — Structural deadness signal, not body prose (unchanged from v2)

`E` is a tracked artifact whose **structural status field** literally begins
(case-insensitive, after trimming surrounding whitespace) with `superseded` or
`tombstoned`. The deadness keyword set is **exactly `{superseded, tombstoned}`**;
`deprecated` / `withdrawn` / `rejected` are **NOT** P0 deadness signals (so
`docs/rfcs/0049-…md:3` `**Status:** deprecated — overtaken by RFC 0088` is **not** a
candidate — a deliberate conservative exclusion). The "structural status field" is read per
kind, handling **both** front-matter spellings found in the tree:

- **rfc** — the title-block `Status:` line in the first ~8 lines, matching **both**
  `^Status:` (bare, e.g. `docs/rfcs/0027-…md:3`) **and** `^\*\*Status:\*\*` (bold, e.g.
  `docs/rfcs/0046-…md:3`, `0048`, `0049`). Both spellings exist; the parser MUST accept
  both or it silently misses the bold-form set.
- **decision** — the **state column**, defined as the **2nd pipe-delimited cell** of a row
  in `docs/decisions/decision-log.md`. The table header (`docs/decisions/decision-log.md:34`)
  is `| ID | Status | Decision | Reason | Consequences | Revisit Trigger |`, so the six
  own-row cells are, by position after the leading pipe: **cell 1 = ID**, **cell 2 = Status
  (the state column)**, **cell 3 = Decision (description)**, **cell 4 = Reason**, **cell 5 =
  Consequences**, **cell 6 = Revisit Trigger**. Clause 1 reads **only cell 2**. A row whose
  cell-2 value begins (case-insensitive) `superseded`/`tombstoned` is structurally dead;
  `superseded by …` text appearing in **another** row's cells, or in this row's non-state
  cells, is **not** a clause-1 deadness signal (it is handled — for this row's own successor —
  by clause 2 below; for other rows it is ignored body prose).
- **doc** — the `Status:` front-matter field of a `docs/**` artifact, same keyword rule.

**Any occurrence of "superseded" in body prose is ignored by clause 1.** This single clause
excludes the largest false-positive surface: the RFC-0170 text itself (`docs/rfcs/0170-…md:3,95`),
the prose cells in `docs/decisions/decision-log.md` (e.g. D270/D231/D037 say "supersede(d)"
*about* other decisions in their description/consequences cells), `docs/reference/spec.md`,
`docs/reference/prd.md`, and the SEED/`workflow.json` of this very run — none of which is a
structural status field.

### Clause 2 — A parseable, LIVE successor — with an explicit `kind=decision` extraction rule (DISCHARGED in v3; CARRIED INTACT in v4 — not reopened)

The deadness signal must name ≥ 1 successor ref that resolves to an existing,
**non-superseded** artifact. **Where the successor ref is parsed from depends on the kind**,
and for `kind=decision` v2 left this ambiguous: clause 1 fixes the *state column* as the
structural status field, but for `D267`/`D081` the state column is the **bare** keyword
`superseded` with the actual successor named in a different own-row cell, so v2's "parse the
successor from the status value" yielded an empty successor set under a literal reading
(true-positive set empty, A1 false) yet "scan the other cells" under a loose reading
(nominates both) — a two-implementer split on the only two rows the corpus depends on. v3
fixes the extraction with a **pure regex over named own-row cells**, no LLM, no judgment.

**rfc / doc successor extraction (unchanged from v2).** Parse the refs after `by` in the
`superseded by <refs>` / `SUPERSEDED by <ref>` clause **of the structural status field
itself** (the `Status:` line), up to the end of the status value / first sentence.

**decision successor extraction (NEW — the mechanical rule).** For a `kind=decision` row
whose state cell (cell 2) is structurally dead per clause 1, the successor refs are
extracted **only** from the row's **own** Decision cell (cell 3) and Consequences cell
(cell 5), by the following fixed procedure:

1. **Allowed successor-source cells, and ONLY these:** cell 3 (Decision/description) and
   cell 5 (Consequences). Cells 1 (ID), 2 (Status), 4 (Reason), and 6 (Revisit Trigger) are
   **never** successor sources, and **no other row's cells are ever a successor source.**
   (Rationale: across the live log, decisions place their own successor in the description
   sentence that opens the row — `D267` — or in the consequences sentence that records what
   replaced them — `D081`; Reason explains *why* and Revisit Trigger names *re-open*
   conditions, neither of which is the authoritative successor.)
2. **Precedence (deterministic single source):** scan cell 3 first, then cell 5. The
   **first** of the two cells that yields a `superseded by`-clause match supplies the entire
   successor set; the other cell is **not** consulted. (So a row can never draw successors
   from two cells at once.)
3. **The match (pure regex):** within the chosen cell, the first case-insensitive occurrence
   of `\bsupersed(?:ed|es) by\s+<reflist>`, where `<reflist>` is one or more refs each of
   the form `D0*[0-9]+` or `RFC[ -]0*[0-9]+`, separated by `/`, `,`, or ` and `. The
   `<reflist>` run **terminates at the sentence/clause boundary** — the first character/token
   that is neither a ref nor a ref-separator (e.g. a `.`, a `:`, or any non-ref word such as
   `for`). Nothing after that boundary is a successor.
4. **Multi-ref split:** the captured `<reflist>` is split on `/`, `,`, and ` and ` into
   individual refs, each zero-padding-normalized: `D087/D094/D104` → `{D087, D094, D104}`;
   `D270` → `{D270}`.

**Worked against the live tree (the only two corpus-bearing rows):**

- **D267** (`docs/decisions/decision-log.md:38`): cell 2 = bare `superseded` (clause 1 ✓);
  cell 3 = `SUPERSEDED by D270. This row formerly kept …` → first `superseded by`-clause is
  in cell 3, `<reflist>` = `D270`, boundary at the `.` → successor set `{D270}`.
- **D081** (`docs/decisions/decision-log.md:220`): cell 2 = bare `superseded` (clause 1 ✓);
  cell 3 = `Accept RFC 0028 V1 …` (no `superseded by` clause → fall through to cell 5);
  cell 5 = `… Superseded by D087/D094/D104 for current production behavior:` → `<reflist>` =
  `D087/D094/D104`, boundary at the word `for` → successor set `{D087, D094, D104}`.

**Liveness test (clause 2 proper, unchanged):** ≥ 1 extracted successor ref must resolve to
an existing artifact whose own structural status does **not** begin
`superseded`/`tombstoned`/`withdrawn`/`deprecated`:

- `RFC NNNN` → `docs/rfcs/NNNN-*.md` exists and is live.
- `D###` → a `docs/decisions/decision-log.md` row that exists with state cell
  `accepted`/`implemented`/`resolved`.

`D270`'s state cell is `implemented` (`docs/decisions/decision-log.md:35`, live) → D267
clause 2 ✓. `D094`/`D104` are accepted/live (the credited v2 reading) → D081 clause 2 ✓.

A **bare** `superseded`/`tombstoned` decision with **no `superseded by`-clause in cell 3 or
cell 5**, or a successor ref that does not resolve live, is **NOT** nominated (clause 2
fails). **Real-tree withheld instance: `D084`** (`docs/decisions/decision-log.md:217`): cell
2 = `superseded`; cell 3 = `Plan a Go-language core for the daemon. D105 temporarily narrowed
this … but D107 restores …`; cell 5 = `RFC 0030's wire framing … RFC 0068 now owns the
concrete Go production port …`. **Neither cell 3 nor cell 5 contains a `superseded by`
clause** — the prose mentions `D105`/`D107`/`RFC 0068` but never in `superseded by <ref>`
form — so the rule extracts **no** successor and D084 is **WITHHELD by clause 2.** This is
the required "bare state-cell-only `superseded` decision with no own-row successor prose is
withheld" case, grounded in the live tree (and a constructed negative-control fixture row —
state cell `superseded`, cells 3/5 with zero `superseded by` text — is the table-driven twin
that withholds independently of tree drift). The same clause also excludes
`docs/reference/todo.md` (`Status: superseded`, no successor) and `docs/rfcs/0028-…md:3`
(`Status: superseded V1 foundation` — names no successor) for the rfc/doc kinds.

### Clause 4 — No live ACTIVE-BASELINE inbound citation (THE RECONCILED CLAUSE — carried intact from v2)

> *(Clauses 3 and 5 are stated after this one; clause 4 is the load-bearing v2 rewrite and is
> presented first. It is CARRIED INTACT — the v2 falsifier credited it and the SEED forbids
> reopening it.)*

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

**Why this is mechanical (unchanged from v2).** The rule is a **pure grep + a fixed token
lexicon + a fixed link/row regex** — it never judges a citation's "importance." The unit of
classification is the matched **physical line**, and in every disposable case in this tree the
ref and its closure token co-occur on **one** line: the roadmap's `Closed-out (do not pick up):
superseded/deprecated 0027, 0028, 0039, 0041 …` line (`docs/operator/rfc-roadmap.md`), the
README per-RFC status rows, and every `superseded by …` prose mention. **When `R` and a
closure token fall on different physical lines, the rule counts the hit (active-baseline) and
WITHHOLDS** — it errs toward *preserve*, never toward *nominate*. That conservative bias is the
correct direction for a cull system (a false negative is a missed cleanup; a false positive is
a wrongful cull), and it means two implementers running the same grep + lexicon read every
corpus row identically.

### Clause 3 — Not in the cull-specific protected root set (REVISED v4 — discharges G1‴: fully static, tree-local)

`E`'s path must not match the **cull-specific protected pathspec**, which v4 makes a **fully
static, tree-local, checked-in manifest** — every term is a literal path/glob resolvable by
`git ls-files` + glob matching against the working tree, with **no external-state term**
(no GitHub API, no network, no open-issue lookup).

*History (why this term-by-term matters).* v2 imported "every entry already in
`.check-docs-ignore`", but that live file is a *docs-link-checker* ignore list whose **`:3` is
`docs/rfcs/` wholesale** and **`:2` is `docs/records/_frozen/`** (verified:
`.check-docs-ignore:1-10`); importing it verbatim protects **every RFC** before clause 2/4 ever
runs, negating §2's `kind='rfc'` candidacy. v3 fixed that by replacing the import with a
cull-specific pathspec that excludes the two actively-scanned candidacy roots `docs/rfcs/` and
`docs/decisions/`, **but left one dynamic catch-all bullet** — "any path referenced by an open
GitHub issue." The v3 falsifier_1 stood on it: it (i) is a GitHub-API dependency, not a
tree-local greppable cull predicate — a path's protection could flip with external issue-body
text outside the tree — and (ii) **over-exposes** the candidacy roots, because in live issue
state open **#585** references `docs/decisions/decision-log.md` and open **#615** references
`docs/rfcs/0170-…md`, so the active decision log would be protected RIGHT NOW and `D267`
(`decision-log.md:38`) / `D081` (`decision-log.md:220`) would get `reachable_from_root=true`
and fail clause 3 **before** nomination — making the published true-positive set non-derivable
and contradicting this clause's own "NOT in the pathspec" claim. **v4 removes that bullet from
the cull predicate entirely.** The cull-specific protected pathspec is now exactly this static
list (and nothing else):

- `docs/records/_frozen/**` — frozen provenance (contains `superseded`-shaped text by design).
- `docs/research/**`, `docs/dogfood/**`, `docs/handoffs/**`, `docs/operator/plans/**`,
  `docs/operator/workflows/**`, `examples/**`, `prompts/**` — provenance / design-run scaffold /
  fixture roots that carry `superseded`-shaped text by design (these mirror `.check-docs-ignore`
  minus `docs/rfcs/`).
- the explicit never-cullable root files: `docs/reference/spec.md`, `docs/reference/prd.md`,
  `README.md`, `ARCHITECTURE.md`, `AGENTS.md`, `CLAUDE.md`, `docs/index.md`,
  `docs/operator/rfc-roadmap.md`.

There is **no other entry** — in particular **no open-issue / external-state term.** Any
issue-linked preservation an operator may want ("don't cull a path an open issue still
references") is kept **strictly as an operator advisory OUTSIDE `cullable_entity`** — it may
annotate a review surface, but it is **never a candidacy input** and never sets
`reachable_from_root`, so the predicate the build implements (and that two implementers run)
stays a pure tree-local computation.

Equivalently: the cull-specific pathspec is **`.check-docs-ignore` with `docs/rfcs/`
subtracted and the never-cullable root files added** (`docs/decisions/` was never in
`.check-docs-ignore`, so no subtraction is needed there). It is checked into the cull source as
its own constant manifest, decoupled from `.check-docs-ignore` so a future edit to that
docs-link file (added for an unrelated link-check reason) can never silently flip the cull
candidacy surface. A path matching the pathspec ⇒ `reachable_from_root = true` ⇒ never
nominated. **`docs/rfcs/*.md` and `docs/decisions/decision-log.md` are NOT in the pathspec, so
RFCs and decisions remain eligible candidates withheld only by clauses 1/2/4 — the candidacy
surface is neither dead by construction nor dependent on any external (open-issue) state.**

### Clause 5 — `kind = branch` is EXCLUDED in P0 (unchanged from v2)

P0 nominates no branch. The banked design branches prove the branch edge is **not
Tier-1-exact**: `git branch -a` shows `backup/rfc-0136-design-2026-06-24` and the others
carry **no ratified `vN+1`** — they were **canceled mid-fan-out and banked as RESUME
SEEDS** (`docs/operator/rfc-roadmap.md`: "design vN banked — run canceled 2026-06-24 …
resume via fresh `-vN+1`"). The RFC's "`rfc-…-design-vN` superseded by a ratified `vN+1`"
edge therefore **does not even match the real banked set**, and naive "a later vN exists ⇒
cullable" would cull the project's own recovery context. The safe rule is **namespace
allowlist by construction**: `backup/*` and `tags/graveyard/*` are protected namespaces, and
a branch is never a P0 candidate. Real branch culling defers to ≥ P2 behind that allowlist.

### `decay_score` and `candidacy_state` (unchanged from v2)

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

### The re-stated G1 corpus under the fully-static pathspec rules (the falsifiable proof — discharges G1‴)

A table-driven test over the live tree (`docs/rfcs/*`, `docs/decisions/decision-log.md`,
the **fully-static** cull-specific protected pathspec, the agent-instruction files, and the
six `backup/*` refs) asserts the predicate produces **exactly** the set below. A dedicated
**protected-pathspec fixture** runs alongside it and proves the clause-3 classification is
tree-local: `docs/decisions/decision-log.md` (rows `D267`/`D081`) and `docs/rfcs/*.md`
classify **`reachable_from_root = false` by clause 3** (eligible — they are not in the static
manifest), while `docs/records/_frozen/<any>.md`, `docs/operator/workflows/<any>.md`,
`examples/<any>.md`, and the named root files (`spec.md`/`README.md`/`AGENTS.md`/…) classify
**`true`** (protected) — and the fixture passes **with no network and no GitHub access**,
asserting the classifier consults **only** the working tree (it would refute the proposal if
the classifier issued any external call or if its output changed when open-issue state changed):

**True positives (MUST be `nominated`) — the genuine zero-active-baseline-citation
superseded set, now mechanically derivable end-to-end:**

- **`decision:D267`** (`docs/decisions/decision-log.md:38`) — cell 2 = `superseded`
  (clause 1 ✓); clause-2 decision rule extracts `{D270}` from cell 3 (`SUPERSEDED by D270.`),
  `D270` = `implemented`/live (clause 2 ✓); not in the cull-specific pathspec (clause 3 ✓);
  `R`'s only two inbound hits in the whole scan set are **its own row** (4d) and **D270's
  row** (4e — D270 is the named successor; the line also carries the closure token
  `supersede`, 4f) → **ZERO** counted inbound citations (clause 4 ✓). **NOMINATED.**
- **`decision:D081`** (`docs/decisions/decision-log.md:220`) — cell 2 = `superseded`
  (clause 1 ✓); clause-2 decision rule falls through cell 3 (no `superseded by`) to cell 5
  and extracts `{D087, D094, D104}` (`Superseded by D087/D094/D104 for …`), `D094`/`D104`
  accepted/live (clause 2 ✓); not in the cull-specific pathspec (clause 3 ✓); `R`'s **only**
  inbound hit in the entire scan set is **its own row** (4d) → **ZERO** counted inbound
  citations (clause 4 ✓). **NOMINATED.**

Both true positives are now **independently confirmable end-to-end** (a falsifier reads cell 2
for deadness, the named cell 3/cell 5 for the successor under the fixed regex, then greps `R`
and finds only self + a named-successor backref) and both exercise **`kind = decision`**
candidacy. **Required withheld table-driven case: `decision:D084`** (`…:217`) — cell 2 =
`superseded`, but no `superseded by`-clause in cell 3 or cell 5 → **WITHHELD by clause 2**
(plus a constructed bare-`superseded` fixture row with zero `superseded by` text → withheld
independently).

**The RFC supersession corpus yields ZERO true positives — RFCs are now ELIGIBLE (clause 3 no
longer protects `docs/rfcs/`) and every front-matter-superseded RFC moves to the PRESERVED set
by clause 2 or clause 4** (the credited v2 derivation, restated; this is now internally
consistent — under v2's literal clause-3 import these would have been "preserved" by blanket
protection, contradicting §3's own model):

- **`rfc:0028`** — `Status: superseded V1 foundation`, names **no** parseable successor →
  withheld by **clause 2**.
- **`rfc:0097`** — `Status: superseded by RFC 0116 / 0122 / 0124`; live **active-baseline**
  cited by **RFC 0101** (`Status: umbrella-of-record`, live: "RFC 0097 is the consumer",
  body lines `130–131,267,281–284`) and **RFC 0103** (`accepted`, live: W6 / "RFC 0097
  self-hosting … proven baseline", `43,213–219`) — neither a named successor nor disposable
  → withheld by **clause 4**. *(The cycle-1 contradiction stays resolved: `rfc:0097` is
  PRESERVED, as `C-G1`'s `verification.gate` mandates.)*
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
  `D084`/`D105`/`D125`/`D174` — each is withheld: `D084` by **clause 2** (no own-row
  `superseded by` clause, above) and the rest by **clause 4** (≥ 1 live active-baseline
  citation — e.g. `D008` in RFC 0072 `355`: "D008 (append-only artifacts): preserved";
  `D125` in `docs/reference/spec.md:1237` as an active `decision_id`). *(The clause-2
  decision rule only ever moves decisions toward MORE conservative withholding; it never
  promotes a still-cited decision to nominated, because clause 4 still withholds any
  live-cited row regardless of its successor parse.)*
- `docs/reference/todo.md` (`Status: superseded`, bare-no-successor — clause 2 — **and**
  live-cited by `AGENTS.md`/`CLAUDE.md` as an "archived pointer" — clause 4);
- the six `backup/rfc-{0136,0164,0165,0166,0168,0169}-*` banked branches (clause 5 +
  protected namespace);
- the RFC-0170 body / this run's SEED / `workflow.json` / decision-log **prose** mentions
  (clause 1 — structural-status-only);
- every `docs/records/_frozen/**` match (clause 3).

The test **refutes the whole proposal** if either true positive (`D267`, `D081`) is shown
to carry a counted inbound citation or to be non-derivable under the stated cell rule, **or**
any preserved-set member is nominated, **or** the required withheld case (`D084` /
the bare-`superseded` fixture) nominates, **or** any corpus row's verdict flips under two
implementers applying the stated field rule + grep + lexicon + cull-specific pathspec. P0 is
tuned for **zero false positives** even at the cost of conservative false negatives (the four
still-cited superseded RFCs, `0028`, and the ten still-cited/parse-withheld superseded
decisions), because in a cull system a false positive is the dangerous error.

**Reconciling with RFC Acceptance Criterion 1 (so the revision does not silently weaken the
RFC).** RFC 0170 AC1 says "a superseded RFC/decision with a ratified successor is
auto-nominated … with **zero false positives** (Tier-1 is exact)." The predicate **refines**
AC1's "auto-nominated" with the exactness AC1 itself demands: *superseded + live successor +
no live active-baseline citation*. On today's tree the genuinely-dead set satisfying all three
is `{D267, D081}`; the still-cited superseded artifacts are correctly withheld as load-bearing
context. The predicate therefore both **fires** (a non-empty true-positive set) and is
**exact** (zero false positives) — both halves of AC1 — and, per the RFC's own provenance
note, the committed design supersedes the sketch's looser Tier-1 bullet where they differ.

---

## 4. `DecayTickSweep` — provably READ-ONLY, ERROR-ISOLATED, and LATENCY-BOUNDED (Claim B / Gate G2) — latency machinery CARRIED INTACT; fold MOVED OFF the `wait`-gating path in v4 (discharges G2‴)

**Where it rides (true piggyback, no new timer — but OFF the `wait`-gating path).** The
recovery scheduler is a single goroutine looping `RunScheduler → SweepOnce`
(`go/pkg/recovery/scheduler.go:36–84`), wired in `go/cmd/striatumd/main.go:883–897` as
`ActiveRunSweep{…}.SweepOnce`. There is already an exact precedent for an **observational fold
riding the recovery tick**: RFC 0137 Phase A wraps `innerSweep` (`main.go:889–897`) so that
**after** the recovery sweep returns, the metrics collector's `Refresh` runs once per tick,
guarded on `sweepCtx.Err() == nil` (`main.go:891`), and **"a fold error must never fail the
recovery sweep: log it and keep serving the last-good snapshot"** (`main.go:880–894`) — the
recovery sweep's own `(result, sweepErr)` is returned unchanged (`main.go:896`). P0 adds the
cull fold **in that same position (after `innerSweep`) and discipline (error logged +
discarded, recovery result returned unchanged)** — so it observes once per ~60 s recovery
cadence (`DefaultSweepInterval = 60 * time.Second`, `scheduler.go:10`) — **but it departs from
the metrics fold in exactly the way G2‴ requires: it is NOT synchronous-blocking.** The fold
position does only **O(1)** work — it launches (or, if a prior scan is still in flight, skips)
a **detached single-in-flight scan goroutine** and returns immediately; it never waits on the
scan. Two facts make this the airtight fix: (1) `innerSweep` is the writer that **refreshes the
recovery cursor** (`ActiveRunSweep.SweepOnce → upsertSchedulerCursor`, `sweep.go:114,246–263`)
and it runs to completion and returns **before** the fold position, so the *current* tick's
cursor refresh is never gated by the fold; and (2) because the fold position returns within
O(1) of `innerSweep` regardless of scan duration, the wrapped `SweepOnce` return — and
therefore `wait(interval)` (`scheduler.go:69,79`) and the **next** tick's `innerSweep` cursor
refresh — are **never** postponed by the cull scan, hung or not. The detached goroutine is the
**same deadline-bounded child goroutine** the v3 latency machinery credited (L1–L4 below); v4
only moves the fold's *join* off the recovery goroutine so the loop never blocks on it.

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

### The panic + returned-error isolation seam (cleared in cycle-1, restated for the detached goroutine)

The cull scan+write body runs under `runDecayTickSweep`, **the top frame of the detached
goroutine**, mirroring `runPerRunSweep` (`sweep.go:32–41`): `recover()` → `log.Printf` loud +
`debug.Stack()` → the goroutine exits cleanly (on panic: drop the in-memory delta, perform no
write). Because the scan runs on its **own** goroutine, the seam **must** live there — a panic
in a detached goroutine that is not recovered *in that goroutine* would propagate to the
runtime and crash the daemon. With the seam, the chain is: **panic / query error / nil row
inside the detached cull scan → recovered + logged inside the seam → the goroutine exits,
dropping the delta and writing nothing → the recovery goroutine (which already returned the
recovery sweep's own result and moved on) is never touched → the daemon does not even bounce**
(strictly stronger than both the v3 return-an-error-to-the-fold chain and the goroutine-level
backstop at `main.go:907–913`, which would `cancel()` for a clean restart — the cull goroutine
never reaches it). The fold position itself still cannot panic the loop: it only does the O(1)
slot-check + spawn, which it performs under the same discard discipline.

### The latency/stall boundary (machinery CREDITED and carried intact; v4 relocates the fold OFF the `wait`-gating path)

The credited fix is a **per-tick deadline strictly below the recovery cadence**, applied to
**both** halves of the scan, plus a **deadline-bounded child goroutine + skip-on-overrun +
compute-then-commit** policy so a blocked scan can never leave a torn write. **v4 keeps every
one of those credited properties** (the `DefaultCullFoldTimeout = 10s` over both halves, the
single-in-flight skip-on-overrun, and L4 compute-then-commit / zero-writes-on-timeout) and
makes exactly one structural change to discharge G2‴: the recovery goroutine **no longer
joins/waits on the scan goroutine at all**. In v3 the recovery goroutine `select`ed on
`{scanDone, cullCtx.Done()}` and was released within the deadline (≤ 10 s) — correct for
"never *held* indefinitely," but the v3 falsifier_2 showed those ≤ 10 s still **delayed the
next cursor refresh** (the wrapped `SweepOnce` return, hence `wait()`, hence the next
`innerSweep`). v4 removes that carrier by construction: the fold position **returns
immediately** after launching the detached scan, so the recovery goroutine waits **zero** on
the scan — a strict strengthening of the credited watchdog guarantee, not a weakening. L1, L2,
and the cooperative L3 bound are unchanged; L3's watchdog and L4's write phase are re-expressed
below for the detached goroutine.

- **L1 — a per-tick deadline (owned by the detached goroutine).** The **detached scan
  goroutine** opens `cullCtx, cancel := context.WithTimeout(sweepCtx, DefaultCullFoldTimeout)`
  and owns `defer cancel()` **inside that goroutine** — `sweepCtx` is the long-lived daemon
  root ctx (it carries no per-tick deadline; see the gap analysis above), so `cullCtx`'s
  lifetime is the **scan's** lifetime and is **not** cut short by the fold position returning
  in O(1) (the fold position must NOT hold the cancel, or it would kill the scan it just
  launched). The new constant **`DefaultCullFoldTimeout = 10 * time.Second`** in
  `go/pkg/recovery` is strictly below `DefaultSweepInterval = 60 * time.Second`
  (`scheduler.go:10`); a static test asserts `DefaultCullFoldTimeout < DefaultSweepInterval` so
  the relationship cannot silently drift. `cullCtx` bounds both halves below and guarantees the
  detached goroutine self-terminates within `DefaultCullFoldTimeout`.
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
  - *Detached watchdog (the airtight guard — v4: off the `wait`-gating path).* The
    **read-only** scan (fs walk + DB read → an in-memory candidacy delta) runs in a
    **detached child goroutine** that the recovery goroutine **never joins or waits on.** The
    fold position checks a single-slot "scan in flight" guard: if free, it launches the scan
    goroutine and **returns immediately (O(1))**; if a prior tick's scan is still in flight, it
    **skips** the cull (skip-on-overrun, logged) and returns immediately. The detached scan
    **self-bounds** via `cullCtx` (deadline + the cooperative `fs.WalkDir`/bounded-reader
    abort above), so even a scan stuck in a blocking syscall is abandoned by its own goroutine
    and **never** holds the recovery goroutine — and, because the fold position never waited on
    it, **never delays the wrapped `SweepOnce` return, `wait(interval)`, or the next tick's
    cursor refresh** (the G2‴ carrier, removed by construction). A permanently-hung scan leaks
    **exactly one** bounded, logged goroutine (the single-slot guard prevents stacking) and
    touches neither recovery nor the scheduler loop timing.
- **L4 — skip-on-overrun with NO torn write (compute-then-commit, on the detached
  goroutine).** The detached goroutine first computes the **full candidacy delta in memory**
  (reads only), and then — **only** when that scan completed before `cullCtx` fired — performs
  the `cullable_entity` UPSERTs as **one all-or-nothing transaction** on the **same detached
  goroutine** (v4: the write no longer runs "back in the recovery goroutine," since the
  recovery goroutine never waits for the delta — that is precisely what moves the fold off the
  `wait`-gating path). If the deadline fires first, the goroutine performs **zero** writes that
  tick: the write phase never begins, the in-memory delta is discarded, and the next tick
  recomputes from scratch. The read result is dropped **in memory** and the write is
  **all-or-nothing, only ever after a complete in-deadline scan** — unchanged in substance from
  the credited v3 compute-then-commit property.

  **Why the off-recovery-goroutine write introduces no single-writer hazard (the v4
  justification).** The daemon is **already a concurrent-writer-over-a-pool** architecture, not
  a literal single-goroutine writer: `runner` is a `db.Pool` wrapping `*pgxpool.Pool`
  (`go/pkg/db/connection.go:138–148,315`), and the **MCP request handlers** (`mutations`), the
  **recovery scheduler**, and the **auto_spawn scheduler** already write to PostgreSQL
  concurrently over that one shared pool (`main.go:172` shares `runner` into
  `startMCPHTTPServer`, `startRecoveryScheduler`, and `startAutoSpawnScheduler`). The detached
  cull write is just one more pooled writer, and it is **confined to `striatumd.cullable_entity`
  alone** — a table **no other writer touches** (the recovery sweep writes only
  `striatumd.scheduler_cursors`; `mutations` write run/job/queue/event rows; none write
  `cullable_entity`), so it can never contend on a row or table with any other writer. It
  acquires its **own short-lived pooled connection** (never sharing the recovery goroutine's
  connection handle), and the **single-in-flight guard (L3)** means at most one cull writer ever
  exists, so two cull scans can never produce a torn delta or a torn write. "Single-writer" is
  therefore preserved where it actually binds — **one writer per `cullable_entity` row, one
  all-or-nothing transaction** — without the cull write sitting on the recovery goroutine's
  critical section.

**B1 — read-only.** The only write `DecayTickSweep` performs is the `INSERT … ON CONFLICT
(kind, ref) DO UPDATE` on `striatumd.cullable_entity` (in the compute-then-commit write
phase, L4). It issues no other INSERT/UPDATE/DELETE, writes no scheduler cursor, fires no
event, and takes **no P0 action**: no tombstone, no deletion, no `cull.*` event, no page, no
`doctor` state, no run-admission effect. Its reads are the explicit-column existing-state
read (`SELECT kind, ref, candidacy_state FROM striatumd.cullable_entity` — no `SELECT *`,
C3) and the **read-only, deadline-bounded filesystem scan** of the scanned-root markdown at
the registered repository checkout path, within the D094 local-first boundary (no hosted,
cloud, telemetry, or transcript surface). **Crucially for B5, on BOTH axes: (value) the cull fold writes nothing
that feeds the recovery cursor — it touches neither `striatumd.scheduler_cursors` nor any
lane/run row, so it cannot advance OR retard `last_lane_advanced_at`, `claimable_job_count`,
`started_at`, or `created_at`, the only inputs to the recovery-cursor wedge (`doctor.go`); and
(timing) because the fold is off the `wait`-gating path (L3), it cannot postpone the next
recovery sweep that REFRESHES the persisted cursor, so it cannot make doctor read a staler
cursor than the no-cull baseline either. The cull fold affects neither the cursor's written
values nor their refresh timing.**

**B2 — panic isolation regression test (cleared, retained; v4 — panic on the detached
goroutine).** Mirror `TestActiveRunSweepPanicDegradesRunAndContinues`
(`go/pkg/recovery/sweep_panic_test.go:156`): inject a panicking Tier-1 scan, assert the
**detached cull goroutine recovers + logs the panic and exits without writing** (the panic
never escapes the cull goroutine to the process), the wrapping recovery tick returns the
recovery sweep's own result unchanged, and the daemon is not canceled. Refuted if an injected
panicking scan crashes the daemon, reaches the recovery goroutine, or leaves a partial write.

**B3 — bounded per-tick cost; `doctor` stays green.** Per-tick work is **O(corpus)**: read
the front-matter head of the scanned markdown roots (169 RFCs + the decision-log + a bounded
`docs/**` set), one bounded inbound-citation grep over those roots + the agent-instruction
files, one **bulk** `SELECT kind, ref, candidacy_state` of the existing rows, and one UPSERT
per *changed* row — **O(corpus), not O(history)** (it never scans `events`, `audit_log`, or
`verdicts`). **P0 adds no `doctor` block at all** (read-only candidacy; the SEED forbids
RED/amber), so `doctor` is byte-identical across ticks. Refuted by a cost test showing
per-tick work scaling with event/audit history, or any new `doctor` class appearing across
ticks.

**B4 — latency assertion (STRENGTHENED in v4 — zero delay, not bounded delay).** A
blocked/slow `DecayTickSweep` scan **cannot delay the next recovery tick at all** — not even
by `DefaultCullFoldTimeout`. Because the fold position launches the detached scan and
**returns in O(1)** (L3), the recovery goroutine **never waits on the scan**: the recovery
sweep's own `(result, sweepErr)` returns unchanged, `wait(interval)` re-enters on the same
schedule as a no-cull baseline (`scheduler.go:69,79`), and the next tick's `innerSweep`
(hence the next cursor refresh, `sweep.go:114,246–263`) runs on the baseline schedule. The
scan's own deadline (L1) + cooperative/detached watchdog (L3) still bound the **detached
goroutine** to ≤ `DefaultCullFoldTimeout` so it self-terminates and leaks at most one bounded
goroutine. **Refuted IF** a blocked cull scan delays the next recovery tick by **any** amount
(including ≤ `DefaultCullFoldTimeout`), holds the recovery goroutine, or produces a partial
`cullable_entity` write. *(The "and does not make doctor worse than baseline" property follows
directly — see the A/B control + refresh-not-deferred test in B5.)*

**B5 — HANG regression test, A/B no-cull control + refresh-not-deferred test (REVISED v4 — the
binding `C-G2` verification.gate).** Two carried facts stay credited. First, the v2
`last_sweep_at` mechanism is and remains **retired**: `go/pkg/reads/doctor.go`
`recoveryCursorQuietSince` (469-480) keys the quiet window on
`last_lane_advanced_at → started_at → created_at` (with a `quiet_since` test-fallback),
**never `last_sweep_at`**; the wedge fires when `claimable_job_count > 0` and that lane-quiet
source is older than `doctorRecoveryCursorWedgedAfter = 5m` (`doctor.go:383,438–445,447–460`),
and the existing test `doctor_recovery_cursor_test.go:33–73` proves a **fresh** `last_sweep_at`
(`now-1m`) with `claimable_job_count=2` and a 10-minute-stale `last_lane_advanced_at` still
returns `ok:false`. So the binding property is **not** an absolute `ok:true` but **"the cull
fold does not make doctor worse than a no-cull baseline."** Second, the v3 A/B control already
proves the **value** half of that property (`Sweeps==2`, recovery result preserved, no
`cullable_entity` write on the timed-out tick, identical wedge inputs — B1).

The v3 falsifier_2 residual was the **timing** half: doctor reads the **persisted**
`scheduler_cursors.last_result_json` (`doctor.go:383-390`), refreshed only by the next recovery
sweep's `upsertSchedulerCursor` (`sweep.go:246-263,284-358`); under the v3 synchronous-join
fold a hung scan still delayed the **next** refresh by up to `DefaultCullFoldTimeout`, opening a
same-wall-clock window where doctor read a staler cursor under A than B near the 5m threshold.
v4 takes the SEED's **preferred option (b)**: the design change (fold off the `wait`-gating
path, L3) **removes the carrier by construction**, so the timing half now holds trivially — and
is **proven by a dedicated refresh-not-deferred test** below, on top of the carried value
control. (No `doctor.go` semantics change; the existing `doctor_recovery_cursor_test.go`
coverage is untouched.)

In `go/pkg/recovery`, mirroring `TestActiveRunSweepPanicDegradesRunAndContinues`
(`sweep_panic_test.go:156`) but for a **BLOCKING, non-returning** scan (NOT a panic):

- **Setup.** Construct two `SweepOnce` configurations over the *same* fake recovery sweep +
  `MaxSweeps = 2`, plus a **fake clock** `clk` and a `Wait` that **advances `clk` by the
  interval** (modeling `SweepOnce` duration + `wait(interval)` instead of the v3 immediate
  no-op `Wait`). The fake recovery sweep (`innerSweep`) is **instrumented to record
  `clk.Now()` at the instant it refreshes the recovery cursor** (its `upsertSchedulerCursor`
  call) on each tick:
  - **A (cull variant, fold OFF the `wait`-gating path):** the recovery sweep wrapped by the
    v4 cull fold, whose detached scan **blocks indefinitely** (e.g. on a channel never closed
    within the test) — past `doctorRecoveryCursorWedgedAfter` (5 min).
  - **B (no-cull control):** the *identical* recovery sweep with **no cull fold attached**,
    everything else byte-identical.
- **Assertions.**
  1. **Recovery result preserved (A == B).** A's wrapped `SweepOnce` returns the recovery
     sweep's own result on each tick (the cull fold's error is logged + discarded, never
     propagated) — identical to B.
  2. **Loop runs → same `Sweeps` (A == B).** `RunScheduler` reports **`Sweeps == 2` for A**,
     equal to control B, **even though A's scan is still blocked** — proving the off-path fold
     never holds the recovery goroutine and the next recovery tick ran.
  3. **Next cursor refresh NOT deferred (the v4 binding timing property — the assertion v3
     lacked).** The recorded `clk` instant of **tick 2's** cursor refresh is **equal** under A
     and B. Because the fold position returns within O(1) of `innerSweep` regardless of the
     blocked scan (L3), tick 2's `innerSweep` — hence the next `scheduler_cursors` refresh —
     fires at the **same wall-clock instant** under the hung-cull variant as under the no-cull
     baseline. So at every instant near the 5 m threshold, doctor reads a cursor **no staler**
     under A than under B.
  4. **No-worse-than-baseline doctor — value half (B1).** The recovery-cursor wedge inputs
     `{claimable_job_count, last_lane_advanced_at, started_at, created_at, run_state}` seen by
     `doctorRecoverySweepCursor` are **identical** under A and B (A's captured write-set
     contains **no** write outside `striatumd.cullable_entity`, and on the timed-out/blocked
     tick **no write at all**). A `go/pkg/reads`-side companion seeds the same
     `scheduler_cursors`/`runs` fixture as `doctor_recovery_cursor_test.go` with vs. without a
     concurrent hung cull tick and asserts the `recovery_sweep_cursor_wedged` / overall `ok`
     outcome is **byte-identical** against `HandleDoctor`. Combined with (3) (same refresh
     timing) and (4) (same values), doctor's **same-instant** verdict is identical under A and
     B — the cull fold can neither create, worsen, nor clear a wedge.
  5. **No torn write (A).** No `cullable_entity` write was performed for the blocked tick (L4).
  6. **Detached goroutine bounded (resource).** A static test asserts
     `DefaultCullFoldTimeout < DefaultSweepInterval` (L1), and the fold's blocked scan
     goroutine observes `cullCtx.Done()` and **terminates at `DefaultCullFoldTimeout`** rather
     than running forever. In v4 the deadline's job is **resource-bounding** the detached
     goroutine (one bounded leak at most, via the single-slot guard), **not** loop protection —
     the loop is protected by the **off-path placement**, which is what (2)/(3) verify.
- **The negative controls that make it bite.**
  - **Timing (the binding one): put the fold back ON the `wait`-gating path.** A variant A′
    that **synchronously joins** the scan (the v3 `select{scanDone, cullCtx.Done()}` structure)
    defers tick 2's refresh by ~`DefaultCullFoldTimeout` (or forever with the deadline also
    removed), so **assertion (3) FAILS** and (with the deadline removed) `Sweeps` stays `1`.
    This is the control proving the test actually binds on the v4 off-path design rather than
    passing vacuously.
  - **Distinct from B2.** B2 proves the **panic** path (the recover seam). B5 proves a scan
    that never returns **and** never panics — which the recover seam structurally cannot catch
    — and it asserts a **doctor property true against current `doctor.go` semantics** (no
    worse than a no-cull baseline, same values **and** same refresh timing), not the v2
    absolute `ok:true`.

*(Option (b) — intentionally changing `doctorRecoverySweepCursor` and
`TestHandleDoctorFlagsRecoverySweepCursorWedgedClaimableRun` to key the quiet window on
`last_sweep_at` — is a coherent alternative the SEED permits, but it changes doctor semantics
and widens blast radius (the existing focused test would have to be rewritten and the wedge
would stop firing for genuinely lane-wedged runs whose sweep cursor is merely fresh). v3
chooses the SEED's **preferred** option (a): it proves the binding property without touching
doctor semantics, so G3/G4 and the existing doctor coverage stay byte-identical.)*

---

## 5. Open Question 1 (peer vs phase) — forward-compatibility, not built (Claim D / Gate G4) — CARRIED INTACT FROM v2 (G4 cleared)

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

## 6. P0 / P1+ boundary — named but NOT built, with the phase each lands in (Gate G4) — CARRIED INTACT FROM v2 (G4 cleared)

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
| **A1** | P0 nominates exactly `{decision:D267, decision:D081}` — superseded decisions with a parseable live successor (extracted by the §3 clause-2 decision rule), unprotected, with **zero active-baseline inbound citation**. | The G1 corpus test shows either is not `nominated`, not derivable under the cell rule, or shows a counted inbound citation on either. |
| **A2** | P0 nominates **none** of the preserved set: `rfc:0097`, `rfc:0027/0039/0041` (clause 4), `rfc:0028` (clause 2), the ten still-cited/parse-withheld superseded decisions, `docs/reference/todo.md`, the six `backup/rfc-*` branches, the RFC-0170 body/SEED/workflow.json/decision-log prose, `_frozen/**`. | Any preserved-set member is `nominated` (a false positive). |
| **A3** | The predicate reads the **structural status field only** (RFC title-block `Status:`/`**Status:**`; decision-row **state cell = cell 2**; doc front-matter), never body prose or another row's cells. | A body "superseded" mention (e.g. `docs/rfcs/0170-…md:95`) or a prose "superseded by" in another decision row produces a candidacy. |
| **A4** | A bare `superseded`/`tombstoned` or a non-resolving successor is **not** nominated. | `rfc:0028` (`superseded V1 foundation`), `docs/reference/todo.md`, a dangling-successor ref, or `decision:D084` (bare state cell, no own-row `superseded by` clause) produces a candidacy. |
| **A4′** | Clauses 2 and 4 are reconciled **mechanically**: a named-successor backref (`RFC 0044`→`0041`, `D270`→`D267`) and a disposable/closure-frame line (roadmap closed-out row, README status rows, reference-link defs) do **not** count; only a live, non-successor, non-disposable active-baseline reference withholds. | Two implementers applying the stated grep + fixed lexicon disagree on any corpus row; or a named-successor backref / closure-frame line is counted as a live citation; or `rfc:0097` is nominated. |
| **A7** | The `kind=decision` successor-extraction rule is **pure-greppable**: successors are read **only** from own-row cell 3 (then cell 5, by precedence) via `\bsupersed(?:ed|es) by\s+<reflist>`, to the sentence/clause boundary, split on `/`,`,`,` and `; cells 1/2/4/6 and other rows' cells are never sources. Table-driven cases prove **D267→{D270}** and **D081→{D087,D094,D104}** nominate and a **bare state-cell-only `superseded` decision with no own-row `superseded by` clause (D084 + a fixture row) is withheld**. | Two implementers disagree on D267/D081's extracted successor set; or D084 / the bare-`superseded` fixture nominates; or the rule draws a successor from cell 1/2/4/6 or another row. |
| **A8** | Clause 3's **cull-specific protected pathspec is fully static and tree-local** (a checked-in manifest with **no external-state term**): `docs/rfcs/` and `docs/decisions/` are **eligible** (withheld only by clauses 1/2/4), while `docs/records/_frozen/**`, the design-scaffold/fixture roots, and the never-cullable root files (`spec.md`/`README.md`/`AGENTS.md`/…) remain protected — and no path's classification depends on open-issue (or any external) state. | An implementer importing `.check-docs-ignore` verbatim protects `docs/rfcs/` (no RFC ever eligible); or the pathspec retains the open-issue (or any external/dynamic) term so a path's protection flips with state outside the tree (re-protecting `docs/decisions/`, making `D267`/`D081` non-derivable); or the rewrite leaves a frozen/`_frozen`/scaffold path eligible. |
| **A5** | P0 emits **no** candidacy from `verdicts.superseded_by_decision_id` (it supersedes a review verdict, not an artifact — PC3). | A superseded verdict row (the `status_superseded_pg_test.go` fixture) produces a `cullable_entity` row. |
| **A6** | P0 emits **no** `kind = branch` candidacy. | Any branch row appears in `cullable_entity`. |
| **B1** | The sweep's only write is the `cullable_entity` UPSERT (in the L4 write phase, on the detached goroutine, own pooled connection); it takes no P0 action, touches no recovery-cursor/lane/run input, and runs **off the `wait`-gating path** so it cannot defer the cursor refresh. | A statement-capture test sees any write outside `striatumd.cullable_entity` or any doctor/admission/cursor state change; or the fold is shown to sit on the `wait`-gating path (recovery goroutine waits on the scan). |
| **B2** | A panic inside the detached cull scan is recovered + logged **in that goroutine** (`runDecayTickSweep` top frame) and the goroutine exits without writing; the recovery sweep's result is returned unchanged and the daemon does not bounce. | An injected panicking scan crashes the daemon, escapes the cull goroutine to the recovery goroutine/process, fails the recovery sweep, or leaves a partial write. |
| **B3** | Per-tick cost is O(corpus) markdown + one bulk SELECT + bounded UPSERTs; no history scan; P0 adds no doctor block. | A cost test shows scaling with event/audit history, or a new doctor class appears across ticks. |
| **B4** | A blocked/slow scan **cannot delay the next recovery tick by ANY amount** (not even ≤ `DefaultCullFoldTimeout`): the fold position launches a detached scan and returns in O(1) (off the `wait`-gating path), so the recovery goroutine never waits on it; the deadline (`DefaultCullFoldTimeout < DefaultSweepInterval`) + skip-on-overrun + compute-then-commit bound the **detached goroutine**. | A blocked cull scan delays the next recovery tick by any amount, holds the recovery goroutine, leaks more than one bounded scan goroutine, or produces a partial `cullable_entity` write. |
| **B5** | The **HANG** A/B test passes — a blocking, non-returning scan still lets the next recovery tick run (`Sweeps==2`, equal to a no-cull control), the **next cursor refresh is not deferred** (tick-2 refresh at the same fake-clock instant under A and B), the cull fold does **not make recovery-cursor doctor state worse than baseline** (same wedge inputs ⇒ same `ok`), and no torn write; the **refresh-not-deferred assertion FAILS if the fold is wired onto the `wait`-gating path** (synchronous join). | The HANG test cannot be made to pass; or the refresh-not-deferred assertion passes even with the fold placed on the `wait`-gating path (proving it vacuous); or the cull fold defers the next cursor refresh or changes any recovery-cursor wedge input vs. the control. |
| **C1** | Migration `0045` ships both read and write authority-inventory rows for `cullable_entity`. | `authority_inventory_static_test.go` (`TestRead/WriteAuthorityInventoryCovers…`) red-mains. |
| **C2** | `0045` GRANTs `SELECT,INSERT,UPDATE` to `striatumd_rw`, carries no owner DDL/FK (≥27 rule), at path `go/pkg/db/sql/`. | The two-role PG apply or `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL` fails, or the runtime role cannot DML the table. |
| **C3** | Every read of `cullable_entity` projects an explicit column list, never `SELECT *`. | A grep/test finds `SELECT *` against `cullable_entity` (the #614 column-scoped-grant 42501 hazard). |
| **D1** | The `(kind, ref)` row shape + ON CONFLICT upsert + extensible `candidacy_state` admit a later phase/toll writer with no schema break. | Adding a phase writer or a `tombstoned` state in P1 requires altering/recreating the P0 table or breaks P0's two-state machine. |
| **PC1/2** | The migration lives at `go/pkg/db/sql/0045_cullable_entity.sql`; `0045` is the free runtime slot. | `go/pkg/db/migrations` is used, or `0045` is already taken. |

---

## 8. Gate mapping (what the adjudicator scores)

- **G1 — Tier-1 exactness (zero false positives; clause-2/clause-4 reconciled; decision
  successor mechanical; protected pathspec well-defined; resurrectable/banked set excluded):**
  A1–A8 + the §3 predicate + the re-stated G1 corpus. The **v4 load-bearing move is the
  fully static, tree-local protected pathspec (A8 — a checked-in manifest with no open-issue
  or any external-state term, excluding `docs/rfcs/`/`docs/decisions/`, retaining
  `_frozen`/scaffold/root protection)**, proven by the protected-pathspec fixture (no network,
  no GitHub). Carried intact and DISCHARGED/credited in v3 (not reopened): the **`kind=decision`
  successor-extraction rule (A7 — own-row cell 3/cell 5, fixed regex, precedence, sentence
  boundary, multi-ref split)**, PC3 (verdicts column is not an artifact edge), the
  structural-status-only clause, the live-successor clause, the active-baseline
  inbound-citation rule (4a–4f, preserving `rfc:0097`/`0027`/`0039`/`0041` and `D174`), and the
  branch-namespace exclusion. The true-positive set is `{D267, D081}` — mechanically derivable
  end-to-end under a fully tree-local predicate — and `rfc:0097`/`0027`/`0039`/`0041` remain
  **preserved** and **eligible**.
- **G2 — read-only safety (error-isolated AND latency-bounded; no action, no page; no doctor
  regression):** B1–B5 + the `runDecayTickSweep` recover seam riding the RFC-0137 metrics-fold
  position + **the per-tick cull-fold deadline (L1), the DB + filesystem dual bound (L2/L3), the
  skip-on-overrun/compute-then-commit policy (L3/L4)** — all carried intact in substance. The
  **v4 load-bearing move is moving the fold OFF the `wait`-gating path** (L3 — the recovery
  goroutine never joins the detached scan, so the fold returns in O(1) and cannot postpone the
  next `scheduler_cursors` refresh), which **removes the v3 refresh-timing carrier by
  construction**. The no-worse-than-baseline doctor property therefore holds on **both** axes —
  **values** (the wedge keys on `claimable_job_count` + `last_lane_advanced_at`, which the cull
  fold never writes — B1/B5.4) and **refresh timing** (the next refresh is not deferred —
  B5.3) — and is **source-true against current `doctor.go`** with **no `doctor.go` semantics
  change**. B5's refresh-not-deferred assertion FAILS if the fold is wired back onto the
  `wait`-gating path.
- **G3 — substrate correctness (slot + grants + read/write inventory + no `SELECT *`):**
  C1–C3 + PC1/PC2. *(Cleared in cycle-1; §2 carried intact, not reopened.)*
- **G4 — forward-compatibility (OQ1 resolved, crisp P0/P1+ boundary):** D1 + §5 + the §6
  deferral table. *(Cleared in cycle-1; §5/§6 carried intact, not reopened.)*

All four are argued as **proven, not merely claimed**, each against a named source site
verified against the live tree at the v4 run base `striatum/rfc-0170-p0-design-v4` and a
refuting test/corpus row. This is the v4 revised claim; the falsifiers re-attack the **fully
static, tree-local protected pathspec** (G1‴) and the **off-`wait`-path fold + refresh-not-
deferred test** (G2‴) — and confirm the carried-intact machinery (the `kind=decision`
successor rule, the clause-4 active-baseline citation rule, the deadline/skip-on-overrun/
compute-then-commit latency bound, G3 substrate, G4 forward-compat) is un-regressed. The cycle
adjudicator ledger — not downstream completion — decides whether the gate clears.

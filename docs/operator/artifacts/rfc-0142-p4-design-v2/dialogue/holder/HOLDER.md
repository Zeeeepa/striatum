# HOLDER (revised, v2) — RFC 0142 P4: the one-shot `striatum daemon deploy` (decouple schema-apply from serve-boot)

author: holder-author-002

> **This is a REVISION.** Design-v1 (`rfc-0142-p4-design`) ran this same
> falsification gate and the adjudicator returned `needs_revision` with three
> material findings — **C1** (finalization boundary unclassifiable), **C2** (0020
> + flag activation not fail-closed), **C3** (runtime-object ownership undefined).
> This spec starts from the v1 `HOLDER.md`
> (`docs/operator/artifacts/rfc-0142-p4-design/dialogue/holder/HOLDER.md`) and
> resolves **every** finding per its prescribed fix in the v1 collaboration ledger
> (`docs/operator/artifacts/rfc-0142-p4-design/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`)
> and `SEED.md`, **without regressing** the parts the v1 ledger judged sound (Q4,
> the per-step Q3-A/Q3-B body + step taxonomy, the shadow-first decoupling shape).
> §0.1 below is the auditable resolution map; the falsifiers should verify each
> finding there, then re-attack §5/§9.
>
> It is a falsifiable implementation spec for RFC 0142 **P4 only** — the one-shot
> deployer that lifts schema mutation out of serve-boot — that `rfc-0142-p4-build`
> can execute contract-first (TDD). RFC 0142 is accepted (D258); this does NOT
> re-open the five-layer design. Primary source:
> `docs/rfcs/0142-safe-by-construction-database-change-deployment.md` (esp. Layer 3,
> the Phasing P4 row, Open Questions 3 + 4). Anchors re-verified against the
> worktree base of `striatum/rfc-0142-p4-design-v2` (cut from `main`); every
> file:line below is current source.

## 0. Root reframe held

Schema mutation must stop being an **implicit side effect of the serving process's
restart** and become an **explicit, ordered, resumable, provenance-tracked
operation owned by a dedicated deployer**. Then the serving daemon holds **zero
DDL on the serving path**, "restart force-commits a half-applied deploy" becomes
structurally impossible, and a bad migration can never wedge the single writer on
boot. Everything below serves that one move; nothing pulls in P5 (rehearse /
expand-contract / fidelity tiering / clone = Q1/Q2).

---

## 0.1 Addressing the design-v1 findings (the gating requirement)

Each finding is resolved by a concrete sub-protocol / state-machine edge /
chosen-and-tested policy — not a claim. The "where" column names the section that
specifies it and the named test that would refute the resolution.

| Finding | Prescribed fix adopted | Where / refuting test |
| --- | --- | --- |
| **C1** finalization boundary unclassifiable (Q3 core) | **Option B + idempotent finalizer.** A distinct `finalizing` cursor state is entered (one durable commit) after the last step; an **idempotent finalizer** then writes the receipt, then the `schema_state` fingerprint, then advances to `complete` **last**. A new §1.3 row classifies `finalizing`/expected-`plan_hash` (any fingerprint) as **resumable finalization** — never serve, never genuine-drift. `complete` is therefore proof that receipt+fingerprint are already durable, so `complete`+mismatch is unambiguous genuine-drift. **No single finalization transaction spans the owner+runtime connections** (operator pin). | §1.2, §1.3, §3.4 / **`T-deploy-resume-finalization-crash`** (F10) |
| **C2** 0020 + `STRIATUM_DEPLOY_DECOUPLED` activation not fail-closed (R3 lockout) | **Deploy-activation interlock fires before `ApplyMigrations`.** A new `CheckDeployActivation` (sibling of `CheckOwnerBundleWatermark`, called at the same boot site) yields a typed non-restartable `awaiting_deploy` / `awaiting_deploy_config` halt whenever the binary embeds the DDL-revoke and the deploy is not complete-and-in-sync — **including when `deploy_cursor` is absent** (treated as incomplete, not error). A **forward-watermark rule** stops a revoke-unaware binary auto-applying over an `applied >= 20` DB. `RequiredOwnerBundleVersion` is **NOT** advanced to 20 (resolves the auto-apply-default contradiction). | §3.3a, §4.2, §4.3 / **`T-deploy-revoke-activation-ordering`** (F11) |
| **C3** runtime-object ownership undefined under owner/admin apply | **Policy 1: runtime objects stay `striatumd_rw`-owned.** The deployer applies each runtime step over the owner connection and, in the **same step transaction**, runs a catalog-diff ownership reconciliation (`ALTER … OWNER TO striatumd_rw` for exactly the objects that step created — all relkinds + sequences, indexes following their table) and re-asserts the migration's DML grants. The landed invariant (bundles 0018/0019 made runtime tables rw-owned) is preserved; §4.1 stands. | §3.3b, §4.4 / **`T-deploy-runtime-object-ownership`** (F12) |

**Carried forward unchanged (v1 ledger §5 — do not re-litigate):** Q4 (plain
verb + three seams, §2); the per-step Q3-A/Q3-B mid-step resumability + step
taxonomy (§1.1, the `in_progress`/`step_committed` body of §1.2); the shadow-first
decoupling shape (default-OFF `STRIATUM_DEPLOY_DECOUPLED`, `ConnectAndVerify`
preserving the P2 watermark interlock + P3 drift gate/self-record, the embed-FS-
derived plan, the hash-chained receipt, `doctor schema_deploy_unrecorded`). The
revision touches only the three defect edges: the **final** boundary (C1), the
**activation** edge (C2), and the **ownership** policy (C3).

---

## 1. Decision Q3 — How atomic is "atomic"? (the hard correctness core)

**Claim (Q3-DECISION).** The **per-step-atomic + resumable-cursor** contract is
sufficient for *every* owner+runtime interleaving P4 ships, **provided** the two
per-step sub-rules (Q3-A/Q3-B, unchanged from v1) hold **and** the **finalization
boundary is closed by a distinct `finalizing` state + an idempotent finalizer**
(new in v2 — closes C1). No step needs a cross-step transaction, and **no
finalization transaction spans the owner+runtime connections** (structurally
impossible — see §3.4).

- **Q3-A — strict sub-protocol where the step is transactional (carry-forward,
  sound).** If a step's DDL can run inside a transaction, the **cursor advance
  commits in the SAME transaction as the step** (over the owner connection post-
  activation — see §3.3b). Crash → the whole transaction rolls back → cursor
  unchanged → resume re-enters the step cleanly. Strictly safer than a separate
  cursor write, so always taken when available.
- **Q3-B — idempotent-resume reconciler where the step is NOT transactional
  (carry-forward, sound).** `CREATE INDEX CONCURRENTLY`, `ALTER TYPE … ADD VALUE`,
  and the handful of auto-committing `ALTER`s cannot share a transaction with the
  cursor. The deployer writes an **`in_progress` marker BEFORE the step**, runs the
  step in its **idempotent / self-reconciling form**, then writes the
  **`step_committed` marker AFTER**. Crash window is *between* the step's own
  commit and the marker; on resume the cursor reads `in_progress(N)` and re-enters
  step N through its reconciler (a no-op or repair by construction).

### 1.1 Step taxonomy (the closed set P4 ships) — unchanged from v1

| Class | Examples | Transactional? | Protocol | Idempotent form |
| --- | --- | --- | --- | --- |
| **T-DDL** runtime | `CREATE TABLE`, additive `ALTER … ADD COLUMN`, `CREATE INDEX` (non-concurrent), the new runtime migrations themselves | Yes | **Q3-A** (step + ownership-reconcile + cursor in one tx) | `IF NOT EXISTS` / `ADD COLUMN IF NOT EXISTS`; re-run is a no-op |
| **T-DDL** owner | owner-bundle DDL (authority/grant/CHECK changes) | Yes (each bundle applies in one tx that stamps `owner_bundle_meta` last — `owner.go:498-528`) | **Q3-A**, reusing the existing per-bundle transaction; cursor advance joins that tx | bundles already idempotent (`IF NOT EXISTS` + `ON CONFLICT (version) DO NOTHING`) |
| **NT-DDL** | `CREATE INDEX CONCURRENTLY`, `ALTER TYPE … ADD VALUE`, `DROP INDEX CONCURRENTLY` | No | **Q3-B** (pre-marker → step → post-marker) | `CREATE INDEX CONCURRENTLY IF NOT EXISTS`; **resume reconciler** drops a left-behind `INVALID` index then recreates; `ADD VALUE … IF NOT EXISTS` (PG12+) |
| **DML-backfill** (bounded in P4; full online backfill is P5) | a one-shot data normalize a runtime migration carries | Yes if small; batched otherwise | Q3-A small / Q3-B batched | `WHERE <not-yet-done>` predicate; re-run resumes from the unprocessed tail |

> The only genuinely hard per-step corner is NT-DDL, reducing to one rule: **a
> half-built `CREATE INDEX CONCURRENTLY` leaves `pg_index.indisvalid = false`**; the
> fingerprint MUST classify that as "incomplete, resume", never "unknown drift,
> panic" — see §1.3. **NT-DDL never creates a new ownable object whose owner is
> wrong:** a CONCURRENTLY index inherits its table's owner (already `striatumd_rw`),
> and `ALTER TYPE … ADD VALUE` creates no relation. So the C3 ownership
> reconciliation (§3.3b) is needed only on transactional object-creating steps,
> where it runs in the step transaction.

### 1.2 `deploy_cursor` states and crash-resume semantics (REVISED for C1)

`deploy_cursor` is a **new runtime-owned singleton table** (runtime migration
**≥ 0044**, modeled byte-for-byte on `0043_schema_state.sql:39-52`: a `CHECK (id =
'singleton')` PK + the `striatumd_rw` GRANT `DO` block). Columns:

```
deploy_cursor(
  id            text PK CHECK (id='singleton'),
  plan_hash     text NOT NULL,         -- the plan this cursor is advancing
  state         text NOT NULL,         -- enum below (CHECK constraint)
  step_index    int  NOT NULL,         -- 0-based index into the ordered plan
  step_id       text NOT NULL,         -- e.g. "runtime:0044" / "owner:0020"
  updated_at    timestamptz NOT NULL DEFAULT now()
)
```

`state ∈ { idle, in_progress, step_committed, finalizing, complete, aborted }`.
The **`finalizing` state is new in v2** and is the C1 fix. Lifecycle for a plan of
N steps:

```
idle ─▶ in_progress(0) ─▶ step_committed(0) ─▶ … ─▶ step_committed(N-1) ─▶ finalizing ─▶ complete
                 │                                                                
                 └─(operator abort / fatal non-resumable error)──▶ aborted        
```

- **Q3-A step (carry-forward):** the transition `in_progress(k) → step_committed(k)`
  and the step DDL commit (plus the C3 ownership reconcile, §3.3b) **are one
  transaction**. No observable `in_progress(k)` after a crash on a transactional
  step. Resume reads `step_committed(k)`, advances to `k+1`.
- **Q3-B step (carry-forward):** three separate commits: `step_committed(k-1) →
  in_progress(k)` (marker), the step's own auto-commit, then `in_progress(k) →
  step_committed(k)`. Crash resume reads `in_progress(k)` and re-enters step k
  through its reconciler.
- **Finalization (NEW — C1).** When `step_index == N-1` is `step_committed`, the
  deployer advances the cursor to **`finalizing`** in **one commit** (over the
  runtime connection — see §3.4), then runs the **idempotent finalizer** (§3.4):
  1. append the **`complete` deploy receipt** (owner connection, `append_audit_row`)
     — guarded idempotent: skip if a receipt for `(plan_hash, state=complete)`
     already exists;
  2. **`RecordSchemaFingerprint`** UPSERT into `schema_state` (idempotent; the
     value is deterministic `ExpectedFingerprint()` and, by §3.2, equals this
     plan's recorded fingerprint);
  3. advance the cursor `finalizing → complete` **last**.

  Because `complete` is written **only after** receipt and fingerprint are durable,
  **a `complete` cursor is proof the deploy is fully finalized.** A crash anywhere
  in finalization leaves the cursor at `finalizing` (never at `complete` with a
  missing receipt/fingerprint), and the finalizer is idempotent, so re-run repairs
  exactly once. This eliminates the v1 unclassifiable state by construction.

### 1.3 The load-bearing disambiguation (REVISED for C1)

Today the drift gate has **one** signal: `LiveFingerprint == ExpectedFingerprint`?
(binary: match / drift, `schema_drift.go:254-274`). P4's `deploy_cursor` is the
**second signal**. The table below is the single most important falsifiable
artifact in the spec; the **`finalizing` row is the C1 fix** and the in-sync row
is tightened to require `state == complete`.

| `deploy_cursor.state` | `plan_hash` | fingerprint | classification | action |
| --- | --- | --- | --- | --- |
| `complete` | == expected | match | **in sync** | serve |
| `in_progress` / `step_committed` | == the binary's plan | (any) | **incomplete, resume** | refuse-to-serve `awaiting_deploy`; `striatum daemon deploy` resumes at the cursor |
| **`finalizing`** | **== expected** | **(any — match or mismatch)** | **resumable finalization** (NEW, C1) | **refuse-to-serve `awaiting_deploy`; re-run runs the idempotent finalizer; NEVER serve, NEVER genuine-drift** |
| absent table / row | n/a | (any) | **deploy not started / pending** | refuse-to-serve `awaiting_deploy` (C2 bootstrap-order case) |
| `complete` but `plan_hash` ≠ expected, OR `aborted` | foreign / n/a | mismatch | **genuine drift** | refuse-to-serve (existing `schema_drift` halt) |
| `complete`, `plan_hash` == expected | == expected | **mismatch** | **genuine post-deploy drift** | refuse-to-serve (`schema_drift`) — unambiguous because `complete` ⇒ fingerprint was durable, so a later mismatch is real divergence, not finalization-incomplete |

The C1 crash window (`complete` with the **expected** `plan_hash` but a missing
receipt/fingerprint) **can no longer occur**: under §1.2, `complete` is the final
write, so reaching it requires the receipt and fingerprint to already be durable.
The interrupted finalization is instead pinned at `finalizing` and classified by
the new row.

---

## 2. Decision Q4 — Is a deploy itself a Striatum run? (carry-forward, sound)

**Claim (Q4-DECISION). Plain verb now (`striatum daemon deploy`); the run-shape
door is held open by three already-present seams.** Unchanged from v1; neither
falsifier contested it and the v1 ledger §5 found it concretely resolved.

**Why not a run now — the bootstrapping paradox is dispositive.** A dogfooded run
needs the daemon up and its schema present to drive `runs`/`jobs`/`sessions`/the
capability ledger. But the deploy is *precisely the operation that brings a fresh
or owner-only database to a serveable schema* (successor to boot-time
`ApplyMigrations` and to `daemon migrate-db`, `daemon.go:167-210`). A run-shaped
deployer cannot bootstrap the database it would itself run on, and would be
unavailable in exactly the incident this RFC closes. So the base deployer is a
**plain verb against a bare / owner-only DB with no running daemon**, the same
context `daemon owner-ddl apply` (`daemon.go:90-159`) and `daemon migrate-db`
already occupy.

**What keeps the run-shape door open (so P5 needs no re-litigation):** (1) the plan
is a content-addressed artifact (`plan_hash`, §3.2); (2) the deploy receipt is
hash-chained into owner-held `audit_log` (§3.4); (3) the verb is a thin CLI over a
`Deployer` package with a pure `Apply(ctx, plan, cursor)` core a future run step
invokes unchanged. **Falsifiable boundary:** if a falsifier shows a real fresh-DB
bring-up the plain verb cannot perform but a run could (it cannot — a run is
strictly more dependency-laden), Q4 flips.

---

## 3. The deployer surface + serve-boot decoupling (shadow-first, by exact code site)

### 3.1 Command site (unchanged from v1)

Add `case "deploy":` to the dispatch in `RunDaemon` (`go/pkg/cli/localcommands/daemon.go:67-81`,
alongside `install|uninstall|status|migrate-db|owner-ddl`) → new
`runDaemonDeploy(...)`. DSN resolution mirrors `runDaemonOwnerDDL`
(`daemon.go:95-121`): `--owner-url` / `--admin-url`, then
`STRIATUM_DAEMON_ADMIN_DB_URL` (`EnvDaemonAdminDBURL`, `daemon.go:24`), then the
daemon DSN. Flags: `--plan` (default embedded), `--dry-run` (print ordered plan +
cursor delta, mutate nothing), `--resume` (default; explicit for clarity),
`--json`. `docs/reference/command-authority-matrix.md` and the authority guardrail
tests gain the `daemon.deploy` row (AGENTS.md change discipline).

The deployer holds **two connections** (both resolved from the owner/admin DSN
context): an **owner connection** for DDL, ownership reconciliation, and the
owner-only `append_audit_row` receipt; and a **runtime view** for the
runtime-owned `deploy_cursor`/`schema_state` reads the boot path will later make as
`striatumd_rw`. The owner role is a member of `striatumd_rw` (or superuser) in the
two-role bootstrap (`authority_bootstrap.go`), so it can write the rw-owned cursor
table and `ALTER … OWNER TO striatumd_rw` (§3.3b) — this is the precondition the
deployer asserts at start.

### 3.2 The deploy plan — derived + embedded (unchanged from v1)

**Claim (PLAN-DERIVED).** The plan is generated deterministically from the same two
embedded `embed.FS` sources the fingerprint already hashes — the runtime migration
FS (`migrations.go:21` `//go:embed sql/*.sql`) and the owner-bundle FS
(`owner.go:156` `//go:embed sql/owner/*.sql`) — **not** a hand-authored manifest.

`BuildPlan(applied_owner, applied_runtime) → DeployPlan`:

- **Steps** = pending owner bundles (ascending `Version`, tagged `owner`,
  `owner.go:196-223`) **then** pending runtime migrations (ascending `Version`,
  tagged `runtime`, `migrations.go:31-105`). Reproduces today's semantics: the
  watermark interlock already requires owner ≥ N before any runtime apply
  (`connection.go:349-352`, `owner.go:124-154`). The plan promotes that into
  **explicit dependency edges** (every `runtime` step depends on the full `owner`
  prefix).
- Each step carries `{role, step_id, sha256, transactional: bool}`. The
  `transactional` bit is the Q3-A/Q3-B selector — a per-step annotation
  (`-- striatum:nontransactional` pragma) defaulting to transactional, validated by
  a build-time test that greps the SQL for known non-transactional verbs
  (`CONCURRENTLY`, `ALTER TYPE … ADD VALUE`) and asserts the pragma matches (a
  mis-annotated step reds the build, not prod — sibling of the landed
  `TestRunnerMigrationsHaveNoNonTransactionalDDL`, `migrations.go:296-299`).
- **`plan_hash`** = sha256 of the canonical step transcript, reusing the
  `composeFingerprint` canonicalization (`schema_drift.go:105-137`). Bound to
  `ExpectedFingerprint` by construction: **a `complete` plan's recorded fingerprint
  == `ExpectedFingerprint()`** (§3.4 step 2), so deployer and drift gate never
  disagree on "is the whole plan applied."

### 3.3 Lifting `ApplyMigrations` out of serve-boot (shadow-first)

**Claim (DECOUPLE).** Serve-boot stops mutating schema, behind a flag, shadow-first.
The single mutation site on the serving path is `ConnectAndMigrate`
(`connection.go:332-404`), reached via `BootstrapAndConnect(…, migrate)`
(`authority_bootstrap.go:181-201`, the `if migrate { ConnectAndMigrate } else {
Connect }` fork), driven by `--migrate` (default `true`, `main.go:76`).

1. **New env `STRIATUM_DEPLOY_DECOUPLED`** (default OFF — shadow-first, cf.
   `EnvSchemaDriftRefuse`, `schema_drift.go:15-28`). OFF → boot behaves exactly as
   today; the decoupling lands to `main` **inert**.
2. When **ON**, `ConnectAndMigrate` becomes **`ConnectAndVerify`**: keep the Layer 2
   watermark interlock (`connection.go:349-352`) and the Layer 3 drift gate
   (`connection.go:376-383`) but **do NOT call `ApplyMigrations`
   (`connection.go:353`)**. Instead run the §1.3 classification and serve only on
   the in-sync row; on any other row refuse-to-serve via the new typed
   `awaiting_deploy` halt (§4.2). The drift self-record (`connection.go:394-402`) is
   **NOT** run on the verify path — only the deployer records the fingerprint
   (§3.4), so a verify-only boot can never overwrite `schema_state` to falsely claim
   in-sync.
3. **No window serving an unmigrated schema (R2).** The decoupled boot verifies
   before serving and refuses on any non-in-sync state; the watermark interlock is
   preserved verbatim.

#### 3.3a The deploy-activation interlock (NEW — C2): fail closed before mutation

The v1 gap (C2): with the DDL-revoke bundle 0020 applied and the flag OFF/absent,
boot reaches `ApplyMigrations` over the **runtime** runner (`connection.go:353`)
because `CheckOwnerBundleWatermark` tolerates a forward bundle
(`owner.go:151-153`); the revoked `CREATE` then yields a raw `42501` that
`main.go:199-228` does not map to a typed halt → crash-loop (#512-class lockout).

**Fix.** A new **`CheckDeployActivation(ctx, runner, decoupledEnabled bool) error`**
(in `go/pkg/db/owner.go` or a new `deploy_activation.go`) is called **at the same
boot site, immediately after `CheckOwnerBundleWatermark` and BEFORE
`ApplyMigrations`** (`connection.go:349-353`), in **both** `ConnectAndMigrate` and
`ConnectAndVerify`. It mutates nothing. Predicate (fail-closed; see §4.2 for the
typed halt and §4.3 for the choreography):

- Let `revokeEmbedded = (LatestOwnerBundleVersion >= DDLRevokeOwnerBundleVersion)`
  where `DDLRevokeOwnerBundleVersion = 20` is a new named constant (the bundle at/
  after which `striatumd_rw` loses `CREATE ON SCHEMA striatumd`).
- **If `!revokeEmbedded`** (a pre-revoke binary, `Latest < 20`): the activation
  interlock is inert *here*, but the **forward-watermark rule** (§4.2) in
  `CheckOwnerBundleWatermark` already halts this binary if it observes `applied >=
  20`. Return nil.
- **If `revokeEmbedded`** (`Latest >= 20`): this binary ships the revoke and MUST
  NOT auto-apply. Read `applied = OwnerBundleVersion(...)` (the `42501`-unreadable
  case is already routed to `awaiting_owner_ddl` by `CheckOwnerBundleWatermark`,
  `owner.go:135-137`). Then:
  - `decoupledEnabled == false` → **`awaiting_deploy_config`** halt: "this binary
    ships the RFC 0142 DDL-revoke; set `STRIATUM_DEPLOY_DECOUPLED=1` and run
    `striatum daemon deploy`." DB untouched. (Prevents both the `42501` auto-apply
    when 0020 is applied AND a legacy self-record that would falsely stamp a
    fingerprint including an unapplied 0020.)
  - `decoupledEnabled == true` → classify the deploy per §1.3, **reading
    `deploy_cursor` defensively** (operator C2 sharpening): if the `deploy_cursor`
    table or row is **absent** (the ≥0044 migration has not run because the deploy
    hasn't), treat as **incomplete → `awaiting_deploy`**, never error-on-missing-
    table. If present and `state != complete` (or `plan_hash`/fingerprint ≠
    expected) → **`awaiting_deploy`**. Only `complete` + in-sync → return nil and
    serve (verify-only). DB untouched in every non-serving case.

Net invariant (the C2 close): **`ApplyMigrations` is never reached over the runtime
runner once the revoke is in play.** Either the binary is revoke-aware and halts
`awaiting_deploy`/`awaiting_deploy_config` before mutation, or it is revoke-unaware
and the forward-watermark rule halts it. No pending runtime migration can execute
under a revoked `CREATE`, so the raw `42501` crash-loop cannot occur.

#### 3.3b Runtime-object ownership under owner/admin apply (NEW — C3): Policy 1

The v1 gap (C3): `applyOne` (`migrations.go:304-355`) runs migration SQL on the
supplied runner with **no `SET ROLE`, no `ALTER … OWNER TO`, no grant
reconciliation**; run over the owner connection (the deployer post-revoke), the
same SQL creates **owner-owned** runtime objects, breaking the landed invariant
that runtime objects are `striatumd_rw`-owned (bundles 0018/0019,
`owner.go:177-178`).

**Chosen policy (one, tested): runtime objects stay `striatumd_rw`-owned.** Reasons:
(1) preserves the deliberately-engineered landed invariant (0018/0019 transferred
runtime-table ownership *to* `striatumd_rw` so runtime migrations may ALTER them);
(2) keeps §4.1's residual-capability argument intact — no §4.1 correction needed;
(3) keeps every runtime object uniform regardless of whether legacy auto-apply or
the deployer created it, so `preflightRuntimeMigrationOwnership` (`migrations.go:158`)
and future ALTER-bearing migrations behave identically.

**Mechanism (`Deployer.applyRuntimeStep`, a deployer-owned wrapper around the
existing `applyOne` body).** `SET ROLE striatumd_rw` is **not** usable (post-0020
`striatumd_rw` lacks `CREATE`, so it would `42501`), so ownership is reconciled
**after** the DDL, **inside the same step transaction**:

1. `BEGIN` (owner connection).
2. Snapshot the owner-role-owned object oids in schema `striatumd` (a catalog read
   over `pg_class` for relkinds `r,p,i,I,S,v,m,t` plus `pg_proc`/`pg_type` if a
   future runtime migration ever creates those kinds — currently none do).
3. Run the migration SQL (the existing `applyOne` DDL + both version stamps +
   in-tx hash verify, `migrations.go:315-349`).
4. Re-snapshot; for **exactly the newly-appeared oids** run
   `ALTER <kind> striatumd.<name> OWNER TO striatumd_rw`. Indexes inherit their
   table's owner (no independent `ALTER INDEX OWNER`), and an owned sequence appears
   as its own new `relkind=S` oid and is reassigned too — so the catalog diff covers
   tables, partitions, indexes, sequences, views, matviews, and **future object
   kinds** without enumerating them by hand. The diff is scoped to **new** oids, so
   it **never** touches pre-existing owner-owned authority/bundle objects.
5. Re-assert the migration's DML grants to `striatumd_rw` (the same `GRANT` block
   the migration file carries, e.g. `0043_schema_state.sql:46-52`) so a created-as-
   owner-then-reassigned object has the serving role's DML regardless of default
   ACLs.
6. Advance `deploy_cursor` (Q3-A: same transaction). `COMMIT`.

The reconciliation is **idempotent**: on resume an already-rw-owned object is not
in the "owner-role-owned" snapshot, so it is skipped. It runs only on transactional
object-creating steps (NT-DDL creates no mis-owned object, §1.1). The owner role
can `ALTER … OWNER TO striatumd_rw` because it is a member of that role (§3.1).

### 3.4 The deploy receipt + the idempotent finalizer (REVISED for C1)

**Claim (RECEIPT).** Every applied step writes a **hash-chained deploy receipt into
the owner-held `audit_log`** via the owner-only SD function `append_audit_row(...)`
(`owner/0001_authority_phase0.sql:223,238`; runtime direct `INSERT` revoked at
`:237`). Payload: `{plan_hash, step_id, role, sha256, state, prev_receipt_hash}`.
`doctor` gains **`schema_deploy_unrecorded`** (modeled on `schemaDriftDoctorBlock`,
`go/pkg/reads/doctor_schema_drift.go:26-77`): WARNING (never hard-red) when
`schema_state.fingerprint` advanced but the matching `complete` receipt is absent in
`audit_log`.

**Why the finalizer is idempotent rather than one transaction (C1 + operator pin).**
The `complete` receipt is appended over the **owner** connection (`append_audit_row`,
owner-only SD function) while `deploy_cursor` and `schema_state` are written over the
**runtime** view of those runtime-owned tables. **These writes cannot share one
transaction across the two connections**, so the only coherent close is an
**idempotent finalizer** (do NOT propose a single finalization transaction spanning
the owner+runtime connections). Finalizer steps and crash-point analysis:

| Crash point | Cursor after crash | §1.3 classification | Re-run repair |
| --- | --- | --- | --- |
| after final step DDL commit, before `finalizing` | `step_committed(N-1)` | incomplete, resume | advance to `finalizing`, run finalizer |
| after `finalizing`, before receipt | `finalizing` | **resumable finalization** | receipt absent → append; fingerprint; `complete` |
| after receipt, before `schema_state` | `finalizing` | **resumable finalization** | receipt present → **skip (exactly-once)**; fingerprint; `complete` |
| after `schema_state`, before `complete` | `finalizing` (fingerprint now matches) | **resumable finalization** (cursor ≠ `complete` ⇒ never serve) | receipt skip; fingerprint UPSERT no-op; `complete` |
| after `complete` | `complete`, fingerprint match, receipt present | in sync | none — serve |

No crash point yields `complete` with a missing receipt/fingerprint (the v1 C1
state), and every interrupted finalization is `finalizing` → classified resumable,
repaired exactly once. `doctor schema_deploy_unrecorded` is green after repair
because the receipt is appended before `complete` (a `complete` cursor always has
its receipt). The receipt idempotency guard is a lookup for a prior
`(plan_hash, state=complete)` receipt in `audit_log` before the SD append.

---

## 4. Serving-role DDL revocation (owner bundle ≥ 0020), without lockout

**Claim (DDL-REVOKE).** The serving role `striatumd_rw` loses its remaining
**object-creation** DDL on the serving path, shipped as **owner bundle 0020**,
**activated only after the deployer is the active mutator** — so failure mode 1 (a
serve-path DDL touching an owner-held table) becomes structurally impossible,
without recreating a #512-class lockout.

### 4.1 What is revoked, precisely (unchanged from v1; §4.1 argument preserved by C3)

The anchor table confirms `striatumd_rw` already holds **no DDL on owner-owned
tables** (`owner/0001_authority_phase0.sql:229-238`; preflight + build guard route
owner-table DDL to bundles, `migrations.go:158`, `owner.go`). The residual
serve-path DDL is **object creation on the runtime schema**. Bundle 0020 revokes
exactly that:

```
-- owner bundle 0020 (DDL-revoke): the serving path holds zero create-DDL.
REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw;
REVOKE TEMPORARY ON DATABASE <db> FROM striatumd_rw;  -- if granted
```

`striatumd_rw` **retains** ownership of (and `ALTER`/`DROP` on) the runtime tables
it already owns — and, by the C3 policy (§3.3b), **new** runtime objects the
deployer creates are reconciled back to `striatumd_rw` ownership too, so the §4.1
invariant "new runtime objects stay `striatumd_rw`-owned" **holds as a tested
property** (it was merely *asserted* in v1; §3.3b now makes it true and
`T-deploy-runtime-object-ownership` proves it). The honest residual is unchanged:
the serving role could still `ALTER` its *own* runtime tables, but **the serving
daemon issues no DDL once `ApplyMigrations` is lifted (§3.3)**, so the serve *path*
is zero-DDL. Full capability revocation (owner re-owns runtime tables) remains a
**follow-up beyond P4** (it would re-open #442), named, not silently assumed.

### 4.2 Typed halts + the forward-watermark rule (NEW — C2)

Two new sibling halts of `AwaitingOwnerDDLError` (`owner.go:58-97`), reusing the
non-restartable exit path (`main.go:200-228`, `exitAwaitingOwnerDDL`):

- **`ErrAwaitingDeploy` / `AwaitingDeployError`** (`awaiting_deploy`): a deploy is
  pending/incomplete (cursor absent, `in_progress`, `step_committed`, or
  `finalizing`). Remediation: "run `striatum daemon deploy`." DB untouched.
- **`ErrAwaitingDeployConfig` / `AwaitingDeployConfigError`** (`awaiting_deploy_config`):
  the binary ships the DDL-revoke but `STRIATUM_DEPLOY_DECOUPLED` is OFF.
  Remediation: "set `STRIATUM_DEPLOY_DECOUPLED=1` and run `striatum daemon deploy`."
  DB untouched.

`main.go` gains the matching `errors.As` arms next to the `AwaitingOwnerDDLError`
and `SchemaDriftError` arms (`main.go:208-228`), so these map to the clean
`exitAwaitingOwnerDDL` parked-`failed` exit, never the fatal fall-through.

**Forward-watermark rule (in `CheckOwnerBundleWatermark`, `owner.go:148-153`).** The
tolerate-forward branch (`applied > required → nil`) is bounded by the revoke
boundary: define the comparison so that when **`applied >= DDLRevokeOwnerBundleVersion
(20)` and this binary does NOT embed the revoke** (`LatestOwnerBundleVersion < 20`),
the watermark returns an **`AwaitingDeployError`** ("this database has activated the
RFC 0142 DDL-revoke; this binary predates the one-shot deployer and cannot serve it
safely — upgrade to a deployer-aware binary") instead of tolerating forward into
`ApplyMigrations`. A revoke-aware binary (`Latest >= 20`) still tolerates forward
across the boundary but is governed by `CheckDeployActivation` (§3.3a). Pre-0020
forward tolerance is unchanged (a botched binary rollback below the revoke still
serves).

**Resolution of the v1 contradiction (auto-apply-default vs `Required = 20`).**
`RequiredOwnerBundleVersion` is **NOT** advanced to 20. The DDL-revoke is gated by
`CheckDeployActivation` + the `STRIATUM_DEPLOY_DECOUPLED` flag, **not** by the
watermark frontier; the watermark interlock keeps `Required = 19` and guards only
the non-revoke owner bundles (≤19), exactly as today. There is therefore **no binary
for which both "auto-apply is the boot default" and "Required = 20" hold**: the
inert-landing binary has `Latest = Required = 19` and auto-applies (legacy default
genuinely intact); the activation binary embeds 0020 (`Latest = 20`, `Required`
still 19) but **never auto-applies** — `CheckDeployActivation` halts it
`awaiting_deploy`/`awaiting_deploy_config` before `ApplyMigrations` — so it has no
auto-apply-default claim to contradict.

### 4.3 The deploy choreography (NEW — C2, made precise)

Two binaries, in order. The split is what makes activation fail-closed without a
fingerprint lie:

1. **Inert-landing binary** (`Latest = Required = 19`; does **not** embed 0020).
   Ships: the `Deployer` engine + `deploy` verb, the ≥0044 `deploy_cursor`
   migration, the `STRIATUM_DEPLOY_DECOUPLED` decoupled boot path (flag OFF), the
   `doctor schema_deploy_unrecorded` block, **and the forward-watermark rule**. Boot
   auto-applies legacy (`striatumd_rw` still has `CREATE`; 0044 is additive,
   runtime-owned — passes `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL`).
   Decoupling inert. After this rollout every serving binary is forward-watermark-
   aware. **Daemon up, serving.**
2. **Activation binary** (`Latest = 20`, embeds 0020; `Required` still 19;
   `CheckDeployActivation` active). On boot with 0020 not yet applied + flag OFF it
   halts `awaiting_deploy_config` (it never legacy-auto-applies, so no `42501` and no
   false fingerprint self-record). The operator then, **with the daemon parked**:
   - sets `STRIATUM_DEPLOY_DECOUPLED=1`;
   - runs **`striatum daemon deploy`** (no daemon serving): the deployer, over the
     owner connection, applies bundle 0020 **and** any pending runtime steps as one
     ordered plan, reconciles ownership (§3.3b), writes the receipt + fingerprint +
     `complete` cursor (§3.4). Because the deployer uses the owner connection, the
     revoked `CREATE` does not block it.
   - restarts the daemon → `ConnectAndVerify` sees `complete` + in-sync → serves,
     verify-only, zero serve-path DDL.

   **Bootstrap-order case (operator C2 sharpening):** if step 2 runs on a DB that
   never ran the inert binary (so `deploy_cursor` is absent), `CheckDeployActivation`
   reads the absent table as **incomplete → `awaiting_deploy`** (not error), and
   `striatum daemon deploy` creates `deploy_cursor` (applying ≥0044 over the owner
   connection, reconciling its ownership to `striatumd_rw`) as the first step before
   recording the first cursor row.

   **Truly-pre-P4 binary (predates even the forward-watermark rule).** It cannot be
   retroactively patched, so it is protected by the choreography precondition: bundle
   0020 is applied **only** by `striatum daemon deploy`, which **preflight-refuses to
   apply 0020 unless the runtime frontier already carries the ≥0044 `deploy_cursor`
   migration** (the inert-landing binary's marker) — i.e. unless a deployer-aware
   binary has booted this DB. This closes the activation gate (the one site we
   control) against an old binary still being the active serving process. Stated as
   a falsifiable assertion + game-day (F11 / `G-old-binary-refuse`).

### 4.4 How 0020 ships without lockout (the R3 sequencing) — REVISED

The deployer applies runtime DDL over the **owner/admin connection** — the same
mechanism `daemon migrate-db --admin-url` (`daemon.go:161-210`) and `owner-ddl
apply` already use — so the "runtime path" the revoke must not break is **already
owner-capable out-of-band**; 0020 does not remove a capability the deploy path
needs. Combined with §4.2 (typed halts + forward-watermark rule) and §3.3a
(activation interlock fires before `ApplyMigrations`), **no ordering — including
every interleaving in `T-deploy-revoke-activation-ordering` — drives boot into a
runtime-runner `ApplyMigrations` under a revoked `CREATE`.** The watermark interlock
advancing is no longer the activation mechanism; the explicit interlock + flag is.

---

## 5. Falsifiable assertions (each paired with the test / game-day that refutes it)

> `T-` = unit/integration test (TDD target for the build run); `G-` = game-day
> against a real two-role cluster. F1–F9 carry forward from v1 (sound); **F10–F12
> are the new assertions that close C1/C2/C3** and are what the cycle-2 falsifiers
> re-attack first.

| # | Assertion | Refuting experiment |
| --- | --- | --- |
| **F1 (Resumability — Q3-A)** | Kill `deploy` after a transactional step *k* commits; re-run resumes at *k+1*, never re-runs *k* or half-applies. | `T-deploy-resume-transactional`; `G-resume-kill-9`. |
| **F2 (Resumability — Q3-B)** | Kill `deploy` during a `CREATE INDEX CONCURRENTLY`; re-run's reconciler drops the `INVALID` index and rebuilds; final state is one VALID index, no double-apply. | `T-deploy-resume-concurrently`; `G-cic-kill`. |
| **F3 (No serve-boot mutation)** | With `STRIATUM_DEPLOY_DECOUPLED=ON`, a boot with a pending plan does NOT apply it; refuses-to-serve `awaiting_deploy` via the non-restartable exit. | `T-boot-decoupled-no-apply`; `G-pending-restart`. |
| **F4 (Fingerprint coherence — §1.3)** | An interrupted deploy is classified "incomplete, resume" / "resumable finalization", never "unknown drift". | `T-classify-incomplete-vs-drift`: matrix over `{cursor state} × {fingerprint match}` asserting the §1.3 table exactly; mutate `plan_hash` to foreign → flips to genuine-drift. |
| **F5 (No regression of P2/P3 — R2)** | Lifting `ApplyMigrations` keeps the watermark interlock + Layer 3 self-record intact; fresh-DB bring-up still works via the verb. | `T-decoupled-preserves-watermark`; `T-fresh-db-via-deploy`; existing `migrations_test.go` two-role guard stays green. |
| **F6 (DDL revocation — R3, structural)** | After bundle 0020, `striatumd_rw` cannot `CREATE` on schema `striatumd`; the deployer (owner connection) still applies runtime steps; no lockout across restart. | `T-revoke-blocks-rw-create`; `T-deploy-after-revoke`; `G-revoke-then-restart`. |
| **F7 (Receipt provenance — R4)** | Every applied step writes a hash-chained deploy receipt; a schema change without one is surfaced by `doctor schema_deploy_unrecorded`. | `T-receipt-per-step`; `T-doctor-unrecorded`. |
| **F8 (Cursor has no double-apply/skip hole — R4)** | No interleaving of crash points across a step boundary yields a skipped or twice-applied step. | `T-deploy-fault-injection`: parametrized crash at each of {before marker, after marker/before step, after step/before commit-marker, after commit-marker} for a T-DDL and an NT-DDL step. |
| **F9 (Plan ≡ fingerprint binding)** | A `complete` deploy's recorded fingerprint equals `ExpectedFingerprint()`. | `T-plan-hash-binds-fingerprint`. |
| **F10 (Finalization boundary closed — C1)** | No crash during finalization leaves a `complete` cursor with a missing receipt/fingerprint; every interrupted finalization is pinned at `finalizing`, classified **resumable finalization** (never serve, never genuine-drift), and repaired exactly once on re-run. | **`T-deploy-resume-finalization-crash`**: kill after each of {final DDL commit; `finalizing`-before-receipt; receipt-before-`schema_state`; `schema_state`-before-`complete`}; assert (a) no boot serves until the cursor reads `complete`, (b) exactly one `complete` receipt after repair (re-run does not double-append), (c) `LiveFingerprint == ExpectedFingerprint` and `doctor schema_deploy_unrecorded` green after repair. `G-finalize-kill`: `kill -9` in the finalization window on a real cluster, `deploy` again, assert serve + single receipt. |
| **F11 (Activation fail-closed — C2)** | With bundle 0020 applied (or a revoke-embedding binary), boot **never** calls `ApplyMigrations` over the runtime runner; it halts `awaiting_deploy`/`awaiting_deploy_config` with the DB untouched and a remediation naming `striatum daemon deploy` / the missing flag. A revoke-unaware binary observing `applied >= 20` halts via the forward-watermark rule, not a `42501` crash-loop. | **`T-deploy-revoke-activation-ordering`**, parametrized over: (a) 0020-before-flag (P4 binary, flag OFF) → `awaiting_deploy_config`; (b) 0020-before-`daemon deploy` (flag ON, `deploy_cursor` absent/incomplete) → `awaiting_deploy`; (c) old-binary (`Latest=19`) + `applied=20` + pending runtime migration → `awaiting_deploy` via forward-watermark, **`ApplyMigrations` NOT called, DB untouched**; (d) P4-binary + flag OFF + pending runtime migration → `awaiting_deploy_config`, **`ApplyMigrations` NOT called**. Assert via a spy that `applyOne` is never entered and the schema is byte-identical. `G-old-binary-refuse`: apply 0020 on a DB an old binary serves; assert clean `failed` park, not a crash-loop; assert `daemon deploy` preflight-refuses 0020 when the ≥0044 marker is absent. |
| **F12 (Runtime-object ownership — C3)** | A runtime migration applied through the deployer's owner connection yields objects **owned by `striatumd_rw`** with the serving role's real DML intact; after bundle 0020 the serving role cannot create objects. | **`T-deploy-runtime-object-ownership`** (two-role pgtest): apply a new runtime migration (creating a table + index + sequence) via the deployer over the owner connection; assert `pg_class.relowner` / `pg_get_userbyid` == `striatumd_rw` for **every** created object (table, its index, its sequence); assert `SET ROLE striatumd_rw` can `SELECT/INSERT/UPDATE/DELETE` the new table; assert `SET ROLE striatumd_rw; CREATE TABLE …` → `42501` after 0020. Re-run the deployer and assert the reconciliation is a no-op (idempotent). |

---

## 6. Shadow-first rollout (the P4 build order) — REVISED

1. **Migration ≥ 0044 `deploy_cursor`** (additive runtime table, modeled on
   `0043_schema_state.sql`; the `state` CHECK includes the new `finalizing` value;
   no FK, runtime-owned — passes `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL`).
2. **`go/pkg/db/deploy.go`**: `DeployPlan`, `BuildPlan`, `Deployer.Apply` (Q3-A/Q3-B
   engine + the **`finalizing` finalizer**, §3.4, advisory-locked via
   `MigrationLockKey`), `applyRuntimeStep` (the **C3 ownership reconciliation**,
   §3.3b), receipt writer. Pure-core + DB-integration tests (F1, F2, F4, F8, F9,
   **F10**, **F12**) — engine + finalization + ownership proven **before** any boot
   path changes.
3. **`runDaemonDeploy`** verb (`daemon.go`) + matrix/authority-guardrail row +
   `--dry-run` + the **0020-activation preflight** (§4.3). F3/F5 wiring.
4. **`CheckDeployActivation`** + the typed `awaiting_deploy` / `awaiting_deploy_config`
   halts + the **forward-watermark rule** in `CheckOwnerBundleWatermark` + the
   `main.go` `errors.As` arms (§3.3a, §4.2). Decoupled boot path
   (`ConnectAndVerify`) behind `STRIATUM_DEPLOY_DECOUPLED` (default OFF). Lands inert
   (**F11**, F3, F5).
5. **`doctor schema_deploy_unrecorded`** block (F7).
6. **Owner bundle 0020** (DDL revoke) — authored, tested in two-role pgtest (F6,
   F12), **not auto-applied** and **not required by the watermark**; activation is
   the operator choreography (§4.3) that flips the flag and runs `daemon deploy`.

Each phase additive and reversible until the operator flips the flag + applies
0020. Self-record before enforce; detection before mutation-relocation; the verb +
finalizer + ownership reconciliation proven before the boot path leans on them.

---

## 7. Boundary & explicit deferrals (unchanged from v1)

- **In P4:** the deployer verb, the deploy plan + `deploy_cursor` (incl. the
  `finalizing` state) + deploy receipt, the idempotent finalizer, the C3 ownership
  reconciliation, lifting `ApplyMigrations` out of serve-boot (flagged), the
  activation interlock + forward-watermark rule, serving-role create-DDL revocation
  (bundle 0020).
- **Explicitly deferred to P5 (Q1/Q2):** `striatum daemon rehearse`,
  `rehearsal_receipt.v1`, fidelity tiering, full-data clone, expand/contract reshape
  primitive, lock-budget guardrail. P4 names the seams (plan_hash artifact, receipt
  chain, `Deployer.Apply` engine) that P5 builds on but ships none of it. Full
  capability revocation (owner re-owns runtime tables) is a named follow-up beyond
  P4 (§4.1), not assumed.
- **Local-first boundary held:** single host, ONE Postgres, ONE daemon as the single
  writer, no hosted services / replicas / external persistence.

---

## 8. Open tensions handed to the falsifiers (attack these hardest)

- **C1 / F10 — the `finalizing` close.** Is the `finalizing`-state + idempotent-
  finalizer airtight for *every* crash point, including a crash that re-enters the
  finalizer twice (does the receipt idempotency lookup race a second deployer under
  `MigrationLockKey`)? Is there any boot that reads `finalizing` + matching
  fingerprint and wrongly serves?
- **C2 / F11 — the forward-watermark rule + the old-binary preflight.** Is the
  `daemon deploy` 0020-preflight (refuse unless the ≥0044 marker is present) a
  sufficient close against a truly-pre-P4 binary still serving across a restart, or
  is there an interleaving where 0020 lands while such a binary is the active writer?
  Does `CheckDeployActivation` reading `deploy_cursor` ever error on a partially-
  created table (created but not yet granted) instead of classifying incomplete?
- **C3 / F12 — the catalog-diff ownership reconciliation.** Does the before/after
  oid diff miss any object kind a runtime migration could create (e.g. a TYPE, a
  trigger function, an owned sequence created implicitly by `GENERATED … AS
  IDENTITY` vs `serial`)? Can the diff ever reassign an owner-bundle object created
  in the same plan (an owner step preceding a runtime step in one `deploy` run)?
- **§4.1 — the residual capability.** Revoking only `CREATE ON SCHEMA` leaves
  `striatumd_rw` able to `ALTER` its own tables. Is "the serve *path* issues no DDL"
  a sufficient P4 close of failure mode 1, or can a falsifier construct a serve-path
  DDL that survives (a DML trigger / `DO` block that issues DDL)?
- **Q4 — bootstrapping.** Any path where plain-verb-now forecloses the run-shape
  later that the three seams (§2) don't keep open?

---

### Anchor table (re-verified against the worktree base of `striatum/rfc-0142-p4-design-v2`)

| Target | Anchor |
| --- | --- |
| serve-boot mutation site (lift this) | `go/pkg/db/connection.go:332` `ConnectAndMigrate`; apply at `:353`; watermark `:349-352`; drift gate `:376-383`; self-record `:399-402` |
| boot fork / `--migrate` flag / typed-halt mapping | `go/pkg/db/authority_bootstrap.go:181-201`; `go/cmd/striatumd/main.go:76,192-198`; typed halts `:200-228` (C2 adds `awaiting_deploy`/`awaiting_deploy_config` arms here) |
| deployer command site | `go/pkg/cli/localcommands/daemon.go:67-81` dispatch; pattern `runDaemonOwnerDDL` `:90-159`, `runDaemonMigrate` `:167-210` |
| runtime apply engine to reuse / wrap (C3) | `go/pkg/db/migrations.go:121-178` `ApplyMigrations`; `applyOne` `:304-355` (no `SET ROLE`/`ALTER OWNER`/grant — the C3 gap); lock `:18,:122-134`; preflight `:158`; frontier `:17` (=43, new ≥0044) |
| owner-bundle engine / per-tx atomicity / watermark + forward-tolerance (C2) | `go/pkg/db/owner.go:264-302` `ApplyOwnerBundles`; `:498-528` `applyOneOwnerBundle` (one tx, stamp last); `CheckOwnerBundleWatermark` `:124-154` (forward-tolerance `:151-153` — the C2 gap); frontier `:23` (=19, revoke bundle ≥0020); `RequiredOwnerBundleVersion` `:35` (stays 19) |
| fingerprint contract (bind plan_hash to) | `go/pkg/db/schema_drift.go:83-100` `ExpectedFingerprint`; `:105-137` `composeFingerprint`; `:145-161` `LiveFingerprint`; `:171-195` `RecordSchemaFingerprint` (C1 finalizer step 2); `:254-274` `CheckSchemaDrift`; `EvaluateSchemaDrift` `:230-237` |
| `deploy_cursor` table model (incl. new `finalizing` state) | `go/pkg/db/sql/0043_schema_state.sql:39-52` (singleton CHECK + `striatumd_rw` GRANT block) |
| doctor block model | `go/pkg/reads/doctor_schema_drift.go:26-77` |
| DDL-revoke surface | `go/pkg/db/sql/owner/0001_authority_phase0.sql:229-238` (role create + revokes/grants); receipt SD fn `append_audit_row` `:223,238` |

<sub>Holder artifact (v2 revision) for the RFC 0142 P4 falsification-gate design
run. Resolves cycle-1 findings C1/C2/C3 (§0.1) without regressing the
carry-forward items; the cycle-2 falsifiers (atomicity/resumability lens;
decoupling/migration-safety lens) re-attack §5 (esp. F10–F12) and the tensions in
§8; the adjudicator's collaboration ledger — not falsifier completion — decides
whether the gate clears.</sub>

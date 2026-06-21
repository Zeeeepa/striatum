# HOLDER — RFC 0142 P0: the two-role pgtest fixture (the claim under test)

author: holder-author-001
role: holder
gate: fg_rfc0142_design (falsification)
scope: RFC 0142 P0 only (Layer 1a) — build-ready spec
status: published claim — falsifiers attack this; the collaboration ledger decides if the gate clears

This is the **leading proposal** for the falsification gate. It restates RFC 0142's
load-bearing claim, then turns **P0 (the two-role pgtest fixture)** into a spec
precise enough to build test-first. Every load-bearing claim below names the
evidence that supports it and the concrete observation that would refute it.
I do not re-derive the RFC; read `RFC-0142.md` and `SEED.md` for the design of record.

---

## 1. The claim under test

**RFC 0142 load-bearing claim (do not re-derive — restated):**

> *Most of these failure modes are symptoms of one coupling: the serving daemon
> mutates its own schema on restart, irreversibly, using the role least
> privileged to do it.* (RFC-0142.md, "Self-applied discipline" + "The reframe".)

P0 does **not** fix that coupling (Layer 3 does). P0 builds the **executable
oracle** that makes failure-mode #1 — the two-role `42501` trap (#442 / D248) —
visible in CI, which every later layer leans on for its differential property
test.

**P0 load-bearing claim (the thing falsifiers should try hardest to break,
verbatim from SEED.md):**

> *"A two-role pgtest fixture that runs the migration suite as a privilege-
> constrained `striatumd_rw` against a cluster with the real owner/runtime
> ownership topology will red the PR for exactly the migrations that would
> `42501` in prod, and green for the ones that wouldn't — with no false reds
> (legal runtime DML mis-flagged) and no false greens (an owner-table touch that
> slips through)."*

The whole P0 spec below exists to make that claim **true and demonstrable**.

---

## 2. Where the gap is today (the thing P0 closes)

Verified against `main` this session (Go module in `go/`):

- `pgtest.Pools(t)` (`go/pkg/pgtest/pgtest.go:51-131`) returns
  `(privileged, unprivileged)`. The **migration suite runs via
  `db.ConnectAndMigrate(ctx, testURL, "pgtest")`** at `pgtest.go:70` — connected
  as the **DSN user** (`STRIATUM_PG_TEST_URL`), which is typically the
  owner/superuser. So a runtime migration that `ALTER`s or FK-references an
  owner-held table is applied **as the owner** and never `42501`s. This is the
  single-role blind spot (SEED anchor #6).
- The existing "unprivileged" pool is **not** the migration path. It creates a
  per-test LOGIN role `striatumd_rw_<db>`, makes it a *member* of `striatumd_rw`,
  and `SET ROLE`s to it on connect (`pgtest.go:89-128`) — but it is used only to
  test the **DML write-boundary REVOKEs**, never to apply migrations. Migrations
  never run under any privilege-constrained role.
- Owner bundles are not auto-applied by the harness; tests opt in with
  `db.ApplyOwnerBundles(ctx, pool.Runner, "test")` run on the **privileged** pool
  (e.g. `go/pkg/db/authority_enforcement_pg_test.go:29`), so the DSN user owns
  the authority objects.

So the gap P0 closes is precisely: **apply the system-under-test migration as a
privilege-constrained `striatumd_rw` against a DB whose ownership split is real**,
and assert the `42501` fires for an owner-table touch and does not fire for a
legal runtime operation.

---

## 3. Build-ready P0 spec

### 3.1 Files to change (test harness + test code ONLY)

1. **`go/pkg/pgtest/pgtest.go`** — add a two-role fixture constructor, e.g.
   `TwoRolePool(t) *db.Pool` (or `TwoRoleRunner(t) db.Runner`). It bootstraps the
   real two-role topology and returns a runner that executes **as
   `striatumd_rw`**. It reuses the existing `ensureRuntimeRole`
   (`pgtest.go:255-281`), `createDatabase` (`:141`), `db.ConnectAndMigrate`, and
   `db.ApplyOwnerBundles` — it does **not** hand-build grants or a hardcoded
   owner-table list.
2. **`go/pkg/db/two_role_pg_test.go`** (new) — the one red regression test, the
   green control(s), a fixture self-check, and the static/live differential
   check. (Co-locating with `migrations_test.go` is acceptable; a new file keeps
   the two-role helpers and the parse-based guards visibly distinct.)

No new `go/pkg/db/sql/NNNN_*.sql` runtime migration. No new
`go/pkg/db/sql/owner/NNNN_*.sql` owner bundle. No daemon code path changes.

### 3.2 Role + ownership topology the fixture must provision

The fixture must reproduce the production ownership split, **not** a hand-built
one. Two roles, mirroring RFC 0110 / D215:

- **owner role** — owns the authority schema, the `SECURITY DEFINER` write
  functions, the append-only `events` / `audit_log` / `artifacts` surfaces, and
  migration bookkeeping (`schema_migrations`, `schema_authority`,
  `owner_bundle_meta`, …). In the fixture this is the **DSN user** that applies
  the bootstrap. (See `write_authority_inventory.go:25-44` for the owner-only /
  SD-gated set.)
- **runtime role `striatumd_rw`** — non-superuser, does DML + ongoing runtime
  migrations. Already created plain (no `SUPERUSER`) by `ensureRuntimeRole`
  (`pgtest.go:255-281`).

**Bootstrap order (this is the honest part — pre-empts "you can't bootstrap as
the constrained role"):** Production never bootstraps the whole schema as
`striatumd_rw`. The runbook applies the historical runtime migrations **as the
owner** at bootstrap, applies owner bundles as the owner, and only *ongoing*
migrations apply as `striatumd_rw` on boot (this is exactly the reasoning the
static guard's comment records, `migrations_test.go:44-48`). The fixture mirrors
this in two phases on one ephemeral DB:

- **Phase A — bootstrap as the owner (DSN user):**
  `db.ConnectAndMigrate` (runtime 0001-0042) **then**
  `db.ApplyOwnerBundles(..., "test")` (owner 0001-0019). This realizes the real
  split: owner-held tables owned by the owner; the bundle-0018/0019 cohort
  (`job_recovery_state`, the supervisor-pointer cohort, …) **transferred to
  `striatumd_rw` ownership** (`owner.go:46-47`).
- **Phase B — system-under-test as `striatumd_rw`:** apply the candidate runtime
  migration / run the red + green probes through a pool whose connections
  `SET ROLE striatumd_rw` (mirror the existing `AfterConnect` at
  `pgtest.go:112-115`, but `SET ROLE striatumd_rw` directly — the canonical
  runtime role, so privileges equal prod exactly, not the per-test member shell).

### 3.3 The exact mechanism that makes a `42501` actually fire

After `SET ROLE striatumd_rw`, PostgreSQL carries out privilege checks **as
`striatumd_rw`** for the duration. `striatumd_rw` is **not a superuser** and is
**not the owner** of the owner-held tables. `ALTER TABLE` / `DROP TABLE` require
table **ownership**; an inbound foreign key requires the **`REFERENCES`**
privilege — and **neither ownership nor superuser is inherited through role
membership** (membership with `INHERIT` conveys *granted* privileges only). So:

- `striatumd_rw` attempting `ALTER TABLE striatumd.events …` → `must be owner of
  table events` → **SQLSTATE 42501** (the #442 shape).
- `striatumd_rw` attempting an FK into `striatumd.repositories` → `permission
  denied for table repositories` → **SQLSTATE 42501** (the D248 shape;
  `repositories` is runtime-DML-*writable* but owner-*owned* and never
  transferred — `striatumd_rw` has DML yet lacks `REFERENCES`, the exact trap).

This is the direct answer to SEED falsification target #1: role *membership* does
**not** leak owner power, because the operations that fail (`ALTER`/`DROP`,
`REFERENCES`) gate on ownership / a separate privilege that membership never
conveys. The fixture's only obligations are (a) `striatumd_rw` is not a superuser
and (b) it owns none of the owner-held tables — both assertable live (see §3.6).

### 3.4 The one red regression test

**`TestRuntimeMigrationOwnerTableTouchIsDeniedTwoRole`** (new,
`two_role_pg_test.go`):

- **Owner table touched:** `striatumd.events` (SD-gated, owner-held, never
  transferred — `write_authority_inventory.go:33`).
- **DDL attempted as `striatumd_rw`:**
  `ALTER TABLE striatumd.events ADD COLUMN p0_probe integer`.
- **Expected:** error with **`pgErr.Code == "42501"`** and message containing
  `must be owner of table events`. This reproduces #442 (the runtime `ALTER` of
  an owner-held table that crash-looped the daemon at boot).
- **Sibling FK form (reproduces D248) in the same test or a paired test:** as
  `striatumd_rw`,
  `CREATE TABLE striatumd.p0_probe_child (id text PRIMARY KEY, repository_id text
  REFERENCES striatumd.repositories(repository_id))` → expected **`42501`**,
  message containing `permission denied for table repositories`. This is the
  RFC-0136-P1 / D248 incident (`REFERENCES striatumd.repositories` FK the runtime
  role lacks `REFERENCES` for).

The test asserts on **SQLSTATE 42501 specifically** (via `pgconn.PgError.Code`),
not just "some error" — so a setup failure cannot masquerade as a pass (see C1).

### 3.5 The green control (proves discrimination, not blanket failure)

**`TestRuntimeMigrationRuntimeOwnedTableAlterSucceedsTwoRole`** (new):

- As `striatumd_rw`,
  `ALTER TABLE striatumd.job_recovery_state ADD COLUMN p0_probe integer`
  → **succeeds**, because owner bundle 0018 transferred `job_recovery_state`
  ownership to `striatumd_rw` (`owner.go:46`). This is the **same `ALTER` verb**
  that is denied on `events`, so a pass here proves the fixture discriminates by
  **ownership**, not by blanket-denying DDL.
- Secondary greens (so the gate is honestly "legal runtime work passes"): a
  legal runtime `INSERT`/`SELECT` on a `runtime_dml` table (e.g.
  `striatumd.leases`) succeeds; a `CREATE TABLE striatumd.p0_new (…)` succeeds
  (a brand-new runtime table is `striatumd_rw`-owned at create time — the
  same premise the FK guard uses, `migrations_test.go:114-118`).

### 3.6 Fixture self-check (positive control for the red test's honesty)

Add assertions, run as the SUT runner before the probes, that the red test fails
for the **right reason**:

- `SELECT current_user` = `striatumd_rw`.
- `SELECT rolsuper FROM pg_roles WHERE rolname = current_user` = `false`.
- `SELECT pg_get_userbyid(relowner) FROM pg_class WHERE oid =
  'striatumd.events'::regclass` ≠ `striatumd_rw`.
- `SELECT pg_get_userbyid(relowner) … 'striatumd.job_recovery_state'::regclass`
  = `striatumd_rw` (the transfer landed; if not, the green control is meaningless
  and the bootstrap is wrong).

If any self-check fails the fixture aborts loudly — a red caused by a broken
fixture is never reported as a real `42501`.

### 3.7 Consistency with the existing static guard (anchor #4) — complement, one source

- `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL` (`migrations_test.go:776-790`)
  and its FK sibling `TestFutureRuntimeMigrationsDoNotFKOwnerHeldTables`
  (`:804-820`) are **static SQL parses**. They derive the runtime-owned set from
  `runtimeOwnedTablesAlterable(t)` (`:57-83`), which **regex-parses the owner
  bundle SQL** for `ALTER TABLE … OWNER TO striatumd_rw` transfers.
- **One source, no drift:** P0's live fixture derives its ownership topology by
  **applying the same owner bundle SQL** (`db.ApplyOwnerBundles`). Both the
  static guard and the live oracle read from **one source — the owner bundle
  files**. P0 MUST NOT introduce a hardcoded owner-table list: "owner-held" ≡
  "live `pg_class.relowner` ≠ `striatumd_rw` after the real bundles are applied."
  This is what keeps the RFC's "generated from one source so they cannot drift"
  claim true at P0.
- **Relationship = complement, not duplicate.** The static guard is the fast
  parse-time pre-filter (catches literal `ALTER striatumd.<owner>` /
  `REFERENCES striatumd.<owner>`); the fixture is the executable ground truth.
  P0 adds:
  - a **differential check** asserting the static guard's verdict and the live
    fixture's `42501`/success agree for a corpus of literal synthetic migrations;
  - a **fixture-only red** for a *dynamic* owner-table touch the static parser
    cannot resolve — `DO $$ BEGIN EXECUTE 'ALTER TABLE striatumd.events ADD
    COLUMN x int'; END $$;` — which `42501`s live but is invisible to the regex.
    This demonstrates the RFC's "static SQL resolution is unsound; the fixture is
    the real oracle" with a **test**, not an assertion.

### 3.8 Boundary check

P0 adds **no** runtime migration, **no** owner bundle, **no** new daemon table,
and **no** daemon behavior change — test harness (`pgtest.go`) + test code only.
The two-role DB is the existing ephemeral per-test database (`createDatabase` +
cleanup drop, `pgtest.go:141-169`), dropped after use. Local-first, single-host,
ONE Postgres boundary intact (AGENTS.md / `docs/reference/spec.md`): no hosted
service, replica-as-dependency, cloud API, telemetry, or external persistence.

---

## 4. Falsifiable claims list

| # | Load-bearing claim | Evidence that SUPPORTS it | Concrete observation that REFUTES it |
|---|---|---|---|
| C1 | `SET ROLE striatumd_rw` faithfully drops owner power | self-check: `current_user=striatumd_rw`, `rolsuper=false`, `relowner(events)≠striatumd_rw`; red `ALTER events` → `42501` "must be owner" | red `ALTER` succeeds; or fails with a **non-42501** error (setup breakage); or the 42501 is not an ownership/permission message |
| C2 | Role membership does not leak owner power | FK red → `42501` "permission denied for table repositories"; striatumd_rw is a member of no role that owns owner tables | the FK or `ALTER` succeeds because striatumd_rw inherited `REFERENCES`/ownership via a membership edge or owns the owner table |
| C3 | The fixture discriminates (no false reds) | green `ALTER job_recovery_state`, runtime DML, and new-table `CREATE` all succeed as striatumd_rw | any legal runtime operation reds → the fixture is "fails always", oracle worthless |
| C4 | One source, no drift (anchor-#4 consistency) | a test asserts the transferred set parsed by `runtimeOwnedTablesAlterable` equals the live set of tables whose `relowner = striatumd_rw` | a table the static guard treats as runtime-owned is owner-owned live (or vice versa) → the two derivations disagree |
| C5 | P0 catches what static parse misses | the `DO/EXECUTE` dynamic `ALTER events` reds live; the static regex does not flag it | the dynamic owner-table touch does **not** red under the live fixture → P0 is no stronger than the static guard for that class |

The whole P0 claim (§1) is the conjunction C1 ∧ C2 ∧ C3: red for exactly the
prod-`42501` migrations, green for the legal ones. C4 and C5 are why P0 can
honestly anchor Layer 1/2 (SEED target #4) and is not redundant with anchor #4.

---

## 5. Known risks (pre-empted, not hidden)

**R1 — Fixture fidelity when the DSN user is a superuser.** If
`STRIATUM_PG_TEST_URL`'s user is a superuser, owner-held tables are owned by a
superuser, not a distinct non-superuser bootstrap role like prod's `halbritt`.
For the **`42501` oracle this is harmless**: `striatumd_rw` (non-superuser,
non-owner) still cannot `ALTER`/`REFERENCE` the owner table, so the failure fires
identically. The residual gap is owner-*side* permissiveness (a superuser owner
can do things a plain owner can't), which P0 never asserts on — P0 asserts only
the runtime-role-**denial** direction. **Mitigation:** the SUT phase must run
**unconditionally** under `SET ROLE striatumd_rw` (never the DSN user), enforced
by the C1 self-check; never `RESET ROLE` inside Phase B. Flag owner-side fidelity
for Layer 4 rehearsal, where it matters.

**R2 — Per-test cost / isolation.** Two-role bootstrap adds owner-bundle apply
(19 bundles, each a transaction) + role setup per ephemeral DB, on top of the 42
runtime migrations. **Mitigations:** (a) **only the new two-role tests pay it** —
the existing single-role `Pool`/`Pools` path is untouched, so suite-wide cost is
bounded to the handful of P0 tests; (b) amortize with a per-package `TestMain`
template DB or `CREATE DATABASE … TEMPLATE` clone of a once-bootstrapped two-role
DB if cost bites; (c) the cluster-wide `striatumd_rw` role race across parallel
`_pg_test.go` packages is already handled by the idempotent `ensureRuntimeRole`
(`pgtest.go:255-281`, swallows `42710`/`23505`), which the two-role pool reuses —
no new cross-package shared mutable state.

**R3 — `SET ROLE` vs connect-as.** `SET ROLE` keeps the session *user* as the DSN
owner, so a stray `RESET ROLE` would escape the constraint. **Mitigation:** the
SUT runner is exclusively the `SET ROLE`'d pool (via `AfterConnect`, mirroring
`pgtest.go:112-115`); it exposes no `RESET ROLE` path, and the C1 self-check
proves the effective role at probe time. A dedicated `striatumd_rw` LOGIN shell is
an acceptable stricter alternative.

**R4 — Does the oracle generalize to Layer 1/2 (SEED target #4)?** P0's
"owner-held" is live `relowner`, derived from the same bundle SQL the Layer 1
denylist will derive from. They cannot drift **iff P1's lint reads the same source
(bundle transfers)**. P0's obligation is only to (a) not hardcode a table list and
(b) expose the transferred-set derivation as a shared helper so P1 consumes it.
If P1 instead hardcodes a list, the anti-drift claim breaks at **P1**, not P0 —
flagged here so P1 honors it.

---

## 6. What would make me concede the P0 shape is wrong

Not a false red or a cost concern (those are constraints to discharge, §5). The
P0 *shape* is invalidated only if a falsifier shows that **`SET ROLE
striatumd_rw` against a real-bundle-bootstrapped DB cannot reproduce the prod
`42501` at all** — e.g. that membership/`INHERIT`/`SECURITY DEFINER`/default-
privilege semantics make `striatumd_rw` effectively own or `REFERENCE` the owner
tables in a way no fixture provisioning can strip without diverging from prod
grants. If that holds, P0's executable-oracle premise fails and the right first
slice is not the fixture. I claim it does **not** hold (§3.3), and C1/C2 are the
tests that would catch me if it did.

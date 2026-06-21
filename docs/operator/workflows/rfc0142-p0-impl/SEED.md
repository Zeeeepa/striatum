# SEED — RFC 0142 P0 implementation (Stage 2: build the two-role pgtest fixture)

This run **builds RFC 0142 P0** — the two-role pgtest fixture + the red regression
test that reproduces the #442 / D248 `42501` trap — **test-first**. The design is
already hardened and build-ready; this is execution, not design.

## Read these first (your context docs)

- **`committee-output/HOLDER.md`** — the build-ready P0 spec (files, fixture
  design, the red test, the green control, consistency with the static guard).
  **This is your primary spec.**
- **`committee-output/COLLABORATION_LEDGER_cycle_1.md`** — the adjudicated verdict
  (`accept_with_findings`) with **binding constraints C1–C5** you MUST discharge,
  each with a verification gate.
- `go/pkg/pgtest/pgtest.go` — the harness you extend.
- `AGENTS.md` — product boundary (P0 is test-harness + test code ONLY).

(Both committee-output files were produced by the Stage 1 design committee, banked
run `run_468d26b3`; the ledger was recovered to main — see that dir's README and
issue #551.)

## What to build (P0 scope)

A **two-role pgtest fixture** that runs the migration suite / the system-under-test
as a **privilege-constrained `striatumd_rw`** against a cluster with the **real
owner/runtime ownership topology**, plus:
- **one red regression test** that touches an owner-held table as the constrained
  role and asserts `SQLSTATE 42501` (reproducing #442 / D248), and
- **a green control** (a legal runtime operation that must succeed under the same
  fixture), so the fixture proves *discrimination*, not blanket failure.

P0 adds **no** runtime migration, **no** owner bundle, **no** daemon behavior
change. It is **test-harness + test code only** (`go/pkg/pgtest/` and a
`*_pg_test.go`, likely in `go/pkg/db/`). It introduces **none** of RFC 0142's
later-layer symbols (`schema_state`, `deploy`, `rehearse`, `requires_owner_bundle`,
fingerprints).

## Binding constraints you MUST discharge (from the ledger — do not weaken)

- **C1 — escape-proof SUT role.** Phase B (the system-under-test execution) MUST
  run through a connection whose LOGIN user is a dedicated **non-superuser,
  non-owner LOGIN role** that is not a member of and does not inherit from the
  owner role — **NOT `SET ROLE striatumd_rw` inside a privileged connection**.
  Gate: a test runs `RESET ROLE; ALTER TABLE striatumd.events ADD COLUMN p0_probe
  integer` as the SUT runner and asserts it **still** fails `42501` (no path back
  to the owner/DSN login).
- **C2 — bootstrap ownership fidelity.** Phase A MUST reproduce prod's actual
  per-table `relowner`: tables created by recent runtime migrations and never
  transferred (`supervisor_buffered_packets`/0038, `event_chain_segments`/0041,
  `verifier_attestations`/0042) MUST be **`striatumd_rw`-owned** in the fixture
  (NOT owner-owned). Discharge by applying historical runtime migrations as
  `striatumd_rw` in prod-faithful order, **or** by asserting the resulting
  `pg_class.relowner` set against the static-guard-derived runtime-owned set
  (`runtimeOwnedTablesAlterable`, see `go/pkg/db/migrations_test.go`). Gate: a
  differential test — a legal runtime `ALTER` on each recent striatumd_rw-owned
  table **succeeds**, while an owner-held `ALTER` **`42501`s**.
- **C3 — non-superuser bootstrap.** Phase A MUST succeed under a **non-superuser
  owner DSN**: the `ALTER TABLE … OWNER TO striatumd_rw` transfers require the
  executor be a member of `striatumd_rw`, so provision it (`GRANT striatumd_rw TO
  CURRENT_USER` before `ApplyOwnerBundles`) and **REVOKE it before Phase B**. Do
  not silently depend on a superuser DSN.
- **C4 — isolation self-check.** The fixture self-check MUST assert, at probe time
  (after C3's grant is revoked), that the SUT role is **not a member of and does
  not inherit from** the owner role (`pg_has_role` / `pg_auth_members`), in
  addition to `rolsuper = false` and `relowner(events)` ≠ SUT role. Abort loudly
  if isolation does not hold, so a red `42501` is trusted only when isolation holds.
- **C5 — search_path fidelity (low).** Pin the Phase B SUT connection's
  `search_path` (and prod session GUCs) to prod's value.
- **F6 (deferred, NOT P0):** dual-deploy-path divergence is a Layer 3/4 deployer
  concern. Do not attempt it here; state P0's boot-model scoping.

## Anchors (verified against main; cite, don't re-derive)

- `pgtest.Pools(t)` (`go/pkg/pgtest/pgtest.go:51-131`) returns `(privileged,
  unprivileged)`; migrations run via `db.ConnectAndMigrate(...)` at `pgtest.go:70`
  as the **DSN user** (owner/superuser) — the single-role blind spot. The existing
  "unprivileged" pool (`pgtest.go:89-128`) is a per-test role made a **member** of
  `striatumd_rw` via `SET ROLE`, used **only** for DML write-boundary tests, never
  for migrations — this is exactly C1's hazard (do not reuse it for the SUT).
- Runtime migration frontier `0042`; owner bundles applied by `db.ApplyOwnerBundles`.
- Static guard: `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL` /
  `runtimeMigrationOwnerDDLViolations` / the runtime-owned set in
  `go/pkg/db/migrations_test.go` — your fixture's "owner-held" notion should derive
  from the **same source** (RFC 0142 "one source, no drift"), complementing (not
  duplicating) the static guard with a live-privilege oracle.

## How to work (TDD) and how it gets verified

1. **Write the red test first** (C1's `RESET ROLE` + owner-table touch → `42501`),
   watch it fail for the *right reason*, then build the fixture to make it pass.
2. Add the **green control** (C2's legal runtime `ALTER` succeeds) and the C2
   differential check, C3 bootstrap, C4 self-check, C5 search_path pin.
3. Keep the change minimal and idiomatic to the surrounding pgtest/db code.

**Verification reality:** the Stage 3 verifier mints sealed receipts for
`go-build` / `go-vet` (module-wide) — make sure the code **builds and vets clean**.
The new test is a **PG-backed `_pg_test.go`** and needs a real two-role cluster
(`STRIATUM_PG_TEST_URL`); the no-network verifier sandbox **cannot run it**, so its
red/green behavior is validated by build/vet + code review here, with the live PG
run left to the operator. Do not weaken the test to make a no-cluster run pass.

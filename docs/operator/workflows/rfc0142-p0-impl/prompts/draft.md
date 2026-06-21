# Task — Draft: build RFC 0142 P0 (two-role pgtest fixture + red regression test)

Read `SEED.md`, `committee-output/HOLDER.md`, and
`committee-output/COLLABORATION_LEDGER_cycle_1.md` (your context docs) first, plus
`go/pkg/pgtest/pgtest.go` and `go/pkg/db/migrations_test.go`. Build P0 **test-first**.

You make **real source changes** in your worktree (this lane has
`publish_source_changes: true`). The actual code is the deliverable; the `DRAFT.md`
handoff artifact just describes it.

## Build (discharge C1–C5 from the ledger — do not weaken any)

1. **Red test first.** Add a PG-backed test (a `*_pg_test.go`, e.g.
   `go/pkg/db/two_role_pg_test.go`, matching the package's existing pgtest usage)
   that, as the **constrained SUT role**, runs
   `RESET ROLE; ALTER TABLE striatumd.events ADD COLUMN p0_probe integer` and
   asserts `SQLSTATE 42501` (reproducing #442 / D248). This is C1's gate.
2. **Two-role fixture.** Extend `go/pkg/pgtest/` with a fixture that provisions the
   real owner/runtime ownership topology and exposes a **dedicated, non-superuser,
   non-owner LOGIN role** for the SUT (C1) — NOT `SET ROLE` inside a privileged
   connection (the existing unprivileged pool is the anti-pattern; do not reuse it
   for the SUT).
3. **Bootstrap ownership fidelity (C2).** Ensure recent runtime tables
   (`supervisor_buffered_packets`/0038, `event_chain_segments`/0041,
   `verifier_attestations`/0042) are `striatumd_rw`-owned in the fixture. Derive
   "owner-held" from the **same source as the static guard**
   (`runtimeOwnedTablesAlterable` in `migrations_test.go`) — one source, no drift.
   Add the **differential check**: a legal runtime `ALTER` on each recent
   striatumd_rw-owned table succeeds; an owner-held `ALTER` `42501`s (green control
   + red).
4. **Non-superuser bootstrap (C3).** Phase A must work under a non-superuser owner
   DSN: `GRANT striatumd_rw TO CURRENT_USER` before the ownership transfers, REVOKE
   before Phase B. Declare/assert the DSN privilege; don't silently need superuser.
5. **Isolation self-check (C4).** At probe time (after C3's grant is revoked),
   assert via `pg_has_role` / `pg_auth_members` that the SUT role is not a member of
   and does not inherit from the owner role (plus `rolsuper=false`,
   `relowner(events)` ≠ SUT role); abort loudly otherwise.
6. **search_path (C5).** Pin the SUT connection's `search_path` to prod's value.

## Constraints

- **Boundary:** test-harness + test code ONLY. No runtime migration, no owner
  bundle, no daemon change. No RFC 0142 later-layer symbols.
- Match the surrounding pgtest/db code style; keep the diff minimal and idiomatic.
- It must **`go build` and `go vet` clean** (the Stage 3 verifier gates on this).
- The PG test needs a real two-role cluster (`STRIATUM_PG_TEST_URL`); the verifier
  sandbox can't run it. Do **not** weaken the test to pass without a cluster —
  guard it the way other `_pg_test.go` suites guard (skip when no PG URL), but keep
  the assertions real.

## Deliverable

Make the source changes, then publish **`docs/operator/artifacts/cc_rfc0142_p0/DRAFT.md`**
(kind `handoff`) describing: files changed, how each of C1–C5 is discharged (map
each to the code), the red test + green control, and how you verified `go build` /
`go vet`. Do not touch `.striatum/`.

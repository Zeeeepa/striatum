# Implement GH #164 — owner bundle 0006 read-scope reduction (RFC 0114, accepted D173)

You are implementing an **accepted, fully designed** RFC. Do not redesign it.
Read these first, in order:

1. `docs/rfcs/0114-read-scope-principals-sessions.md` — the accepted design.
   Every implementation decision below is specified there; where this prompt
   and the RFC disagree, the RFC wins.
2. `docs/decisions/decision-log.md` row **D173** — the acceptance record.
3. `go/pkg/db/sql/owner/0005_token_read_scope.sql` + `go/pkg/db/owner.go` —
   the owner bundle 0005 precedent (D170) this extends. Mirror its SQL
   structure, stamp discipline, and test shape.
4. `go/pkg/db/read_authority_inventory.go` — the read-class inventory.
5. `go/pkg/reads/doctor_pg_read_scope.go` — the doctor posture surface.
6. GH issue #164 — the driving issue.

## Deliverables

1. **`go/pkg/db/sql/owner/0006_identity_read_scope.sql`** — the owner bundle:
   - `ALTER TABLE ... OWNER TO CURRENT_USER` for `striatumd.principals`,
     `striatumd.principal_clients`, `striatumd.client_sessions` **FIRST**.
     These tables are owned by `striatumd_rw` (runtime migrations created
     them), so a plain `REVOKE SELECT` is NOT a boundary — the owner role can
     self-re-grant. Ownership transfer is the load-bearing step (D173).
   - Then owner-owned `SECURITY DEFINER` projections guarded by
     `assert_daemon_authority()` for the runtime read paths that genuinely
     need them, then the revokes:
     - `principals`: projections per the RFC.
     - `principal_clients`: **column gate** — `principal_id` denied; the
       non-gated columns stay directly readable because
       `go/pkg/admin/tokens.go`'s live `UPDATE ... WHERE` reads
       `client_id`/`unlinked_at` (verify this against current source; the
       constraint was discovered live, re-verify it).
     - `client_sessions`: **full deny** (verify there are still zero runtime
       Go consumers before relying on this).
   - Authority capability stamps per the bundle 0005 precedent.
2. **`go/pkg/db/owner.go`**: `LatestOwnerBundleVersion` 5 → 6; capability
   registration consistent with how bundle 0005 added `auth_projection_read`.
3. **Runtime read-path routing**: any runtime reads of the gated surfaces go
   through the authorized projections with the same fallback discipline as
   bundle 0005 (`PostgresAuthorizer` precedent: prefer projection, fall back
   only when the function is absent, repair grant drift via
   `ReassertReadRevokes`-style sweep).
4. **Doctor**: `pg_read_scope.posture` becomes **derived** from stamps +
   privilege/ownership **probes** (not hardcoded): with bundle 0006 applied
   and verified it reports `partial_projection_gated`; without it, the
   current `broad_runtime_select`. `private_read_denial` stays `false`
   (RFC 0113 R2/R3 are NOT in scope). Update `partial_projection_gates`
   entries for the new surfaces.
5. **Guard tests** (pg-gated, live PostgreSQL): mirror the bundle 0005 test
   precedent — column denial, direct runtime read `42501`, unauthorized
   projection call `28000`, authorized projection reads succeed,
   grant-drift repair, **ownership-transfer assertions** (the table owner is
   the database owner after the bundle), and the doctor posture derivation
   in both states (bundle absent → `broad_runtime_select`; applied →
   `partial_projection_gated`).
6. **`CHANGELOG.md`**: entry under `Unreleased`.

## Constraints

- The bundle is **owner-applied out-of-band** (RFC 0079 §5): the daemon
  never applies this DDL at runtime. Tests apply it to their own scratch
  test databases (the pgtest harness creates DBs as the test user, who is
  their owner — that is the correct simulation of the production owner).
- **No new RPC methods** — no `docs/reference/command-authority-matrix.md`
  changes should be needed; if you find yourself adding an RPC, stop and
  record a blocker instead.
- All of `make lint`, `make typecheck`, `make test` green, plus the exact CI
  lint (`golangci-lint`, `0 issues`) from `go/`.
- Run pg-gated tests with the `STRIATUM_PG_TEST_URL` environment variable
  already present in your lane environment. Never read or print
  `STRIATUM_DAEMON_DB_URL`.
- Work only inside your per-job worktree and your declared write scope.
  Commit your work in the worktree with clear messages.

## Artifact

Publish `docs/operator/artifacts/rfc-0114-impl-164/DRAFT.md` (handoff):
what landed where (file list), the named tests and their actual observed
results (paste the go test output lines — do not summarize from memory),
the operator runbook for applying bundle 0006 to the production database
out-of-band, and the doctor posture before/after evidence. Use the byline
supplied in your work packet.

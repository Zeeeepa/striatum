# Draft: RFC 0114 — read-scope least privilege successor (principals + client_sessions)

You are the **author**. Produce the design RFC that extends RFC 0113 R1 (GH
#164) to the next runtime read-scope least-privilege expansion: the
`principals` / `principal_clients` and/or `client_sessions` surfaces.

## Required reading (do this first)

- `docs/rfcs/0113-runtime-read-scope-least-privilege.md` (the RFC you extend)
- `docs/dogfoods/rfc0114-read-scope/CONTEXT.md` (operator-supplied source
  facts: the owner bundle 0005 precedent, the ownership constraint, the doctor
  posture, the guard tests, the read consumers — all with file paths)

You may also read the cited source files directly to verify
(`go/pkg/db/sql/owner/0005_token_read_scope.sql`,
`go/pkg/db/read_authority_inventory.go`, `go/pkg/db/owner.go`,
`go/pkg/reads/doctor_pg_read_scope.go`, `go/pkg/rpc/auth_pg.go`,
`go/pkg/db/sql/0023_principals.sql`, `go/pkg/admin/principals.go`).

## What to produce

Write the RFC at the artifact path declared in your work packet
(`docs/dogfoods/rfc0114-read-scope/artifacts/RFC_DRAFT.md`).

This is **design only**: no `go/` change, no owner bundle applied to any live
database, no daemon restart. The RFC describes the plan; implementation is a
later PR.

Follow the house style of `docs/rfcs/0113-*.md`. Include a title, status line
(`Status: proposed`), date, an `author:` byline line that EXACTLY matches the
`author_line` in your work packet's `expected_artifacts` entry, a `Context:`
line citing RFC 0113 / RFC 0110 / GH #164 and the relevant source files, then:

1. **Problem** — what RFC 0113 R1 left open; why principals/client_sessions are
   the right next surface(s).
2. **Scope and ordering** — pick which surface(s) to close in this successor
   (principals, client_sessions, or both, with the link tables) and JUSTIFY the
   order. Use sensitivity (display_name identity prose vs session linkage) and
   parity risk (which surfaces have live read consumers) as the criteria.
3. **The ownership constraint** (LOAD-BEARING) — `principals`,
   `principal_clients`, and `client_sessions` are owned by the runtime role
   `striatumd_rw`, not the database owner. A plain `REVOKE SELECT FROM
   striatumd_rw` on a table it OWNS does not lock it out. Resolve this
   explicitly (Option A: ALTER TABLE OWNER TO owner role + SECURITY DEFINER
   projection like 0005; Option B: weaker scoping; Option C: defer). Pick one
   and justify. Address the RFC 0079 §5 owner-table trap consequence (future
   schema changes to these tables must move to owner DDL once owner-held).
4. **Owner-bundle plan** — concrete bundle 0006 contents: any `ALTER TABLE ...
   OWNER TO`, the SECURITY DEFINER projection function signature(s) each
   starting with `assert_daemon_authority()`, the `REVOKE SELECT` + column/row
   `GRANT` back, the `schema_authority` capability stamp, the
   `LatestOwnerBundleVersion` bump to 6, the `ownerBundleLabels[6]` entry, and
   the `ReassertReadRevokes` extension for the new capability.
5. **Daemon read-handler changes** — the projection-preferred + direct-fallback
   dual path (mirror `PostgresAuthorizer.authorizeWithProjection`; fallback on
   `42883` so un-adopted DBs still work). Name the handler(s):
   `admin.ListPrincipals` for principals; confirm client_sessions consumers.
   State the parity guarantee (same DTO fields before/after).
6. **Guard tests** — name them. Inventory completeness already exists; add
   denied-table negatives (`has_table_privilege ... = false` after the phase),
   denied-column negatives if column-scoped, projection-success tests, and a
   handler-parity test (doctor principals block / ListPrincipals returns the
   same fields). Reference `go/pkg/db/read_authority_inventory_pg_test.go`.
7. **Doctor posture transition** — flip `pg_read_scope.posture` from
   `broad_runtime_select` to `partial_projection_gated`, and specify HOW doctor
   computes it (derive from stamped projection-read capabilities / denied
   surfaces, not a hard-coded constant). `private_read_denial` stays `false`.
8. **Rollout + verification** — owner-applied out-of-band (`striatum daemon
   owner-ddl apply`), doctor posture check, pgtest privilege negatives, daemon
   restart sequencing, grant-drift repair. State explicitly that NONE of this is
   executed in this design run.
9. **Acceptance criteria** and **Open questions / revisit triggers** (what
   graduates to `private_read_denial`, deferred surfaces).

Keep claims keyed to a doctor posture string. Be concrete and implementable —
a reviewer will check it against the real source files. Heartbeat periodically
during long reads. Publish the artifact, then complete the job.

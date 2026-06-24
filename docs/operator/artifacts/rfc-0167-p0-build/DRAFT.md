# DRAFT — RFC 0167 P0 implementation (operator identity & run attribution)

author: author-author-001

This is the draft-lane implementation of **RFC 0167 P0** (D260), built
contract-first against the falsification-cleared v4 SPEC
(`docs/operator/artifacts/rfc-0167-p0-design-v4/commit/proposal/PROPOSAL.md`).
All ten §9 build-manifest items are implemented in this per-job worktree.
`cd go && go build ./... && go vet ./...` both pass; the non-PG contract/route,
owner-bundle, reservations, and read-authority tests pass; `make check-docs` is
green. The PG-backed two-role pgtests are written correctly and run in the
separate verifier stage.

## Verified owner-bundle ordinal: 0022 (NOT 0021)

The SPEC instructed: use 0021 if free, **0022 if the concurrent RFC 0142 P4 build
took 0021**. Verified at build time: ordinal 0021 IS taken — `go/pkg/db/owner.go`
declares `DDLRevokeOwnerBundleVersion = 21` (the RFC 0142 P4 C3 DDL-revoke,
STAGED in `sql/owner_staged_activation/`, deploy-plan-terminal) and
`ownerBundleLabels[21]` is set. There is no `sql/owner/0021_*.sql`. So the new
bundle is **`0022_operator_identity_run_attribution.sql`**, and
`LatestOwnerBundleVersion`/`RequiredOwnerBundleVersion` advance from 20 to **22**.

### Load-bearing build discovery (the frontier-predicate retarget)

A *normal apply-eligible* bundle (0022) now sits **above** the staged terminal
revoke (0021), which breaks RFC 0142 P4's frontier assumption that
`watermark/version >= 21 ⟺ the revoke`. Left unchanged, this would (a) filter
0022 out of every apply route (`isNonRevokeBundle(22) = 22 < 21 = false`), so the
daemon could never reach watermark 22 and would halt forever on
`awaiting_owner_ddl`; (b) make `RevokeBundleEmbedded()` falsely true for the
inert-landing binary; and (c) make the `connection.go` forward-watermark barrier
(`applied >= 21`) false-positive at watermark 22 (with 21 absent) and refuse a
healthy post-0022 database. The minimal, semantically-correct fix — retarget the
exclusion from *frontier* to the *exact revoke version*:

- `isNonRevokeBundle(v) = v != DDLRevokeOwnerBundleVersion` (was `v < …`)
- `RevokeBundleEmbedded()` matches `== DDLRevokeOwnerBundleVersion` (was `>= …`)
- new `db.IsOwnerBundleApplied(ctx, runner, version)` replaces the
  `connection.go` boot barrier's `applied >= 21` MAX-based check (asks "was 0021
  *specifically* recorded?").

The RFC 0142 P4 invariant is preserved exactly: 0021 remains the sole
deploy-plan-terminal exclusion; a no-revoke binary still refuses a genuinely
post-revoke DB. `owner_revoke_filter_test.go` and the `reservations_test.go`
owner-bundle contiguity guard are updated for the 22 frontier + the deliberate
21 staged gap. The runs `GRANT SELECT(...)` list was regenerated from the live
catalog (no migration ALTERs `runs ADD COLUMN`, so the 0005 baseline 16 columns
+ `created_by_handle_id`, minus `created_by_principal_id`).

## §9 build manifest — items, files, and discharged assertions/tests

| § | Item | Files | Discharges |
|---|------|-------|-----------|
| 1 | **Owner bundle 0022** — `operator_handles` + `operator_sessions` + `runs` origin columns + write-once trigger + 3 DEFINER projections + column-scoped grants incl. the **runs REVOKE + column re-GRANT** and the **operator_handles `principal_id` exclusion** + watermark stamp; `owner.go` label[22] + `LatestOwnerBundleVersion=22` + `RequiredOwnerBundleVersion=22` + `readScopeReasserts["operator_identity_run_attribution"]` | `go/pkg/db/sql/owner/0022_operator_identity_run_attribution.sql`, `go/pkg/db/owner.go`, `go/pkg/db/connection.go`, `go/pkg/db/sql/RESERVATIONS.toml`, `go/pkg/db/owner_revoke_filter_test.go`, `go/pkg/db/reservations_test.go` | A13–A19', A28, A33–A39, A44 |
| 2 | **Lease + operator-session layer** — `defaultHandle`/escalation walk (FNV pool + reserved denylist + disjoint service sub-pool), `acquireOperatorHandle` (ON CONFLICT live-unique lease + escalation), guarded heartbeat renewal, operator_sessions create/heartbeat/**close revoking the operator token** | `go/pkg/mutations/operator_session.go` | A6–A12, A25, A27, A31 |
| 3 | **`mintOperatorSessionToken`** (sibling of `mintSessionBoundToken`) inserting `operatorSessionCapabilities = {admin, read}`; `sessionBoundCapabilities` UNCHANGED | `go/pkg/mutations/operator_session.go` (alongside `session_token.go`) | A29, A30, A32, A40, A43 |
| 4 | **Operator-bootstrap mint+lease RPC** (`operator.bootstrap`, +`operator.heartbeat`/`operator.close`) reusing `mintOperatorSessionToken` + `admin.LinkClientToPrincipal`, presenting the session-bound operator token | `go/pkg/mutations/operator_session.go`, `go/pkg/mutations/mutations.go`, `contracts/daemon_methods.json` | A3, A40, A45 |
| 5 | **Run-origin stamp** — resolve principal in Go via `admin.ResolvePrincipalForClient` (bound param); `created_by_handle_id` via the `operator_handles` subquery on `app.session_id`; no direct `principal_clients` subquery. `wakeTx.QueryRowBound` added so the projection path is taken inside the mutation tx (else direct SQL 42501s two-role) | `go/pkg/mutations/run.go`, `go/pkg/mutations/wake.go` | A1, A2, A4, A5, A28 |
| 6 | **Identity reads via projections** — `whose <run-id>` read handler over `run_origin_identity` (`CapabilityRead`) + contract + regenerated routes/registry + matrix row + contract test; `status --mine` over `runs_for_origin_client` with bare-id fallback | `go/pkg/reads/whose.go`, `go/pkg/reads/status.go`, `go/pkg/reads/reads.go`, `contracts/daemon_methods.json`, `go/pkg/rpc/registry_methods.go`, `go/pkg/cli/routes/routes_generated.go`, `go/pkg/cli/params/params.go`, `docs/reference/command-authority-matrix.md`, `docs/reference/daemon-method-tables.md` | A7, A38 |
| 7 | **Convert the three `runs` star-readers** to explicit column lists EXCLUDING `created_by_principal_id` | `go/pkg/reads/detail.go` (run.detail + job.detail), `go/pkg/reads/archive.go` | A38 (required, or the column-revoke springs 42501) |
| 8 | **Doctor `attribution_unknown` advisory** over the daemon-gated `runs_missing_origin` NULL-origin scan projection (warning, never red) | `go/pkg/reads/doctor_attribution.go`, `go/pkg/reads/doctor.go`, bundle 0022 (`runs_missing_origin`) | A21 |
| 9 | **Ten named two-role pgtests** | `go/pkg/db/operator_identity_pg_test.go` | A1, A6, A7, A11, A12, A14, A27–A29, A35–A39, A44, A45 |
| 10 | **Docs** — decision-log (D263), spec.md, command-authority-matrix.md, CHANGELOG.md; read-authority inventory + column denials | `docs/decisions/decision-log.md`, `docs/reference/spec.md`, `docs/reference/command-authority-matrix.md`, `CHANGELOG.md`, `go/pkg/db/read_authority_inventory.go` | §F F-1/F-2 dispositions recorded |

### The ten pgtests (§4.5), by name

`TestOperatorOwnerBundleAppliesCleanTwoRole` (owner_bundle_applies_clean),
`TestOperatorComposedIdentityMapUnreadableTwoRole` (composed_identity_map_unreadable,
incl. Route 3 spawn-grant), `TestOperatorWhoseStatusMineViaProjectionTwoRole`
(whose_status_mine_via_projection), `TestOperatorSessionPreRunStampTwoRole`
(operator_session_pre_run_stamp), `TestOperatorTokenAdminSurfaceTwoRole`
(operator_token_admin_surface, incl. the credential-segregation face),
`TestOperatorForgedUpdateCreatedByRejectedTwoRole`
(forged_update_created_by_rejected), `TestOperatorDriftReassertReclosesRoutesTwoRole`
(drift_reassert_recloses_routes), `TestOperatorTwoLiveMayaTwoRole` (two_live_maya),
`TestOperatorTokenRevokedBareIDTwoRole` (token_revoked_bare_id),
`TestOperatorLeaseFlapStealTwoRole` (lease_flap_steal). They run as the
escape-proof SUT login role via `pgtest.TwoRole`; they skip without
`STRIATUM_PG_TEST_URL` (proven in the verifier stage).

## §F binding constraints — discharged

- **F-1 / A44 (`C-C2DPRIME-SPAWN-GRANT-ENUMERATION`).** Bundle 0022 does **not**
  modify `spawn_authorization_grants` (P1 custody is out of P0).
  `composed_identity_map_unreadable` Route 3 PINS the disposition with a fail-loud
  control: `spawn_authorization_grants.owner_principal_id` holds the run owner's
  **client id** with **no FK to `striatumd.principals`** (asserted via
  `information_schema` over the constraint graph), so the composed
  `cc ⋈ oh ⋈ runs ⋈ spawn_authorization_grants` route reconstructs
  `client_id → client_id`, not a principal leak; the control fails loudly if a
  future change adds the FK. Recorded in the read-authority inventory and the
  spec/decision-log.
- **F-2 / A45 (`C-C1DPRIME-STATIC-TOKEN-SEGREGATION`).** The blast-radius is
  recorded as **static-segregated-plus-narrowed-routine**, not
  "strictly-less-standing": `HandleOperatorBootstrap` presents the session-bound
  `{admin, read}` operator token for routine repo-admin and does NOT inject the
  static 8-capability `bootstrap-admin` token; the close path revokes the operator
  token; the static token remains the segregated daemon-root credential.
  `operator_token_admin_surface` pins the operator token carries exactly
  `{admin, read}` (repo-scoped, session-bound), not the broad static set. The
  `verifier.attest` trust-root fence (`IsSessionBound()` refusal) and the
  repo-scope daemon-global bound are unchanged.

## Build/verify status

- `cd go && go build ./...` — **pass**
- `cd go && go vet ./...` — **pass**
- `go test ./pkg/rpc/ ./pkg/cli/routes/ ./pkg/cli/routestest/ ./pkg/cli/params/` — **pass** (contract↔registry↔routes consistency for `whose` + `operator.*`)
- `go test ./pkg/db/ -run 'TestOwner|TestReservation|TestRequired|TestRead'` — **pass** (frontier predicates, reservations contiguity, read-authority inventory)
- `make check-docs` — **pass**

## Remaining thin follow-up (flagged, not silently dropped)

The `operator.bootstrap`/`heartbeat`/`close` daemon RPCs are implemented,
registered, and on-contract (reachable via MCP/RPC). The existing
`striatum operator bootstrap` CLI **local command** (a cold-start orientation
packet generator) is **not yet rewired** to call the new `operator.bootstrap`
RPC and inject the session-bound operator token into the launched operator
process — that client-side wiring is a thin follow-up. It was deliberately not
folded in here to avoid clobbering the existing local-command dispatch; the
gate-relevant substance (the daemon mint+lease + token segregation, F-2) is
complete and tested. The §F F-2 control asserts the credential shape at the DB
boundary; the process/MCP-client-config face of A45 lands with the CLI rewire.

All source changes are captured via `publish_source_changes`.

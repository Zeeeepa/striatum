# DRAFT — RFC 0167 P0 implementation (operator identity & run attribution), revision 2

author: author-author-003

This is the **attempt-2 revision** of the draft-lane implementation of **RFC 0167
P0** (D260), built contract-first against the falsification-cleared v4 SPEC
(`docs/operator/artifacts/rfc-0167-p0-design-v4/commit/proposal/PROPOSAL.md`).

Attempt 1 (`author-author-001`) implemented all ten §9 build-manifest items and
cleared the basic compile gate, but the reviewer (`reviewer-reviewer-001`,
`docs/operator/artifacts/rfc-0167-p0-build/review/REVIEW.md`) returned
`needs_revision` with **three blocking findings**. This revision discharges all
three; the attempt-1 substrate (owner bundle 0022, the C2″ composed-route
closure, the three converted star-readers, the projections, the lease/session
layer, the doctor advisory) is carried forward intact and confirmed by the
reviewer's positive checks. `cd go && go build ./...` and `go vet ./...` both
pass; the non-PG contract/route, owner-bundle, reservations, and read-authority
suites pass; `make check-docs` is green. The PG-backed two-role pgtests are
written to run the **real** RPC/authorization paths and execute in the verifier
stage.

## Revision map — the three blocking findings, discharged

### Finding 1 (BLOCKING) — A45 / §F F-2: the operator-bootstrap CLI/process path

**Was:** `striatum operator bootstrap` still rendered the local orientation packet
and never called the new `operator.bootstrap` RPC; the draft punted the CLI rewire
as a "thin follow-up" and the command matrix recorded it as a P0 follow-up. The
SPEC makes the CLI/process path part of P0 (`PROPOSAL.md:1143-1147`,
`PROPOSAL.md:536-539`), and A45's refuting observation is exactly "static
bootstrap-token injection or failure to prove absence from the routine path."

**Now:** `striatum operator bootstrap` **is** the client of the `operator.bootstrap`
RPC. After the daemon orientation reads, when authorized and a `repository_id`
resolves, it calls `operator.bootstrap`, mints the session-bound operator token
+ leases a handle, and **presents** the minted token to the launched operator
process / MCP client by writing it `0600` to `.striatum/scratch/operator-token`
— consumed via the `STRIATUM_MCP_TOKEN_FILE` env contract, which
`agentloop.ResolveTokenMaterial` resolves at **higher precedence than the static
runtime token** (`pkg/agentloop/token.go`). The daemon-root (static
`bootstrap-admin`) credential is used **only** to call the RPC and is never
written to that routine channel, so its absence from the routine repo-admin path
is structural. The raw token is never printed; the packet surfaces only the
non-secret identity (`handle`, `operator_session_id`, `expires_at`,
`capabilities`, `presentation_path`, `static_token_injected=false`).

- Files: `go/cmd/striatum/operator_bootstrap.go` (`operatorIdentitySummary`,
  `bootstrapOperatorIdentity`, `presentOperatorToken`, the markdown/JSON render,
  the `buildOperatorBootstrap` call site), `docs/reference/command-authority-matrix.md`
  (the `operator.bootstrap` row no longer defers the CLI rewire),
  `docs/reference/spec.md`, `CHANGELOG.md`.
- Discharges: **A45 / §F F-2** (the credential-segregation *process face*), with
  the DB-credential face proven by `operator_token_admin_surface` (below).

### Finding 2 (BLOCKING) — gate pgtests were seeded SQL shortcuts, not the real paths

**Was:** `operator_session_pre_run_stamp` hand-set `app.session_id` and inserted
directly into `striatumd.runs` (never calling the real `run.prepare`);
`operator_token_admin_surface` seeded a synthetic capability row and counted it
(never exercising `Authorize` for the method matrix or the `verifier.attest`
fence); `composed_identity_map_unreadable` Route 3 only checked FK-absence (never
ran the composed `cc ⋈ oh ⋈ runs ⋈ spawn_authorization_grants` query nor the
role-column-grants exception).

**Now:** all three are promoted to the **production RPC/authorization paths**
(`go/pkg/db/operator_identity_pg_test.go`, importing `mutations` + `rpc`):

- **`operator_session_pre_run_stamp`** mints each operator session via the real
  `mutations.HandleOperatorBootstrap` RPC, proves the minted token authorizes
  `run.prepare` through the real `rpc.PostgresAuthorizer.Authorize` (capability
  resolved from `rpc.MethodRegistry["run.prepare"]`), then creates each run via
  the real `mutations.HandleRunPrepare` RPC run **as the constrained runtime
  role** — so the two NON-NULL DISTINCT `created_by_handle_id` and distinct
  `whose` are proven through production authorization + the production stamp
  (**A29/A7/A27**). Adds the lane-no-admin denial (**A30/A43**) and the
  closed-session denial (**A31**) through the same authorizer.
- **`operator_token_admin_surface`** mints via the real RPC, then drives the real
  authorizer over the accepted surface (`run.prepare`, `checkpoint.resolve`,
  `review.override`, `branch.confirm` → allowed — **A40**), the real
  `mutations.HandleVerifierAttest` trust-root fence (typed `capability_denied` —
  **A41**), a daemon-global route denial (**A42**), a lane-token denial
  (**A43**), a closed-session denial (**A31**), and pins that the routine
  credential carries exactly `{admin, read}` — not the broad static set
  (**A45**).
- **`composed_identity_map_unreadable` Route 3** seeds the full
  `cc ⋈ oh ⋈ runs ⋈ spawn_authorization_grants` chain over an auto-spawn-captured
  grant and proves the composed route reconstructs `client_id → client_id` (the
  grant's `owner_principal_id` is the run owner's **client id**), plus the
  `has_column_privilege('striatumd_rw', …)` exception scan (the spawn-grant column
  is the one granted `*principal_id*`-named column; every real-principal identity
  column stays ungranted) and the retained FK-absence assertion (**A44 / §F F-1**).

The other seven named pgtests are carried forward unchanged.

### Finding 3 (BLOCKING) — `operator.heartbeat` recorded a future `last_heartbeat_at`

**Was:** `HandleOperatorHeartbeat` set `operator_handles.last_heartbeat_at = $1`
where `$1` was `expiresAt`, so handle liveness diagnostics read into the future
(the §3 shape is `last_heartbeat_at = now()`).

**Now:** the guarded renewal sets `leased_until = $1` (= `expiresAt`) and
`last_heartbeat_at = $2` (= `now`), matching §3 and the
`operator_sessions.last_heartbeat_at = now()` the same function already used.
File: `go/pkg/mutations/operator_session.go`.

## §9 build manifest — items, files, and discharged assertions/tests (carried, intact)

| § | Item | Files | Discharges |
|---|------|-------|-----------|
| 1 | **Owner bundle 0022** — `operator_handles` + `operator_sessions` + `runs` origin columns + write-once trigger + 2 DEFINER projections + column-scoped grants (runs REVOKE + column re-GRANT; `operator_handles` `principal_id` exclusion) + watermark stamp; `owner.go` label[22] + `LatestOwnerBundleVersion=22` + `RequiredOwnerBundleVersion=22` + `readScopeReasserts["operator_identity_run_attribution"]` | `go/pkg/db/sql/owner/0022_operator_identity_run_attribution.sql`, `go/pkg/db/owner.go`, `go/pkg/db/connection.go`, `go/pkg/db/sql/RESERVATIONS.toml`, `go/pkg/db/owner_revoke_filter_test.go`, `go/pkg/db/reservations_test.go` | A13–A19′, A28, A33–A39, A44 |
| 2 | **Lease + operator-session layer** — `defaultHandle`/escalation walk, `acquireOperatorHandle`, guarded heartbeat renewal (**§3 `now()` fix**), `operator_sessions` create/heartbeat/close-revoking-token | `go/pkg/mutations/operator_session.go` | A6–A12, A25, A27, A31 |
| 3 | **`mintOperatorSessionToken`** ({admin, read}); `sessionBoundCapabilities` UNCHANGED | `go/pkg/mutations/operator_session.go` | A29, A30, A32, A40, A43 |
| 4 | **Operator-bootstrap mint+lease RPC** (`operator.bootstrap`/`heartbeat`/`close`), **with `striatum operator bootstrap` as its CLI client presenting the session token** | `go/pkg/mutations/operator_session.go`, `go/pkg/mutations/mutations.go`, `contracts/daemon_methods.json`, `go/cmd/striatum/operator_bootstrap.go` | A3, A40, **A45** |
| 5 | **Run-origin stamp** via `admin.ResolvePrincipalForClient` + the `operator_handles` subquery on `app.session_id` | `go/pkg/mutations/run.go`, `go/pkg/mutations/wake.go` | A1, A2, A4, A5, A28 |
| 6 | **Identity reads via projections** — `whose` + contract + routes/registry + matrix + contract test; `status --mine` with bare-id fallback | `go/pkg/reads/whose.go`, `go/pkg/reads/status.go`, `contracts/daemon_methods.json`, `go/pkg/rpc/registry_methods.go`, `go/pkg/cli/routes/routes_generated.go`, `go/pkg/cli/params/params.go`, `docs/reference/command-authority-matrix.md` | A7, A38 |
| 7 | **Three `runs` star-readers** → explicit columns EXCLUDING `created_by_principal_id` | `go/pkg/reads/detail.go`, `go/pkg/reads/archive.go` | A38 |
| 8 | **Doctor `attribution_unknown` advisory** | `go/pkg/reads/doctor_attribution.go`, `go/pkg/reads/doctor.go` | A21 |
| 9 | **Ten named two-role pgtests — now driving the real RPC/authorization paths** | `go/pkg/db/operator_identity_pg_test.go` | A1, A6, A7, A11, A12, A14, A27–A31, A35–A45 |
| 10 | **Docs** — decision-log (D263), spec.md, command-authority-matrix.md, CHANGELOG.md | `docs/decisions/decision-log.md`, `docs/reference/spec.md`, `docs/reference/command-authority-matrix.md`, `CHANGELOG.md` | §F F-1/F-2 dispositions recorded |

## Verified owner-bundle ordinal: 0022

Re-verified at build time: ordinal **0021** is reserved by the RFC 0142 P4 C3
staged DDL-revoke (`DDLRevokeOwnerBundleVersion = 21`, `ownerBundleLabels[21]`
set, no `sql/owner/0021_*.sql`). So the new bundle is **0022**, and
`LatestOwnerBundleVersion`/`RequiredOwnerBundleVersion` advance from 20 to 22.
The frontier predicates are retargeted from the *frontier* to the *exact revoke
version* (`isNonRevokeBundle`, `RevokeBundleEmbedded`, the new
`IsOwnerBundleApplied` boot-barrier helper) so a normal apply-eligible 0022 above
the staged-terminal 0021 applies cleanly while the revoke stays
deploy-plan-terminal. The runs `GRANT SELECT(...)` list was regenerated from the
live catalog (the 0005 baseline columns + `created_by_handle_id`, minus
`created_by_principal_id`).

## §F binding constraints — discharged

- **F-1 / A44 (`C-C2DPRIME-SPAWN-GRANT-ENUMERATION`).** `composed_identity_map_unreadable`
  Route 3 now runs the composed `cc ⋈ oh ⋈ runs ⋈ spawn_authorization_grants`
  query over an auto-spawn grant and asserts it reconstructs `client_id → client_id`,
  plus the column-privilege exception scan (`owner_principal_id` is the granted
  client-id-holding exception; the real-principal identity columns stay ungranted)
  and the fail-loud FK-absence assertion. 0022 does not modify the table (P1
  custody is out of P0).
- **F-2 / A45 (`C-C1DPRIME-STATIC-TOKEN-SEGREGATION`).** The blast-radius is
  recorded as **static-segregated-plus-narrowed-routine**. The DB-credential face
  is pinned by `operator_token_admin_surface` (the minted operator token carries
  exactly `{admin, read}`, authorizes the accepted routine routes, is fenced at
  `verifier.attest`, and is unreachable for daemon-global admin). The **process
  face** is now real: `striatum operator bootstrap` calls `operator.bootstrap`
  and presents the session-bound token via `STRIATUM_MCP_TOKEN_FILE`; the static
  `bootstrap-admin` token is used only to call the RPC and is never the routine
  repo-admin credential.

## Build / verify status

- `cd go && go build ./...` — **pass**
- `cd go && go vet ./...` — **pass** (compiles every test file, incl. the rewritten pgtests)
- `go test ./pkg/rpc/ ./pkg/cli/routes/ ./pkg/cli/params/ ./pkg/cli/routestest/` — **pass**
- `go test ./pkg/db/ -run 'TestOwner|TestReservation|TestRequired|TestReadAuthority|TestRevoke'` — **pass**
- `go test ./pkg/db/ -run 'TestOperator'` — **skips cleanly without `STRIATUM_PG_TEST_URL`** (run live in the verifier stage)
- `go test ./cmd/striatum/` — **pass** (CLI rewire compiles + leaves the existing CLI tests green)
- `make check-docs` — **pass**

All source changes are captured via `publish_source_changes`.

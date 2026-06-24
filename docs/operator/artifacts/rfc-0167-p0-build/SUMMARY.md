---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
author: author-author-004
run_id: run_a4c3e73e4f7fca11826ba96b7823f4e3
date: 2026-06-24
title: "RFC 0167 P0 — applied implementation summary (operator identity & run attribution)"
tags: ["rfc-0167", "p0", "apply", "synthesis", "operator-identity", "a44", "a45"]
inputs:
  - "docs/operator/artifacts/rfc-0167-p0-design-v4/commit/proposal/PROPOSAL.md"
  - "docs/rfcs/0167-operator-identity-and-run-attribution.md"
  - "docs/operator/artifacts/rfc-0167-p0-build/DRAFT.md"
  - "docs/operator/artifacts/rfc-0167-p0-build/review/REVIEW.md"
  - "docs/operator/artifacts/rfc-0167-p0-build/DECISION_apply_with_doc_reconcile.md"
---

# RFC 0167 P0 — Applied implementation summary

This is the **apply-stage** synthesis for `rfc-0167-p0-build` (D260 / D263). It
finalizes the reviewed RFC 0167 P0 implementation (operator identity & run
attribution) by folding in the operator's accepted disposition and the round‑2
reviewer's one blocking finding. The source changes are captured via
`publish_source_changes`; this is the implementation the **verifier stage seals**
and the operator integrates to `main`.

## What apply changed (the doc/guardrail reconcile)

The round‑2 reviewer (`reviewer-reviewer-002`) returned `needs_revision` with a
single blocking finding: **A45 / §F F‑2 was not source‑truth coherent** — the code
already rewired `striatum operator bootstrap` to call the `operator.bootstrap` RPC
and present the session‑bound operator token, but authoritative docs and guardrail
text still described the command as the old read‑only local composite. The operator
**overrode** that verdict (`DECISION-rfc-0167-p0-build-doc-reconcile`,
`accepted_with_follow_up`): the implementation is otherwise GREEN and the
contradiction is a doc/code consistency gap, not a code defect. The required
follow‑up — reconcile docs/guardrails to state the A45 CLI rewire is implemented in
P0 — is discharged here:

| # | File | Reconcile |
|---|------|-----------|
| 1 | `docs/reference/spec.md` | Cold‑start section (§ AI‑operator cold start) no longer calls `operator bootstrap` a "CLI‑local read composite, not a daemon RPC method and not a new live‑state authority". It is now a custom CLI‑local entrypoint that is the client of `operator.bootstrap`: it mints + presents the session‑bound `{admin, read}` token (written `0600` to `.striatum/scratch/operator-token`, consumed via `STRIATUM_MCP_TOKEN_FILE`; raw token never embedded). Now consistent with the RFC 0167 P0 section already in the same file. |
| 2 | `docs/decisions/decision-log.md` | D263 "**Remaining thin follow-up:** the `striatum operator bootstrap` CLI local-command rewire…" replaced with "**IMPLEMENTED in P0**, not deferred", with the segregation accounting (A45 / §F F‑2). |
| 3 | `go/pkg/cli/localcommands/localcommands.go` | `operator bootstrap` rationale no longer says "bounded read-only … creates no new live state"; it is a custom CLI‑local entrypoint that also calls `operator.bootstrap` (live operator‑session/handle state is daemon‑owned via that RPC). Documentation‑only field (operator is dispatched at `main.go:91` before `localcommands.Lookup`), no behavioral change. |
| 4 | `go/pkg/cli/routestest/routes_freshness_test.go` | `TestOperatorBootstrapIsNotGeneratedDaemonRoute` message reconciled: `operator bootstrap` stays a custom CLI‑local entrypoint, **not** a generated 1:1 route (`operator.bootstrap` is a daemon **method** with no `cli_routes` entry). The assertion (it must not appear in `routes.All()`) is unchanged and still passes. |
| 5 | `go/cmd/striatum/operator_bootstrap.go` | `printOperatorHelp` "Read-only bounded cold-start packet" corrected to note it also calls `operator.bootstrap` to mint + present the session‑bound operator token. |

No source behavior changed in apply — these are coherence edits to the
authoritative product spec, the decision log, the local‑command rationale, the
route‑freshness message, and the CLI help string so an operator reading the source
of truth gets one consistent account of the A45 rewire.

## Verified owner‑bundle ordinal: **0022** (next‑free, re‑verified)

Re‑verified at apply time from the live catalog and constants:

- `go/pkg/db/sql/owner/` contains `0020_owner_bundle_watermark_read.sql` and the
  new `0022_operator_identity_run_attribution.sql`. There is **no** `0021_*.sql`:
  ordinal **0021** is reserved by the RFC 0142 P4 C3 staged DDL‑revoke
  (`DDLRevokeOwnerBundleVersion = 21`, no embedded SQL file). So **0022** is the
  next‑free embedded ordinal above the staged‑terminal 0021.
- `LatestOwnerBundleVersion = 22`; `RequiredOwnerBundleVersion = LatestOwnerBundleVersion`
  (= 22). The frontier predicates are retargeted from the *frontier* to the *exact
  revoke version* (`isNonRevokeBundle(v) == (v != 21)`, `RevokeBundleEmbedded`, and
  the `IsOwnerBundleApplied(version)` boot‑barrier helper in `connection.go`) so a
  normal apply‑eligible 0022 applies cleanly while the 0021 revoke stays
  deploy‑plan‑terminal.
- The runs `GRANT SELECT(...)` list was regenerated from the catalog (0005 baseline
  columns + `created_by_handle_id`, **minus** `created_by_principal_id`).

## §9 build manifest — files changed grouped by item (carried from draft + apply edits)

| § | Item | Files | Discharges |
|---|------|-------|-----------|
| 1 | Owner bundle 0022 (handles + sessions + runs origin columns + write‑once trigger + 2 DEFINER projections + column‑scoped grants incl. runs REVOKE/re‑GRANT and `operator_handles.principal_id` exclusion + watermark; `owner.go` label[22] + `Latest`/`RequiredOwnerBundleVersion=22` + `readScopeReasserts["operator_identity_run_attribution"]`) | `go/pkg/db/sql/owner/0022_operator_identity_run_attribution.sql`, `go/pkg/db/owner.go`, `go/pkg/db/connection.go`, `go/pkg/db/sql/RESERVATIONS.toml`, `go/pkg/db/owner_revoke_filter_test.go`, `go/pkg/db/reservations_test.go` | A13–A19′, A28, A33–A39, A44 |
| 2 | Lease + operator‑session layer (`defaultHandle`/escalation walk, `acquireOperatorHandle`, guarded heartbeat renewal with the §3 `last_heartbeat_at = now()` fix, `operator_sessions` create/heartbeat/close‑revoking‑token) | `go/pkg/mutations/operator_session.go` | A6–A12, A25, A27, A31 |
| 3 | `mintOperatorSessionToken` (`{admin, read}`); `sessionBoundCapabilities` UNCHANGED | `go/pkg/mutations/operator_session.go` | A29, A30, A32, A40, A43 |
| 4 | Operator‑bootstrap mint+lease RPC (`operator.bootstrap`/`heartbeat`/`close`) **with `striatum operator bootstrap` as its CLI client presenting the session token** | `go/pkg/mutations/operator_session.go`, `go/pkg/mutations/mutations.go`, `contracts/daemon_methods.json`, `go/cmd/striatum/operator_bootstrap.go` | A3, A40, **A45** |
| 5 | Run‑origin stamp via `admin.ResolvePrincipalForClient` + `operator_handles` subquery on `app.session_id` | `go/pkg/mutations/run.go`, `go/pkg/mutations/wake.go` | A1, A2, A4, A5, A28 |
| 6 | Identity reads via projections — `whose` (+ contract + routes/registry + matrix + contract test) and `status --mine` with bare‑id fallback | `go/pkg/reads/whose.go`, `go/pkg/reads/status.go`, `contracts/daemon_methods.json`, `go/pkg/rpc/registry_methods.go`, `go/pkg/cli/routes/routes_generated.go`, `go/pkg/cli/params/params.go`, `docs/reference/command-authority-matrix.md` | A7, A38 |
| 7 | Three `runs` star‑readers → explicit columns EXCLUDING `created_by_principal_id` | `go/pkg/reads/detail.go`, `go/pkg/reads/archive.go` | A38 |
| 8 | Doctor `attribution_unknown` advisory | `go/pkg/reads/doctor_attribution.go`, `go/pkg/reads/doctor.go` | A21 |
| 9 | Ten named two‑role pgtests driving the **real** RPC/authorization paths | `go/pkg/db/operator_identity_pg_test.go` | A1, A6, A7, A11, A12, A14, A27–A31, A35–A45 |
| 10 | Docs + apply reconcile (decision‑log D263, spec.md, command‑authority‑matrix, CHANGELOG; localcommands rationale; routes‑freshness message; CLI help) | `docs/decisions/decision-log.md`, `docs/reference/spec.md`, `docs/reference/command-authority-matrix.md`, `CHANGELOG.md`, `go/pkg/cli/localcommands/localcommands.go`, `go/pkg/cli/routestest/routes_freshness_test.go`, `go/cmd/striatum/operator_bootstrap.go` | §F F‑1/F‑2 dispositions recorded; A45 doc face |

## Gate‑critical assertions A35–A45 → discharging test

The ten named two‑role pgtests live in `go/pkg/db/operator_identity_pg_test.go` as
`TestOperator…TwoRole` functions (logical names 1–10 in the file comments). The
gate‑critical assertions and the test that discharges each:

| Assertion | Claim | Discharging test (function / logical name) |
|-----------|-------|--------------------------------------------|
| **A35** | C2″ Route 1 closed — `cc ⋈ oh` on `oh.principal_id` fails `42501` | `TestOperatorComposedIdentityMapUnreadableTwoRole` (`composed_identity_map_unreadable` Route 1) |
| **A36** | C2″ Route 2 closed — `cc ⋈ oh ⋈ runs` on `created_by_principal_id` fails `42501` | `TestOperatorComposedIdentityMapUnreadableTwoRole` (Route 2) |
| **A37** | No third **principal** route over the ACL graph | `TestOperatorComposedIdentityMapUnreadableTwoRole` (`role_column_grants` `*principal_id*` ACL scan) |
| **A38** | Column‑revoke breaks no other read path | `TestOperatorWhoseStatusMineViaProjectionTwoRole` (`whose_status_mine_via_projection`); star‑readers converted (§9 item 7) |
| **A39** | Drift‑proof gate — reassert re‑closes a stray GRANT | `TestOperatorDriftReassertReclosesRoutesTwoRole` (`drift_reassert_recloses_routes`) |
| **A40** | Accepted operator‑admin surface authorized (`run.prepare`, `checkpoint.resolve`, `review.override`, `branch.confirm`) | `TestOperatorTokenAdminSurfaceTwoRole` (`operator_token_admin_surface`) |
| **A41** | Trust‑root fenced — `verifier.attest` refuses the session‑bound operator token (typed `capability_denied`) | `TestOperatorTokenAdminSurfaceTwoRole` (via `HandleVerifierAttest`) |
| **A42** | Repo‑scope bound — daemon‑global admin (`daemon.token.create`/`shutdown`) unreachable | `TestOperatorTokenAdminSurfaceTwoRole` |
| **A43** | Lane ≠ admin (unchanged `sessionBoundCapabilities`) | `TestOperatorTokenAdminSurfaceTwoRole` + `TestOperatorSessionPreRunStampTwoRole` (lane‑no‑admin denial) |
| **A44** | **(BINDING §F F‑1)** Spawn‑grant is a client‑id exception, NOT a third principal route | `TestOperatorComposedIdentityMapUnreadableTwoRole` (`composed_identity_map_unreadable` **Route 3**: `cc ⋈ oh ⋈ runs ⋈ spawn_authorization_grants`) |
| **A45** | **(BINDING §F F‑2)** Credential segregation — routine repo‑admin uses the session token; static bootstrap token segregated | `TestOperatorTokenAdminSurfaceTwoRole` (DB face: token carries exactly `{admin, read}`) **+** `go/cmd/striatum/operator_bootstrap.go` (process face: CLI presents the minted token, never the static `bootstrap-admin`) |

Supporting (non‑gate) assertions also proven by the suite: **A1** (stamp = live‑token
principal via projection), **A6** (live‑unique forces distinct words), **A7** (two
terminals → distinct `whose`, gate‑critical R1b), **A11** (one winner, no deadlock),
**A12** (flap‑resistant renewal), **A14** (write‑once at the DB), **A27** (operator
session buildable pre‑run), **A28** (two‑role stamp safety via projection), **A29**
(operator token authorizes `run.prepare`), **A30/A31** (lane/closed‑session
denials) — across `TestOperatorSessionPreRunStampTwoRole`,
`TestOperatorForgedUpdateCreatedByRejectedTwoRole`, `TestOperatorTwoLiveMayaTwoRole`,
`TestOperatorTokenRevokedBareIDTwoRole`, `TestOperatorLeaseFlapStealTwoRole`,
`TestOperatorOwnerBundleAppliesCleanTwoRole`.

## §F binding‑constraint dispositions

- **F‑1 / A44 (`C-C2DPRIME-SPAWN-GRANT-ENUMERATION`).** `spawn_authorization_grants.owner_principal_id`
  is a **bare client‑id column with NO FK to `principals`** (holds the run owner's
  *client* id). The composed Route 3 query (`cc ⋈ oh ⋈ runs ⋈ spawn_authorization_grants`)
  over an auto‑spawn‑captured grant reconstructs `client_id → client_id`, **not** a
  principal leak; the `information_schema.role_column_grants` `*principal_id*` scan
  records it as the asserted client‑id‑holding exception (fails loudly if a future
  change makes it a real principal), and the FK‑absence assertion is retained. Owner
  bundle 0022 does **not** modify the table (P1 custody is out of P0 scope). Disposition:
  client‑id exception, fail‑loud control in place.
- **F‑2 / A45 (`C-C1DPRIME-STATIC-TOKEN-SEGREGATION`).** Blast‑radius recorded as
  **static‑segregated‑plus‑narrowed‑routine**. *DB‑credential face:* the minted
  operator token carries exactly `{admin, read}`, authorizes the accepted routine
  routes, is fenced at `verifier.attest`, and cannot reach daemon‑global admin
  (`operator_token_admin_surface`). *Process face:* `striatum operator bootstrap`
  calls `operator.bootstrap` and presents the session‑bound token via
  `STRIATUM_MCP_TOKEN_FILE` (`agentloop.ResolveTokenMaterial` reads it at higher
  precedence than the static runtime token); the static `bootstrap-admin` token is
  used only to call the RPC and is **structurally absent** from the routine
  repo‑admin path. The apply reconcile makes the spec/decision‑log/guardrail text
  agree with this implemented face.

## Build / gate status (apply lane, this host)

- `cd go && go build ./...` — **pass**
- `cd go && go vet ./...` — **pass** (compiles every test file, incl. the pgtests)
- `go test ./pkg/rpc/ ./pkg/cli/routes/ ./pkg/cli/params/ ./pkg/cli/routestest/ ./pkg/cli/localcommands/` — **pass** (incl. the reconciled `routes_freshness_test`)
- `go test ./cmd/striatum/` — **pass** (CLI rewire + reconciled help compile; CLI tests green)
- `go test ./pkg/db/ -run 'TestOwner|TestReservation|TestRequired|TestReadAuthority|TestRevoke'` — **pass**
- `go test ./pkg/db/ -run 'TestOperator'` — **skips cleanly without `STRIATUM_PG_TEST_URL`** (runs live in verify)
- `make check-docs` — **pass**

## Residuals the verifier stage must prove

1. **Live two‑role pgtests.** The ten `TestOperator…TwoRole` tests skip without a
   live `STRIATUM_PG_TEST_URL`. The verifier stage must run them **live under the
   two‑role OwnerPool fixture** to seal the security properties: A35–A39 (composed
   read‑scope closure incl. Route 3 spawn‑grant + drift reassert), A40–A43 (admin
   surface + `verifier.attest` fence + repo‑scope bound), **A44** (spawn‑grant
   client‑id exception), **A45** (credential‑segregation DB face), and the supporting
   A1/A6/A7/A11/A12/A14/A27–A31.
2. **Clean owner‑bundle 0022 apply** (A13–A19′) under the non‑superuser owner role,
   plus the write‑once trigger (`forged_update_created_by_rejected`) and the
   forward‑only/idempotent REVOKE — all only fully exercised live.
3. **spec.md / D263 doc consistency** — reconciled in this apply lane; the operator
   verifies the spec/decision‑log coherence before the build integrates to `main`.
4. **`docs/operator/rfc-roadmap.md` re‑triage when P0 ships** — out of this lane's
   write scope (it is under `docs/operator/` but not the build artifact dir). This is
   the operator's integration‑time action, not an apply edit, and is flagged here so
   it is not lost.

All source changes are captured via `publish_source_changes`.

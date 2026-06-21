---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
author: adjudicator-author-001
workflow: "fg_rfc0142_design"
run_id: "run_468d26b3c2745cc0aae764070b3d2e53"
cycle: 1
topic: "RFC 0142 P0 — two-role pgtest fixture (safe-by-construction DB-change deployment)"
participants: ["holder", "falsifier_1", "falsifier_2", "adjudicator"]
verdict: "accept_with_findings"
rationale: "Material challenges landed — but each strikes the holder's specific fixture-construction recipe, not the P0 shape. The recipe defects are: a SET ROLE-inside-a-privileged-connection design that false-greens a 'RESET ROLE; ALTER owner-table' migration; a Phase-A-as-owner bootstrap that leaves recent runtime-created tables (supervisor_buffered_packets/0038, event_chain_segments/0041, verifier_attestations/0042) owner-owned and so false-reds a legal runtime ALTER; a non-superuser ownership-transfer abort during bootstrap; and a §3.6 self-check that asserts rolsuper and direct relowner but misses an inherited-membership edge. No falsifier showed that a privilege-constrained role against a real-bundle-bootstrapped DB cannot reproduce the prod 42501 — the holder's ownership-vs-membership argument (ALTER and REFERENCES gate on ownership / the REFERENCES privilege, neither conveyed by INHERIT) stands. The two-role executable-oracle slice is therefore the right foundation; it clears WITH four binding constraints (escape-proof login role, bootstrap ownership fidelity plus a differential gate, non-superuser bootstrap privilege, and a role-isolation self-check) and one minor search_path-fidelity hardening, with the dual-deploy-path generalization flagged forward to the Layer 3/4 deployer. Verdict: accept_with_findings."
entries:
  - kind: claim
    by: holder
    refs: ["dialogue:1"]
    text: "A two-role pgtest fixture that bootstraps the real owner/runtime ownership topology, runs the migration suite as a privilege-constrained striatumd_rw, and asserts SQLSTATE 42501 fires on an owner-table touch (ALTER striatumd.events; FK into striatumd.repositories) but not on a legal runtime operation, is the executable oracle that reds exactly the migrations that 42501 in prod and greens the rest — no false reds, no false greens. P0 is test-harness plus test code only (pgtest.go and two_role_pg_test.go), adds no migration/bundle/daemon change, and derives 'owner-held' from live pg_class.relowner after the real bundles apply (one source, no drift)."
  - kind: challenge
    by: falsifier_1
    refs: ["dialogue:2"]
    text: "RESET ROLE escape / false green: Phase B uses SET ROLE striatumd_rw inside a connection whose session/login user is the privileged DSN owner/superuser. A candidate migration containing 'RESET ROLE;' or 'SET ROLE NONE;' resets the active role to the session (owner) user and so passes an illegal owner-table ALTER in the fixture, while prod — which logs in directly as striatumd_rw with no owner in the session ancestry — 42501s and crash-loops."
  - kind: challenge
    by: falsifier_1
    refs: ["dialogue:2"]
    text: "Bootstrap ownership drift / false red for recent runtime tables: Phase A applies ALL runtime migrations 0001-0042 as the owner DSN user, so tables created by recent runtime migrations and never transferred by a bundle — supervisor_buffered_packets (0038), event_chain_segments (0041), verifier_attestations (0042) — are owner-owned in the fixture. In prod those tables were created by striatumd_rw on boot and are striatumd_rw-owned. A legal runtime ALTER on them 42501s in the fixture but succeeds in prod: a false red that breaks the no-false-reds half of the claim."
  - kind: challenge
    by: falsifier_1
    refs: ["dialogue:2"]
    text: "search_path / temp-object resolution fidelity: under SET ROLE the search_path defaults to the session (owner) user's configuration; relation resolution or pg_temp shadowing could resolve a migration's relations differently from prod, so the oracle could evaluate against the wrong relation."
  - kind: challenge
    by: falsifier_2
    refs: ["dialogue:3"]
    text: "Non-superuser ownership-transfer abort / broken bootstrap: ApplyOwnerBundles issues ALTER TABLE ... OWNER TO striatumd_rw. A non-superuser owner DSN cannot reassign ownership to a role it is not a member of, and ensureRuntimeRole never grants that membership, so Phase A aborts with 'must be member of role striatumd_rw' (42501) under a least-privilege non-superuser DSN. The fixture only works when the DSN is a superuser — which the holder's own R1 argues is less prod-faithful."
  - kind: challenge
    by: falsifier_2
    refs: ["dialogue:3"]
    text: "Dual deploy-path ownership divergence / false green for admin-deploys: prod has two paths — boot-time auto-migrate as striatumd_rw (new tables striatumd_rw-owned) and out-of-band admin apply as the owner (new tables owner-owned). The fixture models only the boot path, so a create-then-ALTER migration greens in the fixture but would 42501 if applied via the admin/owner DSN."
  - kind: challenge
    by: falsifier_2
    refs: ["dialogue:3"]
    text: "Inherited-membership false green / under-asserted self-check: the §3.6 self-check asserts rolsuper=false and relowner(events) is not striatumd_rw, but not the ABSENCE of a membership/inheritance edge. In a reused cluster (or after the bootstrap grant of striatumd_rw to the owner), if striatumd_rw inherits from the owner role the illegal ALTER succeeds — a false green the self-check does not catch — while prod (INHERIT disabled / no such edge) 42501s."
  - kind: rebuttal
    by: adjudicator
    refs: ["dialogue:1"]
    text: "Shape survives. No falsifier showed that SET ROLE / a privilege-constrained role against a real-bundle-bootstrapped DB cannot reproduce the prod 42501; the holder's §3.3 ownership-vs-membership argument stands (ALTER/DROP gate on ownership and an inbound FK on the REFERENCES privilege, neither conveyed by role membership under INHERIT). Every landed challenge is a fixture-construction defect fixable in the build, not a shape-level false-green/false-red the design cannot close. Hence accept_with_findings, not needs_revision."
  - kind: rebuttal
    by: adjudicator
    refs: ["dialogue:1", "dialogue:2"]
    text: "RESET ROLE escape LANDS and is exactly the gap the holder flagged but left optional in R3 ('a dedicated striatumd_rw LOGIN shell is an acceptable stricter alternative'). Converted to binding C1: Phase B must execute as a dedicated non-superuser, non-owner LOGIN role, not SET ROLE inside a privileged connection — promoting the holder's optional to a hard requirement."
  - kind: rebuttal
    by: adjudicator
    refs: ["dialogue:2", "dialogue:3"]
    text: "The three bootstrap-fidelity challenges LAND against the Phase-A-as-owner recipe, not the shape, and convert to binding C2 (bootstrap must reproduce prod's per-table relowner, gated by a differential check — the holder's §3.7 'live relowner is ground truth' is only true once the bootstrap yields prod's relowner), C3 (Phase A must provision the membership the transfers need and succeed under a non-superuser DSN, then revoke it), and C4 (the self-check must assert striatumd_rw neither belongs to nor inherits from the owner role at probe time, after C3's grant is revoked). C3 and C4 interact: C3's temporary grant must not become C4's leak."
  - kind: rebuttal
    by: adjudicator
    refs: ["dialogue:2", "dialogue:3"]
    text: "Falsifier_1's search_path challenge is largely defused by its own rebuttal (the 42501 gates on the resolved relation's ownership regardless of search_path) but yields cheap fidelity hardening C5 (pin search_path to prod's value). Falsifier_2's dual-deploy-path challenge is material but is a property of the FUTURE Layer 3/4 deployer — the SEED anchors show no current out-of-band runtime-migration-as-owner path — so it does not foreclose P0 and is recorded as finding F6, deferred to the P3/P4 deployer, not a P0 binding constraint."
findings:
  - id: F1
    severity: high
    posture: fixture_fidelity
    status: converted_to_constraint
    challenge: "SET ROLE striatumd_rw inside a privileged owner/superuser connection lets a candidate migration RESET ROLE / SET ROLE NONE back to the owner and pass an illegal owner-table ALTER — a false green relative to prod, where login is directly striatumd_rw."
    closest_acceptable_answer: "Run Phase B as a dedicated non-superuser, non-owner LOGIN role so RESET ROLE falls back to a constrained login, as in prod."
    source_refs: ["dialogue:2"]
  - id: F2
    severity: high
    posture: bootstrap_fidelity
    status: converted_to_constraint
    challenge: "Phase A applies runtime migrations 0001-0042 as the owner, so recent runtime-created tables never transferred by a bundle (supervisor_buffered_packets/0038, event_chain_segments/0041, verifier_attestations/0042) are owner-owned in the fixture though striatumd_rw-owned in prod; a legal runtime ALTER on them false-reds."
    closest_acceptable_answer: "Bootstrap must yield prod's actual per-table relowner: apply historical runtime migrations as striatumd_rw in prod-faithful order, or assert the resulting relowner set against a prod ownership manifest, gated by a differential check."
    affected_invariants: ["one_source_no_drift", "no_false_reds"]
    source_refs: ["dialogue:2", "dialogue:3"]
  - id: F3
    severity: high
    posture: bootstrap_fidelity
    status: converted_to_constraint
    challenge: "ALTER TABLE ... OWNER TO striatumd_rw in ApplyOwnerBundles requires the executing role to be a member of striatumd_rw; ensureRuntimeRole never grants that membership, so Phase A aborts with 'must be member of role striatumd_rw' under a non-superuser owner DSN, making the fixture depend silently on a superuser DSN."
    closest_acceptable_answer: "Phase A grants striatumd_rw to CURRENT_USER (or otherwise provisions the transfer privilege) before ApplyOwnerBundles and revokes it before Phase B; the DSN privilege requirement is declared and asserted, not silently superuser."
    source_refs: ["dialogue:3"]
  - id: F4
    severity: high
    posture: fixture_fidelity
    status: converted_to_constraint
    challenge: "The §3.6 self-check asserts rolsuper=false and relowner(events) is not striatumd_rw but not the absence of a membership/inheritance edge; in a reused cluster or after F3's grant, an inherited owner-membership lets the illegal ALTER succeed — a false green the self-check misses."
    closest_acceptable_answer: "Self-check asserts at probe time, after any bootstrap grant is revoked, that striatumd_rw is not a member of and does not inherit from the owner role (pg_has_role / pg_auth_members), aborting loudly otherwise."
    source_refs: ["dialogue:3"]
  - id: F5
    severity: low
    posture: fixture_fidelity
    status: converted_to_constraint
    challenge: "Under SET ROLE the search_path defaults to the session (owner) user's configuration, so relation resolution / pg_temp shadowing could diverge from prod; low impact because the 42501 oracle gates on the resolved relation's ownership regardless of search_path."
    closest_acceptable_answer: "Pin the Phase B SUT connection's search_path (and any session GUCs prod sets) to prod's value."
    source_refs: ["dialogue:2"]
  - id: F6
    severity: medium
    posture: scope
    status: deferred_with_owner
    challenge: "Out-of-band admin apply (owner DSN) would create runtime tables owner-owned, diverging from the boot-path ownership the fixture models — a false green for admin-deploys. No current out-of-band runtime-migration-as-owner path exists in the SEED anchors; this is a property of the future Layer 3/4 deployer, which can apply a step under either role."
    closest_acceptable_answer: "Layer 3/4 deployer fixture models BOTH role-paths (or structurally constrains runtime-table creation to the runtime role); P0's oracle is honest for the current boot model once its scoping is stated."
    source_refs: ["dialogue:3"]
constraints:
  - id: C1
    source_finding: F1
    posture: fixture_fidelity
    severity: high
    kind: invariant
    binding: true
    text: "Phase B (system-under-test execution) MUST run through a connection whose SESSION/LOGIN user is a dedicated, non-superuser, non-owner role that is not a member of and does not inherit from the owner role — NOT 'SET ROLE striatumd_rw' inside a privileged owner/superuser connection. This makes RESET ROLE / SET ROLE NONE fall back to a constrained login as in prod, closing the false-green escape. Promotes holder R3's optional 'dedicated striatumd_rw LOGIN shell' to a hard requirement."
    source_refs: ["dialogue:2"]
    verification:
      gate: "A regression test runs 'RESET ROLE; ALTER TABLE striatumd.events ADD COLUMN p0_probe integer' as the SUT runner and asserts it still fails with SQLSTATE 42501; the SUT pool exposes no path back to the owner/DSN login."
    final_review_required: true
  - id: C2
    source_finding: F2
    posture: bootstrap_fidelity
    severity: high
    kind: invariant
    binding: true
    text: "The Phase A bootstrap MUST reproduce production's actual per-table ownership: every relation prod leaves striatumd_rw-owned (runtime-migration-created tables not transferred to the owner, including supervisor_buffered_packets/0038, event_chain_segments/0041, verifier_attestations/0042) MUST be striatumd_rw-owned in the fixture, and every owner-held relation owner-owned. 'Apply all runtime migrations as the owner at bootstrap' is insufficient — it inverts ownership for that cohort and produces false reds. Discharge by applying historical runtime migrations as striatumd_rw in prod-faithful order, or by asserting the resulting pg_class.relowner set equals a checked-in prod ownership manifest. The holder's 'one source / live relowner is ground truth' (§3.7 / C4) holds ONLY once the bootstrap yields prod's relowner."
    source_refs: ["dialogue:2", "dialogue:3"]
    verification:
      gate: "A differential test asserts that live pg_class.relowner after bootstrap matches the static-guard-derived runtime-owned set (runtimeOwnedTablesAlterable) and a prod ownership manifest; a legal runtime ALTER on each striatumd_rw-owned recent table (verifier_attestations, event_chain_segments, supervisor_buffered_packets) succeeds while an owner-held ALTER 42501s."
    final_review_required: true
  - id: C3
    source_finding: F3
    posture: bootstrap_fidelity
    severity: high
    kind: gate
    binding: true
    text: "Phase A bootstrap MUST succeed under a NON-superuser owner DSN. The ALTER TABLE ... OWNER TO striatumd_rw transfers require the executing role to be a member of striatumd_rw, so the fixture must explicitly provision that membership (e.g. GRANT striatumd_rw TO CURRENT_USER before ApplyOwnerBundles) and REVOKE it before Phase B (see C4) — or declare and assert the exact DSN privilege it requires. The fixture must not silently depend on a superuser DSN."
    source_refs: ["dialogue:3"]
    verification:
      gate: "Bootstrap runs green under a non-superuser owner DSN in CI; a setup assertion proves the transfer membership is revoked before the first SUT probe so it cannot leak into Phase B."
    final_review_required: true
  - id: C4
    source_finding: F4
    posture: fixture_fidelity
    severity: high
    kind: gate
    binding: true
    text: "The fixture self-check MUST assert, at probe time in Phase B (after any C3 bootstrap grant is revoked), that the SUT role is not a member of and does not inherit privileges from the owner role — via pg_has_role() / pg_auth_members — in addition to rolsuper=false and relowner(events) is not the SUT role. This closes the inherited-membership false green in reused/polluted clusters and guards against C3's temporary grant leaking; the fixture aborts loudly if isolation does not hold, so a red 42501 is only trusted when isolation holds."
    source_refs: ["dialogue:3"]
    verification:
      gate: "A self-check assertion fails the fixture if pg_has_role(SUT_role, owner_role, 'MEMBER') or pg_has_role(..., 'USAGE') is true, or any pg_auth_members edge grants the SUT role owner privileges; the red-test 42501 assertion runs only after isolation is proven."
    final_review_required: true
  - id: C5
    source_finding: F5
    posture: fixture_fidelity
    severity: low
    kind: policy
    binding: false
    text: "The Phase B SUT connection SHOULD pin search_path (and any session GUCs prod sets, e.g. SET search_path TO striatumd, public) to prod's value so relation resolution and pg_temp shadowing cannot diverge from prod. Low severity — the 42501 oracle gates on the resolved relation's ownership regardless of search_path — but a cheap fidelity hardening worth folding into the fixture."
    source_refs: ["dialogue:2"]
    verification:
      expected_stage: "P0 build sets and asserts the SUT pool search_path in the two-role fixture constructor."
    final_review_required: false
branches:
  p0_two_role_fixture: cleared_with_constraints
---

# Collaboration Ledger — RFC 0142 P0 (Cycle 1)

**Verdict: `accept_with_findings`.** The P0 two-role pgtest fixture is the right
first slice. The falsifiers landed real, material gaps — but every one is a
*fixture-construction* defect fixable in the build, not a defect in the P0
*shape*. The four binding constraints (C1–C4) plus one minor hardening (C5) are
the build's discharge list; the dual-deploy-path generalization (F6) is flagged
forward to the Layer 3/4 deployer.

## Dialogue reference map

The `dialogue:<seq>` refs in the front matter map to the curated turns:

| seq | participant | artifact |
| --- | --- | --- |
| `dialogue:1` | holder | `dialogue/holder/HOLDER.md` |
| `dialogue:2` | falsifier_1 | `dialogue/falsifier_1/FALSIFIER.md` |
| `dialogue:3` | falsifier_2 | `dialogue/falsifier_2/FALSIFIER.md` |

## Why the shape survives (the claim under attack)

The P0 load-bearing claim is that a privilege-constrained `striatumd_rw` run
against a real-bundle-bootstrapped DB reds exactly the prod-`42501` migrations
and greens the rest. The shape is invalidated **only** if a falsifier shows that
`SET ROLE`/a constrained role against such a DB **cannot reproduce the prod
`42501` at all** — e.g. that membership/`INHERIT`/`SECURITY DEFINER`/default-
privilege semantics make `striatumd_rw` effectively own or `REFERENCE` the owner
tables. No falsifier showed this. The holder's §3.3 mechanism stands:
`ALTER`/`DROP` gate on table **ownership** and an inbound FK on the
**`REFERENCES`** privilege, and **neither is conveyed by role membership under
`INHERIT`**. Every landed challenge attacks how the fixture is *built*, which the
build can fix — so the verdict clears with constraints rather than blocking.

## Per-challenge adjudication

### Challenge 1 — `RESET ROLE` escape (falsifier_1, `dialogue:2`) → **C1**
- **Landed?** Yes. A real false green: `RESET ROLE;`/`SET ROLE NONE;` in a
  candidate migration escapes back to the privileged session user, so an illegal
  owner-table `ALTER` passes in the fixture but `42501`s in prod (which logs in
  directly as `striatumd_rw`).
- **Holder's answer.** Partially pre-empted: R3 acknowledged the escape and
  offered "a dedicated `striatumd_rw` LOGIN shell is an acceptable stricter
  alternative" — but left it *optional*.
- **Residual gap.** The optional must become mandatory; `SET ROLE` inside a
  privileged connection is not escape-proof.
- **Binding constraint: C1** — Phase B must run as a dedicated non-superuser,
  non-owner LOGIN role.

### Challenge 2 — bootstrap ownership drift / recent runtime tables (falsifier_1, `dialogue:2`; reinforced by falsifier_2, `dialogue:3`) → **C2**
- **Landed?** Yes — and this is the most material finding. The holder's
  Phase-A-**as-owner** bootstrap leaves `supervisor_buffered_packets` (0038),
  `event_chain_segments` (0041), and `verifier_attestations` (0042) **owner-owned**
  in the fixture, whereas prod (runtime role applies these on boot) leaves them
  **`striatumd_rw`-owned**. A legal runtime `ALTER` on them is a **false red**.
- **Holder's answer.** The holder's §3.7 / R4 / C4 assert "live `relowner` is
  ground truth, one source, no drift." That is true of the *derivation*, but the
  *bootstrap* does not reproduce prod's `relowner` for the recent-runtime cohort,
  so the ground truth is the wrong truth.
- **Residual gap.** The bootstrap must produce prod's actual per-table ownership;
  the differential check (holder C4) must gate it.
- **Binding constraint: C2** — bootstrap ownership fidelity + differential gate.

### Challenge 3 — non-superuser ownership-transfer abort (falsifier_2, `dialogue:3`) → **C3**
- **Landed?** Yes. `ALTER ... OWNER TO striatumd_rw` requires the executing role
  to be a *member* of `striatumd_rw`; `ensureRuntimeRole` never grants it, so
  Phase A aborts under a non-superuser owner DSN — making the fixture silently
  depend on a superuser DSN, which the holder's own R1 argues is less prod-faithful.
- **Holder's answer.** R1 contemplated a non-superuser owner for fidelity but did
  not address the `OWNER TO` membership requirement, so the bootstrap as written
  breaks for that case.
- **Residual gap.** Phase A must provision the transfer privilege (e.g. a scoped
  `GRANT striatumd_rw TO CURRENT_USER`) and revoke it before Phase B.
- **Binding constraint: C3** — non-superuser bootstrap privilege.

### Challenge 4 — inherited-membership false green / under-asserted self-check (falsifier_2, `dialogue:3`) → **C4**
- **Landed?** Yes. The §3.6 self-check checks `rolsuper` and direct `relowner`
  but not membership/inheritance edges; in a reused cluster — or after C3's
  bootstrap grant — `striatumd_rw` inheriting from the owner would false-green the
  red test.
- **Holder's answer.** The holder's C1/C2 self-checks are the right idea but stop
  short of asserting role isolation.
- **Residual gap.** Assert `striatumd_rw` is not a member of and does not inherit
  from the owner role at probe time, after C3's grant is revoked. **C3 and C4
  interact** — C3's temporary grant must not become C4's leak.
- **Binding constraint: C4** — role-isolation self-check.

### Challenge 5 — `search_path` / temp-object fidelity (falsifier_1, `dialogue:2`) → **C5 (minor)**
- **Landed?** Weakly. Largely defused by its own rebuttal: the `42501` gates on
  the *resolved* relation's ownership regardless of `search_path`. Worth a cheap
  hardening, not a shape risk.
- **Constraint: C5 (non-binding)** — pin the SUT `search_path` to prod's value.

### Challenge 6 — dual deploy-path divergence (falsifier_2, `dialogue:3`) → **F6 (deferred)**
- **Landed?** Material but out of P0 scope. The "admin-DSN out-of-band runtime
  apply" path is a property of the **future** Layer 3/4 deployer (no such current
  path appears in the SEED anchors); it does not foreclose P0. P0's oracle is
  honest for the current boot model once its scoping is stated.
- **Disposition: F6, deferred to the P3/P4 deployer** — its fixture must model
  both role-paths (or structurally constrain runtime-table creation to the runtime
  role). Not a P0 binding constraint.

## Disposition

- **P0 two-role fixture:** `cleared_with_constraints`.
- **Binding constraints the P0 build must discharge:** **C1** (escape-proof LOGIN
  role), **C2** (bootstrap ownership fidelity + differential gate), **C3**
  (non-superuser bootstrap privilege), **C4** (role-isolation self-check).
- **Minor hardening:** **C5** (pin `search_path`).
- **Flagged forward (not a P0 blocker):** **F6** (dual deploy-path → Layer 3/4
  deployer).

`commit_proposal` should publish the build-ready P0 spec with C1–C4 discharged
inline (each named by id), fold in C5, and carry F6 forward as a noted
non-goal for P0.
